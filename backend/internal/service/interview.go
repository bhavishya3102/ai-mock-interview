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
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/google/uuid"
)

// Store is the persistence interface required by InterviewService.
// Defined here on the consumer side per Go interface guidelines.
type Store interface {
	UpsertUser(ctx context.Context, clerkUserID string) error
	SetUserResume(ctx context.Context, clerkUserID, resumeText string) error
	GetUserResume(ctx context.Context, clerkUserID string) (string, time.Time, error)
	ClearUserResume(ctx context.Context, clerkUserID string) error
	CreateInterview(ctx context.Context, m domain.MockInterview) error
	ListInterviewsByUser(ctx context.Context, clerkUserID string, limit int) ([]domain.InterviewSummary, error)
	GetInterviewByMockID(ctx context.Context, mockID, clerkUserID string) (domain.MockInterview, error)
	UpsertAnswer(ctx context.Context, a domain.UserAnswer) error
	ListAnswersByMockID(ctx context.Context, mockID, clerkUserID string) ([]domain.UserAnswer, error)
	InsertCoachReport(ctx context.Context, cr domain.CoachReport) error
	GetCoachReportByMockID(ctx context.Context, mockID, clerkUserID string) (domain.CoachReport, error)
}

// LLMClient is the LLM interface required by InterviewService.
type LLMClient interface {
	GenerateQuestions(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error)
	EvaluateAnswer(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error)
	TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error)
	ExtractResumeText(ctx context.Context, pdf []byte, mimeType string) (string, error)
	JudgeFollowUp(ctx context.Context, in domain.FollowUpSeed) (string, error)
	GenerateCoachReport(ctx context.Context, in domain.CoachReportSeed) (content string, tokensUsed int, err error)
	GenerateCoachReportStream(ctx context.Context, in domain.CoachReportSeed, onChunk func(text string) error) (tokensUsed int, err error)
	Model() string
}

// maxResumeTextLen caps the stored extracted resume text. A real resume is
// well under this; the cap keeps the question-generation prompt bounded if
// extraction returns something pathological.
const maxResumeTextLen = 20000

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
// FillerCount / WordsPerMinute / LongPauseCount are speech-delivery metrics
// the client aggregates across all recordings for the question. Negative
// values are clamped to 0 by the service.
type SubmitAnswerInput struct {
	MockID         string
	QuestionIndex  int
	UserAnswer     string
	FillerCount    int
	WordsPerMinute int
	LongPauseCount int
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

	// Resume is optional: an empty string just yields role-only questions.
	resumeText, _, err := s.store.GetUserResume(ctx, clerkUserID)
	if err != nil {
		return domain.MockInterview{}, fmt.Errorf("create interview: %w", err)
	}

	questions, err := s.llm.GenerateQuestions(ctx, domain.InterviewSeed{
		JobPosition:     in.JobPosition,
		JobDescription:  in.JobDescription,
		YearsExperience: in.YearsExperience,
		ResumeText:      resumeText,
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
		MockID:         mi.MockID,
		ClerkUserID:    clerkUserID,
		QuestionIndex:  in.QuestionIndex,
		QuestionText:   qa.Question,
		CorrectAnswer:  qa.Answer,
		UserAnswer:     in.UserAnswer,
		Rating:         eval.Rating,
		Feedback:       eval.Feedback,
		FillerCount:    clampNonNeg(in.FillerCount, 10000),
		WordsPerMinute: clampNonNeg(in.WordsPerMinute, 1000),
		LongPauseCount: clampNonNeg(in.LongPauseCount, 1000),
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

// UploadResume extracts text from the candidate's resume PDF via the LLM and
// stores it on the user. The stored text is reused for every interview the
// candidate later creates. Re-uploading replaces the prior resume.
func (s *InterviewService) UploadResume(
	ctx context.Context,
	clerkUserID string,
	pdf []byte,
	mimeType string,
) (domain.ResumeStatus, error) {
	if clerkUserID == "" {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: %w", domain.ErrUnauthorized)
	}
	if len(pdf) == 0 {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: empty file: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: %w", err)
	}

	text, err := s.llm.ExtractResumeText(ctx, pdf, mimeType)
	if err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: no readable resume text: %w", domain.ErrValidation)
	}
	if r := []rune(text); len(r) > maxResumeTextLen {
		text = string(r[:maxResumeTextLen])
	}

	if err := s.store.SetUserResume(ctx, clerkUserID, text); err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume: %w", err)
	}
	// Read back to get the DB-assigned upload timestamp.
	_, uploadedAt, err := s.store.GetUserResume(ctx, clerkUserID)
	if err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("upload resume readback: %w", err)
	}
	return domain.ResumeStatus{Attached: true, UploadedAt: uploadedAt}, nil
}

