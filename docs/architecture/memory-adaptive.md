# Memory & Adaptive — Architecture Plan (MVP slice)

## 1. Goal

Give the coach report a **memory across interviews** so a returning user
sees narrative context like:

> *"Across your last three Frontend interviews, your average on React
> hooks has risen from 5.2 → 6.8 → 7.9 — keep going. The recurring weak
> spot is `useEffect` cleanup, which you've stumbled on in two of three
> sessions."*

Without this, every coach report reads in isolation — a 3rd-time user
sees the same advice as a 1st-time user, missing the most actionable
feedback (longitudinal trend) entirely.

This plan is the **MVP slice** of a larger memory + adaptive vision:

| In scope (this plan) | Deferred |
|---|---|
| pgvector extension + embedding column on `user_answers` | `user_skill_profile` table (compute on demand for now) |
| Best-effort background embed on every new answer | Backfill of pre-existing answers (lazy: only new ones get vectors) |
| Cross-interview history enrichment for coach reports | Topic classification (use raw text similarity) |
| Recurring weakness retrieval via cosine similarity | Adaptive difficulty in `CreateInterview` (separate sub-phase) |
| Backend-only changes | Frontend "trend" badges on Feedback page |

## 2. Data model

Additive to the existing `user_answers` table — no schema changes to
other tables.

```sql
-- migrations/0005_add_embeddings.up.sql
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE user_answers
    ADD COLUMN embedding vector(768);

-- HNSW for approximate-nearest-neighbour search; cosine distance
-- matches the metric Gemini's text-embedding-004 is trained for.
CREATE INDEX idx_user_answers_embedding
    ON user_answers
    USING hnsw (embedding vector_cosine_ops);
```

Conventions:
- **Nullable column**: existing answers have no embedding and will not
  be backfilled in this MVP. New answers get embedded asynchronously.
  Semantic-similarity queries naturally skip rows with NULL embeddings.
- **768 dimensions**: matches `text-embedding-004` default. Smaller
  reduced-dim variants are possible but pollute the schema if we change
  models — sticking with the default keeps the migration simple.
- **HNSW (not IVF-Flat)**: better recall on small-to-medium datasets,
  no `ANALYZE` step needed before queries return useful results.

No new sentinel errors. `ErrLLM` covers embedding failures.

## 3. LLM contract

```go
// backend/internal/llm/gemini.go (additive method)
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error)
```

- Model: `models/text-embedding-004` (768-dim, free tier eligible).
- `TaskType: "RETRIEVAL_DOCUMENT"` when embedding a stored answer,
  `RETRIEVAL_QUERY` when embedding a role to search against. (Different
  task types tune the model for asymmetric retrieval.)
- Errors wrapped with `ErrLLM`.

Cost: roughly $0.000005 per answer (~500 chars) — three orders of
magnitude cheaper than the existing per-answer generation call, so the
budget impact is rounding error.

## 4. Embedding lifecycle

```
                   SubmitAnswer (HTTP)
                          ↓
                  Validate + LLM evaluate (existing)
                          ↓
                  UpsertAnswer to DB (existing)
                          ↓
                  Re-read row (existing)
                          ↓
                  🆕 Launch goroutine(detached ctx):
                          embed(question + " " + user_answer)
                          UpdateAnswerEmbedding(row.id, vec)
                          ↓
                  Return saved answer to caller (existing)
                          ↓
                  HTTP 201
```

Key rules:
- **Detached context** (`context.WithoutCancel(ctx)` available since Go
  1.21): the request context is cancelled the moment the HTTP response
  is sent. Without detaching, the goroutine would die mid-embed and
  leave a NULL row.
- **Best-effort only**: embedding failures are logged via `slog.Warn`
  and dropped. The user-visible submit is already committed; failing it
  retroactively on an embed error would be worse UX than missing one
  vector in the table.
- **Idempotency**: re-embedding an answer is a no-op (UPDATE … WHERE id).
  If a re-submit overwrites the answer text, the next embed pass
  replaces the vector.

## 5. Store query additions

