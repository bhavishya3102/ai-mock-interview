# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Behavioral guidelines

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

### 5. Check skills before building a new feature

**Before starting any new feature, run the `find-skills` skill** to discover whether an installable skill already covers part of the work (e.g. authentication via `clerk`, frontend polish via `frontend-design`, testing via `test-driven-development`, Postgres patterns via `supabase-postgres-best-practices`).

- Run `find-skills` with a short description of the feature you're about to build.
- If a relevant skill exists, invoke it (or note why you're not).
- If nothing fits, proceed normally — but the check is non-optional.

The point is to avoid re-inventing patterns Anthropic ships as skills, and to inherit their conventions instead of drifting.

## Repo shape

Two-app monorepo with **no workspace tool** (intentional — the apps share zero JS/TS code). A root delegating Makefile proxies into [backend/Makefile](backend/Makefile) or [frontend/](frontend/) npm scripts. Always invoke targets from the repo root.

- [backend/](backend/) — Go 1.22+ API
- [frontend/](frontend/) — React 18 + Vite SPA
- [docs/architecture/](docs/architecture/) — **authoritative** pre-implementation plans. When implementation diverges, update the plan in the same change.

## Common commands

Run from the repo root:

```bash
make install                           # go mod tidy + npm install
make dev-backend                       # air hot-reload, :8080
make dev-frontend                      # Vite, :5173 (proxies /api → :8080)
make test                              # backend race tests + Vitest single-run
make lint                              # golangci-lint + ESLint
make migrate-up | migrate-down
make migrate-create name=add_users_table
```

Backend-only (run inside [backend/](backend/) or via `make -C backend`):

```bash
go test ./... -race -count=1                    # all unit tests
go test -tags=integration ./...                 # integration tests (need DATABASE_URL_TEST)
go test ./internal/service -run TestName -v     # single test
make gen                                        # sqlc generate (after editing queries.sql)
make migrate-force version=N                    # recover from dirty migration state
```

Frontend-only (inside [frontend/](frontend/)):

```bash
npm test -- --run                       # single run
npm test -- src/api/interviews.test.ts  # single file
npm run typecheck                       # tsc --noEmit
npm run build                           # tsc -b && vite build
```

## Architecture

### Backend — clean layering with inward dependencies

Composition root is [backend/cmd/server/main.go](backend/cmd/server/main.go). The dependency direction is **handlers → services → interfaces**, with concrete implementations injected at startup. Never short-circuit this — e.g. handlers must not import `store/` directly.

- [internal/domain/](backend/internal/domain/) — pure types and **sentinel errors** (`ErrNotFound`, `ErrUnauthorized`, `ErrValidation`, `ErrConflict`, `ErrLLM`). Must not import http/store/llm. Wrap with `fmt.Errorf("...: %w", err)` and inspect with `errors.Is` at the HTTP boundary to map to status codes. Error messages are low-cardinality so APM tools group them — variable data goes into structured log attributes via slog.
- [internal/service/](backend/internal/service/) — orchestrators. **Defines** `Store` and `LLMClient` interfaces on the consumer side (Go interface guideline). Validation order: identity → domain rules → expensive LLM calls.
- [internal/store/](backend/internal/store/) — `pgxpool` repos + sqlc-generated code in [queries/](backend/internal/store/queries/). Repos translate driver errors (`pgx.ErrNoRows`, `*pgconn.PgError` `23505`) to domain sentinels.
- [internal/http/](backend/internal/http/) — chi v5 handlers, render helpers, error mapping. Routes mounted under `/api/v1`. Auth middleware (`auth.Middleware`) wraps the v1 subrouter; `/healthz` is unauthenticated.
- [internal/auth/](backend/internal/auth/) — Clerk JWKS verifier + middleware; injects clerk user ID into request context.
- [internal/llm/](backend/internal/llm/) — Gemini 2.5 Flash client. Uses **structured output** via `ResponseSchema` + `ResponseMIMEType` — see [schema.go](backend/internal/llm/schema.go) for response schemas, [prompts.go](backend/internal/llm/prompts.go) for templates.
- [internal/config/](backend/internal/config/) — envconfig loader; `cfg.IsProduction()` gates dotenv load.

The `combinedStore` adapter in [main.go](backend/cmd/server/main.go) composes `InterviewRepo` + `AnswerRepo` so the service sees a single `Store` dependency.

### sqlc workflow

[backend/sqlc.yaml](backend/sqlc.yaml) generates `pgx/v5`-compatible Go from [internal/store/queries/queries.sql](backend/internal/store/queries/queries.sql) into [internal/store/queries/](backend/internal/store/queries/). After editing the SQL: `make -C backend gen`. Schema source for sqlc is the migrations directory itself. Settings worth noting: `emit_pointers_for_null_types: true`, `emit_interface: true`.

### Migrations

`golang-migrate` paired files in [backend/migrations/](backend/migrations/). Migrations run against `DATABASE_URL_DIRECT` (non-pooled) because PgBouncer transaction-mode breaks advisory locks. The backend Makefile sets `MIGRATE_DSN ?= $(DATABASE_URL_DIRECT)`.

### Frontend — SPA with strict provider order

Entry [frontend/src/main.tsx](frontend/src/main.tsx) — provider order is **load-bearing**: `ClerkProvider → QueryClientProvider → RouterProvider`. The `<AuthBridge>` registers Clerk's `getToken` with [lib/tokenStore.ts](frontend/src/lib/tokenStore.ts) before the API client fires its first request; breaking this order produces 401s on every call.

- [src/routes/](frontend/src/routes/) — `createBrowserRouter` config in [index.tsx](frontend/src/routes/index.tsx). Auth-gated routes wrapped in `<ProtectedRoute>` under `<DashboardLayout>`. `InterviewSession` and `Feedback` are lazy-loaded.
- [src/api/](frontend/src/api/) — typed fetch wrapper + endpoint functions. [client.ts](frontend/src/api/client.ts) attaches `Authorization: Bearer <token>` from `tokenStore`, parses `ApiErrorBody`, throws `ApiError`/`UnauthorizedError`. Use these — don't call `fetch` directly.
- [src/lib/env.ts](frontend/src/lib/env.ts) — Vite only exposes `VITE_*` vars; validated here.
- `@/` alias maps to `src/` ([vite.config.ts](frontend/vite.config.ts)).

API base path is `VITE_API_BASE_URL` (default `/api/v1`); Vite dev server proxies `/api/*` → `:8080` so the same code path works in dev and behind a reverse proxy in prod. `AllowCredentials` is **false** — Clerk tokens travel in `Authorization`, not cookies.

### Service shape

The interview flow: candidate describes a role → Gemini generates tailored questions → candidate records spoken answers (webcam + speech-to-text) → LLM rates each response with structured feedback. Endpoints under `/api/v1/interviews`:

- `POST /` create, `GET /` list, `GET /{mockId}` detail
- `POST /{mockId}/answers` submit, `POST /{mockId}/transcribe` audio
- `GET /{mockId}/feedback` aggregated evaluation

## Testing discipline

This project follows **TDD**. Iron law: no production code without a failing test first. Red → verify red → minimal green → verify green → refactor. Tests written after implementation pass immediately and prove nothing.

- **Backend.** Unit tests colocated (`*_test.go`). Integration tests gated by `//go:build integration` — they need either `testcontainers-go/postgres` or a Neon dev branch via `DATABASE_URL_TEST`. Use `testify/require` and `httptest`. Coverage targets: ≥80% on `service/` and `store/`, 100% on `domain/errors.go` and `http/errors.go`.
- **Frontend.** Vitest + RTL, tests colocated as `Component.test.tsx`. Mock the network at the boundary with **MSW** ([src/test/msw/](frontend/src/test/msw/)); do not mock internal modules.

## Environment

`.env` files live next to the apps that consume them and are gitignored — only `.env.example` files are committed. Both URLs are required in [backend/.env](backend/.env): `DATABASE_URL` (pooled, runtime) and `DATABASE_URL_DIRECT` (non-pooled, migrations).






