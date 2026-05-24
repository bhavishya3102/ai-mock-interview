package store

import (
	"context"
	"fmt"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// AnswerRepo persists user_answers.
type AnswerRepo struct {
	pool *pgxpool.Pool
}

func NewAnswerRepo(pool *pgxpool.Pool) *AnswerRepo {
	return &AnswerRepo{pool: pool}
}

// UpsertAnswer writes an evaluation row, replacing any prior row for the same
// (mock_id, question_index, clerk_user_id) — re-attempts overwrite the answer,
// rating and feedback. created_at is intentionally preserved so it remains
// "first attempted at" for analytics.
func (r *AnswerRepo) UpsertAnswer(ctx context.Context, a domain.UserAnswer) error {
	const q = `INSERT INTO user_answers (
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
	    long_pause_count  = EXCLUDED.long_pause_count`

	if _, err := r.pool.Exec(
		ctx, q,
		a.MockID,
		a.ClerkUserID,
		a.QuestionIndex,
		a.QuestionText,
		a.CorrectAnswer,
		a.UserAnswer,
		a.Rating,
		a.Feedback,
		a.FillerCount,
		a.WordsPerMinute,
		a.LongPauseCount,
	); err != nil {
		return fmt.Errorf("upsert answer: %w", err)
	}
	return nil
}

// UpdateAnswerEmbedding stores the model-produced vector for a previously
// upserted answer row. Identified by the (mock_id, clerk_user_id,
// question_index) unique tuple so callers don't need to track the row
// UUID. Re-running with a different vector is a normal flow when the
// answer text is re-submitted.
func (r *AnswerRepo) UpdateAnswerEmbedding(
	ctx context.Context,
	mockID string,
	clerkUserID string,
	questionIndex int,
	vec []float32,
) error {
	const q = `UPDATE user_answers
	           SET embedding = $4
	           WHERE mock_id = $1 AND clerk_user_id = $2 AND question_index = $3`
	if _, err := r.pool.Exec(ctx, q,
		mockID, clerkUserID, questionIndex, pgvector.NewVector(vec),
	); err != nil {
		return fmt.Errorf("update answer embedding: %w", err)
	}
	return nil
}

// FindSimilarWeakAnswers returns the user's low-rated past answers that
// are most semantically similar to the supplied query vector. Used to
// surface recurring weaknesses in the coach report. Rows without an
// embedding are skipped (their LEFT-JOIN style WHERE clause filters them
// out implicitly via IS NOT NULL).
//
// ratingThreshold is exclusive — only answers with rating < threshold
// are returned. limit caps the count.
func (r *AnswerRepo) FindSimilarWeakAnswers(
	ctx context.Context,
	clerkUserID string,
	queryVec []float32,
	ratingThreshold int,
	limit int,
) ([]domain.WeakAnswerHit, error) {
	const q = `SELECT mock_id, question_text, user_answer, rating, feedback, created_at
	           FROM user_answers
	           WHERE clerk_user_id = $1
	             AND embedding IS NOT NULL
	             AND rating < $3
	           ORDER BY embedding <-> $2
	           LIMIT $4`

	rows, err := r.pool.Query(ctx, q,
		clerkUserID, pgvector.NewVector(queryVec), ratingThreshold, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find similar weak answers: %w", err)
	}
	defer rows.Close()

	out := []domain.WeakAnswerHit{}
	for rows.Next() {
		var h domain.WeakAnswerHit
		if err := rows.Scan(
			&h.MockID,
			&h.QuestionText,
			&h.UserAnswer,
			&h.Rating,
			&h.Feedback,
			&h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan weak answer row: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weak answer rows: %w", err)
	}
	return out, nil
}

// ListAnswersByMockID returns the user's answers for an interview, ordered
// by question_index ASC.
func (r *AnswerRepo) ListAnswersByMockID(
	ctx context.Context,
	mockID string,
	clerkUserID string,
) ([]domain.UserAnswer, error) {
	const q = `SELECT mock_id, clerk_user_id, question_index, question_text,
	                  correct_answer, user_answer, rating, feedback, created_at,
	                  filler_count, words_per_minute, long_pause_count
	           FROM user_answers
	           WHERE mock_id = $1 AND clerk_user_id = $2
	           ORDER BY question_index ASC`

	rows, err := r.pool.Query(ctx, q, mockID, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("list answers: %w", err)
	}
	defer rows.Close()

	out := []domain.UserAnswer{}
	for rows.Next() {
		var a domain.UserAnswer
		if err := rows.Scan(
			&a.MockID,
			&a.ClerkUserID,
			&a.QuestionIndex,
			&a.QuestionText,
			&a.CorrectAnswer,
			&a.UserAnswer,
			&a.Rating,
			&a.Feedback,
			&a.CreatedAt,
			&a.FillerCount,
			&a.WordsPerMinute,
			&a.LongPauseCount,
		); err != nil {
			return nil, fmt.Errorf("scan answer row: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answer rows: %w", err)
	}
	return out, nil
}
