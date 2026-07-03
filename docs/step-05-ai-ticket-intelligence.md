# AI Support Assistant — Step 5: AI Ticket Intelligence

**Date:** 2026-07-03
**Builds on:** Step 4 — Support Agent Workspace & Ticket Lifecycle
**AI Provider:** Google Gemini (`gemini-2.0-flash`)

---

## Success Criteria Met

| Criterion | Status |
|-----------|--------|
| Auto-analyze every new ticket | ✅ |
| Re-analyze on content update | ✅ |
| Detect category, priority, sentiment, team | ✅ |
| Assign confidence score (0–100) | ✅ |
| Generate summary and tags | ✅ |
| Validate every AI response before storing | ✅ |
| Never block ticket creation on AI failure | ✅ |
| Retry failed analysis via API | ✅ |
| Structured logging (latency, tokens, errors) | ✅ |
| AI provider replaceable without touching services | ✅ |
| AI Analysis tab in Ticket Details UI | ✅ |
| Auto-polling until analysis completes | ✅ |

---

## Architecture

```
HTTP Handler
    ↓
TicketService          (triggers AI after Create / Update)
    ↓
AIService              (orchestrates, manages status transitions)
    ↓
provider.Provider      (interface — Gemini or any future provider)
    ↓
gemini.Client          (REST client, retry logic, structured logs)
    ↓
prompt.Build…          (engineered prompt template)
    ↓
parser.Parse           (extract JSON, strip markdown fences)
    ↓
validator.Validate     (check all 7 fields + confidence range)
```

The AI layer is fully isolated in `internal/ai/`. No AI logic leaks into controllers or the main service layer.

---

## New Backend Package Structure

```
internal/ai/
├── provider/
│   ├── interface.go       # Provider interface + AnalysisRequest / AnalysisResult types
│   └── noop.go            # NoopProvider — used when GEMINI_API_KEY is not set
├── prompt/
│   └── ticket_prompt.go   # Reusable prompt template
├── parser/
│   └── response_parser.go # JSON extraction + markdown fence stripping
├── validator/
│   └── ai_validator.go    # Field presence + allowed-values validation
└── gemini/
    └── client.go          # Google Gemini REST API client
```

---

## Database Changes

### New columns on `tickets`

All columns are nullable and added via GORM `AutoMigrate` (non-destructive).

| Column | Type | Notes |
|--------|------|-------|
| `ai_processing_status` | VARCHAR(20) | `PENDING` (default) → `PROCESSING` → `COMPLETED` / `FAILED` |
| `ai_category` | VARCHAR(100) | AI-detected ticket category |
| `ai_priority` | VARCHAR(20) | AI-suggested priority |
| `ai_sentiment` | VARCHAR(50) | Detected customer sentiment |
| `ai_team` | VARCHAR(100) | Recommended support team |
| `ai_confidence` | INT | 0–100 confidence score |
| `ai_summary` | TEXT | One-sentence issue summary |
| `ai_tags` | TEXT (JSON) | `["payment","refund","transaction"]` stored as JSON string |
| `processed_at` | TIMESTAMP | Timestamp of successful analysis |

### New activity type

`AI_ANALYSIS_COMPLETED` — logged to `ticket_activities` when analysis succeeds, so it appears in the ticket timeline.

---

## AI Processing Workflow

```
Ticket Created (or subject/description updated)
        ↓
TicketService triggers AIService.AnalyzeTicket(id)   ← async goroutine, never blocks
        ↓
AIService marks ticket → PROCESSING
        ↓
gemini.Client calls Gemini REST API (30s timeout, up to 2 retries)
        ↓
        ├── Success →  parser.Parse  →  validator.Validate
        │                   ↓
        │           Store 9 AI fields, mark COMPLETED
        │           Log AI_ANALYSIS_COMPLETED activity
        │
        └── Failure →  Mark FAILED
                        (retry available via POST /retry-ai)
```

### When AI fires

| Action | Triggers AI |
|--------|-------------|
| `POST /api/v1/tickets` (create) | ✅ Always |
| `PUT /api/v1/tickets/:id` when subject or description changes | ✅ |
| Status change (`PATCH /status`) | ❌ |
| Assignment (`PATCH /assign`, `PATCH /take-ownership`) | ❌ |
| Notes, comments | ❌ |

---

## AI Provider Interface

```go
type Provider interface {
    Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResult, error)
}
```

Swapping Google Gemini for OpenAI or Anthropic Claude requires only:
1. A new struct in `internal/ai/<provider>/` that implements this interface
2. A one-line change in `routes.go` to instantiate the new struct

No service, handler, or repository code needs to change.

### NoopProvider

