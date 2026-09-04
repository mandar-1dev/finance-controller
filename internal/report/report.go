// Package report scores a reconciliation run against the seeded ground
// truth. This exists specifically because the brief warns that "one
// cherry-picked match proves nothing" — these are the honest numbers.
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

type Metrics struct {
	TotalGateway       int
	TotalLedger        int
	CorrectMatches     int
	IncorrectMatches   int
	MissedMatches      int // ground truth said match, engine didn't
	CorrectNonMatches  int // ground truth said no match, engine correctly flagged exception
	Precision          float64
	Recall             float64
	MatchRate          float64 // matches / total gateway records
	ExceptionsByReason map[model.ExceptionReason]int

	// Cash position — real money, not just record counts. This is what
	// "run the ... cash position" in the track brief actually means.
	ReconciledPaise     int64 // sum of ledger net amounts behind every confirmed match
	PendingGatewayPaise int64 // gateway-side money sitting in unresolved exceptions
	PendingLedgerPaise  int64 // ledger-side money sitting in unresolved exceptions

	// PendingByReason breaks the same pending money down per exception
	// reason — this is "money at risk" grouped by cause, not just a count
	// of how many records share that cause.
	PendingByReason map[model.ExceptionReason]int64
}

func Compute(gt []model.GroundTruthPair, matches []model.Match, exceptions []model.Exception, gateway []model.GatewayRecord, ledger []model.LedgerRecord) Metrics {
	totalGateway, totalLedger := len(gateway), len(ledger)

	truth := map[string]string{} // gatewayID -> expected ledgerID ("" = no match expected)
	for _, p := range gt {
		truth[p.GatewayID] = p.LedgerID
	}

	got := map[string]string{} // gatewayID -> actual matched ledgerID
	for _, m := range matches {
		got[m.GatewayID] = m.LedgerID
	}

	var correct, incorrect, missed, correctNonMatch int
	for gwID, expectedLedger := range truth {
		actualLedger, matched := got[gwID]
		switch {
		case expectedLedger != "" && matched && actualLedger == expectedLedger:
			correct++
		case expectedLedger != "" && matched && actualLedger != expectedLedger:
			incorrect++
		case expectedLedger != "" && !matched:
			missed++
		case expectedLedger == "" && !matched:
			correctNonMatch++
		case expectedLedger == "" && matched:
			incorrect++ // engine matched something that should have stayed an exception
		}
	}

	precision := 0.0
	if correct+incorrect > 0 {
		precision = float64(correct) / float64(correct+incorrect)
	}
	recall := 0.0
	expectedPositives := correct + missed
	if expectedPositives > 0 {
		recall = float64(correct) / float64(expectedPositives)
	}
	matchRate := 0.0
	if totalGateway > 0 {
		matchRate = float64(len(matches)) / float64(totalGateway)
	}

	byReason := map[model.ExceptionReason]int{}
	for _, ex := range exceptions {
		byReason[ex.Reason]++
	}

	ledgerByID := make(map[string]model.LedgerRecord, len(ledger))
	for _, l := range ledger {
		ledgerByID[l.ID] = l
	}
	gatewayByID := make(map[string]model.GatewayRecord, len(gateway))
	for _, g := range gateway {
		gatewayByID[g.ID] = g
	}

	var reconciled, pendingGateway, pendingLedger int64
	pendingByReason := map[model.ExceptionReason]int64{}
	for _, m := range matches {
		if l, ok := ledgerByID[m.LedgerID]; ok {
			reconciled += l.NetPaise
		}
	}
	for _, ex := range exceptions {
		var amt int64
		switch ex.Kind {
		case "gateway":
			if g, ok := gatewayByID[ex.RecordID]; ok {
				amt = g.AmountPaise
				pendingGateway += amt
			}
		case "ledger":
			if l, ok := ledgerByID[ex.RecordID]; ok {
				amt = l.NetPaise
				pendingLedger += amt
			}
		}
		pendingByReason[ex.Reason] += amt
	}

	return Metrics{
		TotalGateway:        totalGateway,
		TotalLedger:         totalLedger,
		CorrectMatches:      correct,
		IncorrectMatches:    incorrect,
		MissedMatches:       missed,
		CorrectNonMatches:   correctNonMatch,
		Precision:           precision,
		Recall:              recall,
		MatchRate:           matchRate,
		ExceptionsByReason:  byReason,
		ReconciledPaise:     reconciled,
		PendingGatewayPaise: pendingGateway,
		PendingLedgerPaise:  pendingLedger,
		PendingByReason:     pendingByReason,
	}
}

func (m Metrics) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reconciliation Report\n")
	fmt.Fprintf(&b, "======================\n")
	fmt.Fprintf(&b, "Gateway records:      %d\n", m.TotalGateway)
	fmt.Fprintf(&b, "Ledger records:       %d\n", m.TotalLedger)
	fmt.Fprintf(&b, "Match rate:           %.1f%%\n", m.MatchRate*100)
	fmt.Fprintf(&b, "Precision (vs truth): %.1f%%  (%d correct / %d incorrect)\n", m.Precision*100, m.CorrectMatches, m.IncorrectMatches)
	fmt.Fprintf(&b, "Recall (vs truth):    %.1f%%  (%d missed)\n", m.Recall*100, m.MissedMatches)
	fmt.Fprintf(&b, "Correctly-flagged non-matches: %d\n", m.CorrectNonMatches)
	fmt.Fprintf(&b, "\nCash position:\n")
	fmt.Fprintf(&b, "  Reconciled:            %s INR\n", formatPaise(m.ReconciledPaise))
	fmt.Fprintf(&b, "  Pending (gateway side): %s INR\n", formatPaise(m.PendingGatewayPaise))
	fmt.Fprintf(&b, "  Pending (ledger side):  %s INR\n", formatPaise(m.PendingLedgerPaise))
	fmt.Fprintf(&b, "\nExceptions by reason:\n")

	keys := make([]string, 0, len(m.ExceptionsByReason))
	for k := range m.ExceptionsByReason {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-38s %d\n", k, m.ExceptionsByReason[model.ExceptionReason(k)])
	}
	return b.String()
}

func formatPaise(p int64) string {
	return fmt.Sprintf("%.2f", float64(p)/100.0)
}
