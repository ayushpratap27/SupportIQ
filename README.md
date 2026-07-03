# SupportIQ — AI-Powered Support Platform

A production-ready, multi-tenant AI support platform built with **Go + Gin** on the backend and **React + Vite** on the frontend. It combines intelligent ticket triage, AI-generated replies, background job processing, email integration, real-time analytics, enterprise integrations, and full SLA management — all inside a complete multi-tenant SaaS architecture.

---

## Feature Overview

| # | Module | Highlights |
|---|--------|-----------|
| 1 | **Project Foundation** | Go/Gin API, React/Vite SPA, PostgreSQL, TailwindCSS |
| 2 | **Authentication** | JWT access + refresh tokens, role-based access control |
| 3 | **Ticket Management** | Full CRUD, status workflow, priority, pagination & search |
| 4 | **Agent Workspace** | Notes, comments, activity timeline, agent assignment |
| 5 | **AI Ticket Intelligence** | Auto-categorise, prioritise, sentiment analysis via Gemini |
| 6 | **AI Reply (RAG + Approval)** | Knowledge-base retrieval, AI draft, human approval workflow |
| 7 | **Background Processing + WebSocket** | Redis job queue, worker pool, real-time WS push |
| 8 | **Email Integration** | IMAP inbound / SMTP outbound, threading, attachments |
| 9 | **Analytics & Reporting** | Daily metrics, agent performance, AI stats, PDF/CSV reports |
| 10 | **Enterprise Integrations** | Jira, Linear, GitHub Issues, Slack, Webhooks, CRM |
| 11 | **Multi-Tenancy** | Full tenant isolation, SuperAdmin, per-tenant settings |
| 12 | **SLA Management** | Policy CRUD, auto-deadlines, escalation monitor, dashboard |

---

## Technology Stack

### Backend
| Technology | Version | Purpose |
|-----------|---------|---------|
| Go | 1.21+ | Core language |
| Gin | 1.12.0 | HTTP framework |
| GORM | 1.31.2 | ORM + migrations |
| PostgreSQL | 14+ | Primary database |
| Redis | 7+ | Job queue + pub/sub |
| `golang-jwt/jwt/v5` | — | JWT auth |
| `gorilla/websocket` | 1.5.3 | Real-time WebSocket |
| Google Gemini API | — | AI analysis + reply generation |
| AES-256-GCM | — | Email credential encryption |

### Frontend
| Technology | Version | Purpose |
|-----------|---------|---------|
| React | 18 | UI framework |
| Vite | 5 | Build tool + dev server |
| TailwindCSS | 3.3.5 | Utility-first styling |
| React Router | 6 | Client-side routing |
| Axios | — | HTTP client |

---

## Project Structure

```
SupportIQ/
├── backend/
│   ├── cmd/
│   │   ├── main.go                  # API server entry point
│   │   └── worker/main.go           # Background worker entry point
│   └── internal/
│       ├── ai/                      # Gemini client, prompts, parsers
│       ├── analytics/               # Metrics aggregation, scheduler, reports
│       ├── config/                  # Env var loader
│       ├── database/                # PostgreSQL connection + AutoMigrate
│       ├── dto/                     # Request/response data transfer objects
│       ├── email/                   # IMAP/SMTP providers, parser, workers
│       ├── events/                  # WebSocket event type constants
│       ├── handlers/                # HTTP handler structs (one per domain)
│       ├── integrations/            # Jira, Linear, GitHub, Slack, Webhooks…
│       ├── jwt/                     # Token generation + validation
│       ├── knowledge/               # KB retrieval (PostgreSQL full-text)
│       ├── middleware/              # Auth, CORS, RBAC, request logger
│       ├── models/                  # GORM model definitions
│       ├── queue/                   # Queue interface + Redis implementation
│       ├── repositories/            # Data access layer (tenant-scoped)
│       ├── routes/                  # Route registration + dependency wiring
│       ├── services/                # Business logic layer
│       ├── utils/                   # Logger, response helpers
│       └── websocket/               # WS hub (broadcast to all clients)
│
├── frontend/
│   └── src/
│       ├── components/              # Reusable UI (badges, panels, countdown…)
│       ├── contexts/                # AuthContext, WebSocketContext
│       ├── layouts/                 # MainLayout shell
│       ├── pages/                   # Route-level page components
│       │   ├── analytics/           # Dashboard, AI insights, reports…
│       │   ├── tickets/             # List, detail, create, edit…
│       │   └── superadmin/          # Platform overview (SuperAdmin only)
│       ├── routes/                  # React Router definitions
│       └── services/                # Axios API client modules
│
└── docs/                            # Per-step implementation notes
```

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.21+ | https://go.dev/dl/ |
| Node.js | 18+ | https://nodejs.org |
| PostgreSQL | 14+ | https://postgresql.org/download/ |
| Redis | 7+ | https://redis.io/download/ (optional — graceful degradation) |

