# Architecture Plans — Index

This directory holds the authoritative pre-implementation plans for the
ai-mock-interview-v2 platform. The three plan files below are the source of
truth for "how the system should be built." Code and runtime behavior may
evolve, but design decisions are captured here.

## Plans

| File | Scope |
|------|-------|
| [`backend.md`](./backend.md) | Go API: chi router, sqlc + pgx, Clerk JWT middleware, Gemini 2.5 Flash structured output, Postgres schema, REST contract, error handling, testing strategy. |
| [`frontend.md`](./frontend.md) | React + Vite SPA: routing map, component breakdown, TanStack Query, Clerk SPA integration, speech/webcam hooks, Tailwind + shadcn/ui, Vitest setup. |
| [`integration.md`](./integration.md) | Monorepo glue: directory layout, env-var matrix, Neon Postgres setup, golang-migrate workflow, root Makefile, CORS policy, README structure, deployment pointers. |
| [`coach-report.md`](./coach-report.md) | Post-interview narrative coaching report: new `coach_reports` table, idempotent generation, markdown LLM contract, POST/GET endpoints. Phase 1 = non-streaming; SSE streaming deferred to Phase 2. |

## Conventions

- **Plans are authoritative.** When implementation diverges, update the plan
  in the same change. Do not let drift accumulate.
- **No retroactive notes.** These files describe intent up-front; postmortems,
  decision logs, and operational runbooks belong elsewhere (out of scope here).
- **Cross-references** between plans are by relative path (e.g.
  `[backend §6](./backend.md#6-api-contract)`).

## Adjacent root-level docs

- [`../../README.md`](../../README.md) — getting started, prerequisites, run/test/deploy.
- [`../../Makefile`](../../Makefile) — root delegating Makefile.
- [`../../.env.example`](../../.env.example) — pointer to per-app env templates.
