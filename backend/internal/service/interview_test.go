package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// updateEmbeddingCall captures one invocation of UpdateAnswerEmbedding so
// tests can assert that the background embed goroutine actually fired.
type updateEmbeddingCall struct {
	mockID        string
	clerkUserID   string
	questionIndex int
	vec           []float32
}

// fakeStore captures call args + lets each method's behaviour be overridden
// per test via *Func fields. Empty defaults are happy path.
type fakeStore struct {
	mu                       sync.Mutex // guards slices written from background goroutines
	upsertCalls              []string
	createInterviewCalls     []domain.MockInterview
	upsertAnswerCalls        []domain.UserAnswer
	setResumeCalls           []string
	insertCoachReportCalls   []domain.CoachReport
	updateEmbeddingCalls     []updateEmbeddingCall

	upsertUserFunc                 func(ctx context.Context, userID string) error
	setUserResumeFunc              func(ctx context.Context, userID, resumeText string) error
	getUserResumeFunc              func(ctx context.Context, userID string) (string, time.Time, error)
	clearUserResumeFunc            func(ctx context.Context, userID string) error
	createInterviewFunc            func(ctx context.Context, m domain.MockInterview) error
	listInterviewsByUserFunc       func(ctx context.Context, userID string, limit int) ([]domain.InterviewSummary, error)
	getInterviewByMockIDFunc       func(ctx context.Context, mockID, userID string) (domain.MockInterview, error)
	upsertAnswerFunc               func(ctx context.Context, a domain.UserAnswer) error
	listAnswersByMockIDFunc        func(ctx context.Context, mockID, userID string) ([]domain.UserAnswer, error)
	insertCoachReportFunc          func(ctx context.Context, cr domain.CoachReport) error
	getCoachReportByMockIDFunc     func(ctx context.Context, mockID, userID string) (domain.CoachReport, error)
	updateAnswerEmbeddingFunc      func(ctx context.Context, mockID, userID string, questionIndex int, vec []float32) error
	findSimilarWeakAnswersFunc     func(ctx context.Context, userID string, queryVec []float32, ratingThreshold, limit int) ([]domain.WeakAnswerHit, error)
	listUserInterviewSummariesFunc func(ctx context.Context, userID string, limit int) ([]domain.PastInterviewSummary, error)
}

func (f *fakeStore) SetUserResume(ctx context.Context, userID, resumeText string) error {
	f.setResumeCalls = append(f.setResumeCalls, resumeText)
	if f.setUserResumeFunc != nil {
		return f.setUserResumeFunc(ctx, userID, resumeText)
	}
	return nil
}

func (f *fakeStore) GetUserResume(ctx context.Context, userID string) (string, time.Time, error) {
	if f.getUserResumeFunc != nil {
		return f.getUserResumeFunc(ctx, userID)
	}
	return "", time.Time{}, nil
}

func (f *fakeStore) ClearUserResume(ctx context.Context, userID string) error {
	if f.clearUserResumeFunc != nil {
		return f.clearUserResumeFunc(ctx, userID)
	}
	return nil
}

func (f *fakeStore) UpsertUser(ctx context.Context, userID string) error {
	f.upsertCalls = append(f.upsertCalls, userID)
	if f.upsertUserFunc != nil {
		return f.upsertUserFunc(ctx, userID)
	}
	return nil
}

func (f *fakeStore) CreateInterview(ctx context.Context, m domain.MockInterview) error {
	f.createInterviewCalls = append(f.createInterviewCalls, m)
	if f.createInterviewFunc != nil {
		return f.createInterviewFunc(ctx, m)
	}
	return nil
}

func (f *fakeStore) ListInterviewsByUser(ctx context.Context, userID string, limit int) ([]domain.InterviewSummary, error) {
	if f.listInterviewsByUserFunc != nil {
		return f.listInterviewsByUserFunc(ctx, userID, limit)
	}
	return []domain.InterviewSummary{}, nil
}

func (f *fakeStore) GetInterviewByMockID(ctx context.Context, mockID, userID string) (domain.MockInterview, error) {
	if f.getInterviewByMockIDFunc != nil {
		return f.getInterviewByMockIDFunc(ctx, mockID, userID)
	}
	return domain.MockInterview{}, domain.ErrNotFound
}

func (f *fakeStore) UpsertAnswer(ctx context.Context, a domain.UserAnswer) error {
	f.upsertAnswerCalls = append(f.upsertAnswerCalls, a)
	if f.upsertAnswerFunc != nil {
		return f.upsertAnswerFunc(ctx, a)
	}
	return nil
}

func (f *fakeStore) ListAnswersByMockID(ctx context.Context, mockID, userID string) ([]domain.UserAnswer, error) {
	if f.listAnswersByMockIDFunc != nil {
		return f.listAnswersByMockIDFunc(ctx, mockID, userID)
	}
	return []domain.UserAnswer{}, nil
}

func (f *fakeStore) InsertCoachReport(ctx context.Context, cr domain.CoachReport) error {
	f.insertCoachReportCalls = append(f.insertCoachReportCalls, cr)
	if f.insertCoachReportFunc != nil {
		return f.insertCoachReportFunc(ctx, cr)
	}
	return nil
}

func (f *fakeStore) GetCoachReportByMockID(ctx context.Context, mockID, userID string) (domain.CoachReport, error) {
	if f.getCoachReportByMockIDFunc != nil {
		return f.getCoachReportByMockIDFunc(ctx, mockID, userID)
	}
	return domain.CoachReport{}, domain.ErrNotFound
}

func (f *fakeStore) UpdateAnswerEmbedding(ctx context.Context, mockID, userID string, questionIndex int, vec []float32) error {
	f.mu.Lock()
	f.updateEmbeddingCalls = append(f.updateEmbeddingCalls, updateEmbeddingCall{
		mockID:        mockID,
		clerkUserID:   userID,
		questionIndex: questionIndex,
		vec:           vec,
	})
	f.mu.Unlock()
	if f.updateAnswerEmbeddingFunc != nil {
		return f.updateAnswerEmbeddingFunc(ctx, mockID, userID, questionIndex, vec)
	}
	return nil
}

func (f *fakeStore) embedCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.updateEmbeddingCalls)
}

func (f *fakeStore) FindSimilarWeakAnswers(ctx context.Context, userID string, queryVec []float32, ratingThreshold, limit int) ([]domain.WeakAnswerHit, error) {
	if f.findSimilarWeakAnswersFunc != nil {
		return f.findSimilarWeakAnswersFunc(ctx, userID, queryVec, ratingThreshold, limit)
	}
	return nil, nil
}

