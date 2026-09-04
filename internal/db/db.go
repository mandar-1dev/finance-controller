// Package db is a deliberately minimal persistence layer: no ORM, no driver
// dependency. Records are held in memory and flushed to a JSON file, mirroring
// the explicit database/sql style used elsewhere (CoreBank) without requiring
// a running MySQL instance for reviewers to reproduce a demo.
//
// Swapping this for database/sql + MySQL only touches this file — Store is
// the seam. See ARCHITECTURE.md.
package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/mandar-1dev/finance-controller/internal/model"
)

type Snapshot struct {
	Gateway     []model.GatewayRecord   `json:"gateway"`
	Ledger      []model.LedgerRecord    `json:"ledger"`
	GroundTruth []model.GroundTruthPair `json:"ground_truth,omitempty"`
	Matches     []model.Match           `json:"matches"`
	Exceptions  []model.Exception       `json:"exceptions"`
	Audit       []model.AuditEntry      `json:"audit"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data Snapshot
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) flush() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(s.path, b, 0644)
}

func (s *Store) SeedTransactions(gw []model.GatewayRecord, lg []model.LedgerRecord, gt []model.GroundTruthPair) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Gateway = gw
	s.data.Ledger = lg
	s.data.GroundTruth = gt
	s.data.Matches = nil
	s.data.Exceptions = nil
	s.data.Audit = nil
	return s.flush()
}

// ClearRunResults wipes matches/exceptions/audit but keeps the seeded
// transactions and ground truth. Must be called at the start of every
// reconciliation run — otherwise re-running against the same store
// double-counts results.
func (s *Store) ClearRunResults() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Matches = nil
	s.data.Exceptions = nil
	s.data.Audit = nil
	return s.flush()
}

func (s *Store) AppendMatch(m model.Match) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Matches = append(s.data.Matches, m)
	return s.flush()
}

func (s *Store) AppendException(e model.Exception) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Exceptions = append(s.data.Exceptions, e)
	return s.flush()
}

func (s *Store) AppendAudit(a model.AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Audit = append(s.data.Audit, a)
	return s.flush()
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}
