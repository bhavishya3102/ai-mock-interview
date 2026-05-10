# Integration & DevOps Architecture Plan

## 1. Top-level monorepo layout

```
ai-mock-interview-v2/
├── README.md
├── Makefile                   # Root delegating Makefile (proxies to backend/)
├── .gitignore
├── .editorconfig              # 2-space JS/TS/MD, tabs for Go, LF endings
├── .env.example
├── docs/
│   └── architecture/
│       ├── backend.md
│       ├── frontend.md
│       └── integration.md
├── backend/
│   ├── go.mod
│   ├── Makefile
│   ├── .air.toml
│   ├── .env.example
│   ├── cmd/api/main.go
│   ├── internal/...
│   ├── migrations/
│   ├── sqlc.yaml
│   └── tmp/                   # gitignored
└── frontend/
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── .env.example
    ├── index.html
    └── src/...
```

**No npm workspaces / Turborepo / Nx.** Two apps have zero shared JS/TS code (Go ↔ React), independent dependency graphs, independent build/test commands. A workspace tool would add ceremony with no payoff.

## 2. Environment variables

| File | Variables |
|------|-----------|
| `backend/.env` | `DATABASE_URL` (pooled), `DATABASE_URL_DIRECT` (non-pooled, for migrations), `CLERK_SECRET_KEY`, `CLERK_PUBLISHABLE_KEY`, `GEMINI_API_KEY`, `PORT=8080`, `CORS_ALLOWED_ORIGIN=http://localhost:5173`, `APP_ENV=development`, `LOG_LEVEL=debug` |
| `frontend/.env` | `VITE_CLERK_PUBLISHABLE_KEY`, `VITE_API_BASE_URL=/api/v1` |
| `backend/.env.example` | Same keys, redacted (`pk_test_xxx`, `postgresql://user:pass@host/db?sslmode=require`) |
| `frontend/.env.example` | Same |
| `.env.example` (root) | Pointer to subdir examples |

Backend loads `.env` via `github.com/joho/godotenv` only when `APP_ENV != production`. Vite only exposes `VITE_*` to client bundle. `.env` files gitignored.

## 3. Neon Postgres setup walkthrough

1. Sign up at https://console.neon.tech (GitHub OAuth fastest).
2. Create project: name `ai-mock-interview`, region closest, Postgres 16.
3. Auto-created branch is `main`. Create `dev` branch (Branches → New). Use `dev` for local work.
4. On `dev` branch's **Connection Details**, copy **Pooled connection** string:
   ```
   postgresql://USER:PASSWORD@ep-xxx-pooler.region.aws.neon.tech/neondb?sslmode=require
   ```
5. Paste into `backend/.env` as `DATABASE_URL=...`.
6. **Crucial:** keep `?sslmode=require`. For migrations, also keep a non-pooled "direct" URL as `DATABASE_URL_DIRECT` (PgBouncer transaction-mode breaks migrate's advisory locks).
7. Verify: `psql "$DATABASE_URL" -c "select 1"` (or any pg client).

## 4. Migrations workflow (golang-migrate)

Install:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Layout:
```
backend/migrations/
├── 0001_init.up.sql
├── 0001_init.down.sql
└── ...
```

Make targets:
```make
MIGRATE_DSN ?= $(DATABASE_URL_DIRECT)

migrate-up:
	migrate -path migrations -database "$(MIGRATE_DSN)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_DSN)" down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=add_xxx" && exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

migrate-force:
	migrate -path migrations -database "$(MIGRATE_DSN)" force $(version)
```

## 5. Local dev workflow

- Terminal 1 (backend): `cd backend && make dev` — runs `air`, watches `**/*.go`, rebuilds to `tmp/main`. Listens on `:8080`.
- Terminal 2 (frontend): `cd frontend && npm run dev` — Vite on `:5173`.
- Optional Terminal 3: `make migrate-up` from `backend/`.

**Vite proxy** (chosen over CORS in dev):
```ts
server: {
  port: 5173,
  proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } }
}
```
`VITE_API_BASE_URL=/api/v1` and `fetch('/api/v1/interviews')` works in dev + prod (with reverse proxy in deployment). Backend keeps CORS middleware as defence-in-depth for staging/prod.

## 6. Makefile

**Root `Makefile`:**
```make
.PHONY: dev-backend dev-frontend test lint
dev-backend:  ; $(MAKE) -C backend dev
dev-frontend: ; cd frontend && npm run dev
test:         ; $(MAKE) -C backend test && cd frontend && npm test -- --run
lint:         ; $(MAKE) -C backend lint && cd frontend && npm run lint
```

**`backend/Makefile`:**
```make
include .env
export

dev:           ; air
build:         ; go build -o bin/api ./cmd/server
test:          ; go test ./... -race -count=1
test-cover:    ; go test ./... -race -coverprofile=coverage.out
lint:          ; golangci-lint run ./...
gen:           ; sqlc generate
migrate-up:    ...
migrate-down:  ...
migrate-create:...
```

`include .env` + `export` makes `DATABASE_URL` available to `migrate` without wrapper.

## 7. CORS

`github.com/rs/cors`:
- `AllowedOrigins`: split `CORS_ALLOWED_ORIGIN` by comma → `["http://localhost:5173"]` dev, `["https://app.example.com"]` prod.
- `AllowedMethods`: GET, POST, PATCH, DELETE, OPTIONS.
- `AllowedHeaders`: Authorization, Content-Type.
- `AllowCredentials`: false (Clerk tokens in `Authorization: Bearer`, not cookies).
- `MaxAge`: 300.

## 8. Testing

**Go (TDD):**
- Unit colocated: `internal/interview/service_test.go`.
- Integration: `*_integration_test.go` with `//go:build integration` tag, hits Neon `dev` branch via `DATABASE_URL_TEST` (or testcontainers).
- `testify/require`, `httptest`.
- Run: `go test ./... -race -count=1`; `go test -tags=integration ./...`.

**React:**
- Vitest + RTL; tests colocated as `Component.test.tsx`.
- MSW for backend mocks at network boundary.
- Run: `npm test`, `npm test -- --run`.

CI later: GitHub Actions, jobs `backend-test`, `frontend-test`, `lint`. Stub `.github/workflows/ci.yml` when ready.

## 9. README structure

1. Overview
2. Prerequisites — Go 1.22+, Node 20.9+, `migrate`, `air`, `golangci-lint`, Neon account, Clerk account, Google AI Studio key.
3. Setup — `git clone`, copy `.env.example` files, fill keys.
4. Environment variables (table from §2).
5. Database migrations (`make migrate-up`).
6. Run (two-terminal).
7. Project structure.
8. Testing.
9. Troubleshooting (Neon SSL, Clerk JWT clock skew, air not reloading, CORS in staging).
10. Deployment pointer.

## 10. Deployment notes

Backend → Fly.io (single binary, regions near Neon, `fly.toml`); Railway/Render also fine. Frontend → Vercel/Netlify (static SPA from `frontend/`, set `VITE_*` vars, point to backend URL via `VITE_API_BASE_URL`). DB stays on Neon. Set `CORS_ALLOWED_ORIGIN` to deployed frontend URL.

## 11. Git setup

Single repo at monorepo root. Initialize: `git init && git add . && git commit -m "chore: scaffold monorepo"`.

`.gitignore` (root) — already created — covers node_modules/, dist/, .vite/, bin/, tmp/, .env (with `!.env.example` allowlist), editor/OS junk.
