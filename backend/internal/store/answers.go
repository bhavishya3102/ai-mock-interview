package store

import (
	"context"
	"fmt"

	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
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
	    correct_answer, user_answer, rating, feedback
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (mock_id, question_index, clerk_user_id) DO UPDATE
	SET question_text  = EXCLUDED.question_text,
	    correct_answer = EXCLUDED.correct_answer,
	    user_answer    = EXCLUDED.user_answer,
	    rating         = EXCLUDED.rating,
	    feedback       = EXCLUDED.feedback`

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
	); err != nil {
		return fmt.Errorf("upsert answer: %w", err)
	}
	return nil
}

// ListAnswersByMockID returns the user's answers for an interview, ordered
// by question_index ASC.
func (r *AnswerRepo) ListAnswersByMockID(
	ctx context.Context,
	mockID string,
	clerkUserID string,
) ([]domain.UserAnswer, error) {
	const q = `SELECT mock_id, clerk_user_id, question_index, question_text,
	                  correct_answer, user_answer, rating, feedback, created_at
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
