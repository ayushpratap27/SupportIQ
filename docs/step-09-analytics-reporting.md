# Step 09 — Analytics, Reporting & AI Performance Dashboard

## Overview

This step adds a complete analytics platform for managers, administrators, and support leads. The system uses pre-aggregated metrics tables to avoid heavy queries on every request, with a background scheduler that runs hourly aggregation and pushes WebSocket events so dashboards refresh automatically.

---

## Architecture

```
Live DB Tables
(tickets, ai_replies, background_jobs, email_messages)
        │
        ▼
  Aggregator (runs on schedule)
        │
        ├─► daily_ticket_metrics  (one row per calendar day)
        ├─► ai_metrics            (one row per calendar day)
        └─► agent_metrics         (one row per agent, updated in-place)
                │
                ▼
         Analytics Service
                │
        ┌───────┼───────┐
        ▼       ▼       ▼
    REST API  WebSocket  Report Files
                         (CSV / Excel / HTML)
```

---

## New Database Tables

### `daily_ticket_metrics`

| Column | Type | Description |
|---|---|---|
| id | uint | primary key |
| date | date | unique; one row per calendar day |
| tickets_created | int | tickets created on this day |
| tickets_closed | int | tickets moved to CLOSED |
| tickets_resolved | int | tickets moved to RESOLVED |
| tickets_reopened | int | tickets reopened |
| average_resolution_time | numeric(10,2) | avg hours from creation to close |
| average_first_response_time | numeric(10,2) | avg hours to first public comment |
| average_ai_processing_time | numeric(10,2) | avg seconds for AI to process |
| created_at | timestamp | |

### `agent_metrics`

| Column | Type | Description |
|---|---|---|
| id | uint | primary key |
| user_id | uint | unique; FK → users |
| tickets_assigned | int | total ever assigned |
| tickets_resolved | int | total resolved / closed |
| average_resolution_time | numeric(10,2) | avg hours |
| average_reply_time | numeric(10,2) | avg hours from ticket creation to first public comment |
| average_customer_rating | numeric(3,2) | reserved for future rating feature |
| last_calculated | timestamp | when metrics were last aggregated |

### `ai_metrics`

| Column | Type | Description |
|---|---|---|
| id | uint | primary key |
| date | date | unique; one row per calendar day |
| analysis_generated | int | tickets with AI analysis completed |
| replies_generated | int | AI reply drafts created |
| average_confidence | numeric(5,2) | mean AI confidence percentage |
| average_generation_time | numeric(10,2) | mean generation time in ms |
| approval_rate | numeric(5,2) | % of replies approved |
| edit_rate | numeric(5,2) | % of approvals with edits |
| rejection_rate | numeric(5,2) | % of replies rejected |
| retry_rate | numeric(5,2) | % of replies regenerated |
| created_at | timestamp | |

### `reports`

| Column | Type | Description |
|---|---|---|
| id | uint | primary key |
| name | varchar(200) | user-provided name |
| report_type | varchar(50) | tickets / agents / ai / email |
| format | varchar(20) | CSV / EXCEL / HTML |
| status | varchar(20) | PENDING / COMPLETED / FAILED |
| file_path | varchar(500) | server filesystem path (never exposed in API) |
| file_size | int64 | bytes |
| filters | text | JSON of applied filters |
| generated_by | uint | FK → users |
| error_message | text | failure reason if FAILED |
| created_at | timestamp | |
| completed_at | timestamp | when generation finished |

---

## Package Structure

```
backend/internal/
├── analytics/
│   ├── repository.go       — all raw DB queries (live + aggregated)
│   ├── aggregator.go       — computes + upserts aggregated metrics
│   ├── service.go          — orchestrates queries, returns DTOs
│   ├── scheduler.go        — periodic aggregation + WebSocket broadcast
│   └── reports/
│       ├── service.go      — report generation, file storage, download
│       └── collector.go    — tabular data builder for each report type
├── models/
│   └── analytics.go        — DailyTicketMetrics, AgentMetrics, AIMetrics, Report
├── dto/
│   └── analytics.go        — all request/response DTOs
└── handlers/
    └── analytics_handler.go — HTTP handler (all endpoints)
```

---

## REST API

All analytics endpoints require authentication. Admin sees full data; SupportAgents see only personal metrics.

### Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/analytics/overview` | All | Live dashboard summary |
| GET | `/api/v1/analytics/tickets` | All | Ticket distributions + daily trend |
| GET | `/api/v1/analytics/agents` | Admin: all agents; Agent: personal only | Agent leaderboard |
| GET | `/api/v1/analytics/ai` | All | AI performance metrics |
| GET | `/api/v1/analytics/queues` | All | Live job queue health |
| GET | `/api/v1/analytics/email` | All | Email volume + delivery stats |
| GET | `/api/v1/analytics/trends` | All | Combined time-series trend data |
| POST | `/api/v1/analytics/aggregate` | Admin only | Trigger immediate aggregation |
| POST | `/api/v1/analytics/reports` | All | Generate a new report |
| GET | `/api/v1/analytics/reports` | All (scoped) | List report history |
| GET | `/api/v1/analytics/reports/:id` | All (scoped) | Get report metadata |
| GET | `/api/v1/analytics/reports/:id/download` | All (scoped) | Download report file |

