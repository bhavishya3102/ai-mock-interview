package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoachReportRepo persists coach_reports. The single-row-per-interview
// invariant is enforced by the UNIQUE constraint on mock_id; a duplicate
// Insert returns ErrConflict so the service can short-circuit to the
// existing row instead of overwriting.
type CoachReportRepo struct {
	pool *pgxpool.Pool
}

func NewCoachReportRepo(pool *pgxpool.Pool) *CoachReportRepo {
	return &CoachReportRepo{pool: pool}
}

// InsertCoachReport stores a freshly generated report. The caller is
// responsible for not calling this when a report already exists; if it
// does, we surface the unique-violation as ErrConflict so the race winner
// can be re-read.
func (r *CoachReportRepo) InsertCoachReport(ctx context.Context, cr domain.CoachReport) error {
	const q = `INSERT INTO coach_reports (mock_id, content, tokens_used, model)
	           VALUES ($1, $2, $3, $4)`

	if _, err := r.pool.Exec(ctx, q, cr.MockID, cr.Content, cr.TokensUsed, cr.Model); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("insert coach report: %w", domain.ErrConflict)
		}
		return fmt.Errorf("insert coach report: %w", err)
	}
	return nil
}

// GetCoachReportByMockID fetches the report for mockID owned by clerkUserID.
// Ownership is enforced via JOIN with mock_interviews so a not-owned row
// returns ErrNotFound (same as missing) — never leaks existence.
func (r *CoachReportRepo) GetCoachReportByMockID(
	ctx context.Context,
	mockID string,
	clerkUserID string,
) (domain.CoachReport, error) {
	const q = `SELECT cr.mock_id, cr.content, cr.tokens_used, cr.model, cr.created_at
	           FROM coach_reports cr
	           JOIN mock_interviews mi ON mi.mock_id = cr.mock_id
	           WHERE cr.mock_id = $1 AND mi.clerk_user_id = $2`

	var cr domain.CoachReport
	err := r.pool.QueryRow(ctx, q, mockID, clerkUserID).Scan(
		&cr.MockID,
		&cr.Content,
		&cr.TokensUsed,
		&cr.Model,
		&cr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.CoachReport{}, fmt.Errorf("get coach report: %w", domain.ErrNotFound)
		}
		return domain.CoachReport{}, fmt.Errorf("get coach report: %w", err)
	}
	return cr, nil
}
