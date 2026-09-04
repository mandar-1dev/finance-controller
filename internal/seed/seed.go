// Package seed generates a synthetic gateway/ledger dataset with deliberately
// injected mismatches, plus ground truth so internal/report can compute
// honest precision/recall — not just a demo that "looks right". Shared by
// cmd/seed (one-shot CLI generation) and cmd/server (on-demand regeneration
// from the dashboard, so a demo can show the engine handling fresh data
// live instead of the same static batch every time).
package seed

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

const alnumAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I to keep it eyeballable in a demo

// Result is one generated synthetic batch.
type Result struct {
	Gateway []model.GatewayRecord
	Ledger  []model.LedgerRecord
	Truth   []model.GroundTruthPair
}

// Generate produces n clean base transactions (before fault injection adds
// extras like duplicates), deterministically from the given seed — same
// seed always produces the same batch, different seeds produce genuinely
// different data with different exact numbers.
func Generate(n int, randSeed int64) Result {
	rng := rand.New(rand.NewSource(randSeed))
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	var gateway []model.GatewayRecord
	var ledger []model.LedgerRecord
	var truth []model.GroundTruthPair

	feeBps := 236 // ~2.36% typical blended gateway fee, in basis points

	for i := 0; i < n; i++ {
		gwID := fmt.Sprintf("gw_%04d", i)
		lgID := fmt.Sprintf("lg_%04d", i)
		ref := "ORD" + randAlnum(rng, 6)
		amount := int64(50000 + rng.Intn(4_500_00)) // 500.00 - 45,500.00 INR in paise
		fee := amount * int64(feeBps) / 10000
		settledAt := base.Add(time.Duration(i) * time.Hour * 3)

		fault := pickFault(rng)

		gw := model.GatewayRecord{ID: gwID, Ref: ref, AmountPaise: amount, FeePaise: fee, SettledAt: settledAt, Currency: "INR"}
		lg := model.LedgerRecord{ID: lgID, Ref: ref, NetPaise: amount - fee, CreditedAt: settledAt.Add(24 * time.Hour), Currency: "INR"}

		switch fault {
		case "clean":
			gateway = append(gateway, gw)
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: lgID})

		case "missing_ledger": // settlement never landed — e.g. bank rejection
			gateway = append(gateway, gw)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: "", InjectedFault: "missing_ledger"})

		case "missing_gateway": // stray bank credit with no gateway record — e.g. manual refund reversal
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: "", LedgerID: lgID, InjectedFault: "missing_gateway"})

		case "amount_drift": // unexplained shortfall beyond normal fee band
			lg.NetPaise = lg.NetPaise - (amount * 8 / 100) // extra 8% missing — outside 3.5% tolerance
			gateway = append(gateway, gw)
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: "", InjectedFault: "amount_drift"})

		case "late_settlement": // banking holiday delay beyond the normal window
			lg.CreditedAt = settledAt.Add(6 * 24 * time.Hour)
			gateway = append(gateway, gw)
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: "", InjectedFault: "late_settlement"})

		case "duplicate_gateway": // double-capture event
			gw2 := gw
			gw2.ID = gwID + "_dup"
			gateway = append(gateway, gw, gw2)
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: "", InjectedFault: "duplicate_gateway"})
			truth = append(truth, model.GroundTruthPair{GatewayID: gw2.ID, LedgerID: "", InjectedFault: "duplicate_gateway"})

		case "mangled_ref": // ledger ref got truncated/garbled by the bank feed, amount+timing still line up
			lg.Ref = ref[:len(ref)-2] + "XX"
			gateway = append(gateway, gw)
			ledger = append(ledger, lg)
			truth = append(truth, model.GroundTruthPair{GatewayID: gwID, LedgerID: lgID}) // SHOULD still match via fee/date fallback
		}
	}

	return Result{Gateway: gateway, Ledger: ledger, Truth: truth}
}

func randAlnum(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alnumAlphabet[rng.Intn(len(alnumAlphabet))]
	}
	return string(b)
}

func pickFault(rng *rand.Rand) string {
	// Weighted so ~65% are clean and the rest spread across every fault
	// class the AI Finance Controller brief expects an honest exception list
	// to cover.
	r := rng.Float64()
	switch {
	case r < 0.65:
		return "clean"
	case r < 0.75:
		return "missing_ledger"
	case r < 0.82:
		return "missing_gateway"
	case r < 0.88:
		return "amount_drift"
	case r < 0.93:
		return "late_settlement"
	case r < 0.97:
		return "duplicate_gateway"
	default:
		return "mangled_ref"
	}
}
