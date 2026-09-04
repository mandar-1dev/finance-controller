// Priority ranks open exceptions so a finance team investigates the ₹8L
// duplicate settlement before the ₹40 rounding difference — instead of
// working through a flat, unordered list.
//
// The score is a deliberately simple, fully deterministic formula from two
// real signals: how severe this exception *type* typically is, and how much
// money is actually behind this specific record relative to the rest of the
// batch. It is not a machine-learned model and does not pretend to be one —
// every number it produces is directly traceable to the formula below, which
// is the honest tradeoff for a 2-day build: a defensible heuristic beats an
// impressive-looking number nobody can explain under a follow-up question.
package report

import (
	"sort"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

// reasonSeverity is a fixed weight (0-1) per exception reason, reflecting
// how urgently that *type* of exception typically needs a human look —
// independent of the amount involved. Calibrated by what each reason
// actually implies operationally:
//   - duplicate settlements risk double-counting real revenue — highest.
//   - ambiguous candidates mean the engine itself wasn't confident.
//   - amount drift beyond tolerance suggests a real discrepancy, not a fee.
//   - missing counterparts are usually a timing gap, still needs a look.
//   - a settlement merely outside the date window is often just a delay.
var reasonSeverity = map[model.ExceptionReason]float64{
	model.ReasonDuplicateGateway:  0.95,
	model.ReasonAmbiguousMultiple: 0.80,
	model.ReasonAmountMismatch:    0.70,
	model.ReasonCurrencyMismatch:  0.65,
	model.ReasonMissingLedger:     0.55,
	model.ReasonMissingGateway:    0.55,
	model.ReasonDateOutOfWindow:   0.30,
}

const defaultSeverity = 0.5 // fallback for any reason not in the table above

// PrioritizedException is one row in the investigation queue.
type PrioritizedException struct {
	Kind        string                `json:"kind"`
	RecordID    string                `json:"record_id"`
	Reason      model.ExceptionReason `json:"reason"`
	Reasoning   string                `json:"reasoning"`
	AmountPaise int64                 `json:"amount_paise"`
	RiskScore   int                   `json:"risk_score"` // 0-100
	RiskBand    string                `json:"risk_band"`  // "high" | "medium" | "low"
}

// Prioritize scores and sorts every open exception, highest risk first.
// Ties break by amount (larger first) for a stable, explainable order.
func Prioritize(exceptions []model.Exception, gateway []model.GatewayRecord, ledger []model.LedgerRecord) []PrioritizedException {
	gatewayByID := make(map[string]model.GatewayRecord, len(gateway))
	for _, g := range gateway {
		gatewayByID[g.ID] = g
	}
	ledgerByID := make(map[string]model.LedgerRecord, len(ledger))
	for _, l := range ledger {
		ledgerByID[l.ID] = l
	}

	type withAmount struct {
		ex  model.Exception
		amt int64
	}
	withAmounts := make([]withAmount, 0, len(exceptions))
	var maxAmt int64
	for _, ex := range exceptions {
		var amt int64
		switch ex.Kind {
		case "gateway":
			amt = gatewayByID[ex.RecordID].AmountPaise
		case "ledger":
			amt = ledgerByID[ex.RecordID].NetPaise
		}
		withAmounts = append(withAmounts, withAmount{ex, amt})
		if amt > maxAmt {
			maxAmt = amt
		}
	}

	out := make([]PrioritizedException, 0, len(withAmounts))
	for _, wa := range withAmounts {
		severity := reasonSeverity[wa.ex.Reason]
		if severity == 0 {
			severity = defaultSeverity
		}
		amountRatio := 0.0
		if maxAmt > 0 {
			amountRatio = float64(wa.amt) / float64(maxAmt)
		}
		// Reason type dominates the score (80%): a small duplicate
		// settlement still needs review sooner than a large, merely-delayed
		// one — type predicts *what kind* of action is needed. Amount (20%)
		// is a secondary signal that nudges rank and can push a borderline
		// case up a band, but a low-severity reason should never land in
		// the high-risk band purely because the amount is large.
		score := 100*0.8*severity + 100*0.2*amountRatio
		if score > 100 {
			score = 100
		}
		riskScore := int(score + 0.5) // round to nearest int

		band := "low"
		switch {
		case riskScore >= 70:
			band = "high"
		case riskScore >= 40:
			band = "medium"
		}

		out = append(out, PrioritizedException{
			Kind:        wa.ex.Kind,
			RecordID:    wa.ex.RecordID,
			Reason:      wa.ex.Reason,
			Reasoning:   wa.ex.Reasoning,
			AmountPaise: wa.amt,
			RiskScore:   riskScore,
			RiskBand:    band,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].RiskScore != out[j].RiskScore {
			return out[i].RiskScore > out[j].RiskScore
		}
		return out[i].AmountPaise > out[j].AmountPaise
	})
	return out
}
