# ai-mock-interview-v2

AI-powered mock interview platform: users describe a target role, Gemini
generates tailored questions, candidates record spoken answers via webcam +
speech-to-text, and an LLM rates each response with structured feedback.

The repo is a two-app monorepo: a Go API and a React SPA, glued together by a
root delegating Makefile. There is no workspace tool (Turborepo / Nx / npm
workspaces) — the two apps share zero JS/TS code, so a workspace tool would
add ceremony with no payoff.

> Screenshot: _placeholder — add `docs/screenshots/dashboard.png` once UI is live._

---

## Tech stack

| Layer | Choice | Notes |
|-------|--------|-------|
| Backend | **Go 1.22+** | chi v5 router, log/slog, structured errors. |
| Database driver | **jackc/pgx/v5 + pgxpool** | Native Postgres types (JSONB, UUID, timestamptz). |
| Type-safe SQL | **sqlc** | Generates Go from `.sql` — no ORM, no string queries. |
| Migrations | **golang-migrate/migrate v4** | File-based, versioned. |
| Hot reload (dev) | **air** | Watches `**/*.go`, rebuilds to `tmp/main`. |
| Frontend | **React 18.3 + Vite 5 + TS strict** | SWC plugin, HMR. |
| Routing | **react-router-dom v6.26 (data routers)** | `createBrowserRouter`. |
| Server cache | **@tanstack/react-query v5** | Request dedup + background refetch. |
| UI primitives | **Tailwind 3.4 + shadcn/ui (Radix)** | HSL CSS-var tokens. |
| Forms | **react-hook-form + zod** | Typed end-to-end. |
| Auth | **Clerk** (`@clerk/clerk-react` + `clerk-sdk-go/v2`) | RS256 JWT verified server-side via JWKS. |
| Database | **Neon Postgres 16** | Serverless, branching for dev/prod isolation. |
| LLM | **Gemini 2.5 Flash** via `google.golang.org/genai` | Structured output through `ResponseSchema` + `ResponseMIMEType`. |

For deeper rationale on each choice see [`docs/architecture/`](./docs/architecture/index.md).

---

## Prerequisites

Install the following on your dev machine. Versions listed are the floors that
the project has been validated against.

| Tool | Minimum version | Install |
|------|-----------------|---------|
| Go | 1.22 | https://go.dev/dl/ — `go version` |
| Node.js | 20.9 (LTS) | https://nodejs.org or `nvm install 20` — `node -v` |
| Make | any modern GNU make | preinstalled on macOS/Linux |
| Git | 2.34+ | preinstalled |

You also need accounts (free tiers are sufficient for development):

- **Neon** — https://console.neon.tech (sign in with GitHub for fastest setup)
- **Clerk** — https://dashboard.clerk.com
- **Google AI Studio** (for the Gemini API key) — https://aistudio.google.com/apikey

### Go-specific CLIs

These are installed via `go install` and end up in `$GOPATH/bin` (or
`$HOME/go/bin`); make sure that directory is on your `PATH`.

```bash
# golang-migrate — DB migrations
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# air — hot reload for the Go API
go install github.com/air-verse/air@latest

# golangci-lint — aggregate Go linter (preferred install: official script)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
    | sh -s -- -b $(go env GOPATH)/bin v1.59.1
```

Verify:

```bash
migrate -version
air -v
golangci-lint --version
```

A `psql` client is **not** required — any Postgres client (DBeaver, TablePlus,
the Neon SQL Editor) works for ad-hoc queries.

---

## Setup

```bash
# 1. Clone
git clone <repo-url> ai-mock-interview-v2
cd ai-mock-interview-v2

# 2. Copy env templates
cp backend/.env.example  backend/.env
cp frontend/.env.example frontend/.env

# 3. Fill in secrets (see "Environment variables" below)
#    - Neon connection strings (pooled + direct) into backend/.env
#    - Clerk secret + publishable into backend/.env
#    - Clerk publishable into frontend/.env
#    - Gemini API key into backend/.env

# 4. Install dependencies (Go modules + npm packages)
make install

# 5. Apply database migrations
make migrate-up
```

