# AI Support Assistant — Step 4: Support Agent Workspace & Ticket Lifecycle

**Date:** 2026-07-03
**Builds on:** Step 3 — Ticket Management CRUD

---

## Success Criteria Met

| Criterion | Status |
|-----------|--------|
| Internal notes (staff-only) | ✅ |
| Ticket comments / conversation history | ✅ |
| Activity timeline for every ticket | ✅ |
| Auto-log every important action | ✅ |
| Take ownership of unassigned tickets | ✅ |
| My Tickets view (assigned to me) | ✅ |
| Unassigned Tickets view | ✅ |
| Updated Dashboard with all stats | ✅ |
| Tabbed Ticket Details page | ✅ |

---

## Database Design

### `ticket_notes`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UINT (PK) | Auto-increment |
| `ticket_id` | UUID (FK) | → `tickets.id`, indexed |
| `user_id` | UINT (FK) | → `users.id` |
| `note` | TEXT | NOT NULL |
| `is_internal` | BOOL | Always `true` — customers never see notes |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | |

### `ticket_activities`

Immutable audit log. Rows are never edited after creation.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UINT (PK) | Auto-increment |
| `ticket_id` | UUID (FK) | → `tickets.id`, indexed |
| `user_id` | UINT (FK) | → `users.id` |
| `activity_type` | VARCHAR(50) | See activity types below |
| `old_value` | VARCHAR(255) | Previous value (for changes) |
| `new_value` | VARCHAR(255) | New value (for changes) |
| `description` | TEXT | Human-readable summary |
| `created_at` | TIMESTAMP | |

### `ticket_comments`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UINT (PK) | Auto-increment |
| `ticket_id` | UUID (FK) | → `tickets.id`, indexed |
| `user_id` | UINT (FK) | → `users.id` |
| `message` | TEXT | NOT NULL |
| `comment_type` | VARCHAR(20) | `PUBLIC` / `INTERNAL`, default `PUBLIC` |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | |

---

## Activity Types

| Constant | Value | Triggered By |
|----------|-------|-------------|
| `ActivityCreateTicket` | `CREATE_TICKET` | `TicketService.Create` |
| `ActivityUpdateTicket` | `UPDATE_TICKET` | `TicketService.Update` (subject / description / customer fields) |
| `ActivityAssignTicket` | `ASSIGN_TICKET` | `TicketService.Assign` (Admin assigns to agent) |
| `ActivityTakeOwnership` | `TAKE_OWNERSHIP` | `TicketService.TakeOwnership` (agent self-assigns) |
| `ActivityStatusChanged` | `STATUS_CHANGED` | `TicketService.UpdateStatus` |
| `ActivityPriorityChanged` | `PRIORITY_CHANGED` | `TicketService.Update` when priority differs |
| `ActivityCategoryChanged` | `CATEGORY_CHANGED` | `TicketService.Update` when category differs |
| `ActivityCommentAdded` | `COMMENT_ADDED` | `CommentService.Create` |
| `ActivityInternalNoteAdded` | `INTERNAL_NOTE_ADDED` | `NoteService.Create` |
| `ActivityTicketClosed` | `TICKET_CLOSED` | `TicketService.UpdateStatus` when transitioning to `CLOSED` |
| `ActivityTicketReopened` | `TICKET_REOPENED` | Reserved for future use |

Activity logging is **fire-and-forget** — errors are silently suppressed so a logging failure never fails the main operation.

---

## New Backend Files

### `internal/models/ticket_note.go`
`TicketNote` model. `IsInternal` is always `true` — the field exists for forward compatibility if public notes are added later.

### `internal/models/ticket_activity.go`
`TicketActivity` model + all 11 activity type string constants used across services.

### `internal/models/ticket_comment.go`
`TicketComment` model with `CommentType` enum (`PUBLIC` / `INTERNAL`).

### `internal/dto/note.go`
`CreateNoteRequest{note}`, `NoteResponse`.

### `internal/dto/activity.go`
`ActivityResponse` — read-only, never has a request counterpart.

### `internal/dto/comment.go`
`CreateCommentRequest{message, commentType}`, `CommentResponse`.

### `internal/repositories/note_repository.go`

