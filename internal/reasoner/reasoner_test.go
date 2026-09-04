package reasoner

import (
	"context"
	"strings"
	"testing"
)

func TestSummarizeMentionsKeyFigures(t *testing.T) {
	r := RuleBasedReasoner{}
	summary, err := r.Summarize(context.Background(), RunSummary{
		TotalGateway: 90, TotalLedger: 80,
		Matched: 63, Exceptions: 27,
		MatchRatePct: 70, PrecisionPct: 100, RecallPct: 98,
		ReconciledINR: 45210.50, PendingINR: 8120.00,
		TopExceptionReason: "missing_ledger_counterpart", TopExceptionCount: 12,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"63", "90", "70%", "45210.50", "missing_ledger_counterpart", "12", "8120.00", "100%", "98%"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing expected figure %q, got: %s", want, summary)
		}
	}
}

func TestSummarizeHandlesZeroExceptions(t *testing.T) {
	r := RuleBasedReasoner{}
	summary, err := r.Summarize(context.Background(), RunSummary{
		TotalGateway: 10, Matched: 10, Exceptions: 0,
		MatchRatePct: 100, PrecisionPct: 100, RecallPct: 100,
		ReconciledINR: 1000, PendingINR: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(summary, "No open exceptions") {
		t.Errorf("expected explicit no-exceptions statement, got: %s", summary)
	}
}

func TestSummarizeFlagsLowPrecisionForReview(t *testing.T) {
	r := RuleBasedReasoner{}
	summary, err := r.Summarize(context.Background(), RunSummary{
		TotalGateway: 10, Matched: 8, Exceptions: 2,
		MatchRatePct: 80, PrecisionPct: 70, RecallPct: 75,
		ReconciledINR: 500, PendingINR: 100,
		TopExceptionReason: "amount_mismatch_beyond_tolerance", TopExceptionCount: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(summary, "spot-check") {
		t.Errorf("low precision (70%%) should trigger a spot-check recommendation, got: %s", summary)
	}
}

func TestNewFromEnvFallsBackWithoutKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	r := NewFromEnv()
	if _, ok := r.(RuleBasedReasoner); !ok {
		t.Errorf("expected RuleBasedReasoner when GEMINI_API_KEY is unset, got %T", r)
	}
}

func TestNewFromEnvUsesGeminiWhenKeySet(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "fake-key-for-test")
	r := NewFromEnv()
	if _, ok := r.(*GeminiReasoner); !ok {
		t.Errorf("expected *GeminiReasoner when GEMINI_API_KEY is set, got %T", r)
	}
}