---

## Quick Start

### 1. Clone and configure

```bash
git clone https://github.com/ayushpratap27/SupportIQ.git
cd SupportIQ
```

### 2. Backend

```bash
cd backend
cp .env.example .env   # fill in secrets — see Environment Variables below
createdb supportiq
go mod tidy
go run ./cmd           # starts API on :8080
```

### 3. Frontend

```bash
cd frontend
cp .env.example .env   # set VITE_API_URL=http://localhost:8080
npm install
npm run dev            # starts dev server on :5173
```

### 4. Background worker (optional — required for AI jobs)

```bash
cd backend
go run ./cmd/worker    # requires REDIS_URL to be set
```

---

## Environment Variables

Copy `backend/.env.example` to `backend/.env` and fill in the values:

```env
# ── Core ──────────────────────────────────────────────────────────────────────
PORT=8080
APP_ENV=development
DATABASE_URL=postgres://ayush:password@localhost:5432/supportiq?sslmode=disable

# ── Auth ──────────────────────────────────────────────────────────────────────
JWT_ACCESS_SECRET=change-me-access
JWT_REFRESH_SECRET=change-me-refresh

# ── AI (Gemini) ───────────────────────────────────────────────────────────────
GEMINI_API_KEY=                    # leave blank to use no-op provider
GEMINI_MODEL=gemini-1.5-flash
AI_TIMEOUT=30                      # seconds
AI_MAX_RETRIES=2
MAX_REPLY_TOKENS=1024
REPLY_TEMPERATURE=0.3

# ── Redis / Queue ─────────────────────────────────────────────────────────────
REDIS_URL=redis://localhost:6379
QUEUE_NAME=ai_jobs
WORKER_COUNT=3
MAX_RETRIES=3
RETRY_DELAY=5

# ── Email ─────────────────────────────────────────────────────────────────────
EMAIL_POLL_INTERVAL=60             # IMAP poll interval in seconds
MAX_EMAIL_RETRIES=3
ATTACHMENT_PATH=./storage/attachments

# ── Analytics ─────────────────────────────────────────────────────────────────
METRICS_REFRESH_INTERVAL=3600      # seconds
REPORT_STORAGE_PATH=./storage/reports
REPORT_RETENTION_DAYS=30

# ── Integrations ──────────────────────────────────────────────────────────────
INTEGRATION_POLL_INTERVAL=30       # seconds
WEBHOOK_SECRET=                    # optional HMAC secret for outbound webhooks

# ── WebSocket ─────────────────────────────────────────────────────────────────
WEBSOCKET_ORIGIN=http://localhost:5173
```

Frontend `.env`:

```env
VITE_API_URL=http://localhost:8080
```

---

## API Reference

All authenticated endpoints require `Authorization: Bearer <accessToken>`.

### Auth
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | — | Register company + first admin |
| POST | `/api/v1/auth/login` | — | Login, returns access + refresh tokens |
| POST | `/api/v1/auth/logout` | Bearer | Logout |
| GET | `/api/v1/auth/me` | Bearer | Current user |