// GetResume reports whether the user has a resume on file and when it was
// uploaded. The extracted text itself is never returned over the wire.
func (s *InterviewService) GetResume(ctx context.Context, clerkUserID string) (domain.ResumeStatus, error) {
	if clerkUserID == "" {
		return domain.ResumeStatus{}, fmt.Errorf("get resume: %w", domain.ErrUnauthorized)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("get resume: %w", err)
	}
	text, uploadedAt, err := s.store.GetUserResume(ctx, clerkUserID)
	if err != nil {
		return domain.ResumeStatus{}, fmt.Errorf("get resume: %w", err)
	}
	return domain.ResumeStatus{Attached: text != "", UploadedAt: uploadedAt}, nil
}

// DeleteResume removes the stored resume. Idempotent — deleting when no
// resume exists is a no-op success.
func (s *InterviewService) DeleteResume(ctx context.Context, clerkUserID string) error {
	if clerkUserID == "" {
		return fmt.Errorf("delete resume: %w", domain.ErrUnauthorized)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return fmt.Errorf("delete resume: %w", err)
	}
	if err := s.store.ClearUserResume(ctx, clerkUserID); err != nil {
		return fmt.Errorf("delete resume: %w", err)
	}
	return nil
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

// GenerateCoachReport produces (or returns the cached) narrative coaching
// report for a finished interview. The operation is idempotent: if a report
// already exists for mockID we return it without re-calling the LLM, so a
// second POST is free and safe.
//
// Order: identity -> mockID validation -> upsert user -> idempotency check
// -> ownership via parent interview -> pre-condition (answers exist) -> LLM
// -> persist -> readback. A unique-violation on insert (two concurrent
// POSTs) is swallowed and the winning row is returned.
func (s *InterviewService) GenerateCoachReport(
	ctx context.Context,
	clerkUserID string,
	mockID string,
) (domain.CoachReport, error) {
	if clerkUserID == "" {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}

	// Idempotency: if a report already exists for this user+mock, return it.
	// This is both a cost-saver and the natural deduplication path on POST.
	existing, err := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}

	// Ownership check via parent interview row. Returns ErrNotFound if the
	// interview doesn't exist OR the user doesn't own it — never leaks.
	mi, err := s.store.GetInterviewByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}

	answers, err := s.store.ListAnswersByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}
	if len(answers) == 0 {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: no answers submitted: %w", domain.ErrValidation)
	}

	content, tokens, err := s.llm.GenerateCoachReport(ctx, domain.CoachReportSeed{
		JobPosition:     mi.JobPosition,
		YearsExperience: mi.YearsExperience,
		Answers:         answers,
	})
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}

	cr := domain.CoachReport{
		MockID:     mockID,
		Content:    content,
		TokensUsed: tokens,
		Model:      s.llm.Model(),
	}
	if err := s.store.InsertCoachReport(ctx, cr); err != nil {
		// Race: a parallel POST won the insert. Re-fetch and return the
		// winning row so the caller still sees a coherent report.
		if errors.Is(err, domain.ErrConflict) {
			winner, fetchErr := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
			if fetchErr != nil {
				return domain.CoachReport{}, fmt.Errorf("generate coach report after race: %w", fetchErr)
			}
			return winner, nil
		}
		return domain.CoachReport{}, fmt.Errorf("generate coach report: %w", err)
	}

	// Re-read for the DB-assigned created_at.
	saved, err := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("generate coach report readback: %w", err)
	}
	return saved, nil
}