### Query Filters

All GET endpoints accept:

| Parameter | Values | Description |
|---|---|---|
| `period` | today / yesterday / last7 / last30 / last90 | Pre-defined time window |
| `start_date` | YYYY-MM-DD | Custom window start (period=custom) |
| `end_date` | YYYY-MM-DD | Custom window end |
| `agent_id` | integer | Filter to single agent |
| `priority` | LOW / MEDIUM / HIGH / URGENT | |
| `category` | string | |
| `status` | OPEN / IN_PROGRESS / RESOLVED / CLOSED | |
| `source` | WEB / EMAIL | |

---

## Aggregation Strategy

### What is pre-computed (from Aggregator)
- `daily_ticket_metrics` — per-day counts and averages; never queried live for trends
- `ai_metrics` — per-day AI stats; never queried live for trends
- `agent_metrics` — per-agent totals; updated each aggregation cycle

### What is computed live (always fresh)
- Overview counts (open, urgent, etc.)
- Queue status snapshot
- Email volume for selected window (fast indexed queries)
- Tickets by hour (today only)

### Scheduler
- Runs every `METRICS_REFRESH_INTERVAL` seconds (default: 3600 = hourly)
- On startup: aggregates both today and yesterday immediately
- After each cycle: broadcasts `ANALYTICS_REFRESH` WebSocket event to all connected clients
- Manual trigger via `POST /analytics/aggregate` (admin only)

---

## Real-Time Updates

The `Scheduler` broadcasts a `ANALYTICS_REFRESH` WebSocket event after every aggregation cycle:

```json
{ "type": "ANALYTICS_REFRESH", "at": "2026-07-03T10:00:00Z" }
```

Frontend pages subscribe to this event via the `useWebSocket` hook and call their data loaders. The Queue Monitoring page additionally auto-polls every 15 seconds for live queue health.

---

## Report Export

Reports are generated asynchronously in a goroutine. The `POST /analytics/reports` endpoint returns immediately with a `PENDING` record. The file is written to `REPORT_STORAGE_PATH` on completion.

### Formats

| Format | Extension | MIME type | Notes |
|---|---|---|---|
| CSV | `.csv` | `text/csv` | Standard RFC 4180; headers on first row |
| Excel | `.xlsx` | `application/vnd.openxmlformats...` | Styled header row with `github.com/xuri/excelize/v2` |
| HTML | `.html` | `text/html` | Print-optimized template; use browser print → Save as PDF |

### Cleanup
- `REPORT_RETENTION_DAYS` (default 30): files and DB records older than this are deleted
- Cleanup runs on scheduler startup

---

## Frontend Pages

All pages are accessible via the Dashboard navigation links.

### `/analytics` — Analytics Dashboard
- 8 top stat cards: Total Tickets, Open, Resolved Today, Avg Resolution, AI Confidence, AI Approval Rate, Queued Jobs, Emails Today
- Priority alert bar for urgent / high-priority tickets
- Area chart: ticket volume trend (created / resolved / closed)
- Pie: status distribution
- Horizontal bar: priority distribution
- Horizontal bar: top categories
- Bar: tickets by hour (today)
- Period selector: Today / Yesterday / Last 7 / 30 / 90 days

### `/analytics/ai` — AI Insights
- 8 metric cards: analyses, replies, confidence, generation time, approval %, edit %, rejection %, retry count
- Donut: reply outcome distribution (approved / edited / rejected / retried)
- Line chart: confidence & approval rate trend
- Bar chart: daily AI activity (analyses vs replies)
- Horizontal bar: top AI categories
- Pie: sentiment distribution

### `/analytics/agents` — Agent Performance
- Grouped bar chart: assigned / resolved / active per agent
- Progress bars: resolution leaderboard (top 10)
- Progress bars: fastest resolvers (sorted by avg resolution time)
- Full agent table: assigned, resolved, active, avg resolution, avg reply, last calculated
- Admins see all agents; SupportAgents see only their own row

### `/analytics/queues` — Queue Monitoring
- 6 status badges with live counts (QUEUED, PROCESSING, COMPLETED, FAILED, RETRYING, DEAD)
- Avg queue wait time card
- Alert banner for failed / dead letter jobs
- Donut: status distribution
- Horizontal bar: jobs by type
- Auto-refreshes every 15 seconds + on WebSocket `ANALYTICS_REFRESH` event

### `/analytics/reports` — Reports
- Report generator form: name, type (tickets / agents / ai / email), period, format
- Async generation with PENDING → COMPLETED / FAILED transition
- Auto-polls every 3 seconds while PENDING reports exist
- Download button for completed reports
- Report history table with status badges, file size, timestamps

---

## Configuration

```env
# Analytics
REPORT_RETENTION_DAYS=30       # days to keep report files on disk
METRICS_REFRESH_INTERVAL=3600  # seconds between aggregation cycles (1 hour)
REPORT_STORAGE_PATH=./storage/reports
```

---

## Dependencies Added

```
github.com/xuri/excelize/v2 v2.10.1   — Excel report generation
recharts (npm)                          — React charting library
```

---

## Access Control

| Role | Access |
|---|---|
| Admin | All endpoints, all agents, all reports |
| SupportAgent | Overview, tickets, AI stats (read), personal agent metrics only, own reports only |