### Tickets
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/tickets` | Bearer | Create ticket |
| GET | `/api/v1/tickets` | Bearer | List (filter by status, priority, sla_status…) |
| GET | `/api/v1/tickets/:id` | Bearer | Get ticket |
| PUT | `/api/v1/tickets/:id` | Bearer | Update ticket |
| PATCH | `/api/v1/tickets/:id/status` | Bearer | Update status |
| PATCH | `/api/v1/tickets/:id/assign` | Admin | Assign ticket |
| PATCH | `/api/v1/tickets/:id/take-ownership` | Agent | Take ownership |
| DELETE | `/api/v1/tickets/:id` | Admin | Soft delete |
| GET | `/api/v1/tickets/sla` | Bearer | SLA dashboard |
| GET | `/api/v1/my-tickets` | Bearer | My tickets |
| GET | `/api/v1/tickets/unassigned` | Admin | Unassigned tickets |

### Ticket Sub-resources
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/tickets/:id/notes` | Internal notes |
| GET/POST | `/api/v1/tickets/:id/comments` | Public comments |
| GET | `/api/v1/tickets/:id/activity` | Activity timeline |
| GET/POST | `/api/v1/tickets/:id/emails` | Email thread |
| POST | `/api/v1/tickets/:id/send-email` | Send reply email |
| GET | `/api/v1/tickets/:id/ai-analysis` | AI analysis result |
| POST | `/api/v1/tickets/:id/retry-ai` | Retry AI analysis |
| GET | `/api/v1/tickets/:id/reply` | AI draft reply |
| POST | `/api/v1/tickets/:id/reply/generate` | Generate AI reply |
| POST | `/api/v1/tickets/:id/reply/approve` | Approve AI reply |
| POST | `/api/v1/tickets/:id/reply/reject` | Reject AI reply |
| GET | `/api/v1/tickets/:id/integrations` | Linked external issues |
| POST | `/api/v1/tickets/:id/create-jira` | Create Jira issue |
| POST | `/api/v1/tickets/:id/create-linear` | Create Linear issue |
| POST | `/api/v1/tickets/:id/create-github-issue` | Create GitHub issue |

### SLA Policies
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET/POST | `/api/v1/sla-policies` | Admin | List / create SLA policies |
| GET/PUT/DELETE | `/api/v1/sla-policies/:id` | Admin | Get / update / delete |

### Knowledge Base
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/knowledge` | List / create articles |
| GET/PUT/DELETE | `/api/v1/knowledge/:id` | Get / update / delete |

### Analytics
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/analytics/overview` | Platform overview |
| GET | `/api/v1/analytics/tickets` | Ticket stats |
| GET | `/api/v1/analytics/agents` | Agent performance |
| GET | `/api/v1/analytics/ai` | AI metrics |
| GET | `/api/v1/analytics/queues` | Queue monitoring |
| GET | `/api/v1/analytics/email` | Email stats |
| GET | `/api/v1/analytics/trends` | Trend data |
| GET/POST | `/api/v1/analytics/reports` | List / generate reports |
| GET | `/api/v1/analytics/reports/:id/download` | Download report |

### Email Accounts
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/email/accounts` | List / create email accounts |
| PUT/DELETE | `/api/v1/email/accounts/:id` | Update / delete |
| POST | `/api/v1/email/accounts/:id/test` | Test SMTP/IMAP connection |
| GET | `/api/v1/email/monitor` | Email health metrics |

### Integrations
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/integrations` | List / create integrations |
| PUT/DELETE | `/api/v1/integrations/:id` | Update / delete |
| POST | `/api/v1/integrations/:id/test` | Test connection |
| GET | `/api/v1/integrations/:id/events` | Event delivery log |

### Tenant Management (SuperAdmin only)
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/admin/tenants` | List / create tenants |
| GET/PUT/DELETE | `/api/v1/admin/tenants/:id` | Manage tenant |
| GET | `/api/v1/admin/overview` | Platform-wide stats |
| GET/PUT | `/api/v1/settings` | Tenant settings (Admin) |

### Other
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/users/agents` | List support agents |
| GET | `/api/v1/jobs` | Background job monitor |
| GET | `/api/v1/ws?token=...` | WebSocket connection |

---

## Data Model

```
Tenant
 └── User (roles: Admin, SupportAgent, SuperAdmin)
 └── SLAPolicy (per priority, first-response + resolution targets)
 └── Ticket
      ├── TicketActivity (immutable audit log)
      ├── TicketNote (internal)
      ├── TicketComment (public)
      ├── AIReply (draft → approved/rejected)
      ├── EmailMessage (threaded)
      └── TicketIntegration (Jira/Linear/GitHub link)
 └── KnowledgeBase (articles for RAG retrieval)
 └── EmailAccount (IMAP + SMTP credentials, encrypted)
 └── Integration (Jira, Slack, Webhooks…)
 └── BackgroundJob (async job tracker)
 └── DailyTicketMetrics / AgentMetrics / AIMetrics / Report
```

---

