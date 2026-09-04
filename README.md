# AI Finance Controller — Reconciliation Agent

> **Razorpay AI Buildathon 2026 — Track 04: AI Finance Controller**

An AI-assisted financial reconciliation system that automatically matches **payment gateway settlements** against **merchant ledger credits**, identifies unresolved exceptions, calculates financial exposure, and provides an auditable explanation for every decision.

The system is built around one core principle:

> **No financial record should disappear silently.**

Every gateway and ledger record is either matched confidently or surfaced as an actionable exception.

---

## 🚀 What This Project Does

Financial reconciliation is traditionally a repetitive manual process.

This project automates the first-pass reconciliation workflow using:

- Deterministic transaction matching
- Exact reference matching
- Amount/date-based candidate analysis
- Fuzzy matching for ambiguous transactions
- Gemini-powered AI reasoning for genuinely ambiguous cases
- Automatic rule-based fallback when Gemini is unavailable
- Duplicate detection
- Exception classification
- Financial risk prioritization
- Cash-position calculation
- Ground-truth evaluation
- Complete audit logging
- AI-generated executive summaries

The system operates on a **synthetic but realistic dataset** containing deliberate transaction faults and known ground-truth relationships.

---

# ✨ Key Features

### 🔍 Automated Reconciliation

Automatically matches payment gateway settlements with merchant ledger records using a confidence-based reconciliation pipeline.

### 🎯 Deterministic Matching

Exact transaction references are always preferred over weaker amount/date-based matches.

### 🤖 AI-Assisted Reasoning

Ambiguous candidates can be evaluated using the **Google Gemini API**.

If Gemini is unavailable, the system automatically falls back to a deterministic rule-based reasoner.

### 🚨 Exception Management

Unresolved transactions are never discarded.

They are categorized into human-readable exception types such as:

- Missing ledger entry
- Missing gateway settlement
- Amount mismatch
- Date mismatch
- Duplicate settlement
- Ambiguous candidate
- Reference mismatch

### 💰 Money-at-Risk Analysis

The dashboard tracks actual INR exposure rather than only counting transaction records.

Finance teams can therefore see:

**Reconciled Money vs. Pending Money vs. Money at Risk**

### 📊 Ground-Truth Evaluation

Every synthetic transaction contains known ground-truth relationships.

This allows the system to calculate:

- Precision
- Recall
- Match Rate

The metrics are evaluated against the generated dataset rather than a manually selected demo.

### 📝 Complete Audit Trail

Every reconciliation decision is recorded with:

- Transaction information
- Matching decision
- Confidence
- Reason
- Timestamp

### 🔄 Idempotent Runs

Running reconciliation multiple times against the same dataset produces consistent results without creating duplicate matches.

---

# 🏗️ Architecture

```text
                 ┌──────────────────────┐
                 │  Synthetic Dataset   │
                 │      Generator       │
                 └──────────┬───────────┘
                            │
                            ▼
                 ┌──────────────────────┐
                 │   JSON Data Store    │
                 │   data/store.json    │
                 └──────────┬───────────┘
                            │
                            ▼
                 ┌──────────────────────┐
                 │ Reconciliation Engine│
                 └──────────┬───────────┘
                            │
                ┌───────────┴────────────┐
                │                        │
                ▼                        ▼
       ┌─────────────────┐      ┌──────────────────┐
       │ Exact Matching  │      │ Fuzzy / Ambiguous│
       │                 │      │ Candidate Logic  │
       └────────┬────────┘      └────────┬─────────┘
                │                        │
                │                        ▼
                │               ┌──────────────────┐
                │               │   AI Reasoner    │
                │               │ Gemini / Rules   │
                │               └────────┬─────────┘
                │                        │
                └────────────┬───────────┘
                             ▼
                 ┌──────────────────────┐
                 │ Matches / Exceptions│
                 └──────────┬───────────┘
                            │
             ┌──────────────┼──────────────┐
             ▼              ▼              ▼
       ┌──────────┐   ┌──────────┐   ┌───────────┐
       │  Report  │   │ Audit Log│   │ Risk Score│
       └────┬─────┘   └────┬─────┘   └─────┬─────┘
            │              │               │
            └──────────────┼───────────────┘
                           ▼
                 ┌──────────────────────┐
                 │    Web Dashboard     │
                 │    localhost:8080    │
                 └──────────────────────┘
```