func (f *fakeStore) ListUserInterviewSummaries(ctx context.Context, userID string, limit int) ([]domain.PastInterviewSummary, error) {
	if f.listUserInterviewSummariesFunc != nil {
		return f.listUserInterviewSummariesFunc(ctx, userID, limit)
	}
	return nil, nil
}

type fakeLLM struct {
	model                         string
	generateQuestionsFunc         func(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error)
	evaluateAnswerFunc            func(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error)
	transcribeAudioFunc           func(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error)
	extractResumeTextFunc         func(ctx context.Context, pdf []byte, mimeType string) (string, error)
	judgeFollowUpFunc             func(ctx context.Context, in domain.FollowUpSeed) (string, error)
	generateCoachReportFunc       func(ctx context.Context, in domain.CoachReportSeed) (string, int, error)
	generateCoachReportStreamFunc func(ctx context.Context, in domain.CoachReportSeed, onChunk func(string) error) (int, error)
	embedFunc                     func(ctx context.Context, text string) ([]float32, error)
}

func (f *fakeLLM) Embed(ctx context.Context, text string) ([]float32, error) {
	if f.embedFunc != nil {
		return f.embedFunc(ctx, text)
	}
	// Default: a deterministic small vector so tests that don't care about
	// the exact bytes still get a valid response shape.
	return []float32{0.1, 0.2, 0.3}, nil
}

func (f *fakeLLM) Model() string {
	if f.model == "" {
		return "gemini-test"
	}
	return f.model
}

func (f *fakeLLM) GenerateCoachReport(ctx context.Context, in domain.CoachReportSeed) (string, int, error) {
	if f.generateCoachReportFunc != nil {
		return f.generateCoachReportFunc(ctx, in)
	}
	return "## Overall Performance\nfake report", 1234, nil
}

func (f *fakeLLM) GenerateCoachReportStream(
	ctx context.Context,
	in domain.CoachReportSeed,
	onChunk func(string) error,
) (int, error) {
	if f.generateCoachReportStreamFunc != nil {
		return f.generateCoachReportStreamFunc(ctx, in, onChunk)
	}
	// Default: emit two chunks and report fake tokens.
	if err := onChunk("## Overall Performance\n"); err != nil {
		return 0, err
	}
	if err := onChunk("fake streamed body"); err != nil {
		return 0, err
	}
	return 1234, nil
}

func (f *fakeLLM) ExtractResumeText(ctx context.Context, pdf []byte, mimeType string) (string, error) {
	if f.extractResumeTextFunc != nil {
		return f.extractResumeTextFunc(ctx, pdf, mimeType)
	}
	return "extracted resume text", nil
}

func (f *fakeLLM) GenerateQuestions(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error) {
	if f.generateQuestionsFunc != nil {
		return f.generateQuestionsFunc(ctx, in)
	}
	return defaultQuestions(), nil
}

func (f *fakeLLM) EvaluateAnswer(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error) {
	if f.evaluateAnswerFunc != nil {
		return f.evaluateAnswerFunc(ctx, in)
	}
	return domain.Evaluation{Rating: 8, Feedback: "decent answer with minor gaps to address"}, nil
}

func (f *fakeLLM) TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error) {
	if f.transcribeAudioFunc != nil {
		return f.transcribeAudioFunc(ctx, audio, mimeType)
	}
	return domain.TranscriptResult{Transcript: "fake transcript"}, nil
}

func (f *fakeLLM) JudgeFollowUp(ctx context.Context, in domain.FollowUpSeed) (string, error) {
	if f.judgeFollowUpFunc != nil {
		return f.judgeFollowUpFunc(ctx, in)
	}
	return "", nil
}

func defaultQuestions() []domain.GeneratedQA {
	out := make([]domain.GeneratedQA, 0, 5)
	for i := 0; i < 5; i++ {
		out = append(out, domain.GeneratedQA{
			Question: "what is your approach to problem N",
			Answer:   "describe approach with examples and tradeoffs",
		})
	}
	return out
}

func sampleInterview(userID string) domain.MockInterview {
	return domain.MockInterview{
		MockID:          "mock-123",
		ClerkUserID:     userID,
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build distributed systems with Go",
		YearsExperience: 5,
		Questions:       defaultQuestions(),
		CreatedAt:       time.Now().UTC(),
	}
}

const validJobDescription = "Build and maintain distributed Go systems on AWS"

// --- CreateInterview ---