When `GEMINI_API_KEY` is not set, `NoopProvider` is used. It returns an error immediately, which marks the ticket as `FAILED`. The operator can set the key and use `POST /retry-ai` to re-run analysis.

---

## Prompt Engineering

The prompt instructs Gemini to:
- Return **only** a raw JSON object (no markdown, no code fences, no prose)
- Use exact field names and allowed values
- Keep `confidence` as an integer
- Keep `tags` as 2–5 lowercase strings
- Keep `summary` under 150 characters

The parser defensively strips ```` ```json ```` / ```` ``` ```` fences in case the model disobeys, then locates the outermost `{...}` block in the output.

---

## Validation Rules

| Field | Rule |
|-------|------|
| `category` | Must be one of: Payment, Authentication, Technical Issue, Refund, Account, Subscription, General |
| `priority` | Must be one of: Low, Medium, High, Urgent |
| `sentiment` | Must be one of: Positive, Neutral, Frustrated, Angry, Confused |
| `recommended_team` | Must be one of: Finance, Support, Engineering, Sales, Security |
| `confidence` | Integer in range 0–100 |
| `summary` | Non-empty string |
| `tags` | Array with at least one element |

If any rule fails, the ticket is marked `FAILED` and the validation error is logged.

---

## New Backend Files

### `internal/ai/provider/interface.go`
Defines the `Provider` interface and the `AnalysisRequest` / `AnalysisResult` value types shared across all implementations.

### `internal/ai/provider/noop.go`
`NoopProvider` — returns an immediate error when no API key is configured.

### `internal/ai/prompt/ticket_prompt.go`
`BuildTicketAnalysisPrompt(subject, description, customerName, category, priority)` — single function, easily unit-tested, reusable for future providers.

### `internal/ai/parser/response_parser.go`
`Parse(raw string) (*RawAIResponse, error)` — strips markdown fences, finds the JSON object, unmarshals it.

### `internal/ai/validator/ai_validator.go`
`Validate(resp *RawAIResponse) error` — validates all fields against allowed value sets and numeric constraints.

### `internal/ai/gemini/client.go`
`Client` implements `provider.Provider`. Features:
- Configurable timeout and max retries
- Temperature 0.1 (deterministic, structured output)
- Max output tokens 512 (prevents runaway responses)
- Logs latency and token usage at INFO level
- Never logs the API key or the full request URL

### `internal/dto/ai_analysis.go`
`AIAnalysisResponse` — dedicated DTO for `GET /ai-analysis`.

### `internal/services/ai_service.go`
`AIService` — thin orchestration layer:
- `AnalyzeTicket(id)` — fire-and-forget goroutine for new tickets
- `RetryAnalysis(id)` — same goroutine, called from the retry endpoint
- Manages the `PENDING → PROCESSING → COMPLETED / FAILED` state machine
- Logs structured fields: ticket ID, number, category, priority, confidence

### `internal/handlers/ai_handler.go`
Two thin HTTP handlers:
- `GetAnalysis` — reads and returns stored AI fields from the ticket
- `RetryAnalysis` — triggers a goroutine, returns HTTP 202 immediately

---

## Modified Backend Files

### `internal/models/ticket.go`
Added 9 AI fields to the `Ticket` struct. Added `AIStatusPending`, `AIStatusProcessing`, `AIStatusCompleted`, `AIStatusFailed` constants.

### `internal/models/ticket_activity.go`
Added `ActivityAIAnalysisCompleted = "AI_ANALYSIS_COMPLETED"` constant.

### `internal/repositories/ticket_repository.go`
Added `UpdateAIFields(t *Ticket)` — uses GORM `Select` to update only the 9 AI columns, preventing race conditions with concurrent ticket updates.

### `internal/services/ticket_service.go`
- Added `aiService *AIService` field
- Updated `NewTicketService` to accept `*AIService` as fourth argument
- `Create` calls `s.aiService.AnalyzeTicket(created.ID)` after commit
- `Update` calls `s.aiService.AnalyzeTicket(id)` when subject or description changes
- `toTicketResponse` now maps all 9 AI fields

### `internal/dto/ticket.go`
`TicketResponse` extended with all AI fields (`ai_category`, `ai_priority`, `ai_sentiment`, `ai_team`, `ai_confidence`, `ai_summary`, `ai_tags`, `ai_processing_status`, `processed_at`).

### `internal/config/config.go`
Added `GeminiAPIKey`, `GeminiModel`, `AITimeout`, `AIMaxRetries` fields. Added `strconv` import. Added loading + defaults logic (`GEMINI_MODEL` defaults to `gemini-2.0-flash`, `AI_TIMEOUT` defaults to 30s, `AI_MAX_RETRIES` defaults to 2).