## Multi-Tenancy

Every API request is scoped to the authenticated user's tenant. The pattern is enforced at three layers:

1. **JWT** — `TenantID` is embedded in every access token
2. **Middleware** — `Authenticate()` validates the token and injects `tenantID` into the Gin context
3. **Repository** — every query is scoped with `WHERE tenant_id = ?`

SuperAdmins (`tenantID = uuid.Nil`) bypass tenant checks and can manage all tenants via `/api/v1/admin/*`.

---

## SLA Management

When a ticket is created, the system automatically:

1. Looks up the SLA policy matching the ticket's priority (falls back to the default policy)
2. Calculates `first_response_due_at` and `resolution_due_at`
3. Sets `sla_status = ON_TRACK`

A background monitor runs every 60 seconds and transitions tickets through:

| Threshold | Action |
|-----------|--------|
| ≥ 80% elapsed | → `AT_RISK`, logs `SLA_AT_RISK` activity, notifies agent |
| ≥ 90% elapsed | Logs `SLA_ESCALATED` activity, notifies team lead |
| ≥ 100% elapsed | → `BREACHED`, logs `SLA_BREACHED` activity |
| Ticket resolved on time | → `COMPLETED` |

SLA status changes are broadcast over WebSocket so the frontend countdown updates in real time.

---

## Background Processing

The worker process (`cmd/worker/main.go`) drains a Redis queue with a configurable pool of goroutines. Job types:

| Job Type | Description |
|----------|-------------|
| `AI_ANALYSIS` | Run Gemini analysis on a new ticket |
| `RETRY_AI` | Retry failed AI analysis |
| `GENERATE_REPLY` | Generate AI draft reply |
| `REGENERATE_REPLY` | Regenerate a rejected reply |
| `INBOUND_EMAIL` | Process received email |
| `OUTBOUND_EMAIL` | Send queued email |
| `INTEGRATION_EVENT` | Deliver integration webhook |
| `JIRA_SYNC` / `CRM_SYNC` / … | External sync jobs |

Failed jobs are retried with exponential back-off (base `RETRY_DELAY` × 2^n) and moved to a dead-letter queue after `MAX_RETRIES` exhausted.

---

## Real-Time WebSocket Events

Connect via `GET /api/v1/ws?token=<accessToken>`. The server pushes:

| Event | Trigger |
|-------|---------|
| `ticket.ai.completed` | AI analysis finished |
| `ticket.reply.generated` | AI reply generated |
| `ticket.updated` | Ticket fields changed |
| `sla.updated` | SLA status changed |
| `job.completed` | Background job succeeded |
| `job.failed` | Background job failed |
| `analytics.refresh` | Analytics cycle completed |

---

## User Roles

| Role | Permissions |
|------|------------|
| `Admin` | Full access within their tenant |
| `SupportAgent` | Manage tickets, add comments/notes, generate AI replies |
| `SuperAdmin` | Cross-tenant access, manage all tenants and users |

---

## Frontend Pages

| Path | Page | Access |
|------|------|--------|
| `/dashboard` | Overview dashboard | All |
| `/tickets` | Ticket list with filters | All |
| `/tickets/:id` | Ticket detail + SLA countdown | All |
| `/tickets/new` | Create ticket | All |
| `/my-tickets` | My assigned tickets | All |
| `/knowledge-base` | Knowledge base CRUD | All |
| `/email/accounts` | Email account management | Admin |
| `/email/monitor` | Email health monitor | Admin |
| `/analytics` | Analytics dashboard | Admin |
| `/analytics/ai` | AI insights | Admin |
| `/analytics/agents` | Agent performance | Admin |
| `/analytics/queues` | Queue monitoring | Admin |
| `/analytics/reports` | Reports | Admin |
| `/integrations` | Integration management | Admin |
| `/jobs` | Background job monitor | Admin |
| `/sla` | SLA dashboard | Admin |
| `/sla-management` | SLA policy CRUD | Admin |
| `/settings` | Tenant settings | Admin |
| `/admin` | SuperAdmin dashboard | SuperAdmin |

---

## Development

```bash
# Run all tests
cd backend && go test ./...

# Build backend binary
cd backend && go build -o supportiq ./cmd

# Build frontend for production
cd frontend && npm run build

# Lint frontend
cd frontend && npm run lint
```

---

## License

MIT

