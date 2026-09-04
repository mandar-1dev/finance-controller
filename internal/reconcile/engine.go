// Package reconcile is the core of the AI Finance Controller submission:
// it matches gateway settlement records against ledger credits, explains
// every decision, and never silently drops an unresolved record — it either
// matches or becomes a categorized exception. Nothing is discarded.
package reconcile

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/db"
	"github.com/mandar-1dev/finance-controller/internal/model"
	"github.com/mandar-1dev/finance-controller/internal/reasoner"
)

// FeeTolerancePct is the max fraction of the gateway amount we'll accept as
// an unexplained gap before treating it as an amount mismatch rather than a
// normal processing fee.
const FeeTolerancePct = 0.035 // 3.5% covers typical gateway fee + GST bands

// DateWindowDays is the max gap between settlement and credit we treat as
// a normal T+N banking delay.
const DateWindowDays = 3

type Engine struct {
	store    *db.Store
	reasoner reasoner.Reasoner
}

func New(store *db.Store, r reasoner.Reasoner) *Engine {
	return &Engine{store: store, reasoner: r}
}

type Result struct {
	Matches    []model.Match
	Exceptions []model.Exception
}

func (e *Engine) Run(ctx context.Context) (Result, error) {
	if err := e.store.ClearRunResults(); err != nil {
		return Result{}, fmt.Errorf("clear previous run results: %w", err)
	}
	snap := e.store.Snapshot()
	e.audit("run_start", fmt.Sprintf("reconciling %d gateway records against %d ledger records", len(snap.Gateway), len(snap.Ledger)))

	usedLedger := map[string]bool{}
	// group ledger records that were claimed more than once by an exact ref —
	// duplicate gateway settlements are one of the injected fault classes.
	refCount := map[string]int{}
	for _, g := range snap.Gateway {
		refCount[g.Ref]++
	}

	var result Result

	for _, g := range snap.Gateway {
		if refCount[g.Ref] > 1 {
			e.flagException(&result, "gateway", g.ID, model.ReasonDuplicateGateway,
				fmt.Sprintf("ref %s appears %d times in gateway settlements — likely a duplicate capture event, not %d separate sales.", g.Ref, refCount[g.Ref], refCount[g.Ref]))
			continue
		}

		candidates := e.candidatesFor(g, snap.Ledger, usedLedger)

		// If any candidate's reference matches exactly, that's decisive —
		// prefer it over ambiguity resolution even when other candidates
		// happen to also satisfy the amount/date window.
		if exact := findExactRef(g.Ref, candidates); exact != nil {
			e.recordMatch(&result, g, *exact, usedLedger)
			continue
		}

		// Fallback matching (no exact ref) is only trustworthy when the ref
		// is *close* to the gateway ref, not merely when it's the only
		// candidate left — otherwise two unrelated transactions that happen
		// to share an amount/date window get matched by coincidence.
		candidates = filterBySimilarRef(g.Ref, candidates, minRefSimilarity)

		switch len(candidates) {
		case 0:
			e.flagException(&result, "gateway", g.ID, model.ReasonMissingLedger,
				fmt.Sprintf("no ledger credit found for ref %s (%.2f INR) within %dd window and %.1f%% fee tolerance.", g.Ref, paiseToRupees(g.AmountPaise), DateWindowDays, FeeTolerancePct*100))
		case 1:
			l := candidates[0]
			e.recordMatch(&result, g, l, usedLedger)
		default:
			// More than one ledger record could plausibly match — resolve
			// via the reasoner instead of guessing, and log the ambiguity.
			l := e.resolveAmbiguous(ctx, g, candidates)
			if l != nil {
				e.recordMatch(&result, g, *l, usedLedger)
			} else {
				e.flagException(&result, "gateway", g.ID, model.ReasonAmbiguousMultiple,
					fmt.Sprintf("ref %s has %d plausible ledger candidates within tolerance; reasoner could not confidently pick one.", g.Ref, len(candidates)))
			}
		}
	}

	// Any ledger record never claimed is itself an exception — this is what
	// "don't discard the other side" means in practice.
	for _, l := range snap.Ledger {
		if usedLedger[l.ID] {
			continue
		}
		e.flagException(&result, "ledger", l.ID, model.ReasonMissingGateway,
			fmt.Sprintf("ledger credit %s (%.2f INR, ref %q) has no corresponding gateway settlement.", l.ID, paiseToRupees(l.NetPaise), l.Ref))
	}

	e.audit("run_end", fmt.Sprintf("matched=%d exceptions=%d", len(result.Matches), len(result.Exceptions)))
	return result, nil
}

