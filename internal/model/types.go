package model

import "time"

// GatewayRecord is a settlement record as reported by the payment gateway (Razorpay side).
type GatewayRecord struct {
	ID          string    `json:"id"`
	Ref         string    `json:"ref"`          // merchant order/transaction reference
	AmountPaise int64     `json:"amount_paise"` // gross amount captured, in paise
	FeePaise    int64     `json:"fee_paise"`    // gateway fee withheld
	SettledAt   time.Time `json:"settled_at"`
	Currency    string    `json:"currency"`
}

// LedgerRecord is what actually landed in the merchant's bank/ledger.
type LedgerRecord struct {
	ID         string    `json:"id"`
	Ref        string    `json:"ref"` // may be missing/garbled — that's the point
	NetPaise   int64     `json:"net_paise"`
	CreditedAt time.Time `json:"credited_at"`
	Currency   string    `json:"currency"`
}

// MatchReason categorizes exceptions so the report can be honest about *why*
// something didn't reconcile, not just that it didn't.
type ExceptionReason string

const (
	ReasonMissingLedger     ExceptionReason = "missing_ledger_counterpart"
	ReasonMissingGateway    ExceptionReason = "missing_gateway_counterpart"
	ReasonAmountMismatch    ExceptionReason = "amount_mismatch_beyond_tolerance"
	ReasonDateOutOfWindow   ExceptionReason = "settlement_date_out_of_window"
	ReasonDuplicateGateway  ExceptionReason = "duplicate_gateway_settlement"
	ReasonCurrencyMismatch  ExceptionReason = "currency_mismatch"
	ReasonAmbiguousMultiple ExceptionReason = "ambiguous_multiple_candidates"
)

// MatchConfidence is how the engine reached a match — this IS the audit trail.
type MatchConfidence string

const (
	ConfidenceExactRef  MatchConfidence = "exact_reference"
	ConfidenceFeeWindow MatchConfidence = "amount_within_fee_tolerance_and_date_window"
	ConfidenceLLMAssist MatchConfidence = "llm_assisted_low_confidence_review"
)

type Match struct {
	GatewayID  string          `json:"gateway_id"`
	LedgerID   string          `json:"ledger_id"`
	Confidence MatchConfidence `json:"confidence"`
	Reasoning  string          `json:"reasoning"`
	MatchedAt  time.Time       `json:"matched_at"`
}

type Exception struct {
	Kind      string          `json:"kind"` // "gateway" or "ledger"
	RecordID  string          `json:"record_id"`
	Reason    ExceptionReason `json:"reason"`
	Reasoning string          `json:"reasoning"`
	FlaggedAt time.Time       `json:"flagged_at"`
}

// AuditEntry logs every decision the engine makes, matched or not — required
// so the run is explainable and defensible in front of a panel.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // "match" | "exception" | "run_start" | "run_end"
	Detail    string    `json:"detail"`
}

// GroundTruthPair is only present in the synthetic seed data — it lets the
// report compute real precision/recall instead of an unverifiable demo.
type GroundTruthPair struct {
	GatewayID     string `json:"gateway_id"`
	LedgerID      string `json:"ledger_id"` // empty string means "should have no match"
	InjectedFault string `json:"injected_fault,omitempty"`
}
