# Go Backend Architecture Plan — AI Mock Interview Platform

## 1. Module layout

```
backend/
├── cmd/
│   └── server/
│       └── main.go                       # Composition root: wire config, db, llm, router, slog, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go                     # envconfig: PORT, DATABASE_URL, GEMINI_API_KEY, CLERK_SECRET_KEY, CLERK_JWKS_URL, LOG_LEVEL
│   ├── domain/                           # Pure types (no I/O), shared across http/store/llm
│   │   ├── interview.go                  # MockInterview, Question, GeneratedQA structs
│   │   ├── answer.go                     # UserAnswer, Evaluation
│   │   └── errors.go                     # Sentinel errors: ErrNotFound, ErrUnauthorized, ErrValidation, ErrLLM, ErrConflict
│   ├── auth/
│   │   ├── clerk.go                      # JWKS verifier, claim extraction
│   │   ├── middleware.go                 # chi middleware -> injects ClerkUserID into ctx
│   │   └── context.go                    # ctxKey type + UserIDFromContext(ctx)
│   ├── http/                             # Transport layer; handlers depend on services via interfaces
│   │   ├── router.go                     # chi router wiring; mounts /api/v1
│   │   ├── interviews.go                 # 5 handlers: create, list, get, submit answer, list feedback
│   │   ├── render.go                     # writeJSON, writeError, decodeAndValidate
│   │   ├── errors.go                     # mapErrToHTTP(err) -> (status, body)
│   │   └── middleware.go                 # request logging (slog), recoverer, request ID, CORS
│   ├── service/
│   │   └── interview.go                  # Orchestrator: validates -> calls LLM -> persists; defines Store + LLM interfaces here (consumer-side)
│   ├── store/
│   │   ├── postgres.go                   # *pgxpool.Pool factory
│   │   ├── interviews.go                 # InterviewRepo: Insert, ListByUser, GetByMockID
│   │   ├── answers.go                    # AnswerRepo: Insert, ListByMockID
│   │   └── queries/                      # sqlc-generated: queries.sql.go, models.go, db.go
│   ├── llm/
│   │   ├── gemini.go                     # GeminiClient struct; per-request session
│   │   ├── schema.go                     # responseSchema definitions (genai.Schema)
│   │   └── prompts.go                    # constants (no string-mutation hacks)
│   └── platform/
│       └── logger/
│           └── logger.go                 # slog handler init (JSON in prod, text in dev)
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── sqlc.yaml
├── go.mod
├── go.sum
├── Makefile                              # run, test, migrate, lint
└── .env.example
```

**Justification**: Consumer-defined interfaces in `service/` (Store, LLMClient) keep the dependency direction inward (handlers → services → interfaces; concrete `store` and `llm` plug in at `cmd/server/main.go`). `domain` carries pure types so no package imports `http` or `store` for type definitions. `internal/` prevents external import. `platform/` carves out cross-cutting infra.

## 2. Stack choices

| Concern | Choice | Reason |
|---|---|---|
| HTTP router | **chi v5** | Idiomatic `http.Handler`, sub-routers + middleware composition. |
| DB driver | **jackc/pgx/v5 + pgxpool** | Native Postgres types (JSONB, UUID, timestamptz), connection pool. |
| Type-safe SQL | **sqlc** | Generates strongly-typed Go from SQL — no ORM, no string queries. |
| Migrations | **golang-migrate/migrate v4** | Standard, file-based versioned migrations. |
| Config | **kelseyhightower/envconfig** | Struct-tagged, fail-fast on missing required vars. |
| Logger | **log/slog** (stdlib) | Stdlib structured logger. |
| Validation | **go-playground/validator/v10** | Struct-tag validation on request DTOs at HTTP boundary. |
| LLM SDK | **google.golang.org/genai** | Official Google Gen AI Go SDK; supports `ResponseSchema`/`ResponseMIMEType`. Fallback: `github.com/google/generative-ai-go/genai` if the new SDK isn't available. |
| Clerk JWT | **clerk/clerk-sdk-go/v2** | Official; bundles JWKS caching + claim parsing. |
| Testing | **stdlib `testing` + testify/require + testcontainers-go/postgres** | Table-driven; testcontainers for repo tests; mock LLM behind interface. |
| Errors | **stdlib `errors` + `fmt.Errorf("…: %w", err)`** | Sentinels + `errors.Is/As`. |

## 3. Postgres schema

