// Package service holds business logic for the interview workflows. It
// defines the Store and LLMClient interfaces it consumes, keeping the
// dependency direction inward (handlers -> services -> interfaces; concrete
// implementations plug in at cmd/server/main.go).
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/google/uuid"
)

// Store is the persistence interface required by InterviewService.
// Defined here on the consumer side per Go interface guidelines.
type Store interface {
	UpsertUser(ctx context.Context, clerkUserID string) error
	CreateInterview(ctx context.Context, m domain.MockInterview) error
	ListInterviewsByUser(ctx context.Context, clerkUserID string, limit int) ([]domain.InterviewSummary, error)
	GetInterviewByMockID(ctx context.Context, mockID, clerkUserID string) (domain.MockInterview, error)
	UpsertAnswer(ctx context.Context, a domain.UserAnswer) error
	ListAnswersByMockID(ctx context.Context, mockID, clerkUserID string) ([]domain.UserAnswer, error)
}

// LLMClient is the LLM interface required by InterviewService.
type LLMClient interface {
	GenerateQuestions(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error)
	EvaluateAnswer(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error)
	TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error)
	JudgeFollowUp(ctx context.Context, in domain.FollowUpSeed) (string, error)
}

// MaxFollowUpsPerQuestion is the hard cap on follow-ups per main question.
// Enforced by JudgeFollowUp so a misbehaving prompt cannot loop forever.
const MaxFollowUpsPerQuestion = 2

// CreateInterviewInput is the validated input for CreateInterview. The
// HTTP layer constructs it from a request DTO; this layer assumes already-
// validated string lengths but defends against empty/whitespace anyway.
type CreateInterviewInput struct {
	JobPosition     string
	JobDescription  string
	YearsExperience int
}

// SubmitAnswerInput is the validated input for SubmitAnswer.
type SubmitAnswerInput struct {
	MockID        string
	QuestionIndex int
	UserAnswer    string
}

// JudgeFollowUpInput is the validated input for JudgeFollowUp.
type JudgeFollowUpInput struct {
	MockID        string
	QuestionIndex int
	MainAnswer    string
	PriorTurns    []domain.FollowUpTurn
}

// InterviewService orchestrates the interview workflows.
type InterviewService struct {
	store Store
	llm   LLMClient
	// listLimit caps GET /interviews response size.
	listLimit int
}

func NewInterviewService(store Store, llm LLMClient) *InterviewService {
	return &InterviewService{store: store, llm: llm, listLimit: 50}
}

// CreateInterview generates questions via the LLM and persists the interview.
// Validation order: identity first (no leakage on bad auth), then domain rules,
// then expensive LLM call. Lazy user upsert before write.
func (s *InterviewService) CreateInterview(
	ctx context.Context,
	clerkUserID string,
	in CreateInterviewInput,
) (domain.MockInterview, error) {
	if clerkUserID == "" {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", domain.ErrUnauthorized)
	}
	if err := validateCreateInterview(in); err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", err)
	}

	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", err)
	}

	questions, err := s.llm.GenerateQuestions(ctx, domain.InterviewSeed{
		JobPosition:     in.JobPosition,
		JobDescription:  in.JobDescription,
		YearsExperience: in.YearsExperience,
	})
	if err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", err)
	}

	m := domain.MockInterview{
		MockID:          uuid.NewString(),
		ClerkUserID:     clerkUserID,
		JobPosition:     in.JobPosition,
		JobDescription:  in.JobDescription,
		YearsExperience: in.YearsExperience,
		Questions:       questions,
	}
	if err := s.store.CreateInterview(ctx, m); err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", err)
	}

	// Re-read to get the DB-assigned created_at; cheap and avoids server clock drift.
	saved, err := s.store.GetInterviewByMockID(ctx, m.MockID, clerkUserID)
	if err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview readback: %w", err)
	}
	return saved, nil
}

