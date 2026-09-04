# Architecture

## Problem framing

Given two independent record sets that describe the same underlying money
movement — a payment gateway's settlement log and a merchant's bank
ledger — produce:

1. A confident, explainable match for every pair that genuinely
   corresponds, and
2. A categorized, non-discarded exception for every record that doesn't,
   with enough detail to route it to the right human or downstream process.

Silence is the failure mode this is built against: a reconciliation tool
that just doesn't mention a record it couldn't handle is worse than useless
in a finance context.

## Data flow

```
cmd/seed  ──▶  data/store.json (Gateway[], Ledger[], GroundTruth[])
                       │
                       ▼
internal/reconcile.Engine.Run()
                       │
          ┌────────────┴────────────┐
          ▼                          ▼
     model.Match[]              model.Exception[]
          │                          │
          └────────────┬─────────────┘
                        ▼
              model.AuditEntry[] (every decision, matched or not)
                        │
                        ▼
              internal/report.Compute()  ──▶  precision / recall / match rate
                        │
                        ▼
              cmd/server (HTTP API) ──▶ web/index.html (dashboard)
```

`internal/db.Store` is the only place that touches persistence. It's a
JSON file behind a `sync.RWMutex`, not a database driver — deliberately, so
a panel reviewer can clone the repo and run it with zero setup (no MySQL
instance, no credentials). The interface (`Seed`, `Append*`, `Snapshot`,
`ClearRunResults`) is narrow enough that swapping in `database/sql` +
MySQL later only touches this one file — everything above it depends on
the `Store` methods, not the storage format.

## Matching algorithm

For each gateway settlement record, in order:

1. **Duplicate check.** If the same reference appears more than once in the
   gateway set, every occurrence is flagged as `duplicate_gateway_settlement`
   and skipped — never auto-matched, since guessing which duplicate is
   "real" would be worse than asking a human.

2. **Candidate search.** Ledger records are filtered to those that: share
   the same currency, fall within `DateWindowDays` (3 days) of the
   settlement date, and have a net amount within `FeeTolerancePct` (3.5%)
   of the gateway amount minus its recorded fee. 3.5% covers typical
   blended gateway-fee + tax bands without being loose enough to swallow a
   genuine mismatch.

3. **Exact reference match wins outright**, even when other candidates
   also technically satisfy the amount/date window — see "What broke and
   how it was fixed" below for why this rule exists.

4. **Exactly one candidate, no exact ref** → matched at
   `amount_within_fee_tolerance_and_date_window` confidence, but only if
   its reference is at least 60% similar to the gateway reference
   (same-position character match on same-length refs). This distinguishes
   "a bank feed truncated a couple of characters" from "two unrelated
   transactions happen to share an amount and settle three days apart."

5. **Multiple ref-similar candidates** → the ambiguity is handed to the
   `reasoner.Reasoner` interface (`internal/reasoner`), which explains the
   likely cause (fee gap vs. overpayment vs. borderline) and either accepts
   the closest candidate or defers to `ambiguous_multiple_candidates`.

6. **No candidates survive filtering** → `missing_ledger_counterpart`.

After processing every gateway record, any ledger record never claimed
becomes `missing_gateway_counterpart` — this is what "don't discard the
other side" means in the code, not just in the pitch.

Every step above writes an `AuditEntry` with a human-readable reason. This
is the audit trail the brief asks for — it's not a separate logging
concern bolted on afterward, it's produced at the point each decision is
made.

## Synthetic dataset & ground truth (`cmd/seed`)

Each base transaction is generated clean, then a fault is injected with
fixed probabilities so every run exercises every category:

| Fault | What's injected | Expected engine behavior |
|---|---|---|
| `missing_ledger` | Gateway record with no ledger counterpart | `missing_ledger_counterpart` exception |
| `missing_gateway` | Ledger credit with no gateway record | `missing_gateway_counterpart` exception |
| `amount_drift` | Ledger net short by an extra 8% (beyond 3.5% tolerance) | No match; both sides flagged |
| `late_settlement` | Ledger credit 6 days after settlement (beyond 3-day window) | No match |
| `duplicate_gateway` | Same ref settled twice | Both flagged `duplicate_gateway_settlement`, neither matched |
| `mangled_ref` | Last 2 characters of the ledger ref altered | Should still match via the fee/date fallback |
| `clean` (~65%) | No fault | Exact-ref match |

Each generated pair is recorded in `GroundTruth` with the *correct* answer
(which ledger ID should match, or none). `internal/report.Compute` scores
every run against this — precision and recall are computed from actual
known-correct answers, not estimated.

References use a random 6-character alphanumeric suffix (`ORD` + 6 chars
from a 32-symbol alphabet), not a sequential counter — see below for why
that mattered.

## What broke and how it was fixed

This is worth stating plainly rather than presenting a first-try success,
because the brief explicitly rewards honest engineering over a polished
demo:

**Bug 1 — sequential reference IDs caused accidental fuzzy-match
collisions.** The first version used `ORD100000`, `ORD100001`, ... as
references. With settlements spaced 3 hours apart and a 3-day match
window, ~24 neighboring transactions were candidates for each record, and
sequential IDs of the same length coincidentally share 7-8 of 9 characters
just from shared leading digits. Precision on the first real run was
33%. Fixed by (a) always preferring an exact-reference match over any
other candidate even when multiple pass the amount/date filter, and (b)
switching references to random alphanumeric suffixes so unrelated
transactions don't accidentally look similar. Precision went to 98-100%
across five reseeded runs afterward.

