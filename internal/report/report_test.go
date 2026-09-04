package report

import (
	"strings"
	"testing"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

func TestCashPositionSumsReconciledAndPending(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{
		{ID: "g1", Ref: "A", AmountPaise: 100000, FeePaise: 2000, SettledAt: now, Currency: "INR"},
		{ID: "g2", Ref: "B", AmountPaise: 50000, FeePaise: 1000, SettledAt: now, Currency: "INR"},
	}
	ledger := []model.LedgerRecord{
		{ID: "l1", Ref: "A", NetPaise: 98000, CreditedAt: now, Currency: "INR"},
		{ID: "l2", Ref: "C", NetPaise: 30000, CreditedAt: now, Currency: "INR"}, // unmatched ledger record
	}
	matches := []model.Match{
		{GatewayID: "g1", LedgerID: "l1", Confidence: model.ConfidenceExactRef, Reasoning: "exact", MatchedAt: now},
	}
	exceptions := []model.Exception{
		{Kind: "gateway", RecordID: "g2", Reason: model.ReasonMissingLedger, Reasoning: "no counterpart", FlaggedAt: now},
		{Kind: "ledger", RecordID: "l2", Reason: model.ReasonMissingGateway, Reasoning: "no counterpart", FlaggedAt: now},
	}

	m := Compute(nil, matches, exceptions, gateway, ledger)

	if m.ReconciledPaise != 98000 {
		t.Errorf("want ReconciledPaise=98000 (from l1's net), got %d", m.ReconciledPaise)
	}
	if m.PendingGatewayPaise != 50000 {
		t.Errorf("want PendingGatewayPaise=50000 (g2's gross), got %d", m.PendingGatewayPaise)
	}
	if m.PendingLedgerPaise != 30000 {
		t.Errorf("want PendingLedgerPaise=30000 (l2's net), got %d", m.PendingLedgerPaise)
	}
}

func TestCashPositionZeroOnEmptyRun(t *testing.T) {
	m := Compute(nil, nil, nil, nil, nil)
	if m.ReconciledPaise != 0 || m.PendingGatewayPaise != 0 || m.PendingLedgerPaise != 0 {
		t.Errorf("empty run should have zero cash position, got reconciled=%d pendingGw=%d pendingLg=%d",
			m.ReconciledPaise, m.PendingGatewayPaise, m.PendingLedgerPaise)
	}
}

func TestStringIncludesCashPosition(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{{ID: "g1", Ref: "A", AmountPaise: 100000, FeePaise: 2000, SettledAt: now, Currency: "INR"}}
	ledger := []model.LedgerRecord{{ID: "l1", Ref: "A", NetPaise: 98000, CreditedAt: now, Currency: "INR"}}
	matches := []model.Match{{GatewayID: "g1", LedgerID: "l1", Confidence: model.ConfidenceExactRef, MatchedAt: now}}

	m := Compute(nil, matches, nil, gateway, ledger)
	s := m.String()

	if !strings.Contains(s, "Cash position") {
		t.Error("report string should include a Cash position section")
	}
	if !strings.Contains(s, "980.00") { // 98000 paise = 980.00 INR
		t.Errorf("report string should show reconciled amount in rupees, got:\n%s", s)
	}
}
