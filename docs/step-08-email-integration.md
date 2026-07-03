# Step 08 — Email Integration

## Overview

This step transforms the application into an omnichannel support platform by adding a full Email Integration module. Customer emails are automatically converted into tickets, existing conversations are threaded correctly, and approved AI replies are sent back through email.

---

## Architecture

```
Customer Email
      │
      ▼
  IMAP Server
      │
      ▼
InboundWorker (polls every EMAIL_POLL_INTERVAL seconds)
      │
      ├─ Thread Detection (In-Reply-To → References → Thread-ID → Subject)
      │
      ├─ Existing Thread? ──Yes──► Append EmailMessage to existing Ticket
      │
      └─ New Thread? ────────────► Create new Ticket (Source=EMAIL)
                                         │
                                         ▼
                                   Queue AI Analysis
                                         │
                                         ▼
                                   Generate AI Reply
                                         │
                                         ▼
                                   Agent Approves
                                         │
                                         ▼
                              Queue Outbound EmailMessage
                                         │
                                         ▼
                              OutboundWorker → SMTP Send
                                         │
                                         ▼
                              Update Status + Activity Log
```

---

## New Database Tables

### `email_accounts`

| Column | Type | Notes |
|---|---|---|
| id | uint | primary key |
| provider | varchar(50) | SMTP_IMAP / GMAIL / OUTLOOK / SENDGRID / SES / MAILGUN |
| email_address | varchar(255) | unique |
| display_name | varchar(100) | shown in From header |
| imap_host | varchar(255) | |
| imap_port | int | default 993 |
| imap_use_tls | bool | default true |
| smtp_host | varchar(255) | |
| smtp_port | int | default 587 |
| smtp_implicit_tls | bool | false = STARTTLS, true = port 465 |
| username | varchar(255) | |
| encrypted_password | text | AES-256-GCM, never returned in API |
| is_active | bool | controls IMAP polling |
| last_sync_at | timestamp | updated after every poll |
| created_at / updated_at | timestamp | |

### `email_messages`

| Column | Type | Notes |
|---|---|---|
| id | uint | primary key |
| ticket_id | uuid | FK → tickets |
| account_id | uint | FK → email_accounts |
| message_id | varchar(500) | RFC 2822 Message-ID, unique index |
| thread_id | varchar(500) | X-Thread-ID / X-GM-THRID |
| in_reply_to | varchar(500) | threading header |
| references | text | space-separated Message-ID chain |
| direction | varchar(20) | INBOUND / OUTBOUND |
| sender | varchar(255) | |
| recipient | varchar(255) | |
| subject | varchar(500) | |
| body | text | plain text |
| html_body | text | HTML version (inbound only) |
| status | varchar(20) | RECEIVED / QUEUED / SENT / FAILED / DELIVERED / READ |
| attachments_count | int | |
| attachments | jsonb | array of `{filename, content_type, size, storage_path}` |
| error_message | text | SMTP failure reason |
| retry_count | int | outbound retry attempts |
| raw_headers | text | stored for debugging (not exposed via API) |
| received_at | timestamp | |
| sent_at | timestamp | |
| created_at | timestamp | |

---

## Package Structure

```
backend/internal/
├── email/
│   ├── attachments/
│   │   └── storage.go        — Storage interface + LocalStorage (filesystem)
│   ├── crypto/
│   │   └── crypto.go         — AES-256-GCM password encryption/decryption
│   ├── parser/
│   │   └── parser.go         — RFC 2822 parser (stdlib only)
│   ├── providers/
│   │   ├── interface.go      — Sender / Receiver interfaces + shared types
│   │   ├── imap/
│   │   │   └── client.go     — go-imap v1 IMAP implementation
│   │   └── smtp/
│   │       └── client.go     — net/smtp SMTP implementation
│   ├── threading/
│   │   └── detector.go       — Thread detection logic
│   └── workers/
│       ├── inbound_worker.go  — StartInboundWorker goroutine
│       └── outbound_worker.go — StartOutboundWorker goroutine
├── models/
│   ├── email_account.go
│   └── email_message.go
├── repositories/
│   ├── email_account_repository.go
│   └── email_message_repository.go
├── services/
│   ├── email_account_service.go
│   └── email_service.go
├── handlers/
│   └── email_handler.go
└── dto/
    └── email.go
```

---

## Backend Components

### Security — Password Encryption (`internal/email/crypto`)

Passwords are encrypted with **AES-256-GCM** before storage. The 32-byte key is derived by `sha256(JWT_ACCESS_SECRET)` so no extra configuration variable is needed. Encrypted values are never returned through any API endpoint.

### Email Parser (`internal/email/parser`)

Pure stdlib RFC 2822 parser — no external dependencies:
- Decodes `quoted-printable` and `base64` transfer encodings
- Recursively walks `multipart/mixed`, `multipart/alternative`, nested parts
- Extracts `text/plain` and `text/html` body parts separately
- Collects attachments with sanitised filenames (path traversal prevention)
- Decodes RFC 2047 encoded-word headers (non-ASCII subjects)

### Thread Detection (`internal/email/threading`)

Detection order (first match wins):
1. **In-Reply-To** header → exact `message_id` lookup
2. **References** header → each ID checked against `email_messages`
3. **X-Thread-ID / X-GM-THRID** header → thread_id lookup
4. **Subject matching** — normalised subject (strips Re:/Fwd:) + sender address within last 30 days

### IMAP Client (`internal/email/providers/imap`)

- Connects via TLS (port 993) or plain TCP with optional STARTTLS
- Uses `BODY.PEEK[]` to fetch full RFC 2822 message **without** marking as seen
- Marks messages seen only after successful processing
- Reconnects on every poll cycle (stateless, no idle connection kept)

