// Command server exposes the reconciliation results over HTTP for the
// dashboard (web/index.html) — this is what you'd screen-record for the
// pitch video. Run: go run ./cmd/server
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/mandar-1dev/finance-controller/internal/db"
	"github.com/mandar-1dev/finance-controller/internal/reasoner"
	"github.com/mandar-1dev/finance-controller/internal/reconcile"
	"github.com/mandar-1dev/finance-controller/internal/report"
	"github.com/mandar-1dev/finance-controller/internal/seed"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "data/store.json", "path to JSON store file")
	flag.Parse()

	store, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	reason := reasoner.NewFromEnv()
	eng := reconcile.New(store, reason)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/reseed", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		// A fresh, genuinely random seed each call — not a fixed demo
		// number — so clicking this from the dashboard produces a visibly
		// different batch (and different match/exception counts) every
		// time, instead of the same static numbers on every "Run".
		randSeed := time.Now().UnixNano()
		n := 90
		result := seed.Generate(n, randSeed)
		if err := store.SeedTransactions(result.Gateway, result.Ledger, result.Truth); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"seed":    randSeed,
			"gateway": len(result.Gateway),
			"ledger":  len(result.Ledger),
		})
	})

	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		result, err := eng.Run(context.Background())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	})

	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		snap := store.Snapshot()
		m := report.Compute(snap.GroundTruth, snap.Matches, snap.Exceptions, snap.Gateway, snap.Ledger)
		writeJSON(w, m)
	})

	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		snap := store.Snapshot()
		m := report.Compute(snap.GroundTruth, snap.Matches, snap.Exceptions, snap.Gateway, snap.Ledger)

		topReason, topCount := "none", 0
		for reason, count := range m.ExceptionsByReason {
			if count > topCount {
				topReason, topCount = string(reason), count
			}
		}

		text, err := reason.Summarize(r.Context(), reasoner.RunSummary{
			TotalGateway: m.TotalGateway, TotalLedger: m.TotalLedger,
			Matched: len(snap.Matches), Exceptions: len(snap.Exceptions),
			MatchRatePct: m.MatchRate * 100, PrecisionPct: m.Precision * 100, RecallPct: m.Recall * 100,
			ReconciledINR: float64(m.ReconciledPaise) / 100, PendingINR: float64(m.PendingGatewayPaise+m.PendingLedgerPaise) / 100,
			TopExceptionReason: topReason, TopExceptionCount: topCount,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"summary": text})
	})

	mux.HandleFunc("/api/priority", func(w http.ResponseWriter, r *http.Request) {
		snap := store.Snapshot()
		ranked := report.Prioritize(snap.Exceptions, snap.Gateway, snap.Ledger)
		writeJSON(w, ranked)
	})

	mux.HandleFunc("/api/export/exceptions.csv", func(w http.ResponseWriter, r *http.Request) {
		snap := store.Snapshot()
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="exceptions.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"kind", "record_id", "reason", "reasoning", "flagged_at"})
		for _, ex := range snap.Exceptions {
			_ = cw.Write([]string{ex.Kind, ex.RecordID, string(ex.Reason), ex.Reasoning, ex.FlaggedAt.Format(time.RFC3339)})
		}
		cw.Flush()
	})

	mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, store.Snapshot().Audit)
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, store.Snapshot())
	})

	mux.Handle("/", http.FileServer(http.Dir("web")))

	log.Printf("AI Finance Controller listening on %s (dashboard at /)", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