| Method | Description |
|--------|-------------|
| `Create(note)` | Inserts note |
| `FindByID(id)` | Single note with User preloaded |
| `ListByTicketID(ticketID)` | Newest first |

### `internal/repositories/activity_repository.go`

| Method | Description |
|--------|-------------|
| `Create(activity)` | Inserts activity |
| `ListByTicketID(ticketID)` | Chronological (ASC) — for timeline display |
| `ListRecent(limit)` | Newest first — for dashboard feed |

### `internal/repositories/comment_repository.go`

| Method | Description |
|--------|-------------|
| `Create(comment)` | Inserts comment |
| `FindByID(id)` | Single comment with User preloaded |
| `ListByTicketID(ticketID)` | Chronological (ASC) |

### `internal/services/note_service.go`
`Create` — inserts note, reloads with user, logs `INTERNAL_NOTE_ADDED` activity.
`List` — returns all notes for a ticket.

### `internal/services/comment_service.go`
`Create` — inserts comment, reloads with user, logs `COMMENT_ADDED` activity.
`List` — returns all comments for a ticket.

### `internal/handlers/note_handler.go`
`POST /api/v1/tickets/:id/notes` → `Create`
`GET  /api/v1/tickets/:id/notes` → `List`

### `internal/handlers/activity_handler.go`
`GET /api/v1/tickets/:id/activity` → `ListByTicket` (chronological)
`GET /api/v1/activities`            → `ListRecent` (last 20, for dashboard)

### `internal/handlers/comment_handler.go`
`POST /api/v1/tickets/:id/comments` → `Create`
`GET  /api/v1/tickets/:id/comments` → `List`

---

## Modified Backend Files

### `internal/models/ticket_activity.go`
Added all 11 activity type constants (consumed by multiple services without circular imports).

### `internal/dto/ticket.go`
Added `UnassignedOnly bool` to `ListTicketsQuery` — when `true`, the repository adds `WHERE assigned_to IS NULL` instead of filtering by a specific user ID.

### `internal/repositories/ticket_repository.go`
Updated `List` to handle the new `UnassignedOnly` flag:
```go
if q.UnassignedOnly {
    base = base.Where("assigned_to IS NULL")
} else if q.AssignedTo != nil {
    base = base.Where("assigned_to = ?", *q.AssignedTo)
}
```

### `internal/services/ticket_service.go`

**Struct changes:**
- Added `activityRepo *repositories.ActivityRepository`
- `NewTicketService` now requires three arguments

**New private helper:**
```go
func (s *TicketService) logActivity(ticketID, userID, actType, oldVal, newVal, desc string)
```

**Updated methods** (added `userID uint` parameter and activity logging):
- `Update` — logs `PRIORITY_CHANGED`, `CATEGORY_CHANGED`, `UPDATE_TICKET`
- `UpdateStatus` — logs `STATUS_CHANGED` or `TICKET_CLOSED`
- `Assign` — logs `ASSIGN_TICKET` with assignee name

**New methods:**
- `TakeOwnership(id, userID, userRole)` — SupportAgent only; 409 if already assigned
- `MyTickets(userID, query)` — wrapper around `List` with `AssignedTo` preset
- `ListUnassigned(query)` — wrapper around `List` with `UnassignedOnly` preset

**New sentinel error:**
- `ErrAlreadyAssigned` — returned as HTTP 409 when ticket is already assigned

### `internal/handlers/ticket.go`
Updated `Update`, `UpdateStatus`, `Assign` to extract `userID` from context and forward it to the service.

Added three new handlers:
- `TakeOwnership` — `PATCH /api/v1/tickets/:id/take-ownership`
- `MyTickets` — `GET /api/v1/my-tickets`
- `ListUnassigned` — `GET /api/v1/tickets/unassigned`

### `internal/database/database.go`
Added `TicketNote`, `TicketActivity`, `TicketComment` to `AutoMigrate`.

### `internal/routes/routes.go`
Wired all new repositories, services, and handlers. New routes:

```
GET    /api/v1/my-tickets
GET    /api/v1/activities
GET    /api/v1/tickets/unassigned          ← registered before /:id to avoid conflicts
PATCH  /api/v1/tickets/:id/take-ownership
POST   /api/v1/tickets/:id/notes
GET    /api/v1/tickets/:id/notes
POST   /api/v1/tickets/:id/comments
GET    /api/v1/tickets/:id/comments
GET    /api/v1/tickets/:id/activity
```