```sql
-- migrations/0001_init.up.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    clerk_user_id   text        PRIMARY KEY,
    email           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mock_interviews (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    mock_id             text        NOT NULL UNIQUE,
    clerk_user_id       text        NOT NULL REFERENCES users(clerk_user_id) ON DELETE CASCADE,
    job_position        text        NOT NULL,
    job_description     text        NOT NULL,
    years_experience    smallint    NOT NULL CHECK (years_experience BETWEEN 0 AND 60),
    questions           jsonb       NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_mock_interviews_user_created
    ON mock_interviews (clerk_user_id, created_at DESC);

CREATE TABLE user_answers (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    mock_id         text        NOT NULL REFERENCES mock_interviews(mock_id) ON DELETE CASCADE,
    clerk_user_id   text        NOT NULL REFERENCES users(clerk_user_id) ON DELETE CASCADE,
    question_index  smallint    NOT NULL CHECK (question_index >= 0),
    question_text   text        NOT NULL,
    correct_answer  text        NOT NULL,
    user_answer     text        NOT NULL,
    rating          smallint    NOT NULL CHECK (rating BETWEEN 1 AND 10),
    feedback        text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (mock_id, question_index, clerk_user_id)
);
CREATE INDEX idx_user_answers_mock ON user_answers (mock_id, question_index);
```

Conventions: `text` for Clerk IDs (Clerk emits opaque strings), `timestamptz` not `timestamp`, JSONB for variable payloads, `gen_random_uuid()` for surrogate keys, indexes on every FK + ORDER BY column, FK `ON DELETE CASCADE` for ownership.

## 4. Gemini integration

**Use `gemini-2.5-flash` (NOT `gemini-1.5-flash` — deprecated).** Per-request session. Structured output via `ResponseMIMEType: "application/json"` + `ResponseSchema`. NO `replace("```json","")` hacks.

```go
// internal/llm/schema.go
import "google.golang.org/genai"

var QuestionGenSchema = &genai.Schema{
    Type: genai.TypeObject,
    Properties: map[string]*genai.Schema{
        "questions": {
            Type:     genai.TypeArray,
            MinItems: genai.Ptr[int64](5),
            MaxItems: genai.Ptr[int64](5),
            Items: &genai.Schema{
                Type: genai.TypeObject,
                Properties: map[string]*genai.Schema{
                    "question": {Type: genai.TypeString},
                    "answer":   {Type: genai.TypeString},
                },
                Required: []string{"question", "answer"},
            },
        },
    },
    Required: []string{"questions"},
}

var AnswerEvalSchema = &genai.Schema{
    Type: genai.TypeObject,
    Properties: map[string]*genai.Schema{
        "rating":   {Type: genai.TypeInteger, Minimum: genai.Ptr(1.0), Maximum: genai.Ptr(10.0)},
        "feedback": {Type: genai.TypeString, MinLength: genai.Ptr[int64](20)},
    },
    Required: []string{"rating", "feedback"},
}
```

```go
// internal/llm/gemini.go
type Client struct {
    sdk   *genai.Client
    model string  // "gemini-2.5-flash"
}

func (c *Client) GenerateQuestions(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error) {
    cfg := &genai.GenerateContentConfig{
        Temperature:      genai.Ptr[float32](1.0),
        TopP:             genai.Ptr[float32](0.95),
        ResponseMIMEType: "application/json",
        ResponseSchema:   QuestionGenSchema,
    }
    resp, err := c.sdk.Models.GenerateContent(ctx, c.model,
        genai.Text(buildQuestionPrompt(in)), cfg)
    if err != nil {
        return nil, fmt.Errorf("gemini generate: %w", errors.Join(domain.ErrLLM, err))
    }
    var out struct{ Questions []domain.GeneratedQA `json:"questions"` }
    if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
        return nil, fmt.Errorf("gemini decode: %w", domain.ErrLLM)
    }
    return out.Questions, nil
}
```

**SDK fallback:** If `google.golang.org/genai` is not reachable or its API differs at the time of implementation, use `github.com/google/generative-ai-go/genai` which also supports `ResponseMIMEType` + `ResponseSchema`. Confirm SDK existence with `go list -m google.golang.org/genai@latest` first.

## 5. Clerk JWT middleware

