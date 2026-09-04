package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/db"
	"github.com/mandar-1dev/finance-controller/internal/model"
	"github.com/mandar-1dev/finance-controller/internal/reasoner"
)

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "store.json"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestCleanPairMatchesExactly(t *testing.T) {
	store := newTestStore(t)
	settled := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}

	eng := New(store, reasoner.RuleBasedReasoner{})
	result, err := eng.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(result.Matches))
	}
	if len(result.Exceptions) != 0 {
		t.Fatalf("want 0 exceptions, got %d", len(result.Exceptions))
	}
	if result.Matches[0].Confidence != model.ConfidenceExactRef {
		t.Errorf("want exact-ref confidence, got %s", result.Matches[0].Confidence)
	}
}

func TestMissingLedgerCounterpartFlagged(t *testing.T) {
	store := newTestStore(t)
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: time.Now(), Currency: "INR"}}
	if err := store.SeedTransactions(gw, nil, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 0 || len(result.Exceptions) != 1 {
		t.Fatalf("want 0 matches/1 exception, got %d/%d", len(result.Matches), len(result.Exceptions))
	}
	if result.Exceptions[0].Reason != model.ReasonMissingLedger {
		t.Errorf("want ReasonMissingLedger, got %s", result.Exceptions[0].Reason)
	}
}

func TestMissingGatewayCounterpartFlagged(t *testing.T) {
	store := newTestStore(t)
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD9", NetPaise: 5000, CreditedAt: time.Now(), Currency: "INR"}}
	if err := store.SeedTransactions(nil, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Exceptions) != 1 || result.Exceptions[0].Reason != model.ReasonMissingGateway {
		t.Fatalf("want 1 ReasonMissingGateway exception, got %+v", result.Exceptions)
	}
}

func TestAmountDriftBeyondToleranceFlagged(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	// Net should be ~97640; give it 89640 — an extra 8% missing, beyond 3.5% tolerance.
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 89640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 0 {
		t.Fatalf("want 0 matches for out-of-tolerance amount, got %d", len(result.Matches))
	}
	if len(result.Exceptions) != 2 { // both the gateway (missing counterpart) and ledger (missing counterpart) sides get flagged
		t.Fatalf("want 2 exceptions (one per side), got %d: %+v", len(result.Exceptions), result.Exceptions)
	}
}

func TestLateSettlementBeyondWindowFlagged(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(6 * 24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 0 {
		t.Fatalf("want 0 matches for date outside window, got %d", len(result.Matches))
	}
}

func TestDuplicateGatewaySettlementFlagged(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{
		{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"},
		{ID: "g1_dup", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"},
	}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 0 {
		t.Fatalf("want 0 matches when gateway ref is duplicated, got %d", len(result.Matches))
	}
	dupCount := 0
	for _, ex := range result.Exceptions {
		if ex.Reason == model.ReasonDuplicateGateway {
			dupCount++
		}
	}
	if dupCount != 2 {
		t.Fatalf("want 2 duplicate-flagged gateway records, got %d", dupCount)
	}
}

func TestMangledRefStillMatchesViaFeeWindowFallback(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD100001", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1000XX", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 1 {
		t.Fatalf("want 1 fallback match despite mangled ref, got %d", len(result.Matches))
	}
	if result.Matches[0].Confidence != model.ConfidenceFeeWindow {
		t.Errorf("want fee-window fallback confidence, got %s", result.Matches[0].Confidence)
	}
}

func TestCurrencyMismatchNeverAutoMatched(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "USD"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	result, _ := eng.Run(context.Background())
	if len(result.Matches) != 0 {
		t.Fatalf("currency mismatch must never auto-match, got %d matches", len(result.Matches))
	}
}

func TestAuditTrailIsPopulated(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})
	if _, err := eng.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	audit := store.Snapshot().Audit
	if len(audit) < 3 { // run_start, match, run_end at minimum
		t.Fatalf("expected populated audit trail, got %d entries", len(audit))
	}
	if audit[0].Action != "run_start" || audit[len(audit)-1].Action != "run_end" {
		t.Errorf("audit trail should bracket the run: got first=%s last=%s", audit[0].Action, audit[len(audit)-1].Action)
	}
}

func TestRunningTwiceDoesNotDoubleCountResults(t *testing.T) {
	store := newTestStore(t)
	settled := time.Now()
	gw := []model.GatewayRecord{{ID: "g1", Ref: "ORD1", AmountPaise: 100000, FeePaise: 2360, SettledAt: settled, Currency: "INR"}}
	lg := []model.LedgerRecord{{ID: "l1", Ref: "ORD1", NetPaise: 97640, CreditedAt: settled.Add(24 * time.Hour), Currency: "INR"}}
	if err := store.SeedTransactions(gw, lg, nil); err != nil {
		t.Fatal(err)
	}
	eng := New(store, reasoner.RuleBasedReasoner{})

	first, err := eng.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := eng.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Matches) != len(second.Matches) {
		t.Fatalf("re-running should be idempotent: first=%d second=%d", len(first.Matches), len(second.Matches))
	}
	if got := len(store.Snapshot().Matches); got != 1 {
		t.Fatalf("store should hold exactly 1 match after two runs, not accumulate — got %d", got)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