---

## New Frontend Files

### `src/services/noteService.js`
```js
noteService.create(ticketId, { note })
noteService.list(ticketId)
```

### `src/services/commentService.js`
```js
commentService.create(ticketId, { message, commentType? })
commentService.list(ticketId)
```

### `src/services/activityService.js`
```js
activityService.listByTicket(ticketId)   // per-ticket timeline
activityService.getRecent()              // global feed (last 20)
```

### `src/components/tickets/ActivityTimeline.jsx`

Vertical timeline with:
- Emoji icon per activity type
- Color-coded dot per type
- Old value → new value diff chips (red / green)
- Actor name + formatted date

### `src/components/tickets/NotesPanel.jsx`

- Yellow card list (newest first)
- Textarea + "Add Note" button
- Inline error display
- Reloads list after successful submit

### `src/components/tickets/ConversationPanel.jsx`

- Chronological comment thread
- Textarea + "Send" button
- Reloads after successful submit

### `src/pages/tickets/MyTickets.jsx`

Full-featured table page:
- Ticket#, Subject, Customer, Priority, Status, Updated
- Search (submit on enter), status filter, priority filter, clear button
- Pagination (prev/next)
- Clickable rows → detail page
- Fetches via `GET /api/v1/my-tickets`

### `src/pages/tickets/UnassignedTickets.jsx`

- Table: Ticket#, Subject, Customer, Priority, Created, Action
- Per-row **Take Ownership** button
- Success/error message banner
- Refresh button
- Click row to view detail; "Take Ownership" click is isolated (stopPropagation)

---

## Modified Frontend Files

### `src/services/ticketService.js`
Added:
```js
ticketService.takeOwnership(id)        // PATCH /:id/take-ownership
ticketService.getMyTickets(params)     // GET /my-tickets
ticketService.getUnassigned(params)    // GET /tickets/unassigned
```

### `src/pages/tickets/TicketDetails.jsx`

Restructured into a **4-tab interface**:

| Tab | Content |
|-----|---------|
| **Overview** | Description, customer info, sidebar with details + admin assign dropdown |
| **Conversation** | `<ConversationPanel>` |
| **Notes** | `<NotesPanel>` (internal notes, staff only) |
| **Activity** | `<ActivityTimeline>` |

The ticket header (breadcrumb, status badge, Edit / status-progression / Delete buttons) remains persistent across all tabs.

### `src/pages/Dashboard.jsx`

**7 stat cards** (all clickable where a filtered view exists):

| Card | Link |
|------|------|
| Total Tickets | `/tickets` |
| My Tickets | `/my-tickets` |
| Open | — |
| Unassigned | `/tickets/unassigned` |
| In Progress | — |
| Resolved | — |
| Closed | — |

**Two new sections:**
- **Recent Activity** — last 10 global activities with icon, description, actor, date
- **My Recent Tickets** — last 5 tickets assigned to the logged-in user, linked to detail page

Header now includes quick-nav links: Unassigned · My Tickets · All Tickets · Logout.

### `src/routes/index.jsx`
Added two protected routes:
```
/my-tickets           → MyTickets
/tickets/unassigned   → UnassignedTickets
```
Both registered before `/tickets/:id` so React Router matches them correctly.

---

## Security

| Rule | Enforcement |
|------|-------------|
| All endpoints require JWT | `Authenticate` middleware on every protected group |
| Take Ownership → SupportAgent only | Checked in `TicketService.TakeOwnership` |
| Internal notes → authenticated staff | All note endpoints are under the protected group |
| Activity logs → read-only | No update/delete handler exists; `TicketActivity` has no `UpdatedAt` |
| `created_by` always from JWT | Never accepted from request body |

---

## What is intentionally NOT in Step 4

- AI classification or auto-reply
- Email / Slack notifications
- File attachments on notes or comments
- Customer-facing portal (comments are staff-side only)
- SLA timers
- Bulk ticket actions
- Real-time updates (WebSocket / SSE)
- Redis or background workers
- Docker