func TestCreateInterview_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.CreateInterview(context.Background(), "", CreateInterviewInput{
		JobPosition: "x", JobDescription: validJobDescription, YearsExperience: 1,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestCreateInterview_ValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		in   CreateInterviewInput
	}{
		{"position too short", CreateInterviewInput{JobPosition: "a", JobDescription: validJobDescription, YearsExperience: 3}},
		{"position too long", CreateInterviewInput{JobPosition: strings.Repeat("a", 121), JobDescription: validJobDescription, YearsExperience: 3}},
		{"description too short", CreateInterviewInput{JobPosition: "Backend", JobDescription: "tiny", YearsExperience: 3}},
		{"description too long", CreateInterviewInput{JobPosition: "Backend", JobDescription: strings.Repeat("a", 4001), YearsExperience: 3}},
		{"years negative", CreateInterviewInput{JobPosition: "Backend", JobDescription: validJobDescription, YearsExperience: -1}},
		{"years too high", CreateInterviewInput{JobPosition: "Backend", JobDescription: validJobDescription, YearsExperience: 61}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
			_, err := svc.CreateInterview(context.Background(), "user_1", tt.in)
			require.Error(t, err)
			require.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}

func TestCreateInterview_HappyPath(t *testing.T) {
	store := &fakeStore{}
	store.getInterviewByMockIDFunc = func(_ context.Context, mockID, userID string) (domain.MockInterview, error) {
		require.Equal(t, "user_1", userID)
		return sampleInterview(userID), nil
	}
	llm := &fakeLLM{}

	svc := NewInterviewService(store, llm)
	mi, err := svc.CreateInterview(context.Background(), "user_1", CreateInterviewInput{
		JobPosition:     "Backend Engineer",
		JobDescription:  validJobDescription,
		YearsExperience: 5,
	})
	require.NoError(t, err)
	require.Equal(t, "user_1", mi.ClerkUserID)
	require.Len(t, mi.Questions, 5)
	require.Equal(t, []string{"user_1"}, store.upsertCalls, "user upserted exactly once")
	require.Len(t, store.createInterviewCalls, 1)
}

func TestCreateInterview_LLMFailureMaps(t *testing.T) {
	llm := &fakeLLM{
		generateQuestionsFunc: func(_ context.Context, _ domain.InterviewSeed) ([]domain.GeneratedQA, error) {
			return nil, errors.Join(domain.ErrLLM, errors.New("upstream broke"))
		},
	}
	svc := NewInterviewService(&fakeStore{}, llm)
	_, err := svc.CreateInterview(context.Background(), "user_1", CreateInterviewInput{
		JobPosition: "Backend", JobDescription: validJobDescription, YearsExperience: 3,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrLLM)
}

func TestCreateInterview_StoreErrorPropagates(t *testing.T) {
	store := &fakeStore{
		createInterviewFunc: func(_ context.Context, _ domain.MockInterview) error {
			return errors.New("db down")
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.CreateInterview(context.Background(), "user_1", CreateInterviewInput{
		JobPosition: "Backend", JobDescription: validJobDescription, YearsExperience: 3,
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrValidation)
}

// --- GetInterview ---

func TestGetInterview_MissingMockID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GetInterview(context.Background(), "user_1", "")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGetInterview_OwnershipViaNotFound(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return domain.MockInterview{}, domain.ErrNotFound
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.GetInterview(context.Background(), "user_1", "mock-x")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetInterview_HappyPath(t *testing.T) {
	want := sampleInterview("user_1")
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, mockID, userID string) (domain.MockInterview, error) {
			require.Equal(t, "mock-123", mockID)
			require.Equal(t, "user_1", userID)
			return want, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	got, err := svc.GetInterview(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Equal(t, want.MockID, got.MockID)
}

// --- SubmitAnswer ---

func TestSubmitAnswer_Validation(t *testing.T) {
	tests := []struct {
		name string
		in   SubmitAnswerInput
	}{
		{"empty mock id", SubmitAnswerInput{MockID: "", QuestionIndex: 0, UserAnswer: "answer"}},
		{"negative index", SubmitAnswerInput{MockID: "m", QuestionIndex: -1, UserAnswer: "answer"}},
		{"empty user answer", SubmitAnswerInput{MockID: "m", QuestionIndex: 0, UserAnswer: ""}},
		{"too long user answer", SubmitAnswerInput{MockID: "m", QuestionIndex: 0, UserAnswer: strings.Repeat("a", 8001)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
			_, err := svc.SubmitAnswer(context.Background(), "user_1", tt.in)
			require.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}

func TestSubmitAnswer_OwnershipNotFound(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return domain.MockInterview{}, domain.ErrNotFound
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-x", QuestionIndex: 0, UserAnswer: "fine answer",
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSubmitAnswer_QuestionIndexOutOfRange(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-123", QuestionIndex: 10, UserAnswer: "fine answer",
	})
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestSubmitAnswer_LLMFailureMaps(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	llm := &fakeLLM{
		evaluateAnswerFunc: func(_ context.Context, _ domain.AnswerSeed) (domain.Evaluation, error) {
			return domain.Evaluation{}, errors.Join(domain.ErrLLM, errors.New("model timeout"))
		},
	}
	svc := NewInterviewService(store, llm)
	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-123", QuestionIndex: 0, UserAnswer: "fine answer",
	})
	require.ErrorIs(t, err, domain.ErrLLM)
}

func TestSubmitAnswer_ResubmissionUpserts(t *testing.T) {
	// Re-attempting the same question must succeed and overwrite the prior
	// row — the store maps that to UpsertAnswer, so the service should never
	// surface a conflict error on resubmission.
	latest := domain.UserAnswer{
		MockID: "mock-123", ClerkUserID: "user_1",
		QuestionIndex: 0, Rating: 9, Feedback: "much better second take",
	}
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, _, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{latest}, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-123", QuestionIndex: 0, UserAnswer: "first take",
	})
	require.NoError(t, err)

	got, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-123", QuestionIndex: 0, UserAnswer: "second, better take",
	})
	require.NoError(t, err)
	require.Equal(t, 9, got.Rating)
	require.Len(t, store.upsertAnswerCalls, 2, "both attempts hit the store")
	require.Equal(t, "second, better take", store.upsertAnswerCalls[1].UserAnswer)
}

func TestSubmitAnswer_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	saved := domain.UserAnswer{
		MockID: "mock-123", ClerkUserID: "user_1",
		QuestionIndex: 0, Rating: 8, Feedback: "decent answer with minor gaps to address",
		CreatedAt: now,
	}
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, _, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{saved}, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	got, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID: "mock-123", QuestionIndex: 0, UserAnswer: "good and structured answer",
	})
	require.NoError(t, err)
	require.Equal(t, 8, got.Rating)
	require.Len(t, store.upsertAnswerCalls, 1)
	require.Equal(t, 0, store.upsertAnswerCalls[0].QuestionIndex)
}

// --- ListInterviews / ListFeedback ---

func TestListInterviews_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.ListInterviews(context.Background(), "", 0)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestListInterviews_LimitClamping(t *testing.T) {
	var capturedLimit int
	store := &fakeStore{
		listInterviewsByUserFunc: func(_ context.Context, _ string, limit int) ([]domain.InterviewSummary, error) {
			capturedLimit = limit
			return []domain.InterviewSummary{}, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	_, err := svc.ListInterviews(context.Background(), "user_1", 0)
	require.NoError(t, err)
	require.Equal(t, 20, capturedLimit, "default to 20 when limit unspecified")

	_, err = svc.ListInterviews(context.Background(), "user_1", 1000)
	require.NoError(t, err)
	require.Equal(t, 50, capturedLimit, "clamp to listLimit (50)")
}

func TestListFeedback_OwnershipNotFound(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return domain.MockInterview{}, domain.ErrNotFound
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.ListFeedback(context.Background(), "user_1", "mock-x")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListFeedback_HappyPath(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, _, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{{QuestionIndex: 0, Rating: 9, Feedback: "great"}}, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	got, err := svc.ListFeedback(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 9, got[0].Rating)
}

// --- JudgeFollowUp ---

func TestJudgeFollowUp_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.JudgeFollowUp(context.Background(), "", JudgeFollowUpInput{
		MockID:     "mock-123",
		MainAnswer: "an answer",
	})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestJudgeFollowUp_RequiresMainAnswer(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.JudgeFollowUp(context.Background(), "user_1", JudgeFollowUpInput{
		MockID:     "mock-123",
		MainAnswer: "   ",
	})
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestJudgeFollowUp_OwnershipChecked(t *testing.T) {
	// Store returns ErrNotFound by default — non-owner can never reach LLM.
	llmCalled := false
	llm := &fakeLLM{
		judgeFollowUpFunc: func(_ context.Context, _ domain.FollowUpSeed) (string, error) {
			llmCalled = true
			return "should not be called", nil
		},
	}
	svc := NewInterviewService(&fakeStore{}, llm)
	_, err := svc.JudgeFollowUp(context.Background(), "user_1", JudgeFollowUpInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		MainAnswer:    "an answer",
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.False(t, llmCalled, "LLM must not be invoked when ownership check fails")
}

func TestJudgeFollowUp_CapShortCircuitsLLM(t *testing.T) {
	// Exactly MaxFollowUpsPerQuestion turns already exchanged — service must
	// return "" without burning a token.
	llmCalled := false
	llm := &fakeLLM{
		judgeFollowUpFunc: func(_ context.Context, _ domain.FollowUpSeed) (string, error) {
			llmCalled = true
			return "should not be called", nil
		},
	}
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	svc := NewInterviewService(store, llm)

	turns := make([]domain.FollowUpTurn, MaxFollowUpsPerQuestion)
	for i := range turns {
		turns[i] = domain.FollowUpTurn{Question: "fq", Answer: "fa"}
	}

	got, err := svc.JudgeFollowUp(context.Background(), "user_1", JudgeFollowUpInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		MainAnswer:    "an answer",
		PriorTurns:    turns,
	})
	require.NoError(t, err)
	require.Equal(t, "", got, "cap reached — service must return empty without LLM call")
	require.False(t, llmCalled)
}

func TestJudgeFollowUp_PassesThroughLLMResult(t *testing.T) {
	llm := &fakeLLM{
		judgeFollowUpFunc: func(_ context.Context, in domain.FollowUpSeed) (string, error) {
			require.Equal(t, "Backend Engineer", in.JobPosition)
			require.Equal(t, "an answer", in.MainAnswer)
			require.Empty(t, in.PriorTurns)
			return "  Can you walk me through the trade-offs?  ", nil
		},
	}
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	svc := NewInterviewService(store, llm)
	got, err := svc.JudgeFollowUp(context.Background(), "user_1", JudgeFollowUpInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		MainAnswer:    "an answer",
	})
	require.NoError(t, err)
	require.Equal(t, "Can you walk me through the trade-offs?", got)
}

// --- TranscribeAudio ---

func TestTranscribeAudio_PassesThroughAnalysis(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	llm := &fakeLLM{
		transcribeAudioFunc: func(_ context.Context, _ []byte, _ string) (domain.TranscriptResult, error) {
			// LLM returns filler+wpm; longPauseCount comes from the caller (client VAD).
			return domain.TranscriptResult{
				Transcript: "  spoken words  ",
				Analysis:   domain.SpeechAnalysis{FillerCount: 4, WordsPerMinute: 142},
			}, nil
		},
	}
	svc := NewInterviewService(store, llm)
	got, err := svc.TranscribeAudio(context.Background(), "user_1", "mock-123", []byte("not-empty"), "audio/webm", 2)
	require.NoError(t, err)
	require.Equal(t, "spoken words", got.Transcript, "service must trim whitespace")
	require.Equal(t, 4, got.Analysis.FillerCount)
	require.Equal(t, 142, got.Analysis.WordsPerMinute)
	require.Equal(t, 2, got.Analysis.LongPauseCount, "service must overwrite LongPauseCount with the caller-supplied value")
}

func TestTranscribeAudio_OwnershipChecked(t *testing.T) {
	llmCalled := false
	llm := &fakeLLM{
		transcribeAudioFunc: func(_ context.Context, _ []byte, _ string) (domain.TranscriptResult, error) {
			llmCalled = true
			return domain.TranscriptResult{}, nil
		},
	}
	svc := NewInterviewService(&fakeStore{}, llm)
	_, err := svc.TranscribeAudio(context.Background(), "user_1", "mock-x", []byte("not-empty"), "audio/webm", 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.False(t, llmCalled, "LLM must not be invoked when ownership check fails")
}

func TestJudgeFollowUp_QuestionIndexOutOfRange(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return sampleInterview("user_1"), nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	_, err := svc.JudgeFollowUp(context.Background(), "user_1", JudgeFollowUpInput{
		MockID:        "mock-123",
		QuestionIndex: 99,
		MainAnswer:    "an answer",
	})
	require.ErrorIs(t, err, domain.ErrValidation)
}

// --- Resume ---

func TestUploadResume_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.UploadResume(context.Background(), "", []byte("%PDF-data"), "application/pdf")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestUploadResume_EmptyFile(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.UploadResume(context.Background(), "user_1", nil, "application/pdf")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestUploadResume_HappyPath(t *testing.T) {
	uploadedAt := time.Now().UTC()
	store := &fakeStore{
		getUserResumeFunc: func(_ context.Context, _ string) (string, time.Time, error) {
			return "  Jane Doe resume  ", uploadedAt, nil
		},
	}
	llm := &fakeLLM{
		extractResumeTextFunc: func(_ context.Context, _ []byte, mimeType string) (string, error) {
			require.Equal(t, "application/pdf", mimeType)
			return "  Jane Doe — Senior Engineer  ", nil
		},
	}
	svc := NewInterviewService(store, llm)

	status, err := svc.UploadResume(context.Background(), "user_1", []byte("%PDF-1.7 bytes"), "application/pdf")
	require.NoError(t, err)
	require.True(t, status.Attached)
	require.Equal(t, uploadedAt, status.UploadedAt)
	require.Equal(t, []string{"Jane Doe — Senior Engineer"}, store.setResumeCalls,
		"extracted text must be trimmed before storage")
}

func TestUploadResume_NoReadableText(t *testing.T) {
	llm := &fakeLLM{
		extractResumeTextFunc: func(_ context.Context, _ []byte, _ string) (string, error) {
			return "   ", nil
		},
	}
	svc := NewInterviewService(&fakeStore{}, llm)
	_, err := svc.UploadResume(context.Background(), "user_1", []byte("%PDF-"), "application/pdf")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestUploadResume_LLMFailureMaps(t *testing.T) {
	llm := &fakeLLM{
		extractResumeTextFunc: func(_ context.Context, _ []byte, _ string) (string, error) {
			return "", errors.Join(domain.ErrLLM, errors.New("model down"))
		},
	}
	svc := NewInterviewService(&fakeStore{}, llm)
	_, err := svc.UploadResume(context.Background(), "user_1", []byte("%PDF-"), "application/pdf")
	require.ErrorIs(t, err, domain.ErrLLM)
}

func TestGetResume_ReportsAttached(t *testing.T) {
	uploadedAt := time.Now().UTC()
	store := &fakeStore{
		getUserResumeFunc: func(_ context.Context, _ string) (string, time.Time, error) {
			return "stored resume text", uploadedAt, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	status, err := svc.GetResume(context.Background(), "user_1")
	require.NoError(t, err)
	require.True(t, status.Attached)
	require.Equal(t, uploadedAt, status.UploadedAt)
}

func TestGetResume_ReportsNotAttached(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	status, err := svc.GetResume(context.Background(), "user_1")
	require.NoError(t, err)
	require.False(t, status.Attached)
	require.True(t, status.UploadedAt.IsZero())
}

func TestDeleteResume_CallsClear(t *testing.T) {
	cleared := false
	store := &fakeStore{
		clearUserResumeFunc: func(_ context.Context, userID string) error {
			require.Equal(t, "user_1", userID)
			cleared = true
			return nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})
	require.NoError(t, svc.DeleteResume(context.Background(), "user_1"))
	require.True(t, cleared)
}

func TestCreateInterview_PassesResumeToLLM(t *testing.T) {
	store := &fakeStore{
		getUserResumeFunc: func(_ context.Context, _ string) (string, time.Time, error) {
			return "Jane led a Postgres migration at Acme", time.Now().UTC(), nil
		},
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
	}
	var seenSeed domain.InterviewSeed
	llm := &fakeLLM{
		generateQuestionsFunc: func(_ context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error) {
			seenSeed = in
			return defaultQuestions(), nil
		},
	}
	svc := NewInterviewService(store, llm)
	_, err := svc.CreateInterview(context.Background(), "user_1", CreateInterviewInput{
		JobPosition: "Backend Engineer", JobDescription: validJobDescription, YearsExperience: 5,
	})
	require.NoError(t, err)
	require.Equal(t, "Jane led a Postgres migration at Acme", seenSeed.ResumeText,
		"stored resume text must flow into the question-generation seed")
}

// --- GenerateCoachReport ---

func sampleAnswer(mockID string, idx int, rating int) domain.UserAnswer {
	return domain.UserAnswer{
		MockID:        mockID,
		ClerkUserID:   "user_1",
		QuestionIndex: idx,
		QuestionText:  "Explain topic " + string(rune('A'+idx)),
		CorrectAnswer: "Reference answer body",
		UserAnswer:    "Candidate's actual answer",
		Rating:        rating,
		Feedback:      "Some specific feedback about this answer that is reasonably long.",
		CreatedAt:     time.Now().UTC(),
	}
}

func TestGenerateCoachReport_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GenerateCoachReport(context.Background(), "", "mock-123")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestGenerateCoachReport_RequiresMockID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GenerateCoachReport(context.Background(), "user_1", "   ")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGenerateCoachReport_IdempotentWhenExisting(t *testing.T) {
	existing := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nalready generated",
		TokensUsed: 999,
		Model:      "gemini-cached",
		CreatedAt:  time.Now().UTC(),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, mockID, userID string) (domain.CoachReport, error) {
			require.Equal(t, "mock-123", mockID)
			require.Equal(t, "user_1", userID)
			return existing, nil
		},
	}
	llm := &fakeLLM{
		generateCoachReportFunc: func(_ context.Context, _ domain.CoachReportSeed) (string, int, error) {
			t.Fatal("LLM must NOT be called when a report already exists")
			return "", 0, nil
		},
	}
	svc := NewInterviewService(store, llm)

	got, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Equal(t, existing, got)
	require.Empty(t, store.insertCoachReportCalls, "no insert when report already exists")
}

func TestGenerateCoachReport_NotOwnedInterviewReturnsNotFound(t *testing.T) {
	store := &fakeStore{
		// No existing coach report → falls through to ownership check.
		getInterviewByMockIDFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return domain.MockInterview{}, domain.ErrNotFound
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	_, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGenerateCoachReport_RejectsWhenNoAnswers(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		// listAnswersByMockIDFunc unset → default empty slice
	}
	llm := &fakeLLM{
		generateCoachReportFunc: func(_ context.Context, _ domain.CoachReportSeed) (string, int, error) {
			t.Fatal("LLM must NOT be called when no answers exist")
			return "", 0, nil
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGenerateCoachReport_LLMFailurePropagatesAsErrLLM(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 7)}, nil
		},
	}
	llm := &fakeLLM{
		generateCoachReportFunc: func(_ context.Context, _ domain.CoachReportSeed) (string, int, error) {
			return "", 0, fmt.Errorf("upstream blew up: %w", domain.ErrLLM)
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.ErrorIs(t, err, domain.ErrLLM)
}

func TestGenerateCoachReport_HappyPath_PersistsAndReturnsReadback(t *testing.T) {
	// First GetCoachReportByMockID call returns NotFound (no cache), second
	// (post-insert readback) returns the saved row.
	var getCalls int
	saved := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nyou did well",
		TokensUsed: 1500,
		Model:      "gemini-test",
		CreatedAt:  time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			getCalls++
			if getCalls == 1 {
				return domain.CoachReport{}, domain.ErrNotFound
			}
			return saved, nil
		},
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{
				sampleAnswer(mockID, 0, 8),
				sampleAnswer(mockID, 1, 5),
			}, nil
		},
	}
	var seenSeed domain.CoachReportSeed
	llm := &fakeLLM{
		model: "gemini-test",
		generateCoachReportFunc: func(_ context.Context, in domain.CoachReportSeed) (string, int, error) {
			seenSeed = in
			return "## Overall Performance\nyou did well", 1500, nil
		},
	}
	svc := NewInterviewService(store, llm)

	got, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Equal(t, saved, got)

	// LLM seed assembly: role + experience + both answers reached the LLM.
	require.Equal(t, "Backend Engineer", seenSeed.JobPosition)
	require.Equal(t, 5, seenSeed.YearsExperience)
	require.Len(t, seenSeed.Answers, 2)

	// Persistence: exactly one insert with the LLM output + model attribution.
	require.Len(t, store.insertCoachReportCalls, 1)
	require.Equal(t, "mock-123", store.insertCoachReportCalls[0].MockID)
	require.Equal(t, 1500, store.insertCoachReportCalls[0].TokensUsed)
	require.Equal(t, "gemini-test", store.insertCoachReportCalls[0].Model)
	require.Contains(t, store.insertCoachReportCalls[0].Content, "## Overall Performance")
}

func TestGenerateCoachReport_RaceConflictReturnsWinner(t *testing.T) {
	// Simulate two concurrent POSTs: the cache miss happens first, the
	// other request wins the insert, and our insert returns ErrConflict.
	// The service must swallow the conflict and re-read the winner.
	var getCalls int
	winner := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nwinning content",
		TokensUsed: 2000,
		Model:      "gemini-test",
		CreatedAt:  time.Now().UTC(),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			getCalls++
			if getCalls == 1 {
				return domain.CoachReport{}, domain.ErrNotFound
			}
			return winner, nil
		},
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 7)}, nil
		},
		insertCoachReportFunc: func(_ context.Context, _ domain.CoachReport) error {
			return fmt.Errorf("simulated race: %w", domain.ErrConflict)
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	got, err := svc.GenerateCoachReport(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Equal(t, winner, got)
}

// --- GetCoachReport ---

func TestGetCoachReport_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GetCoachReport(context.Background(), "", "mock-123")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestGetCoachReport_RequiresMockID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GetCoachReport(context.Background(), "user_1", "")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGetCoachReport_NotFoundWhenMissing(t *testing.T) {
	// Default fakeStore.getCoachReportByMockIDFunc returns ErrNotFound.
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.GetCoachReport(context.Background(), "user_1", "mock-123")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetCoachReport_HappyPath(t *testing.T) {
	want := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nsome report",
		TokensUsed: 800,
		Model:      "gemini-test",
		CreatedAt:  time.Now().UTC(),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, mockID, userID string) (domain.CoachReport, error) {
			require.Equal(t, "mock-123", mockID)
			require.Equal(t, "user_1", userID)
			return want, nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	got, err := svc.GetCoachReport(context.Background(), "user_1", "mock-123")
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// --- SubmitAnswer background embedding ---

func TestSubmitAnswer_LaunchesBackgroundEmbed_OnHappyPath(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, userID string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{
				sampleAnswer(mockID, 0, 7),
			}, nil
		},
	}
	var embedCalled string
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, text string) ([]float32, error) {
			embedCalled = text
			return []float32{0.5, 0.6, 0.7}, nil
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		UserAnswer:    "my answer about goroutines and channels",
	})
	require.NoError(t, err)

	// Background goroutine fires-and-forgets: poll until it lands.
	require.Eventually(t, func() bool {
		return store.embedCallCount() == 1 && embedCalled != ""
	}, time.Second, 5*time.Millisecond)

	// Embedded text combines the question text and the user's answer so
	// the vector captures both sides of the topic surface.
	require.Contains(t, embedCalled, "my answer about goroutines and channels")

	store.mu.Lock()
	call := store.updateEmbeddingCalls[0]
	store.mu.Unlock()
	require.Equal(t, "mock-123", call.mockID)
	require.Equal(t, "user_1", call.clerkUserID)
	require.Equal(t, 0, call.questionIndex)
	require.Equal(t, []float32{0.5, 0.6, 0.7}, call.vec)
}

func TestSubmitAnswer_EmbeddingFailure_DoesNotAffectResponse(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, userID string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 7)}, nil
		},
	}
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("upstream embed failure: %w", domain.ErrLLM)
		},
	}
	svc := NewInterviewService(store, llm)

	answer, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		UserAnswer:    "an answer",
	})
	require.NoError(t, err, "embed failure must not surface in the response")
	require.Equal(t, 0, answer.QuestionIndex)

	// Give the goroutine time to do its (failing) work, then assert no
	// embedding was persisted — the failure was swallowed.
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 0, store.embedCallCount(), "no embedding persisted on LLM failure")
}

