// Package reasoner supplies the "AI" step: when the deterministic matcher
// can't confidently resolve a candidate pair, the reasoner is asked to
// classify the likely cause and suggest a resolution in plain language.
//
// Reasoner is an interface so the engine never depends on a specific model
// provider, and so `go run` works out of the box with zero API keys — set
// GEMINI_API_KEY to switch to the live Gemini backend.
package reasoner

import (
	"context"
	"fmt"
	"strings"
)

type Verdict struct {
	Explanation string
	Suggestion  string
}

// RunSummary is the input to Summarize — a plain snapshot of a completed
// reconciliation run, deliberately decoupled from the report package so
// reasoner has no import dependency on it.
type RunSummary struct {
	TotalGateway, TotalLedger             int
	Matched, Exceptions                   int
	MatchRatePct, PrecisionPct, RecallPct float64
	ReconciledINR, PendingINR             float64
	TopExceptionReason                    string
	TopExceptionCount                     int
}

type Reasoner interface {
	Explain(ctx context.Context, gatewayRef string, gatewayAmt, ledgerAmt int64, daysApart int) (Verdict, error)
	// Summarize turns a completed run's metrics into a short plain-English
	// executive summary — what a finance lead would want to read before
	// opening the exception list themselves.
	Summarize(ctx context.Context, run RunSummary) (string, error)
}

// RuleBasedReasoner is the offline default: no network calls, deterministic,
// always available. This is what the engine falls back to when no LLM
// credentials are configured, so the pipeline never silently degrades.
type RuleBasedReasoner struct{}

func (RuleBasedReasoner) Explain(_ context.Context, ref string, gatewayAmt, ledgerAmt int64, daysApart int) (Verdict, error) {
	diff := gatewayAmt - ledgerAmt
	switch {
	case diff > 0 && daysApart <= 2:
		return Verdict{
			Explanation: fmt.Sprintf("ref %s: ledger short by %d paise within a normal fee window — likely an uncaptured fee or rounding delta, not fraud.", ref, diff),
			Suggestion:  "auto-approve if within merchant's historical fee band, else route to finance for a one-line check",
		}, nil
	case diff < 0:
		return Verdict{
			Explanation: fmt.Sprintf("ref %s: ledger credit exceeds gateway settlement by %d paise — possible duplicate credit or misattributed transfer.", ref, -diff),
			Suggestion:  "hold and escalate — overpayment mismatches should not auto-resolve",
		}, nil
	case daysApart > 2:
		return Verdict{
			Explanation: fmt.Sprintf("ref %s: amounts align but settlement gap is %d days, beyond the normal T+2 window.", ref, daysApart),
			Suggestion:  "check for a banking holiday delay before treating as an exception",
		}, nil
	default:
		return Verdict{
			Explanation: fmt.Sprintf("ref %s: amounts and timing are both borderline; no single rule explains the gap confidently.", ref),
			Suggestion:  "manual review",
		}, nil
	}
}

// Summarize is deterministic template-based text generation, not a model
// call — always available, always fast, always the same for the same input.
// This is the honest default; GeminiReasoner overrides it with a real LLM
// call when GEMINI_API_KEY is set.
func (RuleBasedReasoner) Summarize(_ context.Context, r RunSummary) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "This run reconciled %d of %d gateway settlements (%.0f%% match rate), ", r.Matched, r.TotalGateway, r.MatchRatePct)
	fmt.Fprintf(&b, "confirming ₹%.2f against the ledger. ", r.ReconciledINR)
	if r.Exceptions > 0 {
		fmt.Fprintf(&b, "%d records need attention — the largest single cause is %s (%d records), ", r.Exceptions, r.TopExceptionReason, r.TopExceptionCount)
		fmt.Fprintf(&b, "leaving ₹%.2f unresolved pending manual review. ", r.PendingINR)
	} else {
		fmt.Fprintf(&b, "No open exceptions. ")
	}
	fmt.Fprintf(&b, "Precision against ground truth is %.0f%%, recall %.0f%% — ", r.PrecisionPct, r.RecallPct)
	if r.PrecisionPct >= 95 {
		fmt.Fprintf(&b, "matches produced are trustworthy enough to auto-post without a second review.")
	} else {
		fmt.Fprintf(&b, "spot-check matches before auto-posting.")
	}
	return b.String(), nil
}
