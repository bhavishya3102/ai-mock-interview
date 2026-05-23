package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// fakeStore captures call args + lets each method's behaviour be overridden
// per test via *Func fields. Empty defaults are happy path.
type fakeStore struct {
	upsertCalls          []string
	createInterviewCalls []domain.MockInterview
	upsertAnswerCalls    []domain.UserAnswer
	setResumeCalls       []string

	upsertUserFunc           func(ctx context.Context, userID string) error
	setUserResumeFunc        func(ctx context.Context, userID, resumeText string) error
	getUserResumeFunc        func(ctx context.Context, userID string) (string, time.Time, error)
	clearUserResumeFunc      func(ctx context.Context, userID string) error
	createInterviewFunc      func(ctx context.Context, m domain.MockInterview) error
	listInterviewsByUserFunc func(ctx context.Context, userID string, limit int) ([]domain.InterviewSummary, error)
	getInterviewByMockIDFunc func(ctx context.Context, mockID, userID string) (domain.MockInterview, error)
	upsertAnswerFunc         func(ctx context.Context, a domain.UserAnswer) error
	listAnswersByMockIDFunc  func(ctx context.Context, mockID, userID string) ([]domain.UserAnswer, error)
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

type fakeLLM struct {
	generateQuestionsFunc func(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error)
	evaluateAnswerFunc    func(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error)
	transcribeAudioFunc   func(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error)
	extractResumeTextFunc func(ctx context.Context, pdf []byte, mimeType string) (string, error)
	judgeFollowUpFunc     func(ctx context.Context, in domain.FollowUpSeed) (string, error)
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