func TestSubmitAnswer_EmbeddingNotCalled_WhenSubmitFails(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
	}
	var embedCalls int
	llm := &fakeLLM{
		evaluateAnswerFunc: func(_ context.Context, _ domain.AnswerSeed) (domain.Evaluation, error) {
			return domain.Evaluation{}, fmt.Errorf("eval failed: %w", domain.ErrLLM)
		},
		embedFunc: func(_ context.Context, _ string) ([]float32, error) {
			embedCalls++
			return nil, nil
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.SubmitAnswer(context.Background(), "user_1", SubmitAnswerInput{
		MockID:        "mock-123",
		QuestionIndex: 0,
		UserAnswer:    "x",
	})
	require.Error(t, err)

	time.Sleep(20 * time.Millisecond) // give any rogue goroutine a chance to fire
	require.Equal(t, 0, embedCalls, "embedding must NOT be attempted when submit fails")
	require.Equal(t, 0, store.embedCallCount())
}

// --- CreateInterview adaptive enrichment ---

func TestBuildInterviewSeed_FirstTimeUser_NoEnrichment(t *testing.T) {
	// Default fakeStore returns nil for history and weak hits. No matter
	// the LLM result, seed should be the basic shape with empty enrichment.
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})

	seed := svc.buildInterviewSeed(context.Background(),
		CreateInterviewInput{
			JobPosition:     "Backend Engineer",
			JobDescription:  "Build Go services",
			YearsExperience: 5,
		},
		"user_1",
		"resume text here",
	)
	require.Equal(t, "Backend Engineer", seed.JobPosition)
	require.Equal(t, "resume text here", seed.ResumeText)
	require.Empty(t, seed.PastInterviews)
	require.Empty(t, seed.WeakAreas)
}