### SMTP Client (`internal/email/providers/smtp`)

- STARTTLS path (`net/smtp.SendMail`) for port 587
- Implicit TLS path (manual `tls.Dial` + `smtp.NewClient`) for port 465
- Builds RFC 2822 messages with correct `In-Reply-To` + `References` headers for email client threading
- Sends `multipart/alternative` when HTML body present, plain text otherwise
- `quoted-printable` encoding for all body parts

### Attachment Storage (`internal/email/attachments`)

```
Storage interface {
    Save(ticketID, filename string, data []byte) (string, error)
    BasePath() string
}
```

`LocalStorage` saves files to `<ATTACHMENT_PATH>/<ticketID>/<nanosecond>_<filename>`. Swap the interface for S3/GCS/Azure Blob without changing business logic.

### Background Workers

**InboundWorker** — polls every `EMAIL_POLL_INTERVAL` seconds:
1. Load all active accounts with an IMAP host
2. Build receiver (decrypt password)
3. `FetchUnread` → parse → `ProcessInbound`
4. `MarkSeen` after successful processing
5. Update `last_sync_at`

**OutboundWorker** — runs every `EMAIL_POLL_INTERVAL` seconds:
1. `ProcessQueuedOutbound` — fetch QUEUED outbound messages, send via SMTP, update status
2. Every 5× interval: `RetryFailedOutbound` — re-queue FAILED messages below `MAX_EMAIL_RETRIES`

### AI Reply → Email Integration

When an agent calls `POST /tickets/:id/reply/approve`, `ReplyHandler.ApproveReply` now automatically calls `EmailService.QueueReplyForTicket`. This creates an `OUTBOUND / QUEUED` email message containing:
- Formatted reply body with greeting, AI text, signature, and ticket reference number
- `In-Reply-To` pointing to the last inbound Message-ID
- `References` containing the full thread chain
- The outbound worker delivers it on the next tick

---

## REST API

### Email Account Management (Admin only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/email/accounts` | List all accounts (no passwords) |
| POST | `/api/v1/email/accounts` | Create account |
| PUT | `/api/v1/email/accounts/:id` | Update account (omit password to keep existing) |
| DELETE | `/api/v1/email/accounts/:id` | Delete account |
| POST | `/api/v1/email/accounts/:id/test?protocol=smtp\|imap` | Test connection |
| GET | `/api/v1/email/monitor` | Stats dashboard |
| POST | `/api/v1/email/sync` | Trigger immediate sync (accepted async) |

### Ticket Email Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/tickets/:id/emails` | All emails in thread (chronological) |
| POST | `/api/v1/tickets/:id/send-email` | Send a manual email from the ticket |

---

## Activity Timeline Events

| Activity Type | Trigger |
|---|---|
| `EMAIL_RECEIVED` | Inbound email processed |
| `EMAIL_QUEUED` | Outbound email queued (on AI reply approval or manual send) |
| `EMAIL_SENT` | SMTP delivery confirmed |
| `EMAIL_FAILED` | SMTP delivery failed |
| `EMAIL_DELIVERED` | Future: delivery receipt |
| `ATTACHMENT_ADDED` | Inbound email contained attachments |

---

## Configuration (`.env`)

```env
# Email Configuration
EMAIL_POLL_INTERVAL=60      # seconds between IMAP polls
MAX_EMAIL_RETRIES=3         # max SMTP retry attempts before leaving in FAILED state
ATTACHMENT_PATH=./storage/attachments   # local attachment root directory
```

When `EMAIL_POLL_INTERVAL=0` the system defaults to 60 seconds. Workers start automatically when the API server starts — no separate process needed.

---

## Frontend

### Email Accounts Page (`/email/accounts`) — Admin only

- Lists all configured accounts with provider badge, IMAP/SMTP host:port, active status
- Create / Edit modal with full IMAP + SMTP configuration form
- "Test IMAP" and "Test SMTP" buttons verify live credentials without saving
- Password field is write-only — displayed as blank on edit

### Email Monitor Page (`/email/monitor`) — Admin only

- Six stat cards: Total Accounts, Active Accounts, Queued, Failed, Sent Today, Received Today
- Connected accounts table with last sync timestamp
- "Sync Now" button triggers immediate mailbox poll
- Auto-refreshes every 30 seconds

### Email Conversation Tab (Ticket Details — "Email" tab)

- Shows full INBOUND (blue) / OUTBOUND (green) email thread in chronological order
- Status badge per message (RECEIVED, QUEUED, SENT, FAILED)
- Expand / collapse individual email bodies
- Attachment metadata list (filename, content-type)
- Inline compose form to send a manual email to the customer
- Error message displayed for FAILED outbound messages

### Dashboard

Added "Email Accounts" and "Email Monitor" navigation links (admin only).

---

## Dependencies Added

```
github.com/emersion/go-imap v1.2.1
github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21  (transitive)
```

Standard library only for SMTP, email parsing, and crypto — no additional packages required.

---

## Supported Providers

| Provider | Status |
|---|---|
| SMTP + IMAP (generic) | ✅ Implemented |
| Gmail | ✅ Works via SMTP_IMAP with App Password |
| Microsoft Outlook | ✅ Works via SMTP_IMAP with App Password |
| SendGrid | 🔜 Future (replace Sender implementation) |
| Amazon SES | 🔜 Future (replace Sender implementation) |
| Mailgun | 🔜 Future (replace Sender implementation) |

The `Sender` and `Receiver` interfaces are the only integration points — adding a new provider requires implementing those two interfaces.
