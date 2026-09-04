// Command reconcile runs the matching engine against whatever is in the
// store and prints the honest metrics report. Run: go run ./cmd/reconcile
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/mandar-1dev/finance-controller/internal/db"
	"github.com/mandar-1dev/finance-controller/internal/reasoner"
	"github.com/mandar-1dev/finance-controller/internal/reconcile"
	"github.com/mandar-1dev/finance-controller/internal/report"
)

func main() {
	dbPath := flag.String("db", "data/store.json", "path to JSON store file")
	flag.Parse()

	store, err := db.Open(*dbPath)
	if err != nil {
		panic(err)
	}

	eng := reconcile.New(store, reasoner.NewFromEnv())
	result, err := eng.Run(context.Background())
	if err != nil {
		panic(err)
	}

	snap := store.Snapshot()
	metrics := report.Compute(snap.GroundTruth, result.Matches, result.Exceptions, snap.Gateway, snap.Ledger)
	fmt.Println(metrics.String())
	fmt.Printf("\n%d matches, %d exceptions logged. Full audit trail in %s\n", len(result.Matches), len(result.Exceptions), *dbPath)
}