func TestBuildInterviewSeed_ReturningUser_EnrichesWithHistoryAndWeak(t *testing.T) {
	history := []domain.PastInterviewSummary{
		{MockID: "mock-prev", JobPosition: "Backend Engineer", AvgRating: 5.4, AnsweredCount: 5},
	}
	weak := []domain.WeakAnswerHit{
		{MockID: "mock-prev", QuestionText: "Explain MVCC", Rating: 4, Feedback: "no vacuum"},
	}
	store := &fakeStore{
		listUserInterviewSummariesFunc: func(_ context.Context, userID string, limit int) ([]domain.PastInterviewSummary, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, coachHistoryLimit, limit)
			return history, nil
		},
		findSimilarWeakAnswersFunc: func(_ context.Context, userID string, vec []float32, threshold, limit int) ([]domain.WeakAnswerHit, error) {
			require.Equal(t, "user_1", userID)
			require.NotEmpty(t, vec)
			require.Equal(t, coachWeakRatingMax, threshold)
			require.Equal(t, coachWeakHitsLimit, limit)
			return weak, nil
		},
	}
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, text string) ([]float32, error) {
			require.Contains(t, text, "Backend Engineer", "role embed should include the job position")
			require.Contains(t, text, "distributed systems", "role embed should include the job description")
			return []float32{0.1, 0.2, 0.3}, nil
		},
	}
	svc := NewInterviewService(store, llm)

	seed := svc.buildInterviewSeed(context.Background(),
		CreateInterviewInput{
			JobPosition:     "Backend Engineer",
			JobDescription:  "Build distributed systems",
			YearsExperience: 4,
		},
		"user_1",
		"",
	)
	require.Equal(t, history, seed.PastInterviews)
	require.Equal(t, weak, seed.WeakAreas)
}