### `backend/.env`
Added four new variables:
```
GEMINI_API_KEY=          # Set your key here
GEMINI_MODEL=gemini-2.0-flash
AI_TIMEOUT=30
AI_MAX_RETRIES=2
```

### `internal/routes/routes.go`
- Instantiates `gemini.Client` (or `NoopProvider`) based on `cfg.GeminiAPIKey`
- Creates `AIService` before `TicketService` (no circular dependency)
- Passes `aiService` to `NewTicketService`
- Registers `GET /:id/ai-analysis` and `POST /:id/retry-ai`

---

## New API Endpoints

### `GET /api/v1/tickets/:id/ai-analysis`
Returns the current AI analysis state for a ticket.

```json
// PENDING / PROCESSING
{ "status": "success", "data": { "processing_status": "PROCESSING" } }

// COMPLETED
{
  "status": "success",
  "data": {
    "processing_status": "COMPLETED",
    "category": "Payment",
    "priority": "High",
    "sentiment": "Frustrated",
    "recommended_team": "Finance",
    "confidence": 94,
    "summary": "Customer's payment was deducted but the order was not fulfilled.",
    "tags": ["payment", "refund", "transaction"],
    "processed_at": "2026-07-03T10:15:32Z"
  }
}

// FAILED
{ "status": "success", "data": { "processing_status": "FAILED" } }
```

### `POST /api/v1/tickets/:id/retry-ai`
Queues a retry for a `FAILED` (or any) ticket. Returns HTTP 202 immediately.

```json
{ "status": "success", "message": "AI analysis queued for retry" }
```

---

## New Frontend Files

### `src/services/aiService.js`
```js
aiService.getAnalysis(ticketId)    // GET /:id/ai-analysis
aiService.retryAnalysis(ticketId)  // POST /:id/retry-ai
```

### `src/components/tickets/AIAnalysisPanel.jsx`

Auto-polling component (polls every 3.5 s while `PENDING` or `PROCESSING`):

| State | UI |
|-------|----|
| PENDING / PROCESSING | Animated blue spinner + "AI is analyzing this ticket…" |
| FAILED | Warning icon + description + **Retry Analysis** button |
| COMPLETED | Full result panel (see below) |

**Completed panel layout:**
- Blue summary card
- Confidence progress bar (green ≥ 85%, amber ≥ 60%, red < 60%)
- 2×2 grid: Detected Category · Suggested Priority · Customer Sentiment · Recommended Team
- Tag pills (`#payment`, `#refund`, …)
- Footer with analysis timestamp and "Re-analyze" link

**Badge colors:**
| Priority | Color |
|----------|-------|
| Urgent | Red |
| High | Orange |
| Medium | Amber |
| Low | Green |

| Sentiment | Color |
|-----------|-------|
| Positive | Green |
| Neutral | Gray |
| Frustrated | Orange |
| Angry | Red |
| Confused | Purple |

---

## Modified Frontend Files

### `src/pages/tickets/TicketDetails.jsx`
Added **AI Analysis** as the fifth tab. The tab renders `<AIAnalysisPanel ticketId={id} />` which self-manages polling and state — no changes needed to the rest of the TicketDetails component.

---

## Logging

Every significant AI event is logged in structured JSON via Logrus:

| Event | Level | Fields |
|-------|-------|--------|
| Gemini API response received | INFO | `latency_ms`, `model`, `tokens_total`, `tokens_prompt`, `tokens_response` |
| Retrying after failure | INFO | `attempt`, `model` |
| Non-200 HTTP status | WARN | `status`, `latency_ms` |
| Parse failure | WARN | `error` |
| Validation failure | WARN | `error` |
| Analysis started | INFO | `ticket_id`, `ticket_number` |
| Analysis completed | INFO | `ticket_id`, `ticket_number`, `category`, `priority`, `confidence` |
| Analysis failed | ERROR | `ticket_id`, `ticket_number`, `error` |

The API key and full request URL are **never** logged.

---

## Security

| Rule | Enforcement |
|------|-------------|
| AI endpoints require JWT | All routes are under the `Authenticate` middleware group |
| API key never exposed | Not logged; passed only as a URL query param in the outbound HTTP request |
| AI cannot modify ticket fields | `UpdateAIFields` touches only `ai_*` / `processed_at` columns |
| Prompt injection awareness | Ticket content is inserted into the prompt; the model is instructed to treat it as data, not instructions. AI output is always validated before storage. |

---

## What is intentionally NOT in Step 5

- AI reply generation
- RAG / knowledge base retrieval
- AI-driven ticket routing (auto-assign based on AI category)
- Email / Slack notifications triggered by AI results
- Bulk re-analysis of existing tickets
- Fine-tuning or custom model training
- Background job queue (analysis runs in Go goroutines, not a worker queue)
- Docker
