# Step 07 — Workflow Automation & Background Processing

## Overview

This step replaces the synchronous goroutine-based AI analysis with a **Redis-backed job queue**, adds a **WebSocket notification hub** so the browser receives real-time updates when worker jobs complete, and provides an **admin Job Monitor** page for observability.

---

## Architecture

```
Browser  ──WS──►  API Server  ──enqueue──►  Redis  ◄──dequeue──  Worker binary
           ◄──pub/sub events──              (lists, sorted sets)
```

| Component | Location |
|---|---|
| Queue interface | `backend/internal/queue/queue.go` |
| Redis queue client | `backend/internal/queue/redisqueue/client.go` |
| WebSocket hub | `backend/internal/websocket/hub.go` |
| Worker processor | `backend/worker/processor/processor.go` |
| Worker entry-point | `backend/cmd/worker/main.go` |
| Event constants | `backend/internal/events/events.go` |
| Background job model | `backend/internal/models/background_job.go` |
| Job repository | `backend/internal/repositories/job_repository.go` |
| Job service | `backend/internal/services/job_service.go` |
| Job HTTP handler | `backend/internal/handlers/job_handler.go` |

---

## Backend Changes

### New packages

**`internal/queue`** — `Queue` interface and `Job` struct used by both the API and worker.

**`internal/queue/redisqueue`** — Redis implementation:
- Main queue: `LPUSH` / `BRPOP` on `queue:<name>`
- Retry queue: sorted set `queue:<name>:retry` scored by execution timestamp
- Dead-letter queue: `LPUSH` / `LRANGE` on `queue:<name>:dead`
- `MoveDueRetryJobs` — promoted by a 2-second ticker inside the processor
- `PublishEvent` / `Subscribe` — Redis pub/sub on channel `events:notifications`

**`internal/events`** — typed event constants: `ticket.ai.completed`, `ticket.reply.generated`, `ticket.reply.failed`, `ticket.updated`, `job.completed`, `job.failed`.

**`internal/websocket`** — gorilla/websocket hub:
- `Hub.Run()` — fan-out broadcast goroutine
- `Hub.ServeWS(w, r, userID)` — upgrades HTTP to WS, starts client goroutines
- `Hub.Broadcast(payload)` — sends JSON to all connected clients
- Ping/pong keepalive every 30 s

**`worker/processor`** — `Processor` with N worker goroutines:
- Exponential backoff retries: base × 2^(attempt-1)
- Max retries configurable via `RETRY_DELAY` / `MAX_RETRIES`
- Publishes Redis pub/sub event after every job completion/failure

**`worker/handlers`** — `AIAnalysisHandler`, `GenerateReplyHandler`, `RegenerateReplyHandler`

**`cmd/worker/main.go`** — standalone worker binary, graceful shutdown via SIGTERM/SIGINT

### Modified files

| File | Change |
|---|---|
| `config/config.go` | Added `RedisURL`, `WorkerCount`, `QueueName`, `MaxRetries`, `RetryDelay`, `WebSocketOrigin` |
| `database/database.go` | Added `BackgroundJob` to `AutoMigrate` |
| `routes/routes.go` | Init hub + optional Redis queue; WebSocket endpoint `GET /api/v1/ws?token=`; admin `/jobs` routes; pub/sub→WS bridge goroutine |
| `services/ticket_service.go` | Added `jobSvc` field + `SetJobService()`; `Create`/`Update` use queue when available, fall back to goroutine |

### New HTTP endpoints (admin only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/ws?token=<jwt>` | WebSocket upgrade (any authenticated user) |
| GET | `/api/v1/jobs` | List jobs with pagination |
| GET | `/api/v1/jobs/statistics` | Counts by status |
| GET | `/api/v1/jobs/:id` | Single job detail |
| POST | `/api/v1/jobs/:id/retry` | Manually retry a failed/dead job |

---

## Frontend Changes

### New files

| File | Purpose |
|---|---|
| `src/services/websocketService.js` | Singleton WS manager with auto-reconnect (exponential backoff) |
| `src/services/jobService.js` | API client for `/jobs` endpoints |
| `src/contexts/WebSocketContext.jsx` | React context + `useWebSocket()` hook |
| `src/components/RealtimeToast.jsx` | Bottom-right toast panel for WS events |
| `src/pages/JobMonitor.jsx` | Admin table with stats cards, filters, retry, pagination, auto-refresh every 10 s |

### Modified files

| File | Change |
|---|---|
| `src/App.jsx` | Wrapped with `<WebSocketProvider>` + `<RealtimeToast>` |
| `src/routes/index.jsx` | Added `/jobs` route |
| `src/pages/Dashboard.jsx` | Added "Job Monitor" nav link (admin only) |
| `src/pages/tickets/TicketDetails.jsx` | Subscribes to `ticket.ai.completed` / `ticket.reply.generated` / `ticket.updated` — auto-reloads when event matches current ticket ID |

---

## Configuration (`.env`)

```env
# Queue / Worker
REDIS_URL=redis://localhost:6379
WORKER_COUNT=3
QUEUE_NAME=ai_jobs
MAX_RETRIES=3
RETRY_DELAY=5
WEBSOCKET_ORIGIN=http://localhost:5173
```

`REDIS_URL` is optional. When absent the system falls back silently to the existing goroutine-based analysis — no features break.

---

## Running the Worker

```bash
# In a second terminal, from backend/
export PATH=$PATH:/opt/homebrew/bin
go run ./cmd/worker/main.go
```

The API server and worker are independent processes. Both read from the same Redis instance.

---

## Job Status Flow

```
QUEUED → PROCESSING → COMPLETED
                    ↘ FAILED → RETRYING → COMPLETED
                                        ↘ DEAD
```

---

## Real-time Notification Flow

1. Worker completes job → calls `redisQ.PublishEvent(ctx, event)` → publishes JSON to `events:notifications`
2. API server's `subscribeToWorkerEvents` goroutine receives message → calls `hub.Broadcast(payload)`
3. Hub fans out to all connected WebSocket clients
4. `RealtimeToast` shows a dismissible notification
5. `TicketDetails` auto-refreshes AI Analysis / AI Reply tabs if the event matches the open ticket

---

## Dependencies Added

```
github.com/redis/go-redis/v9 v9.21.0
github.com/gorilla/websocket v1.5.3
```
