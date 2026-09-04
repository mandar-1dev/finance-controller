package report

import (
	"testing"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

func TestPrioritizeSortsHighestRiskFirst(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{
		{ID: "g_small_dup", AmountPaise: 5000, SettledAt: now, Currency: "INR"},   // ₹50, duplicate
		{ID: "g_big_delay", AmountPaise: 500000, SettledAt: now, Currency: "INR"}, // ₹5000, just late
	}
	exceptions := []model.Exception{
		{Kind: "gateway", RecordID: "g_big_delay", Reason: model.ReasonDateOutOfWindow, Reasoning: "late", FlaggedAt: now},
		{Kind: "gateway", RecordID: "g_small_dup", Reason: model.ReasonDuplicateGateway, Reasoning: "dup", FlaggedAt: now},
	}

	ranked := Prioritize(exceptions, gateway, nil)

	if len(ranked) != 2 {
		t.Fatalf("want 2 ranked entries, got %d", len(ranked))
	}
	// A small duplicate should still outrank a large but merely-delayed
	// settlement, because reason severity is weighted higher than amount.
	if ranked[0].RecordID != "g_small_dup" {
		t.Errorf("want duplicate settlement ranked first despite smaller amount, got %s first", ranked[0].RecordID)
	}
	if ranked[0].RiskBand != "high" {
		t.Errorf("want duplicate settlement in high risk band, got %s", ranked[0].RiskBand)
	}
}

func TestPrioritizeTieBreaksOnAmount(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{
		{ID: "g1", AmountPaise: 10000, SettledAt: now, Currency: "INR"},
		{ID: "g2", AmountPaise: 90000, SettledAt: now, Currency: "INR"},
	}
	exceptions := []model.Exception{
		{Kind: "gateway", RecordID: "g1", Reason: model.ReasonMissingLedger, FlaggedAt: now},
		{Kind: "gateway", RecordID: "g2", Reason: model.ReasonMissingLedger, FlaggedAt: now},
	}
	ranked := Prioritize(exceptions, gateway, nil)
	if ranked[0].RecordID != "g2" {
		t.Errorf("same reason, want larger amount ranked first, got %s first", ranked[0].RecordID)
	}
}

func TestPrioritizeUnknownReasonGetsDefaultSeverity(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{{ID: "g1", AmountPaise: 10000, SettledAt: now, Currency: "INR"}}
	exceptions := []model.Exception{
		{Kind: "gateway", RecordID: "g1", Reason: model.ExceptionReason("some_new_reason_not_in_table"), FlaggedAt: now},
	}
	ranked := Prioritize(exceptions, gateway, nil)
	if len(ranked) != 1 {
		t.Fatalf("want 1 ranked entry, got %d", len(ranked))
	}
	if ranked[0].RiskScore <= 0 {
		t.Errorf("unknown reason should still get a default severity score, got %d", ranked[0].RiskScore)
	}
}

func TestPrioritizeEmptyInputReturnsEmpty(t *testing.T) {
	ranked := Prioritize(nil, nil, nil)
	if len(ranked) != 0 {
		t.Errorf("want empty result for no exceptions, got %d", len(ranked))
	}
}

func TestPendingByReasonSumsCorrectAmounts(t *testing.T) {
	now := time.Now()
	gateway := []model.GatewayRecord{{ID: "g1", AmountPaise: 50000, SettledAt: now, Currency: "INR"}}
	ledger := []model.LedgerRecord{{ID: "l1", NetPaise: 30000, CreditedAt: now, Currency: "INR"}}
	exceptions := []model.Exception{
		{Kind: "gateway", RecordID: "g1", Reason: model.ReasonMissingLedger, FlaggedAt: now},
		{Kind: "ledger", RecordID: "l1", Reason: model.ReasonMissingGateway, FlaggedAt: now},
	}
	m := Compute(nil, nil, exceptions, gateway, ledger)
	if m.PendingByReason[model.ReasonMissingLedger] != 50000 {
		t.Errorf("want 50000 for ReasonMissingLedger, got %d", m.PendingByReason[model.ReasonMissingLedger])
	}
	if m.PendingByReason[model.ReasonMissingGateway] != 30000 {
		t.Errorf("want 30000 for ReasonMissingGateway, got %d", m.PendingByReason[model.ReasonMissingGateway])
	}
}