// ListInterviews returns the user's interviews. limit==0 means use the
// service default; values are clamped to [1, listLimit].
func (s *InterviewService) ListInterviews(
	ctx context.Context,
	clerkUserID string,
	limit int,
) ([]domain.InterviewSummary, error) {
	if clerkUserID == "" {
		return nil, fmt.Errorf("list interviews: %w", domain.ErrUnauthorized)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return nil, fmt.Errorf("list interviews: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > s.listLimit {
		limit = s.listLimit
	}
	out, err := s.store.ListInterviewsByUser(ctx, clerkUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list interviews: %w", err)
	}
	return out, nil
}

// GetInterview returns one interview if owned by clerkUserID. Not-found is
// indistinguishable from not-owned to avoid leaking existence.
func (s *InterviewService) GetInterview(
	ctx context.Context,
	clerkUserID string,
	mockID string,
) (domain.MockInterview, error) {
	if clerkUserID == "" {
		return domain.MockInterview{}, fmt.Errorf("get interview: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return domain.MockInterview{}, fmt.Errorf("get interview: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.MockInterview{}, fmt.Errorf("get interview: %w", err)
	}
	m, err := s.store.GetInterviewByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.MockInterview{}, fmt.Errorf("get interview: %w", err)
	}
	return m, nil
}

// SubmitAnswer evaluates a user's answer via the LLM and persists it.
// Order: ownership -> bounds check against question count -> LLM -> upsert.
// Re-submissions for the same (mock_id, question_index) overwrite the prior
// row, so users can re-attempt a question and replace their evaluation.
func (s *InterviewService) SubmitAnswer(
	ctx context.Context,
	clerkUserID string,
	in SubmitAnswerInput,
) (domain.UserAnswer, error) {
	if clerkUserID == "" {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", domain.ErrUnauthorized)
	}
	if err := validateSubmitAnswer(in); err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", err)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", err)
	}

	mi, err := s.store.GetInterviewByMockID(ctx, in.MockID, clerkUserID)
	if err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", err)
	}
	if in.QuestionIndex < 0 || in.QuestionIndex >= len(mi.Questions) {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: question index out of range: %w", domain.ErrValidation)
	}
	qa := mi.Questions[in.QuestionIndex]

	eval, err := s.llm.EvaluateAnswer(ctx, domain.AnswerSeed{
		JobPosition:   mi.JobPosition,
		QuestionText:  qa.Question,
		CorrectAnswer: qa.Answer,
		UserAnswer:    in.UserAnswer,
	})
	if err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", err)
	}

	a := domain.UserAnswer{
		MockID:        mi.MockID,
		ClerkUserID:   clerkUserID,
		QuestionIndex: in.QuestionIndex,
		QuestionText:  qa.Question,
		CorrectAnswer: qa.Answer,
		UserAnswer:    in.UserAnswer,
		Rating:        eval.Rating,
		Feedback:      eval.Feedback,
	}
	if err := s.store.UpsertAnswer(ctx, a); err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer: %w", err)
	}

	saved, err := s.store.ListAnswersByMockID(ctx, mi.MockID, clerkUserID)
	if err != nil {
		return domain.UserAnswer{}, fmt.Errorf("submit answer readback: %w", err)
	}
	for _, x := range saved {
		if x.QuestionIndex == in.QuestionIndex {
			return x, nil
		}
	}
	// The row was just upserted; if it isn't in the readback something is
	// fundamentally wrong with the connection — surface as a server fault,
	// not silently succeed.
	return domain.UserAnswer{}, errors.New("submit answer: upserted row missing from readback")
}