1. **Init**: `clerk.SetKey(cfg.ClerkSecretKey)`. SDK lazy-fetches and caches JWKS keyed by `kid`.
2. **Middleware**: reads `Authorization: Bearer <jwt>`, calls `jwt.Verify(ctx, &jwt.VerifyParams{Token: token})`, validates signature (RS256), `iss`, `exp`, `nbf`, `azp`.
3. **Claim extraction**: from `*clerk.SessionClaims`, take `Subject` (`user_…` ID).
4. **Context injection**: `ctx = auth.WithUserID(ctx, claims.Subject)`. Handlers retrieve via `auth.UserIDFromContext(ctx)`. Missing/invalid → 401 with low-cardinality message; technical error logged with slog.
5. **Lazy upsert**: on each authed request, `INSERT … ON CONFLICT (clerk_user_id) DO UPDATE SET updated_at = now()`. JIT-create flow without webhook.

## 6. API contract

All endpoints under `/api/v1`. All require Clerk JWT except `/healthz`. Errors use envelope `{"error":{"code":"…","message":"…"}}`.

### POST `/api/v1/interviews`
- Request: `{jobPosition (2-120), jobDescription (10-4000), yearsExperience (0-60)}`
- Response 201: `{mockId, jobPosition, jobDescription, yearsExperience, questions: [{question,answer}], createdAt}`
- Errors: 400, 401, 502 (LLM), 500

### GET `/api/v1/interviews`
- Query: `?limit=20&cursor=<created_at_iso>` (optional)
- Response 200: `{items: [InterviewSummary], nextCursor}`. Summary excludes `questions`.
- Errors: 401, 500

### GET `/api/v1/interviews/:mockId`
- Auth required, ownership enforced (`WHERE mock_id=$1 AND clerk_user_id=$2`)
- Response 200: full interview shape
- Errors: 401, 404 (covers not-found + not-owned), 500

### POST `/api/v1/interviews/:mockId/answers`
- Request: `{questionIndex (0-4), userAnswer (1-8000)}`
- Response 201: `{questionIndex, rating: int 1-10, feedback, createdAt}`
- Errors: 400, 401, 404, 409 (already answered), 502, 500

### GET `/api/v1/interviews/:mockId/feedback`
- Response 200: `{items: [SubmitAnswerResponse]}` ordered by `question_index ASC`
- Errors: 401, 404, 500

## 7. Error handling

Sentinels in `domain/errors.go`:
```go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrValidation   = errors.New("validation failed")
    ErrConflict     = errors.New("conflict")
    ErrLLM          = errors.New("llm failure")
)
```

Wrap with context. Repo translates `pgx.ErrNoRows` → `domain.ErrNotFound`; `*pgconn.PgError` code `23505` → `domain.ErrConflict`. HTTP layer maps via `errors.Is` to status codes (400/401/404/409/502/500). Handler logs once with slog (request_id, user_id, mock_id), returns low-cardinality user message. Services NEVER log + return; only outermost handler logs.

## 8. Testing

- **Handler tests** (`internal/http/*_test.go`): table-driven, `httptest.NewRecorder`, fake service implementing consumer-defined interface.
- **Service tests** (`internal/service/*_test.go`): mock Store + LLMClient interfaces with simple struct mocks. Verifies validation order, ownership checks, error mapping.
- **Repo tests** (`internal/store/*_test.go`): `testcontainers-go/postgres` per package; `golang-migrate` runs at suite setup; `t.Cleanup(container.Terminate)`.
- **E2E smoke**: boot server with fakes for Clerk + Gemini, run all 5 endpoints in sequence.
- Run: `go test ./... -race -count=1`. Coverage target ≥80% on `service/` + `store/`, 100% on `domain/errors.go` + `http/errors.go`.

## 9. go.mod entries

| Module | Reason |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/jackc/pgx/v5` | Postgres driver + pool |
| `github.com/golang-migrate/migrate/v4` | Migrations (CLI + library) |
| `github.com/kelseyhightower/envconfig` | Env config |
| `github.com/go-playground/validator/v10` | DTO validation |
| `google.golang.org/genai` (or `github.com/google/generative-ai-go/genai` fallback) | Gemini SDK |
| `github.com/clerk/clerk-sdk-go/v2` | JWT + JWKS |
| `github.com/google/uuid` | Mock ID generation |
| `github.com/rs/cors` | CORS middleware |
| `github.com/joho/godotenv` | Load .env in dev |
| `github.com/testcontainers/testcontainers-go` + `/modules/postgres` | Integration tests |
| `github.com/stretchr/testify` | Assertions |
| `golang.org/x/sync/errgroup` | Coordinated goroutines |

`log/slog`, `errors`, `net/http`, `encoding/json`, `context` are stdlib.
