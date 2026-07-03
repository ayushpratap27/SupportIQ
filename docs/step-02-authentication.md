# AI Support Assistant — Step 2: Authentication & User Management

**Date:** 2026-07-03
**Builds on:** Step 1 — Project Foundation

---

## What was built

A complete, production-ready authentication and user management module including JWT-based auth, RBAC middleware, and a fully wired React frontend with protected routes.

---

## 2.1 Success Criteria Met

| Criterion | Status |
|-----------|--------|
| User can register | ✅ |
| User can login | ✅ |
| Passwords are hashed (bcrypt) | ✅ |
| JWT works (access + refresh) | ✅ |
| Protected routes work | ✅ |
| `/me` returns current user | ✅ |
| Logout works | ✅ |
| Axios automatically sends JWT | ✅ |
| Unauthorized users are redirected | ✅ |
| RBAC middleware exists | ✅ |

---

## 2.2 Database — `users` table

Auto-migrated via GORM on server startup.

| Column | Type | Notes |
|--------|------|-------|
| `id` | SERIAL PRIMARY KEY | Auto-increment |
| `name` | VARCHAR(100) | NOT NULL |
| `email` | VARCHAR(255) | UNIQUE, NOT NULL |
| `password_hash` | VARCHAR(255) | NOT NULL, never returned in JSON |
| `role` | VARCHAR(20) | `Admin` or `SupportAgent`, default `SupportAgent` |
| `is_active` | BOOLEAN | default `true` |
| `created_at` | TIMESTAMP | managed by GORM |
| `updated_at` | TIMESTAMP | managed by GORM |

---

## 2.3 New Backend Files

### `internal/models/user.go`
GORM model for the `users` table. `PasswordHash` is tagged `json:"-"` — it is never serialised into any API response.

### `internal/dto/auth.go`
Data Transfer Objects (request and response shapes).
- `RegisterRequest` — validated via Gin binding tags
- `LoginRequest` — validated via Gin binding tags
- `UserResponse` — safe public representation (no hash)
- `AuthResponse` — wraps tokens + user, returned on login/register

### `internal/jwt/jwt.go`
Reusable JWT helpers:
- `GenerateTokenPair(userID, email, role, accessSecret, refreshSecret)` → access token (15 min) + refresh token (7 days)
- `ValidateToken(tokenStr, secret)` → `*Claims` or error
- `Claims` struct embeds `RegisteredClaims` with `UserID`, `Email`, `Role`
- Signs with `HS256`; rejects any other algorithm

### `internal/validators/auth.go`
Password strength validator. Rules enforced:
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character (in addition to the min-8 binding tag)

### `internal/services/auth_service.go`
All business logic for auth. Handlers stay thin.
- `Register` — checks email uniqueness → bcrypt hash → DB insert → token pair
- `Login` — constant-time bcrypt compare (prevents timing attacks), then token pair
- `GetUserByID` — returns safe `UserResponse` DTO

Sentinel errors (`ErrEmailTaken`, `ErrInvalidCredentials`, `ErrUserNotFound`) are used by handlers to pick the right HTTP status code.

### `internal/handlers/auth.go`
Thin HTTP layer — parse → validate → delegate to service → respond.
- `POST /api/v1/auth/register` → 201 Created
- `POST /api/v1/auth/login` → 200 OK
- `POST /api/v1/auth/logout` → 200 OK (stateless; client drops token)
- `GET /api/v1/auth/me` → 200 OK (protected)

### `internal/middleware/auth.go`
`Authenticate(db, cfg)` middleware:
1. Reads `Authorization: Bearer <token>` header
2. Validates JWT signature and expiry
3. Loads user from database (ensures user still exists)
4. Stores `userID`, `userRole`, `user` in Gin context

### `internal/middleware/rbac.go`
`RequireRole(roles ...models.Role)` middleware:
- Must be chained after `Authenticate`
- Reads `userRole` from context
- Returns `403 Forbidden` if role not in the allowed set
- Example: `middleware.RequireRole(models.RoleAdmin)`

