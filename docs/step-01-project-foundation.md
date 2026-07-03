# AI Support Assistant — Build Progress

---

## Step 1 — Project Foundation

**Date:** 2026-07-03

### What was built

The complete project skeleton for a production-ready AI SaaS application named **AI Support Assistant**.

---

### 1.1 Workspace Structure

```
SupportIQ/
├── backend/       # Go REST API
├── frontend/      # React SPA
├── docs/          # This folder — build progress notes
└── README.md      # Setup & architecture overview
```

---

### 1.2 Backend — Go / Gin / PostgreSQL / GORM

#### Tech choices
| Package | Role |
|---------|------|
| `gin-gonic/gin` | HTTP router and middleware framework |
| `gorm.io/gorm` | ORM for PostgreSQL |
| `gorm.io/driver/postgres` | GORM PostgreSQL driver |
| `gin-contrib/cors` | CORS middleware |
| `sirupsen/logrus` | Structured JSON logger |
| `joho/godotenv` | `.env` file loader |

#### Folder structure created

```
backend/
├── cmd/
│   └── main.go                    # Entry point
├── internal/
│   ├── config/
│   │   └── config.go              # Env var loader
│   ├── database/
│   │   └── database.go            # PostgreSQL connection pool
│   ├── handlers/
│   │   └── health.go              # Health check handler
│   ├── middleware/
│   │   ├── cors.go                # CORS policy
│   │   └── logger.go              # Per-request structured logging
│   ├── models/                    # GORM model structs (empty — future steps)
│   ├── routes/
│   │   └── routes.go              # Route registration
│   ├── services/                  # Business logic layer (empty — future steps)
│   └── utils/
│       ├── logger.go              # App-wide logrus logger
│       └── response.go            # Reusable JSON response helpers
├── .env                           # Local secrets (git-ignored)
├── .env.example                   # Template for contributors
├── .gitignore
└── go.mod
```

#### What each file does

**`cmd/main.go`**
- Loads config → connects DB → builds router → starts HTTP server
- Graceful shutdown on `SIGINT` / `SIGTERM` with a 10-second drain timeout
- Sets `gin.ReleaseMode` when `APP_ENV=production`

**`internal/config/config.go`**
- Reads `PORT`, `DATABASE_URL`, `APP_ENV` from environment
- Fails fast with a clear error if `DATABASE_URL` is missing
- Uses `godotenv` to load `.env` in development; silently ignored in production

**`internal/database/database.go`**
- Opens a GORM connection to PostgreSQL
- Connection pool: 25 max open, 10 idle, 5-minute max lifetime
- Logs a success message on connect

**`internal/handlers/health.go`**
- Handles `GET /api/v1/health`
- Constructed via `NewHealthHandler()` — dependency injection ready
- Returns: `{"status":"success","message":"Backend running successfully"}`

**`internal/middleware/cors.go`**
- Whitelists `localhost:5173` (Vite) and `localhost:3000`
- Allows standard HTTP methods and headers
- 12-hour preflight cache

**`internal/middleware/logger.go`**
- Logs every request: method, path, status code, latency, client IP
- Uses the shared logrus logger — JSON output, compatible with log aggregators

**`internal/routes/routes.go`**
- Creates `gin.New()` (not `gin.Default()` — full control over middleware)
- Wires: Recovery → RequestLogger → CORS → route handlers
- All routes live under `/api/v1`

**`internal/utils/logger.go`**
- Package-level `*logrus.Logger`, JSON format, initialized once via `init()`
- Imported by any package that needs to log

**`internal/utils/response.go`**
- `SendSuccess(c, statusCode, message, data)` — wraps response in `{status, message, data}`
- `SendError(c, statusCode, message)` — wraps error in `{status, message}`
- Eliminates duplicated `c.JSON(...)` calls across handlers

#### Environment variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `PORT` | Port the server listens on | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://ayush:pass@localhost:5432/supportiq?sslmode=disable` |
| `APP_ENV` | Environment name | `development` / `production` |

#### API endpoint implemented

```
GET /api/v1/health

Response 200:
{
  "status": "success",
  "message": "Backend running successfully"
}
```

---

### 1.3 Frontend — React / Vite / TailwindCSS / Axios / React Router

#### Tech choices
| Package | Role |
|---------|------|
| `react` + `react-dom` | UI library |
| `vite` | Dev server and build tool |
| `tailwindcss` | Utility-first CSS framework |
| `axios` | HTTP client |
| `react-router-dom` | Client-side routing |

#### Folder structure created

```
frontend/
├── src/
│   ├── components/                # Shared UI components (empty — future steps)
│   ├── hooks/                     # Custom React hooks (empty — future steps)
│   ├── layouts/
│   │   └── MainLayout.jsx         # Page shell with <Outlet />
│   ├── pages/
│   │   └── Home.jsx               # Home page
│   ├── routes/
│   │   └── index.jsx              # React Router route definitions
│   ├── services/
│   │   └── api.js                 # Axios instance + service modules
│   ├── App.jsx                    # BrowserRouter root
│   ├── index.css                  # Tailwind directives
│   └── main.jsx                   # ReactDOM entry point
├── index.html                     # HTML shell
├── vite.config.js                 # Vite configuration
├── tailwind.config.js             # Tailwind content paths
├── postcss.config.js              # PostCSS + Autoprefixer
├── package.json
├── .env                           # VITE_API_URL (git-ignored)
├── .env.example
└── .gitignore
```

#### What each file does

**`src/services/api.js`**
- Single Axios instance reading base URL from `VITE_API_URL` — never hardcoded
- Response interceptor in place for future global error handling
- Exports named service modules (`healthService`) — one per domain

**`src/pages/Home.jsx`**
- On mount, calls `GET /api/v1/health` via `healthService.check()`
- Three states: `loading` (animating pulse) → `online` (🟢 Backend Connected) → `offline` (🔴 Backend Offline)

**`src/routes/index.jsx`**
- All routes defined in one file — easy to extend
- Wraps routes inside `MainLayout` using React Router nested routes

**`src/layouts/MainLayout.jsx`**
- Outer page shell with `min-h-screen` background
- Renders child routes via `<Outlet />`

**`src/App.jsx`**
- Mounts `<BrowserRouter>` and `<AppRoutes>`

#### Environment variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `VITE_API_URL` | Backend base URL for Axios | `http://localhost:8080` |

---

### 1.4 Database Setup

- PostgreSQL database: `supportiq`
- User: `ayush` (macOS Homebrew default superuser)
- Created with: `createdb supportiq`

---

### 1.5 How to Run

**Backend**
```bash
cd backend
export PATH=$PATH:/opt/homebrew/bin
go run ./cmd
# Server starts on http://localhost:8080
```

**Frontend**
```bash
cd frontend
npm run dev
# App starts on http://localhost:5173
```

**Health check**
```bash
curl http://localhost:8080/api/v1/health
# {"status":"success","message":"Backend running successfully"}
```

---

### 1.6 What is intentionally NOT in Step 1

The following are planned for future steps and have empty placeholder directories:

- Authentication (JWT / sessions)
- Ticket CRUD API
- AI integration
- Email integration
- Slack integration
- Analytics
- Background workers
- Redis
- Docker / containerisation
