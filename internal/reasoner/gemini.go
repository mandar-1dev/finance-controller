package reasoner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// GeminiReasoner calls Gemini directly over HTTP (no SDK dependency) when
// GEMINI_API_KEY is set. Falls back to RuleBasedReasoner on any error so a
// flaky network never takes down the reconciliation run.
type GeminiReasoner struct {
	apiKey   string
	model    string
	client   *http.Client
	fallback Reasoner
}

func NewFromEnv() Reasoner {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return RuleBasedReasoner{}
	}
	return &GeminiReasoner{
		apiKey:   key,
		model:    "gemini-2.5-flash",
		client:   &http.Client{Timeout: 15 * time.Second},
		fallback: RuleBasedReasoner{},
	}
}

type geminiReq struct {
	Contents []geminiContent `json:"contents"`
}
type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}
type geminiPart struct {
	Text string `json:"text"`
}
type geminiResp struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func (g *GeminiReasoner) Explain(ctx context.Context, ref string, gatewayAmt, ledgerAmt int64, daysApart int) (Verdict, error) {
	prompt := fmt.Sprintf(
		"You are a finance reconciliation assistant. Gateway settlement ref %s recorded %d paise; "+
			"the matching ledger credit is %d paise, %d day(s) apart. In under 40 words, state the most "+
			"likely cause and a one-line suggested resolution. Format: 'Explanation: ...\\nSuggestion: ...'",
		ref, gatewayAmt, ledgerAmt, daysApart,
	)
	reqBody, _ := json.Marshal(geminiReq{Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}}})
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return g.fallback.Explain(ctx, ref, gatewayAmt, ledgerAmt, daysApart)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return g.fallback.Explain(ctx, ref, gatewayAmt, ledgerAmt, daysApart)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return g.fallback.Explain(ctx, ref, gatewayAmt, ledgerAmt, daysApart)
	}

	var gr geminiResp
	if err := json.Unmarshal(body, &gr); err != nil || len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return g.fallback.Explain(ctx, ref, gatewayAmt, ledgerAmt, daysApart)
	}
	text := gr.Candidates[0].Content.Parts[0].Text
	return Verdict{Explanation: text, Suggestion: "see explanation"}, nil
}

// Summarize asks Gemini directly for a genuine natural-language executive
// summary of the run. Falls back to the deterministic template on any error
// so the dashboard never shows a blank summary.
func (g *GeminiReasoner) Summarize(ctx context.Context, r RunSummary) (string, error) {
	fallback := RuleBasedReasoner{}
	prompt := fmt.Sprintf(
		"You are a finance operations assistant writing a 3-sentence executive summary of a payment "+
			"reconciliation run for a merchant's finance lead. Facts: %d of %d gateway settlements matched "+
			"(%.0f%% match rate), ₹%.2f reconciled, %d exceptions open (top cause: %s, %d records), "+
			"₹%.2f pending review, precision %.0f%%, recall %.0f%% against ground truth. Be direct and "+
			"specific — no filler, no restating the instructions.",
		r.Matched, r.TotalGateway, r.MatchRatePct, r.ReconciledINR, r.Exceptions, r.TopExceptionReason,
		r.TopExceptionCount, r.PendingINR, r.PrecisionPct, r.RecallPct,
	)
	reqBody, _ := json.Marshal(geminiReq{Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}}})
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fallback.Summarize(ctx, r)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fallback.Summarize(ctx, r)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fallback.Summarize(ctx, r)
	}

	var gr geminiResp
	if err := json.Unmarshal(body, &gr); err != nil || len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return fallback.Summarize(ctx, r)
	}
	return gr.Candidates[0].Content.Parts[0].Text, nil
}
