# Step 6: AI Reply Generation, RAG & Human Approval Workflow

## Overview

This step implements the full AI reply pipeline grounded in the company's internal knowledge base (Retrieval Augmented Generation), combined with a human-in-the-loop approval workflow before any reply reaches the customer.

---

## What Was Built

### Knowledge Base (RAG Foundation)

A `knowledge_bases` PostgreSQL table stores company documents used to ground every AI reply. The AI is **never allowed to invent policies** — it can only answer using the content stored here.

**Supported categories:**
- FAQ
- Refund Policy
- Shipping Policy
- Subscription Policy
- Account Policy
- Payment Policy
- General Documentation

**Architecture:** A `Retriever` interface decouples retrieval logic from storage. The current `PostgresRetriever` performs case-insensitive keyword search. Replacing it with a vector database (pgvector, Pinecone, Weaviate) requires only a new implementation of the interface — no service changes.

---

### AI Reply Model & Versioning

Every generation creates a new `ai_replies` record. Previous versions are **never overwritten or deleted**, providing a complete audit history.

```
ai_replies
├── id
├── ticket_id
├── generated_reply      ← original AI output (never changed)
├── edited_reply         ← agent's edited version (nullable)
├── confidence           ← 0–100 integer
├── status               ← GENERATED | APPROVED | REJECTED | REGENERATED | SENT
├── model                ← e.g. gemini-2.0-flash
├── prompt_version       ← v1
├── generation_time      ← milliseconds
├── approved_by          ← FK to users (nullable)
├── approved_at          ← timestamp (nullable)
├── created_at
└── updated_at
```

---

### Reply Generation Pipeline

```
Ticket Created
     ↓
AI Analysis Completes
     ↓
Knowledge Retrieval (RAG)
     ↓
Prompt Builder (ticket context + KB documents)
     ↓
Gemini API
     ↓
Parse & Validate JSON response
     ↓
Persist as GENERATED draft
     ↓
Support Agent reviews in UI
     ↓
Approve / Edit / Reject / Regenerate
```

---

### Gemini Client Refactor

The existing `gemini.Client` was refactored to:
- Extract a shared `callAPI(ctx, prompt, temperature, maxTokens) → rawText` method
- Reuse it for both `Analyze` (ticket analysis) and `GenerateReply` (reply generation)
- Expose `NewClientWithReplyConfig` for configurable temperature and max tokens

---

### Reply Prompt (RAG-Enhanced)

```
You are a professional customer support AI assistant.
Return ONLY a valid JSON object.

--- TICKET DETAILS ---
Subject: ...
Description: ...
Category: ...
Priority: ...
Customer Sentiment: ...

--- RELEVANT KNOWLEDGE BASE DOCUMENTS ---
[Document 1] Refund Policy
...full document content...

--- END OF KNOWLEDGE BASE DOCUMENTS ---

Instructions:
- NEVER invent company policies.
- Use ONLY the provided documents.
- Be empathetic and concise.

{"reply": "...", "confidence": 94}
```

---

### Human Approval Workflow

| Action | Status Transition | Activity Log |
|--------|------------------|--------------|
| Generate | → GENERATED | `AI_REPLY_GENERATED` |
| Approve | GENERATED → APPROVED | `AI_REPLY_APPROVED` |
| Edit | GENERATED (edited_reply saved) | `AI_REPLY_EDITED` |
| Reject | GENERATED → REJECTED | `AI_REPLY_REJECTED` |
| Regenerate | GENERATED → REGENERATED + new GENERATED | `AI_REPLY_REGENERATED` |

---

## API Endpoints

### Knowledge Base (Admin Only)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/knowledge-base` | List with search, category filter, pagination |
| POST | `/api/v1/knowledge-base` | Create new document |
| PUT | `/api/v1/knowledge-base/:id` | Update document |
| DELETE | `/api/v1/knowledge-base/:id` | Delete document |