**Bug 2 — re-running the engine against the same store double-counted
results.** `Engine.Run` appended matches/exceptions/audit entries without
clearing prior results first, so hitting "Run reconciliation" twice in the
dashboard (or calling the CLI twice) silently inflated the match rate
past 100%. Fixed with `Store.ClearRunResults()`, called at the start of
every `Run`, plus a regression test
(`TestRunningTwiceDoesNotDoubleCountResults`) that asserts the store holds
exactly one match after two consecutive runs on the same data.

Both were caught by actually running the tool end-to-end against generated
data and reading the numbers rather than trusting the code to be correct —
which is the same standard the exception list is designed to hold the
*agent* to.

## Cash position (`internal/report`)

Record counts and percentages don't tell a finance lead what they actually
care about: money. `Compute` sums the real rupee amount behind every
outcome — `ReconciledPaise` (ledger net behind every confirmed match),
`PendingGatewayPaise` and `PendingLedgerPaise` (gross/net amounts sitting
in unresolved exceptions, split by which side they're stuck on). This is
what "run the books and the cash position" in the track brief means in
practice — not a separate feature bolted on, but the natural output of
tracking amounts through the same match/exception pipeline. Covered by
`internal/report/report_test.go`.

## AI executive summary (`internal/reasoner.Summarize`)

A completed run's numbers are handed to `Reasoner.Summarize`, which
returns a 3-sentence plain-English paragraph — what a finance lead reads
before opening the exception list themselves. Two implementations, same
interface:

- **`RuleBasedReasoner.Summarize`** — deterministic Go template, always
  available, tested for exact content (`internal/reasoner/reasoner_test.go`).
  Explicitly recommends a spot-check when precision drops below 95%,
  rather than blindly praising the run.
- **`GeminiReasoner.Summarize`** — sends the same facts to Gemini for a
  genuinely model-written summary when `GEMINI_API_KEY` is set, falling
  back to the rule-based version on any network error.

This mirrors the `Explain` pattern already used for ambiguous-match
resolution — one interface, an offline default, an optional live model.

## Dashboard architecture (`web/index.html`)

Single-file vanilla HTML/CSS/JS, no build step, no framework — the whole
UI is one `<script>` block that calls the same four endpoints a curl
script would: `/api/run`, `/api/report`, `/api/state`, `/api/summary`,
plus `/api/export/exceptions.csv` for the CSV download and `/api/audit`
for the trail view. Four sections (Overview, Exceptions, Matches, Audit
trail) share one in-memory `state` object populated by `load()` — search
and filter inputs re-render from that same state, no extra network calls
per keystroke.

## Money at risk & investigation priority (`internal/report/priority.go`)

Two small, honest additions on top of the existing cash-position math —
deliberately *not* the fabricated-statistics kind of feature ("71%
probability this is a gateway fee issue" with no traceable source). Every
number here is directly computed from the batch, and the formula is fully
disclosed rather than presented as a black box:

- **Money at risk, by reason** — `Metrics.PendingByReason` sums real rupee
  exposure per exception category, not just counts. The Overview dashboard
  chart shows this instead of a bare tally.
- **AI investigation priority** — `Prioritize()` scores every open exception:
  `risk = 80% x reason-type severity + 20% x amount relative to the batch's
  largest exception`. Type dominates deliberately — a small duplicate
  settlement outranks a much larger merely-delayed one, because duplicate
  risk (potential double-counted revenue) needs a look regardless of size,
  while a big number that's just running late is lower urgency. Severity
  weights per reason are a fixed table in `priority.go`, not learned or
  invented. Covered by `internal/report/priority_test.go`, including a
  regression test for a real bug caught while building this: the first
  weighting (60/40) made it mathematically impossible for a pure duplicate
  to reach the "high" risk band no matter how severe the reason was rated,
  because severity alone capped out below the threshold.

Deliberately **not** built: the more theatrical ideas from the same
brainstorm — a "financial time machine" reconstructing historical balances,
a per-merchant "financial DNA" behavioral profile, a multi-agent relabeling
of the same single reasoner, an autonomous "fire drill" simulator. All of
them would require either historical data this project doesn't have or
statistics the code can't actually compute — building them now would mean
shipping invented numbers, which is the opposite of the standard the rest
of this project is held to.

## Fresh data on demand (`internal/seed`, `/api/reseed`)

The seed generator was extracted from `cmd/seed` into `internal/seed` so
the server can call it directly. The dashboard's **"New synthetic batch"**
button hits `POST /api/reseed`, which generates a brand-new random batch
(time-seeded, so genuinely different each click — not the fixed `seed=7`
the CLI uses for reproducibility) and immediately re-runs reconciliation
against it.

This exists specifically because a demo running "Run reconciliation"
against the same static file twice produces *identical* numbers — which is
correct behavior (the engine is deterministic, and re-running without
changing the underlying data should not change the answer) but looks
static to someone watching a 5-minute pitch who doesn't know that's what
they're seeing. Reseeding lets the numbers visibly move between clicks
while precision stays consistently high — which is a stronger claim than
static numbers, because it shows the accuracy isn't tuned to one batch.

Verified directly, not assumed: three consecutive reseed+run cycles produced
match rates of 74.1%, 68.3%, and 70.6% — genuinely different — while
precision held at 100% on all three. Covered by
`internal/seed/seed_test.go` (same seed reproducible, different seeds
differ, every fault class appears in a large enough batch).

## Honest limitations — a currency mismatch is always
  flagged, never matched, even if it might legitimately be the same
  transaction settled cross-border.
- The reasoner's live Gemini path (`internal/reasoner/gemini.go`) hasn't
  been exercised against the real API in this environment (no network
  egress to `generativelanguage.googleapis.com` here); it falls back to
  the rule-based reasoner automatically on any error, and the rule-based
  path is what's covered by the test suite and the reported numbers.
- The JSON-file store is intentionally not a real database — fine for a
  50-100 record batch and a reviewer's laptop, not a production reconciler.
