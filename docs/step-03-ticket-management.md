# AI Support Assistant — Step 3: Ticket Management

**Date:** 2026-07-03
**Builds on:** Step 2 — Authentication & User Management

---

## Success Criteria Met

| Criterion | Status |
|-----------|--------|
| Create tickets | ✅ |
| View tickets (list + single) | ✅ |
| Search tickets | ✅ |
| Filter tickets (status, priority, category, assigned_to, created_by) | ✅ |
| Update tickets | ✅ |
| Assign tickets (Admin only) | ✅ |
| Change status (enforced transitions) | ✅ |
| Soft delete (Admin only) | ✅ |
| Auto-generated ticket numbers (TKT-000001) | ✅ |
| UUID exposed in APIs, no integer IDs | ✅ |
| Pagination with total count / pages | ✅ |

---

## Database Design

### `tickets` table

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | Primary key, auto-generated via `BeforeCreate` hook |
| `ticket_number` | VARCHAR(20) | Unique, auto-generated `TKT-XXXXXX` |
| `subject` | VARCHAR(150) | NOT NULL |
| `description` | TEXT | NOT NULL |
| `status` | VARCHAR(20) | `OPEN` / `IN_PROGRESS` / `RESOLVED` / `CLOSED`, default `OPEN` |
| `priority` | VARCHAR(20) | `LOW` / `MEDIUM` / `HIGH` / `URGENT`, default `MEDIUM` |
| `category` | VARCHAR(50) | `GENERAL`, default `GENERAL` |
| `source` | VARCHAR(20) | `WEB`, default `WEB` |
| `assigned_to` | UINT (nullable FK) | → `users.id` |
| `created_by` | UINT (FK) | → `users.id`, set from JWT |
| `customer_name` | VARCHAR(100) | NOT NULL |
| `customer_email` | VARCHAR(255) | NOT NULL |
| `created_at` | TIMESTAMP | Managed by GORM |
| `updated_at` | TIMESTAMP | Managed by GORM |
| `deleted_at` | TIMESTAMP (nullable) | GORM soft delete — deleted rows hidden from all queries |

### `ticket_counters` table

A single row (`id = 1`) holding the last used sequence value.
Used by the repository with a `SELECT … FOR UPDATE` row-level lock to generate sequential ticket numbers safely under concurrent writes.

---

## New Backend Files

### `internal/models/ticket.go`

- Defines `TicketStatus`, `TicketPriority`, `TicketCategory`, `TicketSource` enum types
- `IsValidStatusTransition(from, to)` — enforces the linear workflow: `OPEN → IN_PROGRESS → RESOLVED → CLOSED`
- `FormatTicketNumber(n)` — formats `int64` as `TKT-000001`
- `Ticket` model with UUID primary key, soft delete, and preloaded `Creator` / `Assignee` associations
- `TicketCounter` model — single-row sequence table

### `internal/dto/ticket.go`

| DTO | Purpose |
|-----|---------|
| `CreateTicketRequest` | `subject`, `description`, `customerName`, `customerEmail` |
| `UpdateTicketRequest` | All editable fields (omitempty — partial update) |
| `UpdateStatusRequest` | `status` with enum validation |
| `AssignTicketRequest` | `assignedTo` user ID |
| `ListTicketsQuery` | `page`, `limit`, `search`, `status`, `priority`, `category`, `assigned_to`, `created_by` |
| `TicketResponse` | Safe public representation (UUID, no raw DB types) |
| `ListTicketsResponse` | `items`, `total_count`, `current_page`, `total_pages`, `limit` |

### `internal/repositories/ticket_repository.go`

Pure database access — zero business logic.

| Method | Description |
|--------|-------------|
| `NextTicketNumber(tx)` | `SELECT FOR UPDATE` on counter row, increments atomically |
| `Create(tx, ticket)` | Inserts new ticket inside caller's transaction |
| `FindByID(id)` | Preloads `Creator` and `Assignee` |
| `Update(ticket)` | `SAVE` all fields |
| `SoftDelete(id)` | GORM `Delete` — sets `deleted_at`, hides from future queries |
| `List(query)` | Full-text search across 4 columns, all filters, `ORDER BY created_at DESC`, paginated |

### `internal/repositories/user_repository.go`

| Method | Description |
|--------|-------------|
| `FindByID(id)` | Used by ticket service to validate assignee |
| `ListByRole(role)` | Returns active users with given role (used for agent list) |

### `internal/services/ticket_service.go`

All business logic. Controllers call service; service calls repository.

| Method | Business Rules |
|--------|---------------|
| `Create` | Wraps counter increment + insert in one transaction; sets default status/priority/category/source; `created_by` from JWT |
| `List` | Normalises page/limit defaults (page ≥ 1, limit 1–100); calculates `total_pages` |
| `GetByID` | 404 on missing ticket |
| `Update` | Only updates non-empty fields (omitempty patch behaviour) |
| `UpdateStatus` | Calls `IsValidStatusTransition`; rejects invalid moves with 400 |
| `Assign` | Checks caller is Admin; validates assignee exists and is SupportAgent |
| `Delete` | Checks caller is Admin; soft-deletes only |

Sentinel errors (`ErrTicketNotFound`, `ErrInvalidTransition`, `ErrForbidden`, `ErrAssigneeNotFound`, `ErrAssigneeNotAgent`) allow handlers to map them to correct HTTP status codes.

### `internal/handlers/ticket.go`

Thin HTTP layer for all 7 ticket endpoints. Each handler:
1. Parses UUID from URL param (400 on invalid)
2. Binds and validates JSON body
3. Delegates to service
4. Writes consistent JSON response

### `internal/handlers/user.go`

`GET /api/v1/users/agents` — returns all active `SupportAgent` users. Used by the frontend assignment dropdown.

