// Command seed generates a synthetic gateway/ledger dataset with deliberately
// injected mismatches, and writes ground truth alongside it so the report
// step can compute honest precision/recall — not just a demo that "looks
// right". Run: go run ./cmd/seed
package main

import (
	"flag"
	"fmt"

	"github.com/mandar-1dev/finance-controller/internal/db"
	"github.com/mandar-1dev/finance-controller/internal/seed"
)

func main() {
	n := flag.Int("n", 60, "number of clean base transactions to generate (before fault injection adds extras)")
	randSeed := flag.Int64("seed", 42, "random seed, for reproducible datasets")
	dbPath := flag.String("db", "data/store.json", "path to JSON store file")
	flag.Parse()

	result := seed.Generate(*n, *randSeed)

	store, err := db.Open(*dbPath)
	if err != nil {
		panic(err)
	}
	if err := store.SeedTransactions(result.Gateway, result.Ledger, result.Truth); err != nil {
		panic(err)
	}

	fmt.Printf("Seeded %d gateway records, %d ledger records, %d ground-truth pairs -> %s\n",
		len(result.Gateway), len(result.Ledger), len(result.Truth), *dbPath)
}
