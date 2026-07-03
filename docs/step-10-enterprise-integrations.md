# Step 10 — Enterprise Integrations

## Overview

Step 10 adds a full **Enterprise Integrations** module that connects SupportIQ to ten external platforms. Every integration follows a provider interface pattern, stores credentials encrypted, and delivers events reliably via a durable event queue with retry logic.

---

## Architecture

```
TicketActivity rows  ─→  Integration Worker (poll every 30s)
                              │
                              ├─ Map activity type → event type
                              ├─ Filter by provider.SupportedEvents()
                              └─ Create IntegrationEvent (PENDING)

Integration Worker (event processor, every 30s)
    ├─ Fetch PENDING events (batch 50)
    ├─ Decrypt provider config (AES-256-GCM)
    ├─ Build provider via Registry
    ├─ provider.Notify(ctx, Event)
    │   ├─ SUCCESS → mark PROCESSED
    │   └─ FAIL    → increment retry_count
    │               └─ retry_count ≥ 5 → mark DEAD
    └─ TicketIntegration row saved for issue-tracker links
```

---

## Supported Providers

| Provider    | Type           | Key Features |
|-------------|---------------|--------------|
| Slack       | Notification   | Rich block messages with colour-coded priority |
| Teams       | Notification   | O365 MessageCard with facts and deep-link action |
| Discord     | Notification   | Embed with colour, fields, and timestamp |
| Jira        | Issue Tracker  | Creates issues via REST API v3, ADF description |
| Linear      | Issue Tracker  | Creates issues via GraphQL mutation |
| GitHub      | Issue Tracker  | Creates issues via REST API v3 with labels |
| Webhook     | Generic        | HMAC-SHA256 signed POST, configurable headers |
| Salesforce  | CRM            | Creates Cases and Contacts via REST API |
| HubSpot     | CRM            | Creates Tickets and Contacts via CRM API v3 |
| Google Cal  | Calendar       | Creates Meet-enabled calendar events |

---

## Event Type Mapping

| TicketActivity constant        | Integration event |
|-------------------------------|------------------|
| `ActivityCreateTicket`         | `ticket.created` |
| `ActivityStatusChanged`        | `ticket.status_changed` (or `ticket.closed` if status=CLOSED) |
| `ActivityAssignTicket`         | `ticket.assigned` |
| `ActivityAIAnalysisCompleted`  | `ai.analysis_complete` |
| `ActivityReplyApproved`        | `reply.approved` |
| `ActivityEmailFailed`          | `email.failed` |

---

## New Files

### Backend

| File | Purpose |
|------|---------|
| `internal/models/integration.go` | `Integration`, `IntegrationEvent`, `TicketIntegration` models |
| `internal/dto/integration.go` | Request/response DTOs |
| `internal/integrations/provider/interface.go` | `Provider`, `IssueProvider`, `CRMProvider`, `CalendarProvider` interfaces + event constants |
| `internal/integrations/providers/slack/provider.go` | Slack webhook — rich block messages |
| `internal/integrations/providers/teams/provider.go` | Teams webhook — MessageCard |
| `internal/integrations/providers/discord/provider.go` | Discord webhook — embeds |
| `internal/integrations/providers/jira/provider.go` | Jira Cloud REST v3 |
| `internal/integrations/providers/linear/provider.go` | Linear GraphQL API |
| `internal/integrations/providers/github/provider.go` | GitHub REST v3 |
| `internal/integrations/providers/webhook/provider.go` | Generic HMAC-signed webhook |
| `internal/integrations/providers/salesforce/provider.go` | Salesforce REST API |
| `internal/integrations/providers/hubspot/provider.go` | HubSpot CRM API v3 |
| `internal/integrations/providers/gcal/provider.go` | Google Calendar API v3 |
| `internal/integrations/registry.go` | Provider factory |
| `internal/integrations/worker.go` | Background event poller + dispatcher |
| `internal/repositories/integration_repository.go` | Database layer for all integration models |
| `internal/services/integration_service.go` | Business logic, CRUD, issue creation, config encryption |
| `internal/handlers/integration_handler.go` | HTTP handlers |

### Frontend

| File | Purpose |
|------|---------|
| `frontend/src/services/integrationService.js` | API client for all integration endpoints |
| `frontend/src/pages/Integrations.jsx` | Admin management UI with CRUD, test, and status badges |

---

## Modified Files

| File | Change |
|------|--------|
| `internal/models/background_job.go` | Added 5 integration job types |
| `internal/config/config.go` | Added `IntegrationPollInterval`, `WebhookSecret` |
| `internal/database/database.go` | Added 3 integration tables to AutoMigrate |
| `internal/routes/routes.go` | Wired integration service, handler, worker, and all routes |
| `backend/.env` | Added `INTEGRATION_POLL_INTERVAL`, `WEBHOOK_SECRET` |
| `frontend/src/routes/index.jsx` | Added `/integrations` route |
| `frontend/src/pages/Dashboard.jsx` | Added Integrations nav link (admin only) |

---

## API Endpoints

### Integration Management (Admin only)

```
GET    /api/v1/integrations              List all integrations
POST   /api/v1/integrations              Create integration
PUT    /api/v1/integrations/:id          Update integration
DELETE /api/v1/integrations/:id          Delete integration
POST   /api/v1/integrations/:id/test     Test connection
GET    /api/v1/integrations/:id/events   Event delivery log
```

### Ticket Integrations (All authenticated users)

```
GET  /api/v1/tickets/:id/integrations         List external issue links
POST /api/v1/tickets/:id/create-jira          Create Jira issue
POST /api/v1/tickets/:id/create-linear        Create Linear issue
POST /api/v1/tickets/:id/create-github-issue  Create GitHub issue
```

---

## Security Notes

- **Credentials are never stored in plaintext.** All integration configs are encrypted with AES-256-GCM before DB persistence and decrypted only at dispatch time.
- **Webhook signing** uses HMAC-SHA256 (`X-SupportIQ-Signature: sha256=...`).
- **Delivery idempotency** via `X-SupportIQ-Delivery` UUID header.
- Integration configuration fields are never included in API responses.

---

## Configuration (`.env`)

```env
INTEGRATION_POLL_INTERVAL=30   # seconds between event polls
WEBHOOK_SECRET=                # default signing secret for outgoing webhooks
```

Individual integration credentials are stored per-integration in the database (encrypted) and are not required in `.env`.

---

## Database Tables

| Table | Description |
|-------|-------------|
| `integrations` | Configured provider connections with encrypted config |
| `integration_events` | Durable event queue with retry tracking |
| `ticket_integrations` | Links between tickets and external issues |