### Reply Workflow

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/tickets/:id/reply` | Get latest reply draft |
| POST | `/api/v1/tickets/:id/reply/generate` | Generate reply |
| POST | `/api/v1/tickets/:id/reply/regenerate` | Discard draft, generate new |
| PATCH | `/api/v1/tickets/:id/reply/edit` | Save edited reply |
| POST | `/api/v1/tickets/:id/reply/approve` | Approve reply |
| POST | `/api/v1/tickets/:id/reply/reject` | Reject reply |

---

## New Environment Variables

```env
MAX_REPLY_TOKENS=1024      # max tokens for reply generation
REPLY_TEMPERATURE=0.3      # creativity (0.0 = deterministic, 1.0 = creative)
```

---

## Backend Files Added

```
backend/internal/
├── models/
│   ├── knowledge_base.go          ← KnowledgeBase model + categories
│   └── ai_reply.go                ← AIReply model + status constants
├── ai/reply/
│   ├── provider/
│   │   ├── interface.go           ← ReplyProvider interface
│   │   └── noop.go                ← NoopReplyProvider (no API key)
│   ├── prompt/
│   │   └── reply_prompt.go        ← RAG-enhanced prompt builder
│   ├── parser/
│   │   └── reply_parser.go        ← JSON response parser
│   └── validator/
│       └── reply_validator.go     ← Reply + confidence validation
├── knowledge/retrieval/
│   ├── retriever.go               ← Retriever interface
│   └── postgres.go                ← PostgreSQL keyword search impl
├── repositories/
│   ├── knowledge_repository.go    ← CRUD + keyword search
│   └── reply_repository.go        ← Create, Update, FindLatest, FindAll
├── services/
│   ├── knowledge_service.go       ← Knowledge base CRUD logic
│   └── reply_service.go           ← Full reply workflow orchestration
├── handlers/
│   ├── knowledge_handler.go       ← REST handlers for KB endpoints
│   └── reply_handler.go           ← REST handlers for reply endpoints
└── dto/
    ├── knowledge.go               ← Knowledge request/response DTOs
    └── reply.go                   ← Reply request/response DTOs
```

## Backend Files Modified

| File | Change |
|------|--------|
| `ai/gemini/client.go` | Extracted `callAPI`, added `GenerateReply`, `NewClientWithReplyConfig` |
| `models/ticket_activity.go` | Added 5 reply activity type constants |
| `config/config.go` | Added `MaxReplyTokens`, `ReplyTemperature` fields |
| `services/ai_service.go` | Added `replySvc` field + `SetReplyService()`, triggers reply on analysis complete |
| `database/database.go` | Added `KnowledgeBase`, `AIReply` to `AutoMigrate` |
| `routes/routes.go` | Wired all new repos, services, handlers and registered all new routes |

---

## Frontend Files Added

```
frontend/src/
├── components/tickets/
│   └── AIReplyPanel.jsx    ← Reply viewer with approve/edit/reject/regenerate
├── pages/
│   └── KnowledgeBase.jsx   ← Admin CRUD page with search, filters, modals
└── services/
    ├── replyService.js     ← API client for all reply endpoints
    └── knowledgeService.js ← API client for KB CRUD endpoints
```

## Frontend Files Modified

| File | Change |
|------|--------|
| `pages/tickets/TicketDetails.jsx` | Added "AI Reply" tab (6 tabs total) |
| `routes/index.jsx` | Added `/knowledge-base` protected route |
| `pages/Dashboard.jsx` | Added "Knowledge Base" nav link for admin users |

---

## UI Features

### AI Reply Panel (Ticket Details → AI Reply tab)

- **No reply state** — Generate button with explanation
- **Confidence bar** — Green (≥85%), Amber (≥60%), Red (<60%)
- **Status badge** — GENERATED / APPROVED / REJECTED / REGENERATED / SENT
- **Action buttons** — Approve ✓, Edit ✏️, Reject ✗, Regenerate 🔄
- **Edit mode** — Textarea with Save/Cancel; original preserved below
- **Copy button** — Copies the active reply to clipboard
- **Approved info** — Shows approver name and timestamp

### Knowledge Base Admin Page

- Stats row: total documents, active count, category count
- Search bar + category filter dropdown
- Paginated document table with inline Edit/Delete
- Create and Edit modals with full form validation
- Active toggle per document

---

## Error Handling

| Scenario | Behaviour |
|----------|-----------|
| KB retrieval DB error | Returns 422 "knowledge base unavailable" |
| No matching KB documents | Returns 422 with hint to add articles |
| Gemini API failure | Returns 422; previous draft is preserved |
| Approve non-GENERATED reply | Returns 400 with current status in message |
| Edit non-GENERATED reply | Returns 400 with current status in message |
| No API key configured | `NoopReplyProvider` returns immediate error |

---

## Security

- Knowledge base CRUD requires `Admin` role
- Reply approval requires `Admin` or `SupportAgent` role
- All reply endpoints require valid JWT (`Authenticate` middleware)
- Gemini API key never logged
