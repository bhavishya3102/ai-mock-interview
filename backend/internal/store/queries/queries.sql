-- Queries for sqlc generation. Even though the repo files use hand-written
-- pgxpool calls today, this file is the source of truth for generated code
-- via `make gen`. Keep names and shapes in sync.

-- name: UpsertUser :exec
INSERT INTO users (clerk_user_id)
VALUES ($1)
ON CONFLICT (clerk_user_id)
DO UPDATE SET updated_at = now();

-- name: SetUserResume :exec
UPDATE users
SET resume_text = $2, resume_uploaded_at = now()
WHERE clerk_user_id = $1;

-- name: GetUserResume :one
SELECT resume_text, resume_uploaded_at
FROM users
WHERE clerk_user_id = $1;

-- name: ClearUserResume :exec
UPDATE users
SET resume_text = NULL, resume_uploaded_at = NULL
WHERE clerk_user_id = $1;

-- name: CreateInterview :exec
INSERT INTO mock_interviews (
    mock_id, clerk_user_id, job_position, job_description,
    years_experience, questions
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListInterviewsByUser :many
SELECT mock_id, job_position, job_description, years_experience, created_at
FROM mock_interviews
WHERE clerk_user_id = $1
ORDER BY created_at DESC, mock_id DESC
LIMIT $2;

-- name: GetInterviewByMockID :one
SELECT mock_id, clerk_user_id, job_position, job_description,
       years_experience, questions, created_at
FROM mock_interviews
WHERE mock_id = $1 AND clerk_user_id = $2;

-- name: UpsertAnswer :exec
INSERT INTO user_answers (
    mock_id, clerk_user_id, question_index, question_text,
    correct_answer, user_answer, rating, feedback,
    filler_count, words_per_minute, long_pause_count
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (mock_id, question_index, clerk_user_id) DO UPDATE
SET question_text     = EXCLUDED.question_text,
    correct_answer    = EXCLUDED.correct_answer,
    user_answer       = EXCLUDED.user_answer,
    rating            = EXCLUDED.rating,
    feedback          = EXCLUDED.feedback,
    filler_count      = EXCLUDED.filler_count,
    words_per_minute  = EXCLUDED.words_per_minute,
    long_pause_count  = EXCLUDED.long_pause_count;

-- name: ListAnswersByMockID :many
SELECT mock_id, clerk_user_id, question_index, question_text,
       correct_answer, user_answer, rating, feedback, created_at,
       filler_count, words_per_minute, long_pause_count
FROM user_answers
WHERE mock_id = $1 AND clerk_user_id = $2
ORDER BY question_index ASC;

-- name: InsertCoachReport :exec
INSERT INTO coach_reports (mock_id, content, tokens_used, model)
VALUES ($1, $2, $3, $4);

-- name: GetCoachReportByMockID :one
SELECT cr.mock_id, cr.content, cr.tokens_used, cr.model, cr.created_at
FROM coach_reports cr
JOIN mock_interviews mi ON mi.mock_id = cr.mock_id
WHERE cr.mock_id = $1 AND mi.clerk_user_id = $2;