---

# 🛠️ Tech Stack

| Layer           | Technology                    |
| --------------- | ----------------------------- |
| Backend         | Go                            |
| API Server      | Go HTTP Server                |
| Data Store      | JSON                          |
| AI Reasoning    | Google Gemini API             |
| Frontend        | HTML / CSS / JavaScript       |
| Testing         | Go Test                       |
| Data Generation | Custom Go Synthetic Generator |
| Architecture    | Modular Go packages           |

> **No external database server is required.** The project uses a lightweight JSON-backed store for the demo.

---

# 📁 Project Structure

```text
finance-controller/
│
├── cmd/
│   ├── seed/
│   │   └── main.go
│   │
│   ├── reconcile/
│   │   └── main.go
│   │
│   └── server/
│       └── main.go
│
├── internal/
│   ├── model/
│   │   └── shared transaction types
│   │
│   ├── db/
│   │   └── JSON-backed data store
│   │
│   ├── reconcile/
│   │   └── reconciliation engine
│   │
│   ├── reasoner/
│   │   ├── reasoner.go
│   │   └── gemini.go
│   │
│   └── report/
│       └── precision / recall / match-rate calculations
│
├── web/
│   └── index.html
│
├── data/
│   └── store.json
│
├── ARCHITECTURE.md
├── go.mod
└── README.md
```

---

# ⚡ Quick Start

## Prerequisites

Make sure you have:

- **Go 1.22+**
- A modern web browser
- Git

Gemini is **optional**. The project works offline using the rule-based reasoner.

---

## 1. Clone the Repository

```bash
git clone <YOUR_GITHUB_REPOSITORY_URL>
cd finance-controller
```

---

## 2. Verify Go

```bash
go version
```

Go **1.22 or higher** is recommended.

---

## 3. Build the Project

```bash
go build ./...
```

This verifies that all project packages compile successfully.

---

## 4. Run the Tests

```bash
go test ./...
```

The test suite validates important parts of the system including:

- Transaction fault classes
- Reconciliation logic
- Cash-position calculations
- Risk scoring
- AI summary generation
- Synthetic data generation

---

## 5. Generate Synthetic Data

Generate the initial dataset:

```bash
go run ./cmd/seed -n 90 -seed 7
```

This creates:

```text
data/store.json
```

The generated dataset contains realistic gateway and ledger transactions with deliberately injected fault cases and ground-truth relationships.

### Seed Parameters

| Parameter | Description                                  |
| --------- | -------------------------------------------- |
| `-n 90`   | Generates approximately 90 synthetic records |
| `-seed 7` | Uses deterministic random seed `7`           |

Using the same seed produces the same dataset, which makes testing and demonstrations reproducible.

---

# 📊 6. Run Reconciliation from the CLI

Before opening the dashboard, you can run the reconciliation engine directly:

```bash
go run ./cmd/reconcile
```

The command prints the reconciliation report to the terminal, including metrics such as:

```text
Match Rate
Precision
Recall
Reconciled Amount
Pending Amount
Exceptions
```

This is useful as a backend sanity check.

---

# 🌐 7. Start the Dashboard

Run:

```bash
go run ./cmd/server
```

The server starts on:

```text
http://localhost:8080
```

Open the URL in your browser:

**http://localhost:8080**

Keep the terminal running while using the dashboard.

---

# 🤖 Optional — Live Gemini-Powered AI

Gemini is **optional**.

Without a Gemini API key, the system uses:

```text
RuleBasedReasoner
        ↓
Deterministic reasoning
        ↓
Offline
```

With a Gemini API key:

```text
Ambiguous Transaction
        ↓
Gemini Reasoner
        ↓
AI-assisted Decision
```