// JudgeFollowUp asks the LLM whether the candidate needs one more probing
// follow-up. Returns "" when no further probe is warranted — either because
// the LLM judged the answer good enough or because the hard cap has been
// reached. Ownership of mockID and a valid questionIndex are required.
func (s *InterviewService) JudgeFollowUp(
	ctx context.Context,
	clerkUserID string,
	in JudgeFollowUpInput,
) (string, error) {
	if clerkUserID == "" {
		return "", fmt.Errorf("judge follow-up: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(in.MockID) == "" {
		return "", fmt.Errorf("judge follow-up: %w", domain.ErrValidation)
	}
	if strings.TrimSpace(in.MainAnswer) == "" {
		return "", fmt.Errorf("judge follow-up: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return "", fmt.Errorf("judge follow-up: %w", err)
	}
	mi, err := s.store.GetInterviewByMockID(ctx, in.MockID, clerkUserID)
	if err != nil {
		return "", fmt.Errorf("judge follow-up: %w", err)
	}
	if in.QuestionIndex < 0 || in.QuestionIndex >= len(mi.Questions) {
		return "", fmt.Errorf("judge follow-up: question index out of range: %w", domain.ErrValidation)
	}
	// Hard cap: never call the LLM once the limit has been hit. The empty
	// string tells the caller "move on" without burning a token.
	if len(in.PriorTurns) >= MaxFollowUpsPerQuestion {
		return "", nil
	}

	qa := mi.Questions[in.QuestionIndex]
	follow, err := s.llm.JudgeFollowUp(ctx, domain.FollowUpSeed{
		JobPosition:  mi.JobPosition,
		MainQuestion: qa.Question,
		MainAnswer:   in.MainAnswer,
		PriorTurns:   in.PriorTurns,
	})
	if err != nil {
		return "", fmt.Errorf("judge follow-up: %w", err)
	}
	return strings.TrimSpace(follow), nil
}

// TranscribeAudio runs the audio through the LLM and returns the transcript
// plus delivery metrics (filler count, words/min, long pauses).
//
// longPauseCount comes from the client's voice-activity detection during
// recording — counting silent gaps from server-side audio analysis would
// require an audio decoder, and LLMs are unreliable at it. The service
// clamps it to a sane range.
//
// Ownership of mockID is enforced first so a non-owner can never burn the
// LLM quota of the real owner.
func (s *InterviewService) TranscribeAudio(
	ctx context.Context,
	clerkUserID, mockID string,
	audio []byte,
	mimeType string,
	longPauseCount int,
) (domain.TranscriptResult, error) {
	if clerkUserID == "" {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: %w", domain.ErrValidation)
	}
	if len(audio) == 0 {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: empty audio: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: %w", err)
	}
	if _, err := s.store.GetInterviewByMockID(ctx, mockID, clerkUserID); err != nil {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: %w", err)
	}
	result, err := s.llm.TranscribeAudio(ctx, audio, mimeType)
	if err != nil {
		return domain.TranscriptResult{}, fmt.Errorf("transcribe audio: %w", err)
	}
	result.Transcript = strings.TrimSpace(result.Transcript)
	if longPauseCount < 0 {
		longPauseCount = 0
	} else if longPauseCount > 1000 {
		longPauseCount = 1000
	}
	result.Analysis.LongPauseCount = longPauseCount
	return result, nil
}

// ListFeedback returns all answers for an interview the user owns.
func (s *InterviewService) ListFeedback(
	ctx context.Context,
	clerkUserID string,
	mockID string,
) ([]domain.UserAnswer, error) {
	if clerkUserID == "" {
		return nil, fmt.Errorf("list feedback: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return nil, fmt.Errorf("list feedback: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	// Ownership check via GetInterviewByMockID; if the user doesn't own the
	// interview we return ErrNotFound from the get and never touch answers.
	if _, err := s.store.GetInterviewByMockID(ctx, mockID, clerkUserID); err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	answers, err := s.store.ListAnswersByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	return answers, nil
}

func validateCreateInterview(in CreateInterviewInput) error {
	jp := strings.TrimSpace(in.JobPosition)
	jd := strings.TrimSpace(in.JobDescription)
	switch {
	case len(jp) < 2 || len(jp) > 120:
		return domain.ErrValidation
	case len(jd) < 10 || len(jd) > 4000:
		return domain.ErrValidation
	case in.YearsExperience < 0 || in.YearsExperience > 60:
		return domain.ErrValidation
	}
	return nil
}

func validateSubmitAnswer(in SubmitAnswerInput) error {
	if strings.TrimSpace(in.MockID) == "" {
		return domain.ErrValidation
	}
	if in.QuestionIndex < 0 {
		return domain.ErrValidation
	}
	ua := strings.TrimSpace(in.UserAnswer)
	if len(ua) < 1 || len(ua) > 8000 {
		return domain.ErrValidation
	}
	return nil
}