func TestBuildInterviewSeed_EmbedFailure_StillReturnsHistory(t *testing.T) {
	history := []domain.PastInterviewSummary{
		{MockID: "mock-prev", JobPosition: "Backend Engineer", AvgRating: 6.0, AnsweredCount: 5},
	}
	store := &fakeStore{
		listUserInterviewSummariesFunc: func(_ context.Context, _ string, _ int) ([]domain.PastInterviewSummary, error) {
			return history, nil
		},
		findSimilarWeakAnswersFunc: func(_ context.Context, _ string, _ []float32, _, _ int) ([]domain.WeakAnswerHit, error) {
			t.Fatal("weak search must NOT run when embed failed")
			return nil, nil
		},
	}
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("embed down: %w", domain.ErrLLM)
		},
	}
	svc := NewInterviewService(store, llm)

	seed := svc.buildInterviewSeed(context.Background(),
		CreateInterviewInput{
			JobPosition:     "Backend Engineer",
			JobDescription:  "Build distributed systems",
			YearsExperience: 4,
		},
		"user_1",
		"",
	)
	require.Equal(t, history, seed.PastInterviews)
	require.Empty(t, seed.WeakAreas)
}

func TestCreateInterview_FeedsEnrichedSeedToLLM(t *testing.T) {
	// End-to-end shape: CreateInterview wires buildInterviewSeed into
	// the GenerateQuestions call, so the LLM observes the enrichment.
	history := []domain.PastInterviewSummary{
		{MockID: "mock-prev", JobPosition: "Backend Engineer", AvgRating: 5.4, AnsweredCount: 5},
	}
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listUserInterviewSummariesFunc: func(_ context.Context, _ string, _ int) ([]domain.PastInterviewSummary, error) {
			return history, nil
		},
	}
	var seenSeed domain.InterviewSeed
	llm := &fakeLLM{
		generateQuestionsFunc: func(_ context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error) {
			seenSeed = in
			return defaultQuestions(), nil
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.CreateInterview(context.Background(), "user_1", CreateInterviewInput{
		JobPosition:     "Backend Engineer",
		JobDescription:  validJobDescription,
		YearsExperience: 5,
	})
	require.NoError(t, err)
	require.Equal(t, history, seenSeed.PastInterviews,
		"CreateInterview must surface past-interview history to the question-generation LLM")
}

// --- Coach report enrichment ---

func TestBuildCoachReportSeed_EnrichesWithHistoryAndFiltersCurrentInterview(t *testing.T) {
	mi := sampleInterview("user_1")
	answers := []domain.UserAnswer{sampleAnswer(mi.MockID, 0, 7)}

	history := []domain.PastInterviewSummary{
		{MockID: mi.MockID, JobPosition: "Backend Engineer", AvgRating: 7.0, AnsweredCount: 1, CreatedAt: time.Now()},
		{MockID: "mock-prev", JobPosition: "Backend Engineer", AvgRating: 6.4, AnsweredCount: 5, CreatedAt: time.Now().Add(-7 * 24 * time.Hour)},
	}
	weak := []domain.WeakAnswerHit{
		{MockID: mi.MockID, QuestionText: "current Q", Rating: 4, Feedback: "current weak"},
		{MockID: "mock-prev", QuestionText: "prior Q", Rating: 5, Feedback: "prior weak"},
	}

	store := &fakeStore{
		listUserInterviewSummariesFunc: func(_ context.Context, userID string, limit int) ([]domain.PastInterviewSummary, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, coachHistoryLimit, limit)
			return history, nil
		},
		findSimilarWeakAnswersFunc: func(_ context.Context, userID string, vec []float32, threshold, limit int) ([]domain.WeakAnswerHit, error) {
			require.Equal(t, "user_1", userID)
			require.NotEmpty(t, vec)
			require.Equal(t, coachWeakRatingMax, threshold)
			require.Equal(t, coachWeakHitsLimit, limit)
			return weak, nil
		},
	}
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, text string) ([]float32, error) {
			require.Contains(t, text, mi.JobPosition, "role embed should include the job position surface")
			return []float32{0.1, 0.2, 0.3}, nil
		},
	}
	svc := NewInterviewService(store, llm)

	seed := svc.buildCoachReportSeed(context.Background(), mi, answers)
	require.Equal(t, history, seed.PastInterviews)
	// Current interview's weak hit is filtered out — only the prior one
	// survives because the LLM is about to re-narrate the current.
	require.Len(t, seed.RecurringWeak, 1)
	require.Equal(t, "mock-prev", seed.RecurringWeak[0].MockID)
}