If Gemini encounters a network or API error, the application automatically falls back to the rule-based reasoner.

---

## Windows PowerShell — Session Only

This keeps the API key available only for the current PowerShell session.

```powershell
$env:GEMINI_API_KEY = "your_key_here"
go run ./cmd/server
```

---

## Windows PowerShell — Permanent

To save the environment variable for your Windows user account:

```powershell
[System.Environment]::SetEnvironmentVariable("GEMINI_API_KEY", "your_key_here", "User")
```

After setting it permanently, open a **new PowerShell window** and run:

```powershell
go run ./cmd/server
```

> Never commit your real Gemini API key to GitHub.

---

## Linux / macOS

```bash
export GEMINI_API_KEY="your_key_here"
go run ./cmd/server
```

---

# 🔄 Generate a Fresh Dataset

If you want to completely reset the existing dataset and generate a new one from the CLI:

### PowerShell

```powershell
Remove-Item -Recurse -Force data
go run ./cmd/seed -n 90 -seed 7
```

Then run:

```powershell
go run ./cmd/reconcile
```

Or restart the dashboard:

```powershell
go run ./cmd/server
```

### Why remove the `data` directory?

The `data` directory contains the generated JSON datastore:

```text
data/store.json
```

Removing it ensures that you start with a clean synthetic dataset.

---

# 🎨 Dashboard

The dashboard contains five primary views.

## 1. Overview

Provides a high-level financial snapshot:

- Match Rate
- Precision
- Recall
- Reconciled Amount
- Pending Amount
- Money at Risk
- Exception distribution

The dashboard focuses on **actual INR exposure**, not just transaction counts.

---

## 2. Priority Queue

Every unresolved exception receives a deterministic risk score.

The score combines:

```text
80% → Exception Severity
20% → Amount Exposure
```

This prioritizes cases that require human attention.

---

## 3. Exceptions

Displays every unresolved transaction.

Users can:

- Search exceptions
- Filter exceptions
- Inspect exception details
- Review failure reasons
- Export exceptions as CSV

CSV endpoint:

```text
/api/export/exceptions.csv
```

---

## 4. Matches

Displays every confirmed reconciliation.

Each match contains:

- Gateway transaction
- Ledger transaction
- Confidence level
- Matching method
- Reasoning

Examples:

```text
Exact Reference Match
Fee/Date Fallback
```

---

## 5. Audit Trail

Displays the complete reconciliation decision history.

Each decision is timestamped and recorded so the entire reconciliation process remains explainable and auditable.

---

# 🧠 AI Executive Summary

The dashboard provides an executive summary through:

```text
/api/summary
```

The summary is generated from the **actual reconciliation results**.

### Without Gemini

A deterministic template generates the summary.

### With Gemini

Gemini generates a natural-language financial summary using the current run's actual metrics.

The summary is therefore based on the current reconciliation run rather than being a hardcoded demo message.

---

# 📈 Evaluation

The system was evaluated across multiple randomly generated datasets.

Using five different seeds with approximately **80–100 records per run**, the observed performance was:

| Metric     | Observed Range |
| ---------- | -------------: |
| Precision  |        98–100% |
| Recall     |         97–99% |
| Match Rate |         69–80% |

These metrics are calculated against the ground-truth relationships generated with the synthetic dataset.

The goal is not to maximize the match rate at any cost.

Instead, the system intentionally leaves uncertain transactions unresolved rather than making unsafe financial guesses.

---

# 🔍 Reconciliation Strategy

The reconciliation engine follows a confidence-based matching hierarchy.

## Step 1 — Exact Reference Match

If a reliable transaction reference matches, it is preferred.

```text
Gateway Reference
       ↓
Exact Ledger Reference
       ↓
CONFIRMED
```

---

## Step 2 — Candidate Search

When an exact reference is unavailable, the engine searches for potential matches using:

- Reference similarity
- Amount
- Transaction date
- Other transaction attributes

A fuzzy match still requires sufficient reference similarity.

Amount and date proximity alone are not sufficient.

---

## Step 3 — Ambiguous Candidate Resolution

