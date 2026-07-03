# Step 11 — Multi-Tenancy

## Overview

Converted SupportIQ from a single-tenant application to a fully isolated multi-tenant SaaS platform. Multiple companies (tenants) can now use the same deployment with complete data isolation at every layer.

---

## Requirements Met

- **Tenants table** — full CRUD with status, subscription plan, domain, max users/tickets limits
- **`tenant_id` on all business tables** — every model now carries a UUID tenant scope
- **JWT with `TenantID`** — claims include `TenantID uuid.UUID`; token generation and validation updated
- **Tenant Middleware** — validates JWT, resolves tenant, injects into Gin context; SuperAdmin bypasses tenant check
- **Repository tenant isolation** — every repository has a `scoped(tenantID)` helper; all queries are tenant-filtered
- **SuperAdmin role** — `RoleSuperAdmin` bypasses all tenant checks; `tenantID = uuid.Nil`
- **Tenant Management APIs** — full CRUD at `/api/v1/admin/tenants`, platform overview at `/api/v1/admin/overview`
- **Tenant Settings APIs** — `GET/PUT /api/v1/settings` for the current tenant admin
- **Frontend** — Register with company name, Tenant Settings page, SuperAdmin Dashboard

---

## Architecture Pattern

```go
// Repository scoping
func (r *XRepo) scoped(tenantID uuid.UUID) *gorm.DB {
    return r.db.Where("tenant_id = ?", tenantID)
}

// Every service method signature
func (s *XService) Create(tenantID uuid.UUID, req ...) (..., error)

// Every handler extraction
tenantID := middleware.GetTenantID(c)

// SuperAdmin: tenantID = uuid.Nil, bypasses tenant checks
```

---

## New Files

| File | Description |
|------|-------------|
| `backend/internal/models/tenant.go` | Tenant model with `TenantStatus` and `SubscriptionPlan` enums |
| `backend/internal/dto/tenant.go` | Request/response DTOs: `CreateTenantRequest`, `UpdateTenantRequest`, `TenantResponse`, `SuperAdminOverview`, `RegisterWithTenantRequest` |
| `backend/internal/repositories/tenant_repository.go` | CRUD + `CountUsers`, `CountTickets`, `PlatformStats`, `AllActiveTenantIDs()` |
| `backend/internal/services/tenant_service.go` | List, Create, GetByID, Update, Delete, GetPlatformOverview, GetSettings, UpdateSettings |
| `backend/internal/handlers/tenant_handler.go` | HTTP handlers for all tenant management endpoints |
| `frontend/src/services/tenantService.js` | API client for tenant endpoints |
| `frontend/src/pages/TenantSettings.jsx` | Tenant admin settings form |
| `frontend/src/pages/superadmin/SuperAdminDashboard.jsx` | Platform-wide stats + tenant list for SuperAdmin |

---

## Modified Files

### Models (all added `TenantID uuid.UUID`)
- `user.go` — `RoleSuperAdmin`, unique index on `(tenant_id, email)`
- `ticket.go`, `ticket_activity.go`, `ticket_note.go`, `ticket_comment.go`
- `ai_reply.go`, `knowledge_base.go`, `email_account.go`, `email_message.go`
- `analytics.go` — composite unique indexes on `(tenant_id, date)`
- `integration.go`, `background_job.go`

### JWT / Middleware
- `jwt/jwt.go` — `Claims.TenantID uuid.UUID`; `GenerateTokenPair` takes `tenantID`
- `middleware/auth.go` — validates JWT, resolves tenant, sets context; `GetTenantID()` helper
- `middleware/rbac.go` — `RequireSuperAdmin()`, SuperAdmin bypasses role checks

### Repositories (all tenant-scoped)
All 12 repositories updated: ticket, user, activity, note, comment, knowledge, reply, job, email_account, email_message, integration, analytics.

Notable additions:
- `EmailAccountRepository.FindActive()` — alias for inbound worker
- `EmailAccountRepository.ListAllActive()` — cross-tenant for workers
- `IntegrationRepository.FindAllEnabled()` / `FindAllPendingEvents()` — cross-tenant for background workers

### Services (all take `tenantID` as first param)
auth, ticket, note, comment, knowledge, reply, job, integration, email_account, tenant, analytics

### Handlers (all pass `middleware.GetTenantID(c)`)
All 12 handlers updated: auth, ticket, activity, ai, analytics, comment, note, knowledge, reply, integration, email, user.

### Queue / Workers
- `queue/queue.go` — added `TenantID string` to `Job` struct
- `worker/handlers/ai_analysis.go` — uses `FindByIDUnscoped`, stamps `TenantID` on activity
- `worker/handlers/generate_reply.go` — passes `tenantID` from job to `replySvc`
- `email/workers/inbound_worker.go` — uses `FindActive()` (cross-tenant)
- `services/email_account_service.go` — added `BuildReceiver()` for IMAP

### Routes
- Added `/api/v1/settings` (tenant admin) and `/api/v1/admin/tenants` + `/api/v1/admin/overview` (SuperAdmin)
- Wired `TenantRepository` to analytics aggregator and integration worker

### Frontend
- `Register.jsx` — added Company Name field, posts `company_name` to `RegisterWithTenant`
- `routes/index.jsx` — `/settings` and `/admin` routes added

---

## API Endpoints Added

### SuperAdmin (`RoleSuperAdmin` required)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/tenants` | List all tenants |
| POST | `/api/v1/admin/tenants` | Create tenant |
| GET | `/api/v1/admin/tenants/:id` | Get tenant by ID |
| PUT | `/api/v1/admin/tenants/:id` | Update tenant |
| DELETE | `/api/v1/admin/tenants/:id` | Delete tenant |
| GET | `/api/v1/admin/overview` | Platform-wide stats |

### Tenant Settings (`RoleAdmin` required for PUT)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/settings` | Get current tenant settings |
| PUT | `/api/v1/settings` | Update current tenant settings |

### Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/register` | Register with tenant (creates tenant + admin) |

---

## Database Changes

`AutoMigrate` now includes `&models.Tenant{}`. The `TicketCounter` seeder was removed — counters are created per-tenant on first ticket. All existing tables get `tenant_id` column via migration (defaulting to `uuid.Nil` for existing rows).

---

## Security

- All data access is gated behind `WHERE tenant_id = ?` at the repository layer
- JWT includes `tenant_id`; middleware validates tenant is active before every request
- SuperAdmin (`uuid.Nil` tenant) is the only identity that can cross tenant boundaries
- Cross-tenant operations (email polling, integration workers, analytics aggregation) use dedicated unscoped methods with explicit documentation