func TestBuildCoachReportSeed_FirstTimeUser_NoEnrichment(t *testing.T) {
	mi := sampleInterview("user_1")
	answers := []domain.UserAnswer{sampleAnswer(mi.MockID, 0, 7)}

	// Default fakeStore returns nil for both history and weak hits.
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})

	seed := svc.buildCoachReportSeed(context.Background(), mi, answers)
	require.Empty(t, seed.PastInterviews)
	require.Empty(t, seed.RecurringWeak)
	// Basic fields still populated — enrichment is purely additive.
	require.Equal(t, mi.JobPosition, seed.JobPosition)
	require.Equal(t, answers, seed.Answers)
}

func TestBuildCoachReportSeed_EmbedFailure_StillReturnsHistory(t *testing.T) {
	mi := sampleInterview("user_1")
	history := []domain.PastInterviewSummary{
		{MockID: "mock-prev", JobPosition: "Backend Engineer", AvgRating: 6.0, AnsweredCount: 5, CreatedAt: time.Now()},
	}
	store := &fakeStore{
		listUserInterviewSummariesFunc: func(_ context.Context, _ string, _ int) ([]domain.PastInterviewSummary, error) {
			return history, nil
		},
		findSimilarWeakAnswersFunc: func(_ context.Context, _ string, _ []float32, _, _ int) ([]domain.WeakAnswerHit, error) {
			t.Fatal("must not query weak answers when embed failed")
			return nil, nil
		},
	}
	llm := &fakeLLM{
		embedFunc: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("embed down: %w", domain.ErrLLM)
		},
	}
	svc := NewInterviewService(store, llm)

	seed := svc.buildCoachReportSeed(context.Background(), mi, []domain.UserAnswer{sampleAnswer(mi.MockID, 0, 7)})
	require.Equal(t, history, seed.PastInterviews)
	require.Empty(t, seed.RecurringWeak)
}