For a step-by-step Neon setup walkthrough see
[`docs/architecture/integration.md` §3](./docs/architecture/integration.md#3-neon-postgres-setup-walkthrough).

---

## Environment variables

`.env` files live next to the apps that consume them and are gitignored. Only
`.env.example` files are committed.

### `backend/.env`

| Name | Purpose | Example |
|------|---------|---------|
| `DATABASE_URL` | **Pooled** Neon connection (used by API at runtime). | `postgresql://user:pass@ep-xxx-pooler.region.aws.neon.tech/neondb?sslmode=require` |
| `DATABASE_URL_DIRECT` | **Non-pooled** Neon connection (used by `migrate` — PgBouncer transaction-mode breaks advisory locks). | `postgresql://user:pass@ep-xxx.region.aws.neon.tech/neondb?sslmode=require` |
| `CLERK_SECRET_KEY` | Server-side Clerk key; verifies session tokens via JWKS. | `sk_test_xxx` |
| `CLERK_PUBLISHABLE_KEY` | Mirrors frontend's; useful for server-rendered checks. | `pk_test_xxx` |
| `GEMINI_API_KEY` | Google AI Studio key for `gemini-2.5-flash`. | `AIzaSy...` |
| `PORT` | API listen port. | `8080` |
| `CORS_ALLOWED_ORIGIN` | Comma-separated origins permitted by CORS middleware. | `http://localhost:5173` (dev), `https://app.example.com` (prod) |
| `APP_ENV` | When ≠ `production`, the API loads `.env` via godotenv. | `development` |
| `LOG_LEVEL` | slog level: `debug` / `info` / `warn` / `error`. | `debug` |

### `frontend/.env`

Vite only exposes `VITE_*`-prefixed variables to the client bundle.

| Name | Purpose | Example |
|------|---------|---------|
| `VITE_CLERK_PUBLISHABLE_KEY` | Initializes `<ClerkProvider>` in the SPA. | `pk_test_xxx` |
| `VITE_API_BASE_URL` | Backend base path. In dev Vite proxies `/api` → `:8080`. | `/api/v1` |

---

## Database migrations

Migrations live in `backend/migrations/` as paired `NNNN_name.up.sql` /
`NNNN_name.down.sql` files, applied with `golang-migrate`. The root Makefile
proxies the targets so you do not have to `cd backend/` first.

```bash
make migrate-up                                    # apply all pending migrations
make migrate-down                                  # roll back the most recent migration
make migrate-create name=add_user_preferences     # scaffold next migration pair
```

Migrations run against `DATABASE_URL_DIRECT` (the non-pooled connection),
because PgBouncer transaction-mode pooling breaks `golang-migrate`'s advisory
locks. Keep both URLs in `backend/.env`.

---

## Run

Two terminals (no Docker, no orchestration script — keep it simple):

```bash
# Terminal 1 — backend API on :8080
make dev-backend

# Terminal 2 — frontend SPA on :5173
make dev-frontend
```

Open http://localhost:5173. The Vite dev server proxies `/api/*` →
`http://localhost:8080`, so the SPA can call `fetch('/api/v1/interviews')`
both in development and in production (behind a reverse proxy) without any
client-side branching.

---

## Project structure

```
ai-mock-interview-v2/
├── README.md                # this file
├── Makefile                 # root delegating targets (proxies into backend/)
├── .editorconfig            # 2-space JS/TS/MD/YAML; tabs for Go and Makefiles
├── .env.example             # pointer; real templates are per-app
├── .gitignore
├── docs/
│   └── architecture/
│       ├── index.md         # plans overview
│       ├── backend.md       # Go API plan
│       ├── frontend.md      # React SPA plan
│       └── integration.md   # monorepo glue, env, deploy
├── backend/                 # Go API — own Makefile, sqlc.yaml, migrations
│   ├── cmd/server/          # composition root (wire config, db, llm, router)
│   ├── internal/
│   │   ├── config/          # envconfig
│   │   ├── domain/          # pure types + sentinel errors
│   │   ├── auth/            # Clerk JWKS + middleware
│   │   ├── http/            # chi handlers, render helpers, error mapping
│   │   ├── service/         # orchestrators; defines Store + LLM interfaces
│   │   ├── store/           # pgxpool repos + sqlc-generated code
│   │   ├── llm/             # Gemini client + ResponseSchema definitions
│   │   └── platform/logger/ # slog setup
│   └── migrations/
└── frontend/                # React SPA
    ├── src/
    │   ├── routes/          # createBrowserRouter config + page modules
    │   ├── components/      # ui/ (shadcn) + feature folders
    │   ├── api/             # fetch wrapper + typed endpoint functions
    │   ├── hooks/           # TanStack Query, speech, webcam
    │   ├── lib/             # env validation, token store, formatters
    │   └── test/            # Vitest setup, MSW handlers
    └── vite.config.ts
```

For deep architectural rationale see [`docs/architecture/`](./docs/architecture/index.md).

---

## Testing

This project follows **test-driven development** — see the
[`test-driven-development` skill](../../.agents/skills/test-driven-development/SKILL.md)
for the discipline. The iron law applies: **no production code without a
failing test first.** Red → verify red → minimal green → verify green →
refactor. Tests written after implementation pass immediately, which proves
nothing.

What that means in practice:

- **Backend (Go).** Unit tests are colocated (`internal/interview/service_test.go`),
  integration tests use the `//go:build integration` tag and either a
  `testcontainers-go/postgres` instance or a Neon `dev` branch via
  `DATABASE_URL_TEST`. Use `testify/require` and `httptest`.
- **Frontend (React).** Vitest + React Testing Library, tests colocated as
  `Component.test.tsx`. Mock the network at the boundary with MSW; do not mock
  internal modules.

Run everything:

```bash
make test           # backend + frontend in single-run mode
make lint           # golangci-lint + ESLint
```

Per-app:

```bash
cd backend  && go test ./... -race -count=1
cd backend  && go test -tags=integration ./...
cd frontend && npm test                 # watch mode
cd frontend && npm test -- --run        # single run
cd frontend && npm run test:coverage
```

Coverage targets per the backend plan: ≥80% on `service/` and `store/`, 100%
on `domain/errors.go` and `http/errors.go`.

---

## Troubleshooting

**Neon connection fails with `SSL is required`.**
Append `?sslmode=require` to both `DATABASE_URL` and `DATABASE_URL_DIRECT`.
Neon does not accept unencrypted connections.

**Migrations hang or report `database is locked`.**
You are pointing `migrate` at the pooled URL. PgBouncer transaction-mode
breaks `golang-migrate`'s advisory locks. Set `MIGRATE_DSN` to
`DATABASE_URL_DIRECT` (or set `DATABASE_URL_DIRECT` in `backend/.env` — the
backend Makefile already wires it via `MIGRATE_DSN ?= $(DATABASE_URL_DIRECT)`).

**Clerk requests fail with `JWT is expired` or `token used before issued`.**
Likely clock skew between your machine and Clerk's auth servers. Sync your
system clock (`sudo ntpdate pool.ntp.org` on Linux, "Set time automatically"
on macOS/Windows). Clerk allows a small leeway window, but more than ~30s of
drift will reject every request.

**`air` does not reload on file change.**
Confirm `air` is on `$PATH` (`which air`). If running on macOS or in WSL,
file-watcher event coalescing can drop events for very fast saves; see
`backend/.air.toml` — increase `delay_ms` or extend `include_ext` to cover the
file types you are editing. Make sure you launched `make dev-backend` from
within the repo (working directory matters for relative paths in `.air.toml`).

**CORS errors in staging or production.**
Set `CORS_ALLOWED_ORIGIN` on the backend to the deployed frontend URL
(comma-separated for multiple). The dev workflow uses Vite's proxy and
sidesteps CORS entirely; staging/prod need the middleware configured
correctly. `AllowCredentials` is `false` because Clerk tokens travel in
`Authorization: Bearer`, not cookies.

**`psql: command not found`.**
You do not need it. Use the Neon SQL Editor in the web console, or any
Postgres GUI (DBeaver, TablePlus, Postico). Migrations and the API only need
the connection strings.

**Frontend gets 401 on every API call.**
The `<AuthBridge>` component must be mounted inside `<ClerkProvider>` so it
can register Clerk's `getToken` with `lib/tokenStore.ts` before the API
client fires its first request. Verify the provider order in `main.tsx`:
`ClerkProvider → QueryClientProvider → RouterProvider`.

---

## Deployment

Pointer-only here — no production runbook yet.

- **Backend** → Fly.io (single static binary, regions near Neon, `fly.toml`),
  Railway, or Render. Set all `backend/.env` variables as platform secrets.
  Use `DATABASE_URL_DIRECT` for the migration step in your release pipeline.
- **Frontend** → Vercel or Netlify. Build command `npm run build`, output
  directory `dist/`. Set the `VITE_*` vars in the platform's env UI before the
  build, since Vite inlines them at build time. Point `VITE_API_BASE_URL` at
  your deployed backend URL (or use a same-origin reverse proxy).
- **Database** → Neon. Use the `main` branch for production and a separate
  branch (e.g. `dev`) for staging.
- **CORS** → set `CORS_ALLOWED_ORIGIN` to the deployed frontend origin.

---

## Architecture documents

The plan files in [`docs/architecture/`](./docs/architecture/index.md) are the
source of truth for design decisions:

- [`docs/architecture/backend.md`](./docs/architecture/backend.md) — Go API
- [`docs/architecture/frontend.md`](./docs/architecture/frontend.md) — React SPA
- [`docs/architecture/integration.md`](./docs/architecture/integration.md) — monorepo, env, deploy

When implementation diverges from a plan, update the plan in the same change.