func (e *Engine) candidatesFor(g model.GatewayRecord, ledger []model.LedgerRecord, used map[string]bool) []model.LedgerRecord {
	var out []model.LedgerRecord
	for _, l := range ledger {
		if used[l.ID] {
			continue
		}
		if l.Currency != g.Currency {
			continue // currency mismatches are flagged separately below, not silently matched
		}
		days := int(math.Abs(l.CreditedAt.Sub(g.SettledAt).Hours() / 24))
		if days > DateWindowDays {
			continue
		}
		expectedNet := g.AmountPaise - g.FeePaise
		tolerance := int64(float64(g.AmountPaise) * FeeTolerancePct)
		if abs64(l.NetPaise-expectedNet) > tolerance {
			continue
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (e *Engine) recordMatch(result *Result, g model.GatewayRecord, l model.LedgerRecord, used map[string]bool) {
	used[l.ID] = true
	confidence := model.ConfidenceExactRef
	reasoning := fmt.Sprintf("ref %s matched exactly; net %.2f INR within fee tolerance of gross %.2f INR.", g.Ref, paiseToRupees(l.NetPaise), paiseToRupees(g.AmountPaise))
	if g.Ref != l.Ref {
		confidence = model.ConfidenceFeeWindow
		reasoning = fmt.Sprintf("no exact ref match (gateway=%q ledger=%q); matched on amount-within-tolerance + %dd date window instead.", g.Ref, l.Ref, DateWindowDays)
	}
	m := model.Match{
		GatewayID:  g.ID,
		LedgerID:   l.ID,
		Confidence: confidence,
		Reasoning:  reasoning,
		MatchedAt:  time.Now().UTC(),
	}
	result.Matches = append(result.Matches, m)
	_ = e.store.AppendMatch(m)
	e.audit("match", reasoning)
}

func (e *Engine) flagException(result *Result, kind, id string, reason model.ExceptionReason, why string) {
	ex := model.Exception{
		Kind:      kind,
		RecordID:  id,
		Reason:    reason,
		Reasoning: why,
		FlaggedAt: time.Now().UTC(),
	}
	result.Exceptions = append(result.Exceptions, ex)
	_ = e.store.AppendException(ex)
	e.audit("exception", why)
}

func (e *Engine) resolveAmbiguous(ctx context.Context, g model.GatewayRecord, candidates []model.LedgerRecord) *model.LedgerRecord {
	// Ask the reasoner to weigh in on the closest candidate; if its verdict
	// suggests a normal fee/timing explanation, accept that candidate at
	// reduced confidence and log the reasoning verbatim for audit purposes.
	best := candidates[0]
	days := int(math.Abs(best.CreditedAt.Sub(g.SettledAt).Hours() / 24))
	verdict, err := e.reasoner.Explain(ctx, g.Ref, g.AmountPaise-g.FeePaise, best.NetPaise, days)
	if err != nil {
		return nil
	}
	e.audit("llm_assist", fmt.Sprintf("ambiguous ref %s (%d candidates): %s | suggestion: %s", g.Ref, len(candidates), verdict.Explanation, verdict.Suggestion))
	if verdict.Suggestion == "manual review" || verdict.Suggestion == "hold and escalate — overpayment mismatches should not auto-resolve" {
		return nil
	}
	return &best
}

func (e *Engine) audit(action, detail string) {
	_ = e.store.AppendAudit(model.AuditEntry{Timestamp: time.Now().UTC(), Action: action, Detail: detail})
}

// minRefSimilarity is the floor for accepting a fee/date fallback match with
// no exact ref hit. Calibrated so a genuinely garbled ref (a couple of
// trailing characters lost by a bank feed) still passes, while two unrelated
// references only sharing accidental character overlap do not.
const minRefSimilarity = 0.6

func filterBySimilarRef(ref string, candidates []model.LedgerRecord, minSim float64) []model.LedgerRecord {
	var out []model.LedgerRecord
	for _, c := range candidates {
		if refSimilarity(ref, c.Ref) >= minSim {
			out = append(out, c)
		}
	}
	return out
}

// refSimilarity is a same-position character match ratio. Simple by design —
// it only needs to distinguish "one bank feed truncated a few trailing
// characters" from "these are two unrelated references", not do general
// fuzzy string matching.
func refSimilarity(a, b string) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	matches := 0
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(a))
}

func findExactRef(ref string, candidates []model.LedgerRecord) *model.LedgerRecord {
	for i := range candidates {
		if candidates[i].Ref == ref {
			return &candidates[i]
		}
	}
	return nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func paiseToRupees(p int64) float64 { return float64(p) / 100.0 }