---

## Modified Backend Files

### `internal/database/database.go`

Added `Ticket` and `TicketCounter` to `AutoMigrate`. Seeds the counter row (`id=1, last_value=0`) on startup via `FirstOrCreate`.

### `internal/routes/routes.go`

All ticket and user routes are mounted under a shared `middleware.Authenticate` group so no route is publicly accessible.

```
POST   /api/v1/tickets             Create ticket
GET    /api/v1/tickets             List tickets (search, filter, paginate)
GET    /api/v1/tickets/:id         Single ticket
PUT    /api/v1/tickets/:id         Update ticket
PATCH  /api/v1/tickets/:id/status  Update status
PATCH  /api/v1/tickets/:id/assign  Assign ticket (Admin check in service)
DELETE /api/v1/tickets/:id         Soft delete (Admin check in service)
GET    /api/v1/users/agents        List SupportAgent users
```

---

## New Frontend Files

### `src/services/ticketService.js`

Eight reusable methods wrapping the ticket API: `createTicket`, `getTickets`, `getTicket`, `updateTicket`, `updateStatus`, `assignTicket`, `deleteTicket`, `getAgents`.

### `src/components/tickets/StatusBadge.jsx`

Color-coded badge: blue (OPEN), amber (IN_PROGRESS), green (RESOLVED), gray (CLOSED).

### `src/components/tickets/PriorityBadge.jsx`

Color-coded badge: gray (LOW), blue (MEDIUM), orange (HIGH), red (URGENT).

### `src/components/Toast.jsx`

`useToast()` hook (returns `{ toast, showToast }`) + `<Toast toast={...} />` component. Dismisses automatically after 3.5 seconds.

### `src/utils/format.js`

`formatDate(dateStr)` — formats ISO timestamps to human-readable dates.

### `src/pages/tickets/TicketList.jsx`

| Feature | Implementation |
|---------|---------------|
| Table | Ticket#, Subject, Customer, Priority, Status, Assigned To, Created |
| Search | Debounced form submit — searches subject, description, ticket_number, customer_name |
| Filters | Status dropdown, Priority dropdown |
| Clear | Resets all filters and search |
| Refresh | Re-fetches current query |
| Pagination | Prev / Next with page X of Y display |
| Clickable rows | Navigates to `/tickets/:id` |
| Empty state | "No tickets found" message |
| Loading state | Pulsing text while fetching |

### `src/pages/tickets/CreateTicket.jsx`

- Fields: Subject, Description, Customer Name, Customer Email
- Client-side validation (min length, email format)
- Backend error displayed inline
- Success toast → auto-navigates to the new ticket's detail page

### `src/pages/tickets/TicketDetails.jsx`

- Displays all ticket fields in a 2-column layout (content + sidebar)
- **Status progression**: shows the next valid action button only (`Start Progress` / `Mark Resolved` / `Close Ticket`); CLOSED state shows no button
- **Edit**: navigates to edit page
- **Assign** (Admin only): agent dropdown populated from `GET /api/v1/users/agents`
- **Delete** (Admin only): confirms via browser dialog, then soft-deletes and navigates back

### `src/pages/tickets/EditTicket.jsx`

- Pre-fills all editable fields from `GET /api/v1/tickets/:id`
- Updates via `PUT /api/v1/tickets/:id`
- Success toast → auto-navigates back to detail page

---

## Modified Frontend Files

### `src/pages/Dashboard.jsx`

Added ticket overview stats section. Makes 5 parallel requests on mount (`Promise.all`) to calculate:

| Stat | Query |
|------|-------|
| Total Tickets | `GET /api/v1/tickets?limit=1` → `total_count` |
| Open | `…&status=OPEN` |
| In Progress | `…&status=IN_PROGRESS` |
| Resolved | `…&status=RESOLVED` |
| Closed | `…&status=CLOSED` |

Also added a **View Tickets** link in the header.

### `src/routes/index.jsx`

Added four protected ticket routes:

```
/tickets          → TicketList
/tickets/new      → CreateTicket
/tickets/:id      → TicketDetails
/tickets/:id/edit → EditTicket
```

---

## API Reference

### `POST /api/v1/tickets`
```json
// Request
{ "subject": "Payment deducted", "description": "Money deducted but recharge failed.", "customerName": "Rahul", "customerEmail": "rahul@gmail.com" }

// Response 201
{ "status": "success", "message": "Ticket created successfully", "data": { "id": "uuid", "ticket_number": "TKT-000001", ... } }
```

### `GET /api/v1/tickets`
```
?page=1&limit=20&search=payment&status=OPEN&priority=HIGH
```
```json
{ "status": "success", "data": { "items": [...], "total_count": 42, "current_page": 1, "total_pages": 3, "limit": 20 } }
```

### `PATCH /api/v1/tickets/:id/status`
```json
{ "status": "IN_PROGRESS" }
// 400 if transition is invalid (e.g. CLOSED → OPEN)
```

### `PATCH /api/v1/tickets/:id/assign`
```json
{ "assignedTo": 5 }
// 403 if caller is not Admin
// 400 if user is not SupportAgent
```

---

## Security

- All ticket and user routes require a valid JWT (`Authenticate` middleware)
- Assign and Delete operations are gated inside the service layer by checking `userRole == Admin`
- `created_by` is always taken from the JWT context — never accepted from the request body
- `ticket_number` is always server-generated — never accepted from the request body
- Soft delete preserves audit history; rows are never permanently removed

---

## What is intentionally NOT in Step 3

- AI classification / auto-reply
- Email notifications on ticket creation / update
- Slack integration
- Ticket comments / activity feed
- File attachments
- SLA tracking
- Analytics / reporting dashboard
- Redis / background workers
- Docker
