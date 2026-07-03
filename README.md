# AI Support Assistant — Project Foundation

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.21+ | https://go.dev/dl/ |
| Node.js | 18+ | https://nodejs.org |
| PostgreSQL | 14+ | https://www.postgresql.org/download/ |

---

## Folder Structure

```
SupportIQ/
├── backend/                    # Go / Gin / GORM API
│   ├── cmd/
│   │   └── main.go             # Entry point: wires config → DB → router → server
│   ├── internal/
│   │   ├── config/             # Env var loader (godotenv)
│   │   ├── database/           # PostgreSQL connection pool (GORM)
│   │   ├── handlers/           # HTTP handler structs (one file per domain)
│   │   ├── middleware/         # CORS, request logger
│   │   ├── models/             # GORM model structs (empty – future)
│   │   ├── routes/             # Route registration + middleware wiring
│   │   ├── services/           # Business logic layer (empty – future)
│   │   └── utils/              # logger.go, response.go — shared helpers
│   ├── .env                    # Local secrets (git-ignored)
│   ├── .env.example            # Template for new contributors
│   └── go.mod
│
└── frontend/                   # React / Vite / TailwindCSS SPA
    ├── src/
    │   ├── components/         # Reusable UI components (empty – future)
    │   ├── hooks/              # Custom React hooks (empty – future)
    │   ├── layouts/            # Page shell components (MainLayout)
    │   ├── pages/              # Route-level page components (Home)
    │   ├── routes/             # React Router definitions
    │   ├── services/           # Axios API client + service modules
    │   ├── App.jsx             # BrowserRouter root
    │   ├── index.css           # Tailwind directives
    │   └── main.jsx            # ReactDOM entry
    ├── .env                    # VITE_API_URL (git-ignored)
    ├── .env.example
    └── package.json
```

---

## Backend Setup

```bash
# 1. Install Go from https://go.dev/dl/ then:
cd backend

# 2. Copy env and fill in your values
cp .env.example .env

# 3. Create the database
createdb supportiq

# 4. Resolve Go modules
go mod tidy

# 5. Run
go run ./cmd
```

The server starts on `http://localhost:8080`.

### Health check

```
GET http://localhost:8080/api/v1/health

{
  "status": "success",
  "message": "Backend running successfully"
}
```

---

## Frontend Setup

```bash
cd frontend

# 1. Install dependencies (already done if you ran npm install)
npm install

# 2. Copy env
cp .env.example .env

# 3. Start dev server
npm run dev
```

Open `http://localhost:5173`.  
The Home page calls `/api/v1/health` on load and shows:
- 🟢 Backend Connected — when the API responds
- 🔴 Backend Offline — when the API is unreachable

---

## Why each package exists

| Package | Purpose |
|---------|---------|
| `config` | Single place to read all env vars; fails fast if required vars are missing |
| `database` | Owns GORM setup and connection pool tuning — kept separate from business logic |
| `handlers` | Thin HTTP layer: parse request → call service → write response |
| `middleware` | Cross-cutting concerns (CORS, structured request logging) wired at router level |
| `models` | GORM structs — co-located with DB schema definition |
| `routes` | Wires handlers + middleware onto a `gin.Engine`; single view of all endpoints |
| `services` | Business logic, isolated from HTTP and DB details |
| `utils` | Logger and response helpers reused across all packages |