// --- StreamCoachReport ---

func TestStreamCoachReport_RequiresUserID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.StreamCoachReport(context.Background(), "", "mock-123", func(string) error { return nil })
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestStreamCoachReport_RequiresMockID(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.StreamCoachReport(context.Background(), "user_1", "  ", func(string) error { return nil })
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestStreamCoachReport_RequiresOnChunkCallback(t *testing.T) {
	svc := NewInterviewService(&fakeStore{}, &fakeLLM{})
	_, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", nil)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestStreamCoachReport_CachedHitEmitsFullContentAndSkipsLLM(t *testing.T) {
	existing := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nfull cached body",
		TokensUsed: 999,
		Model:      "gemini-cached",
		CreatedAt:  time.Now().UTC(),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			return existing, nil
		},
	}
	llm := &fakeLLM{
		generateCoachReportStreamFunc: func(_ context.Context, _ domain.CoachReportSeed, _ func(string) error) (int, error) {
			t.Fatal("LLM must NOT be called when a report already exists")
			return 0, nil
		},
	}
	svc := NewInterviewService(store, llm)

	var chunks []string
	got, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", func(s string) error {
		chunks = append(chunks, s)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, existing, got)
	// Cached hit is one atomic chunk containing the full content.
	require.Equal(t, []string{existing.Content}, chunks)
	require.Empty(t, store.insertCoachReportCalls, "no insert on cached hit")
}

func TestStreamCoachReport_RejectsWhenNoAnswers(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	_, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", func(string) error { return nil })
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestStreamCoachReport_HappyPath_StreamsAndPersists(t *testing.T) {
	// First Get returns NotFound (no cache); second (readback) returns saved.
	var getCalls int
	saved := domain.CoachReport{
		MockID:     "mock-123",
		Content:    "## Overall Performance\nfreshly streamed body",
		TokensUsed: 1500,
		Model:      "gemini-test",
		CreatedAt:  time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			getCalls++
			if getCalls == 1 {
				return domain.CoachReport{}, domain.ErrNotFound
			}
			return saved, nil
		},
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 8)}, nil
		},
	}
	llm := &fakeLLM{
		model: "gemini-test",
		generateCoachReportStreamFunc: func(_ context.Context, _ domain.CoachReportSeed, onChunk func(string) error) (int, error) {
			// Emit three chunks; the service must buffer them all for persistence.
			require.NoError(t, onChunk("## Overall Performance\n"))
			require.NoError(t, onChunk("freshly "))
			require.NoError(t, onChunk("streamed body"))
			return 1500, nil
		},
	}
	svc := NewInterviewService(store, llm)

	var chunks []string
	got, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", func(s string) error {
		chunks = append(chunks, s)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, saved, got)

	// Caller observed every chunk in order.
	require.Equal(t, []string{"## Overall Performance\n", "freshly ", "streamed body"}, chunks)

	// Server persisted the full concatenated buffer with model + tokens.
	require.Len(t, store.insertCoachReportCalls, 1)
	require.Equal(t, "## Overall Performance\nfreshly streamed body", store.insertCoachReportCalls[0].Content)
	require.Equal(t, 1500, store.insertCoachReportCalls[0].TokensUsed)
	require.Equal(t, "gemini-test", store.insertCoachReportCalls[0].Model)
}

func TestStreamCoachReport_CallbackErrorAbortsWithoutPersisting(t *testing.T) {
	store := &fakeStore{
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 8)}, nil
		},
	}
	cbErr := fmt.Errorf("client disconnected")
	llm := &fakeLLM{
		generateCoachReportStreamFunc: func(_ context.Context, _ domain.CoachReportSeed, onChunk func(string) error) (int, error) {
			// The first chunk's callback fails; the LLM wrapper bubbles
			// that error up wrapped — service must NOT persist.
			if err := onChunk("partial"); err != nil {
				return 0, fmt.Errorf("stream: %w", err)
			}
			return 0, nil
		},
	}
	svc := NewInterviewService(store, llm)

	_, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", func(string) error {
		return cbErr
	})
	require.Error(t, err)
	require.Empty(t, store.insertCoachReportCalls, "partial report must not be persisted")
}

func TestStreamCoachReport_RaceConflictReturnsWinner(t *testing.T) {
	var getCalls int
	winner := domain.CoachReport{
		MockID:    "mock-123",
		Content:   "## Overall Performance\nwinning cached body",
		Model:     "gemini-test",
		CreatedAt: time.Now().UTC(),
	}
	store := &fakeStore{
		getCoachReportByMockIDFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			getCalls++
			if getCalls == 1 {
				return domain.CoachReport{}, domain.ErrNotFound
			}
			return winner, nil
		},
		getInterviewByMockIDFunc: func(_ context.Context, _, userID string) (domain.MockInterview, error) {
			return sampleInterview(userID), nil
		},
		listAnswersByMockIDFunc: func(_ context.Context, mockID, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{sampleAnswer(mockID, 0, 7)}, nil
		},
		insertCoachReportFunc: func(_ context.Context, _ domain.CoachReport) error {
			return fmt.Errorf("simulated race: %w", domain.ErrConflict)
		},
	}
	svc := NewInterviewService(store, &fakeLLM{})

	got, err := svc.StreamCoachReport(context.Background(), "user_1", "mock-123", func(string) error { return nil })
	require.NoError(t, err)
	require.Equal(t, winner, got)
}