// StreamCoachReport is the streaming variant of GenerateCoachReport. Each
// model-produced chunk is forwarded to onChunk as it arrives; the persisted
// report (with model + tokens + DB-assigned timestamp) is returned once the
// stream completes.
//
// Cached-hit behavior: if a report already exists for mockID, the full
// stored content is emitted as a SINGLE onChunk call before returning the
// cached row. No LLM call, no token spend — from the caller's perspective
// the contract is identical, only the wall-clock latency differs.
//
// If onChunk returns an error mid-stream, generation aborts and that error
// is returned wrapped. The buffered partial content is NOT persisted — a
// half-written report is useless to the candidate.
func (s *InterviewService) StreamCoachReport(
	ctx context.Context,
	clerkUserID string,
	mockID string,
	onChunk func(text string) error,
) (domain.CoachReport, error) {
	if clerkUserID == "" {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", domain.ErrValidation)
	}
	if onChunk == nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: nil onChunk: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}

	// Cached hit: emit full content as a single chunk and return early.
	if existing, err := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID); err == nil {
		if cbErr := onChunk(existing.Content); cbErr != nil {
			return domain.CoachReport{}, fmt.Errorf("stream coach report: cached chunk callback: %w", cbErr)
		}
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}

	mi, err := s.store.GetInterviewByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}
	answers, err := s.store.ListAnswersByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}
	if len(answers) == 0 {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: no answers submitted: %w", domain.ErrValidation)
	}

	// Buffer chunks server-side for the final persist; forward each to the
	// caller's onChunk. A callback failure aborts without persisting.
	var buf strings.Builder
	tokens, err := s.llm.GenerateCoachReportStream(
		ctx,
		domain.CoachReportSeed{
			JobPosition:     mi.JobPosition,
			YearsExperience: mi.YearsExperience,
			Answers:         answers,
		},
		func(chunk string) error {
			buf.WriteString(chunk)
			return onChunk(chunk)
		},
	)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}

	cr := domain.CoachReport{
		MockID:     mockID,
		Content:    buf.String(),
		TokensUsed: tokens,
		Model:      s.llm.Model(),
	}
	if err := s.store.InsertCoachReport(ctx, cr); err != nil {
		// Race with a concurrent generator: another caller won the insert.
		// Return the winning row; the chunks we already streamed remain
		// meaningful to the client (same prompt, similar content).
		if errors.Is(err, domain.ErrConflict) {
			winner, fetchErr := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
			if fetchErr != nil {
				return domain.CoachReport{}, fmt.Errorf("stream coach report after race: %w", fetchErr)
			}
			return winner, nil
		}
		return domain.CoachReport{}, fmt.Errorf("stream coach report: %w", err)
	}

	saved, err := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("stream coach report readback: %w", err)
	}
	return saved, nil
}

// GetCoachReport fetches an existing report. Returns ErrNotFound if the
// user has not generated one yet (or doesn't own the interview).
func (s *InterviewService) GetCoachReport(
	ctx context.Context,
	clerkUserID string,
	mockID string,
) (domain.CoachReport, error) {
	if clerkUserID == "" {
		return domain.CoachReport{}, fmt.Errorf("get coach report: %w", domain.ErrUnauthorized)
	}
	if strings.TrimSpace(mockID) == "" {
		return domain.CoachReport{}, fmt.Errorf("get coach report: %w", domain.ErrValidation)
	}
	if err := s.store.UpsertUser(ctx, clerkUserID); err != nil {
		return domain.CoachReport{}, fmt.Errorf("get coach report: %w", err)
	}
	cr, err := s.store.GetCoachReportByMockID(ctx, mockID, clerkUserID)
	if err != nil {
		return domain.CoachReport{}, fmt.Errorf("get coach report: %w", err)
	}
	return cr, nil
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

// clampNonNeg returns v clamped into [0, max]. Negative client inputs are
// treated as zero rather than rejected — the metrics are advisory.
func clampNonNeg(v, max int) int {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
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