If multiple candidates remain plausible:

```text
Ambiguous Candidates
        ↓
RuleBasedReasoner
        OR
GeminiReasoner
        ↓
Decision
```

The reasoner provides an additional decision layer for genuinely ambiguous cases.

---

## Step 4 — Exception

If the system cannot reach a sufficiently confident decision:

```text
UNRESOLVED
    ↓
Exception
    ↓
Risk Score
    ↓
Human Review
```

No uncertain financial transaction is silently accepted.

---

# 🛡️ Important Design Decisions

## No Silent Drops

Every gateway and ledger record is accounted for.

```text
Gateway / Ledger Record
          ↓
     ┌────┴────┐
     ▼         ▼
   Match    Exception
```

---

## Exact References Have Priority

Exact references are preferred over amount/date proximity.

This reduces accidental matches between unrelated transactions that happen to have similar values.

---

## Duplicate Settlements Are Not Automatically Matched

Potential duplicate gateway settlements are flagged for human review instead of being automatically assigned to a ledger transaction.

---

## Money Matters More Than Row Counts

The system calculates the actual financial value behind:

- Successful matches
- Pending transactions
- Exceptions
- Risk exposure

This provides a more meaningful view for finance operations.

---

## Deterministic by Default

The default rule-based reasoning path is:

- Offline
- Reproducible
- Deterministic
- Testable

Gemini is an optional enhancement rather than a hard dependency.

---

## Idempotent Reconciliation

Running:

```bash
go run ./cmd/reconcile
```

multiple times against the same dataset produces consistent results.

The system does not continuously create duplicate matches.

---

# 🎬 Recommended Demo Flow

For a Razorpay Buildathon presentation or pitch video, use this sequence.

### Terminal

```bash
go build ./...
go test ./...
go run ./cmd/seed -n 90 -seed 7
go run ./cmd/reconcile
go run ./cmd/server
```

Then open:

```text
http://localhost:8080
```

### Dashboard

1. Open **Overview**
2. Click **Run Reconciliation**
3. Show the updated metrics
4. Open **Priority Queue**
5. Open **Exceptions**
6. Export the exceptions CSV
7. Open **Matches**
8. Show confidence and reasoning
9. Open **Audit Trail**
10. Show the **AI Executive Summary**
11. Click **New Synthetic Batch**
12. Run reconciliation again
13. Show that the new dataset produces different results

This demonstrates that the dashboard is connected to the actual reconciliation engine rather than displaying hardcoded numbers.

---

# 🔄 Reproducible Demo

To reproduce the same dataset:

```bash
go run ./cmd/seed -n 90 -seed 7
```

To create a different dataset:

```bash
go run ./cmd/seed -n 90 -seed 10
```

The same seed produces the same synthetic dataset, while changing the seed produces a different batch.

This makes the system easy to test and demonstrate consistently.

---

# 📚 Documentation

For a deeper explanation of the system, see:

```text
ARCHITECTURE.md
```

It covers:

- Fault injection
- Matching rules
- Risk scoring
- Reconciliation decisions
- Testing
- Design trade-offs
- Bugs discovered and fixed
- System architecture

---

# 🎯 Project Goal

The goal of this project is not simply to match as many transactions as possible.

The goal is to build a reconciliation system that is:

**Accurate → Explainable → Auditable → Risk-aware → Deterministic**

A financial AI system should know when it is confident — and, equally importantly, when it is **not**.

---

# 🏆 Razorpay AI Buildathon 2026

Built for:

**Razorpay AI Buildathon 2026**

**Track 04 — AI Finance Controller**

This project demonstrates how AI-assisted automation can reduce manual reconciliation effort while maintaining financial safety through:

- Deterministic matching
- AI-assisted ambiguity resolution
- Exception handling
- Risk prioritization
- Financial exposure analysis
- Auditability
- Reproducible evaluation

---

## 👨‍💻 Project

**AI Finance Controller — Reconciliation Agent**

Built with **Go + HTML/CSS/JavaScript + Google Gemini API**

---