---

## 2.4 Modified Backend Files

### `internal/config/config.go`
Added `JWTAccessSecret` and `JWTRefreshSecret` fields. Both are required — server fails fast if missing.

### `internal/database/database.go`
Added `db.AutoMigrate(&models.User{})` — runs on every startup, safe to re-run (no-op if schema is current).

### `internal/routes/routes.go`
Added auth route group under `/api/v1/auth`:
```
POST  /api/v1/auth/register   → public
POST  /api/v1/auth/login      → public
POST  /api/v1/auth/logout     → public (stateless)
GET   /api/v1/auth/me         → Authenticate middleware required
```

### `.env`
Added `JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET`. **Change these in production.**

---

## 2.5 New Frontend Files

### `src/services/api.js` (updated)
Exported `TOKEN_KEY = 'access_token'`.

Added Axios **request interceptor**: reads token from `localStorage` and attaches `Authorization: Bearer <token>` to every outgoing request.

Added Axios **response interceptor**: on `401 Unauthorized`, clears the token and redirects to `/login` (skips redirect if already on `/login` to avoid loops).

### `src/services/authService.js`
All auth API calls in one file:
- `register(data)`, `login(data)`, `logout()`, `getMe()`

### `src/contexts/AuthContext.jsx`
Global auth state via React Context + `useAuth()` hook.
- Restores session on mount by calling `/me` with the stored token
- `loading` flag prevents flash of unauthenticated content
- Exposes `user`, `loading`, `login()`, `register()`, `logout()`

### `src/components/ProtectedRoute.jsx`
React Router layout route that:
- Shows a loading screen while session is restoring
- Redirects to `/login` if no authenticated user
- Renders `<Outlet />` (child routes) when authenticated

### `src/pages/Login.jsx`
- Email + password form
- Calls `useAuth().login()` on submit
- Shows field-level and API error messages
- Already-logged-in users are redirected to `/dashboard`

### `src/pages/Register.jsx`
- Name, email, password form
- Client-side validation (name length, email format, password length)
- Backend password strength errors displayed inline
- Already-logged-in users are redirected to `/dashboard`

### `src/pages/Dashboard.jsx`
- Fetches fresh user data via `GET /api/v1/auth/me` on every mount
- Displays: name (welcome), email, role badge, active status
- Logout button calls `useAuth().logout()` then navigates to `/login`

---

## 2.6 Security Practices

- Passwords hashed with `bcrypt.DefaultCost` — never stored in plain text
- Password hash field tagged `json:"-"` — impossible to leak via API
- Login returns the **same error** for wrong email and wrong password (prevents user enumeration)
- JWT signed with `HS256` — algorithm is explicitly verified; other algorithms rejected
- JWT secrets loaded from environment variables — never hardcoded
- Access tokens expire in **15 minutes**; refresh tokens in **7 days**
- CORS origin whitelist prevents cross-origin token theft

---

## 2.7 API Reference

### `POST /api/v1/auth/register`
```json
// Request
{ "name": "Ayush", "email": "ayush@example.com", "password": "Secret@123" }

// Response 201
{
  "status": "success",
  "message": "Registration successful",
  "data": {
    "accessToken": "...",
    "refreshToken": "...",
    "user": { "id": 1, "name": "Ayush", "email": "...", "role": "SupportAgent", "is_active": true }
  }
}
```

### `POST /api/v1/auth/login`
```json
// Request
{ "email": "ayush@example.com", "password": "Secret@123" }

// Response 200 — same shape as register
```

### `GET /api/v1/auth/me` (requires Bearer token)
```json
// Response 200
{
  "status": "success",
  "message": "User retrieved",
  "data": { "id": 1, "name": "Ayush", "email": "...", "role": "SupportAgent", "is_active": true }
}
```

---

## 2.8 What is intentionally NOT in Step 2

- Refresh token rotation endpoint
- Token blacklisting (Redis — future step)
- Password reset / forgot password flow
- Email verification
- OAuth / social login
- Admin user management screens
