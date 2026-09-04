package seed

import "testing"

func TestGenerateIsDeterministicForSameSeed(t *testing.T) {
	a := Generate(90, 7)
	b := Generate(90, 7)
	if len(a.Gateway) != len(b.Gateway) || len(a.Ledger) != len(b.Ledger) {
		t.Fatalf("same seed should produce identical record counts: a=%d/%d b=%d/%d",
			len(a.Gateway), len(a.Ledger), len(b.Gateway), len(b.Ledger))
	}
	for i := range a.Gateway {
		if a.Gateway[i].Ref != b.Gateway[i].Ref || a.Gateway[i].AmountPaise != b.Gateway[i].AmountPaise {
			t.Fatalf("same seed should produce identical record %d, got different ref/amount", i)
		}
	}
}

func TestGenerateDiffersAcrossSeeds(t *testing.T) {
	a := Generate(90, 1)
	b := Generate(90, 2)
	same := 0
	for i := range a.Gateway {
		if i < len(b.Gateway) && a.Gateway[i].Ref == b.Gateway[i].Ref {
			same++
		}
	}
	if same == len(a.Gateway) {
		t.Error("different seeds produced identical refs across the whole batch — RNG isn't varying")
	}
}

func TestGenerateProducesEveryFaultClass(t *testing.T) {
	// With n=200 and fixed fault probabilities, every class should appear
	// at least once — this guards against a seed/threshold regression
	// silently dropping a fault category from the batch.
	r := Generate(200, 7)
	if len(r.Truth) == 0 {
		t.Fatal("expected non-empty ground truth")
	}
	faults := map[string]bool{}
	for _, p := range r.Truth {
		if p.InjectedFault != "" {
			faults[p.InjectedFault] = true
		}
	}
	for _, want := range []string{"missing_ledger", "missing_gateway", "amount_drift", "late_settlement", "duplicate_gateway"} {
		if !faults[want] {
			t.Errorf("expected fault class %q to appear in a 200-record batch, none found", want)
		}
	}
}