```sql
-- name: UpdateAnswerEmbedding :exec
UPDATE user_answers
SET embedding = $2
WHERE id = $1;

-- name: FindSimilarWeakAnswers :many
-- "What past answers, with rating below the threshold, are most
--  semantically similar to the role we're now coaching for?"
SELECT mock_id, question_index, question_text, user_answer, rating,
       feedback, created_at
FROM user_answers
WHERE clerk_user_id = $1
  AND embedding IS NOT NULL
  AND rating < $3
ORDER BY embedding <-> $2
LIMIT $4;

-- name: ListUserInterviewSummaries :many
-- Per-interview rollup: mock_id, role, attempt date, average rating.
-- Used to surface "this is your Nth interview, avg X" context — does
-- NOT require any embeddings.
SELECT mi.mock_id, mi.job_position, mi.created_at,
       COALESCE(AVG(ua.rating)::numeric(3,1), 0) AS avg_rating,
       COUNT(ua.id) AS answered_count
FROM mock_interviews mi
LEFT JOIN user_answers ua
       ON ua.mock_id = mi.mock_id AND ua.clerk_user_id = mi.clerk_user_id
WHERE mi.clerk_user_id = $1
GROUP BY mi.mock_id, mi.job_position, mi.created_at
ORDER BY mi.created_at DESC
LIMIT $2;
```

`<->` is pgvector's cosine-distance operator (lower = more similar)
when the index uses `vector_cosine_ops`.

## 6. Service enrichment flow

```go
// service/interview.go (modified)
domain.CoachReportSeed{
    JobPosition:     mi.JobPosition,
    YearsExperience: mi.YearsExperience,
    Answers:         answers,
    PastInterviews:  history,           // 🆕
    RecurringWeak:   weakHits,          // 🆕
}
```

Before the existing LLM call, the service runs three additional steps:

1. `ListUserInterviewSummaries(userID, 5)` — cheap pure-SQL query, no
   LLM cost, always populated for returning users.
2. `llm.Embed(jobPosition + " " + jobDescription)` — one embedding call
   to get the search vector for the current role.
3. `FindSimilarWeakAnswers(userID, vec, ratingThreshold=6, limit=5)` —
   pgvector cosine-distance query.

Failures of step 2 or 3 are logged and the enrichment fields are left
empty — the coach report still generates, just without the historical
context. This is the same robustness rule as the embedding lifecycle.

`PastInterviews` deliberately includes the *current* interview's mockID
so the prompt can refer to "this attempt" by index ("your 4th Frontend
interview").

## 7. Prompt extension

A new section is appended to `buildCoachReportPrompt` **only when
`PastInterviews` has at least one row beyond the current one**:

```
HISTORICAL CONTEXT (for the Progress Tracking section below):
- 2026-03-12  Frontend Engineer    avg 5.8/10
- 2026-04-02  Frontend Engineer    avg 6.4/10
- 2026-05-10  Frontend Engineer    avg 7.1/10  ← current

Recurring weak answers from prior interviews on similar topics:
- "What does useEffect cleanup do?" — got 4/10, feedback: "confused mount with unmount"
- "When should you use useMemo?" — got 5/10, feedback: "no mention of dependencies"

Use this in a new section titled '### Progress Tracking' between
'### Areas to Improve' and '### Recommended Next Steps'. Cite specific
numerical movement; if a recurring weak topic was finally answered well
this time, name it as a win.
```

For first-time users (no past rows), the section is omitted entirely
and the prompt is byte-identical to today's coach report prompt — no
regression risk for new users.

## 8. Testing

| Layer | Cases |
|---|---|
| `llm/prompts_test.go` | enriched prompt includes history table; first-time user prompt unchanged; speech metrics still present |
| `service/interview_test.go` | `SubmitAnswer` launches embed goroutine on success; embed failure does NOT fail the submit; `GenerateCoachReport` populates seed with history when store returns rows; missing embedding for role gracefully falls back to no enrichment; existing race + ownership tests still pass |
| `store/answers.go` | hand-written, covered transitively via service tests (per existing project convention) |

No new HTTP-layer tests — the public contract is unchanged.

## 9. Out of scope (Phase 4 successors)

- **`user_skill_profile` aggregation table**: computing on demand keeps
  state minimal. Promote to a table only if query latency becomes a
  problem (won't matter under ~10k answers per user).
- **Backfill of pre-existing answers**: optional later job; the coach
  report degrades gracefully without historical embeddings since
  `PastInterviews` rollups don't need them.
- **Adaptive difficulty in `CreateInterview`**: next sub-phase. Will
  reuse the same embedding column to find weak topics and inject them
  into the question-generation prompt.
- **Topic classification**: raw-text cosine similarity gets us 80% of
  the value at 0% of the operational cost.
- **Frontend "improving" badges**: backend ships first; UI work tracked
  separately.
