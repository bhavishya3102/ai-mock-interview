package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres unique-violation SQLSTATE.
const pgUniqueViolation = "23505"

// InterviewRepo persists mock_interviews + manages the lazy users upsert.
type InterviewRepo struct {
	pool *pgxpool.Pool
}

func NewInterviewRepo(pool *pgxpool.Pool) *InterviewRepo {
	return &InterviewRepo{pool: pool}
}

// UpsertUser ensures a row exists in users for clerkUserID. Idempotent.
// Called lazily on the first authenticated request to a protected endpoint
// (no webhook needed for JIT user creation).
func (r *InterviewRepo) UpsertUser(ctx context.Context, clerkUserID string) error {
	const q = `INSERT INTO users (clerk_user_id)
	           VALUES ($1)
	           ON CONFLICT (clerk_user_id)
	           DO UPDATE SET updated_at = now()`
	if _, err := r.pool.Exec(ctx, q, clerkUserID); err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

// SetUserResume stores the candidate's extracted resume text and stamps the
// upload time. Assumes the users row already exists (callers UpsertUser first).
func (r *InterviewRepo) SetUserResume(ctx context.Context, clerkUserID, resumeText string) error {
	const q = `UPDATE users
	           SET resume_text = $2, resume_uploaded_at = now()
	           WHERE clerk_user_id = $1`
	if _, err := r.pool.Exec(ctx, q, clerkUserID, resumeText); err != nil {
		return fmt.Errorf("set user resume: %w", err)
	}
	return nil
}

// GetUserResume returns the stored resume text and its upload time. When no
// resume is on file, returns empty text and a zero time with a nil error —
// "no resume" is a normal state, not an error.
func (r *InterviewRepo) GetUserResume(ctx context.Context, clerkUserID string) (string, time.Time, error) {
	const q = `SELECT resume_text, resume_uploaded_at
	           FROM users WHERE clerk_user_id = $1`
	var (
		text       *string
		uploadedAt *time.Time
	)
	if err := r.pool.QueryRow(ctx, q, clerkUserID).Scan(&text, &uploadedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, nil
		}
		return "", time.Time{}, fmt.Errorf("get user resume: %w", err)
	}
	if text == nil || uploadedAt == nil {
		return "", time.Time{}, nil
	}
	return *text, *uploadedAt, nil
}

// ClearUserResume removes the stored resume, returning the user to the
// no-resume state.
func (r *InterviewRepo) ClearUserResume(ctx context.Context, clerkUserID string) error {
	const q = `UPDATE users
	           SET resume_text = NULL, resume_uploaded_at = NULL
	           WHERE clerk_user_id = $1`
	if _, err := r.pool.Exec(ctx, q, clerkUserID); err != nil {
		return fmt.Errorf("clear user resume: %w", err)
	}
	return nil
}

// CreateInterview inserts a new mock interview row.
func (r *InterviewRepo) CreateInterview(ctx context.Context, m domain.MockInterview) error {
	questions, err := json.Marshal(m.Questions)
	if err != nil {
		return fmt.Errorf("marshal questions: %w", err)
	}

	const q = `INSERT INTO mock_interviews (
	    mock_id, clerk_user_id, job_position, job_description,
	    years_experience, questions
	) VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := r.pool.Exec(
		ctx, q,
		m.MockID,
		m.ClerkUserID,
		m.JobPosition,
		m.JobDescription,
		m.YearsExperience,
		string(questions),
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("create interview: %w", domain.ErrConflict)
		}
		return fmt.Errorf("create interview: %w", err)
	}
	return nil
}

// ListInterviewsByUser returns the user's interviews newest-first (no Questions).
func (r *InterviewRepo) ListInterviewsByUser(
	ctx context.Context,
	clerkUserID string,
	limit int,
) ([]domain.InterviewSummary, error) {
	const q = `SELECT mock_id, job_position, job_description, years_experience, created_at
	           FROM mock_interviews
	           WHERE clerk_user_id = $1
	           ORDER BY created_at DESC, mock_id DESC
	           LIMIT $2`

	rows, err := r.pool.Query(ctx, q, clerkUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list interviews: %w", err)
	}
	defer rows.Close()

	out := []domain.InterviewSummary{}
	for rows.Next() {
		var s domain.InterviewSummary
		if err := rows.Scan(
			&s.MockID,
			&s.JobPosition,
			&s.JobDescription,
			&s.YearsExperience,
			&s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan interview row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate interview rows: %w", err)
	}
	return out, nil
}

// GetInterviewByMockID fetches one interview owned by clerkUserID. Ownership
// is enforced in the WHERE clause — a not-owned row returns ErrNotFound, the
// same as missing, so we never leak existence.
func (r *InterviewRepo) GetInterviewByMockID(
	ctx context.Context,
	mockID string,
	clerkUserID string,
) (domain.MockInterview, error) {
	const q = `SELECT mock_id, clerk_user_id, job_position, job_description,
	                  years_experience, questions, created_at
	           FROM mock_interviews
	           WHERE mock_id = $1 AND clerk_user_id = $2`

	var (
		m            domain.MockInterview
		questionsRaw []byte
	)
	err := r.pool.QueryRow(ctx, q, mockID, clerkUserID).Scan(
		&m.MockID,
		&m.ClerkUserID,
		&m.JobPosition,
		&m.JobDescription,
		&m.YearsExperience,
		&questionsRaw,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.MockInterview{}, fmt.Errorf("get interview: %w", domain.ErrNotFound)
		}
		return domain.MockInterview{}, fmt.Errorf("get interview: %w", err)
	}

	if err := json.Unmarshal(questionsRaw, &m.Questions); err != nil {
		return domain.MockInterview{}, fmt.Errorf("unmarshal questions: %w", err)
	}
	return m, nil
}
