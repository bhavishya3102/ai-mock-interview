package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/auth"
	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/bhavishya3102/ai-mock-interview/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeSvc implements InterviewService for handler tests. Per-test override
// via *Func fields.
type fakeSvc struct {
	createInterviewFunc     func(ctx context.Context, userID string, in service.CreateInterviewInput) (domain.MockInterview, error)
	listInterviewsFunc      func(ctx context.Context, userID string, limit int) ([]domain.InterviewSummary, error)
	getInterviewFunc        func(ctx context.Context, userID, mockID string) (domain.MockInterview, error)
	submitAnswerFunc        func(ctx context.Context, userID string, in service.SubmitAnswerInput) (domain.UserAnswer, error)
	listFeedbackFunc        func(ctx context.Context, userID, mockID string) ([]domain.UserAnswer, error)
	transcribeAudioFunc     func(ctx context.Context, userID, mockID string, audio []byte, mimeType string, longPauseCount int) (domain.TranscriptResult, error)
	judgeFollowUpFunc       func(ctx context.Context, userID string, in service.JudgeFollowUpInput) (string, error)
	uploadResumeFunc        func(ctx context.Context, userID string, pdf []byte, mimeType string) (domain.ResumeStatus, error)
	getResumeFunc           func(ctx context.Context, userID string) (domain.ResumeStatus, error)
	deleteResumeFunc        func(ctx context.Context, userID string) error
	generateCoachReportFunc func(ctx context.Context, userID, mockID string) (domain.CoachReport, error)
	getCoachReportFunc      func(ctx context.Context, userID, mockID string) (domain.CoachReport, error)
}

func (f *fakeSvc) CreateInterview(ctx context.Context, userID string, in service.CreateInterviewInput) (domain.MockInterview, error) {
	return f.createInterviewFunc(ctx, userID, in)
}
func (f *fakeSvc) ListInterviews(ctx context.Context, userID string, limit int) ([]domain.InterviewSummary, error) {
	return f.listInterviewsFunc(ctx, userID, limit)
}
func (f *fakeSvc) GetInterview(ctx context.Context, userID, mockID string) (domain.MockInterview, error) {
	return f.getInterviewFunc(ctx, userID, mockID)
}
func (f *fakeSvc) SubmitAnswer(ctx context.Context, userID string, in service.SubmitAnswerInput) (domain.UserAnswer, error) {
	return f.submitAnswerFunc(ctx, userID, in)
}
func (f *fakeSvc) ListFeedback(ctx context.Context, userID, mockID string) ([]domain.UserAnswer, error) {
	return f.listFeedbackFunc(ctx, userID, mockID)
}
func (f *fakeSvc) TranscribeAudio(ctx context.Context, userID, mockID string, audio []byte, mimeType string, longPauseCount int) (domain.TranscriptResult, error) {
	if f.transcribeAudioFunc == nil {
		return domain.TranscriptResult{}, nil
	}
	return f.transcribeAudioFunc(ctx, userID, mockID, audio, mimeType, longPauseCount)
}
func (f *fakeSvc) JudgeFollowUp(ctx context.Context, userID string, in service.JudgeFollowUpInput) (string, error) {
	if f.judgeFollowUpFunc == nil {
		return "", nil
	}
	return f.judgeFollowUpFunc(ctx, userID, in)
}
func (f *fakeSvc) UploadResume(ctx context.Context, userID string, pdf []byte, mimeType string) (domain.ResumeStatus, error) {
	if f.uploadResumeFunc == nil {
		return domain.ResumeStatus{}, nil
	}
	return f.uploadResumeFunc(ctx, userID, pdf, mimeType)
}
func (f *fakeSvc) GetResume(ctx context.Context, userID string) (domain.ResumeStatus, error) {
	if f.getResumeFunc == nil {
		return domain.ResumeStatus{}, nil
	}
	return f.getResumeFunc(ctx, userID)
}
func (f *fakeSvc) DeleteResume(ctx context.Context, userID string) error {
	if f.deleteResumeFunc == nil {
		return nil
	}
	return f.deleteResumeFunc(ctx, userID)
}
func (f *fakeSvc) GenerateCoachReport(ctx context.Context, userID, mockID string) (domain.CoachReport, error) {
	if f.generateCoachReportFunc == nil {
		return domain.CoachReport{}, nil
	}
	return f.generateCoachReportFunc(ctx, userID, mockID)
}
func (f *fakeSvc) GetCoachReport(ctx context.Context, userID, mockID string) (domain.CoachReport, error) {
	if f.getCoachReportFunc == nil {
		return domain.CoachReport{}, nil
	}
	return f.getCoachReportFunc(ctx, userID, mockID)
}

// withUserID returns a request whose context carries a Clerk user ID — used
// to simulate the auth middleware having already run.
func withUserID(req *http.Request, userID string) *http.Request {
	if userID == "" {
		return req
	}
	return req.WithContext(auth.WithUserID(req.Context(), userID))
}

// route wraps handler routes for tests so chi's URLParam works.
func interviewRouter(h *InterviewHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/api/v1/interviews", h.Create)
	r.Get("/api/v1/interviews", h.List)
	r.Get("/api/v1/interviews/{mockId}", h.Get)
	r.Post("/api/v1/interviews/{mockId}/answers", h.SubmitAnswer)
	r.Get("/api/v1/interviews/{mockId}/feedback", h.ListFeedback)
	r.Post("/api/v1/interviews/{mockId}/transcribe", h.Transcribe)
	r.Post("/api/v1/interviews/{mockId}/coach-report", h.GenerateCoachReport)
	r.Get("/api/v1/interviews/{mockId}/coach-report", h.GetCoachReport)
	r.Post("/api/v1/resume", h.UploadResume)
	r.Get("/api/v1/resume", h.GetResume)
	r.Delete("/api/v1/resume", h.DeleteResume)
	return r
}

func sampleMockInterview() domain.MockInterview {
	return domain.MockInterview{
		MockID:          "mock-abc",
		ClerkUserID:     "user_1",
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build distributed Go services",
		YearsExperience: 5,
		Questions: []domain.GeneratedQA{
			{Question: "explain X", Answer: "X is..."},
			{Question: "explain Y", Answer: "Y is..."},
			{Question: "explain Z", Answer: "Z is..."},
			{Question: "explain W", Answer: "W is..."},
			{Question: "explain V", Answer: "V is..."},
		},
		CreatedAt: time.Now().UTC(),
	}
}

// --- Create ---

func TestCreate_Unauthorized_NoUserInContext(t *testing.T) {
	h := NewInterviewHandler(&fakeSvc{}, discardLogger())
	body := `{"jobPosition":"Backend Engineer","jobDescription":"Build distributed Go services","yearsExperience":5}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(body))
	rec := httptest.NewRecorder()

	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "unauthorized")
}

func TestCreate_BadRequestCases(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid json", "not-json"},
		{"missing fields", `{}`},
		{"position too short", `{"jobPosition":"a","jobDescription":"long enough description for ten plus","yearsExperience":3}`},
		{"description too short", `{"jobPosition":"Backend","jobDescription":"short","yearsExperience":3}`},
		{"years out of range", `{"jobPosition":"Backend","jobDescription":"long enough description here","yearsExperience":61}`},
		{"unknown field", `{"jobPosition":"Backend","jobDescription":"long enough description here","yearsExperience":3,"junk":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInterviewHandler(&fakeSvc{}, discardLogger())
			req := withUserID(
				httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(tt.body)),
				"user_1",
			)
			rec := httptest.NewRecorder()
			interviewRouter(h).ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "validation_failed")
		})
	}
}

func TestCreate_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		createInterviewFunc: func(_ context.Context, userID string, in service.CreateInterviewInput) (domain.MockInterview, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, "Backend Engineer", in.JobPosition)
			return sampleMockInterview(), nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	body := `{"jobPosition":"Backend Engineer","jobDescription":"Build distributed Go services","yearsExperience":5}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp interviewResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "mock-abc", resp.MockID)
	require.Len(t, resp.Questions, 5)
}

func TestCreate_LLMFailureMapsTo502(t *testing.T) {
	svc := &fakeSvc{
		createInterviewFunc: func(_ context.Context, _ string, _ service.CreateInterviewInput) (domain.MockInterview, error) {
			return domain.MockInterview{}, errors.Join(domain.ErrLLM, errors.New("upstream"))
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	body := `{"jobPosition":"Backend Engineer","jobDescription":"Build distributed Go services","yearsExperience":5}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "llm_failure")
}

func TestCreate_UnknownErrorMapsTo500(t *testing.T) {
	svc := &fakeSvc{
		createInterviewFunc: func(_ context.Context, _ string, _ service.CreateInterviewInput) (domain.MockInterview, error) {
			return domain.MockInterview{}, errors.New("totally unexpected")
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	body := `{"jobPosition":"Backend Engineer","jobDescription":"Build distributed Go services","yearsExperience":5}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "internal_error")
}

// --- List ---

func TestList_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		listInterviewsFunc: func(_ context.Context, _ string, _ int) ([]domain.InterviewSummary, error) {
			return []domain.InterviewSummary{
				{MockID: "m1", JobPosition: "BE", JobDescription: "x", YearsExperience: 3, CreatedAt: time.Now().UTC()},
			}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got struct {
		Items []domain.InterviewSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Items, 1)
	require.Equal(t, "m1", got.Items[0].MockID)
}

func TestList_InvalidLimit(t *testing.T) {
	svc := &fakeSvc{
		listInterviewsFunc: func(_ context.Context, _ string, _ int) ([]domain.InterviewSummary, error) {
			return []domain.InterviewSummary{}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews?limit=abc", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestGet_NotFound(t *testing.T) {
	svc := &fakeSvc{
		getInterviewFunc: func(_ context.Context, _, _ string) (domain.MockInterview, error) {
			return domain.MockInterview{}, domain.ErrNotFound
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-x", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "not_found")
}

func TestGet_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		getInterviewFunc: func(_ context.Context, _, mockID string) (domain.MockInterview, error) {
			require.Equal(t, "mock-abc", mockID)
			return sampleMockInterview(), nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-abc", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var got interviewResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "mock-abc", got.MockID)
}

// --- SubmitAnswer ---

func TestSubmitAnswer_BadIndex(t *testing.T) {
	h := NewInterviewHandler(&fakeSvc{}, discardLogger())
	body := `{"questionIndex":-1,"userAnswer":"answer"}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/answers", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSubmitAnswer_DuplicateMapsTo409(t *testing.T) {
	svc := &fakeSvc{
		submitAnswerFunc: func(_ context.Context, _ string, _ service.SubmitAnswerInput) (domain.UserAnswer, error) {
			return domain.UserAnswer{}, domain.ErrConflict
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	body := `{"questionIndex":0,"userAnswer":"my answer"}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/answers", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "conflict")
}

func TestSubmitAnswer_NotFoundFromInterview(t *testing.T) {
	svc := &fakeSvc{
		submitAnswerFunc: func(_ context.Context, _ string, _ service.SubmitAnswerInput) (domain.UserAnswer, error) {
			return domain.UserAnswer{}, domain.ErrNotFound
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	body := `{"questionIndex":0,"userAnswer":"my answer"}`
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/answers", strings.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSubmitAnswer_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		submitAnswerFunc: func(_ context.Context, _ string, in service.SubmitAnswerInput) (domain.UserAnswer, error) {
			require.Equal(t, "mock-abc", in.MockID)
			require.Equal(t, 0, in.QuestionIndex)
			return domain.UserAnswer{
				MockID: in.MockID, QuestionIndex: 0, Rating: 7,
				Feedback: "good but missed structure", CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	body, err := json.Marshal(map[string]any{
		"questionIndex": 0,
		"userAnswer":    "this is a thoughtful answer",
	})
	require.NoError(t, err)

	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/answers", bytes.NewReader(body)),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var got submitAnswerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, 7, got.Rating)
	require.Equal(t, 0, got.QuestionIndex)
}

// --- ListFeedback ---

func TestListFeedback_NotFound(t *testing.T) {
	svc := &fakeSvc{
		listFeedbackFunc: func(_ context.Context, _, _ string) ([]domain.UserAnswer, error) {
			return nil, domain.ErrNotFound
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(
		httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-x/feedback", nil),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListFeedback_HappyPath(t *testing.T) {
	now := time.Now().UTC()
	svc := &fakeSvc{
		listFeedbackFunc: func(_ context.Context, _, _ string) ([]domain.UserAnswer, error) {
			return []domain.UserAnswer{
				{
					QuestionIndex: 0,
					QuestionText:  "explain X",
					UserAnswer:    "my X take",
					CorrectAnswer: "X is the canonical answer",
					Rating:        9,
					Feedback:      "great",
					CreatedAt:     now,
				},
				{
					QuestionIndex: 1,
					QuestionText:  "explain Y",
					UserAnswer:    "",
					CorrectAnswer: "Y is the canonical answer",
					Rating:        6,
					Feedback:      "ok",
					CreatedAt:     now,
				},
			}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())
	req := withUserID(
		httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-abc/feedback", nil),
		"user_1",
	)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Items []feedbackResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Items, 2)
	require.Equal(t, 0, got.Items[0].QuestionIndex)
	require.Equal(t, "explain X", got.Items[0].Question)
	require.Equal(t, "my X take", got.Items[0].UserAnswer)
	require.Equal(t, "X is the canonical answer", got.Items[0].CorrectAnswer)
	require.Equal(t, 9, got.Items[0].Rating)
	require.Equal(t, "great", got.Items[0].Feedback)
	require.Equal(t, 1, got.Items[1].QuestionIndex)
	require.Equal(t, "", got.Items[1].UserAnswer)
}

// --- Transcribe ---

// audioMultipart builds a multipart body with the audio part plus the
// longPauseCount field, mirroring what the browser's MediaRecorder + VAD
// produces.
func audioMultipart(t *testing.T, mimeType string, payload []byte, longPauseCount int) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="audio"; filename="answer.webm"`)
	hdr.Set("Content-Type", mimeType)
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.WriteField("longPauseCount", strconv.Itoa(longPauseCount)))
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func TestTranscribe_HappyPath_IncludesAnalysis(t *testing.T) {
	svc := &fakeSvc{
		transcribeAudioFunc: func(_ context.Context, _, mockID string, audio []byte, mimeType string, longPauseCount int) (domain.TranscriptResult, error) {
			require.Equal(t, "mock-abc", mockID)
			require.Equal(t, "audio/webm", mimeType)
			require.NotEmpty(t, audio)
			require.Equal(t, 2, longPauseCount, "handler must forward longPauseCount form field")
			return domain.TranscriptResult{
				Transcript: "um so I uh built it",
				Analysis: domain.SpeechAnalysis{
					FillerCount:    3,
					WordsPerMinute: 118,
					LongPauseCount: longPauseCount,
				},
			}, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	body, contentType := audioMultipart(t, "audio/webm", []byte("fake-opus-bytes"), 2)
	req := withUserID(
		httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/transcribe", body),
		"user_1",
	)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got transcribeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "um so I uh built it", got.Transcript)
	require.Equal(t, 3, got.Analysis.FillerCount)
	require.Equal(t, 118, got.Analysis.WordsPerMinute)
	require.Equal(t, 2, got.Analysis.LongPauseCount)
}

// --- error mapping table (sanity check on the central mapper) ---

func TestMapErrToHTTP(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", domain.ErrValidation, http.StatusBadRequest, "validation_failed"},
		{"unauth", domain.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{"not found", domain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"conflict", domain.ErrConflict, http.StatusConflict, "conflict"},
		{"llm", domain.ErrLLM, http.StatusBadGateway, "llm_failure"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErrToHTTP(tt.err)
			require.Equal(t, tt.wantStatus, got.Status)
			require.Equal(t, tt.wantCode, got.Code)
		})
	}
}

// --- GenerateCoachReport (POST /interviews/{mockId}/coach-report) ---

func sampleCoachReport() domain.CoachReport {
	return domain.CoachReport{
		MockID:     "mock-abc",
		Content:    "## Overall Performance\nyou did well on the React questions",
		TokensUsed: 1500,
		Model:      "gemini-test",
		CreatedAt:  time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC),
	}
}

func TestGenerateCoachReport_Unauthorized(t *testing.T) {
	h := NewInterviewHandler(&fakeSvc{}, discardLogger())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/coach-report", nil)
	rec := httptest.NewRecorder()

	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGenerateCoachReport_HappyPath_Returns201WithBody(t *testing.T) {
	want := sampleCoachReport()
	svc := &fakeSvc{
		generateCoachReportFunc: func(_ context.Context, userID, mockID string) (domain.CoachReport, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, "mock-abc", mockID)
			return want, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	req := withUserID(httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/coach-report", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var got coachReportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "mock-abc", got.MockID)
	require.Equal(t, want.Content, got.Content)
	require.Equal(t, want.TokensUsed, got.TokensUsed)
	require.Equal(t, want.Model, got.Model)
}

func TestGenerateCoachReport_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"validation (no answers)", domain.ErrValidation, http.StatusBadRequest, "validation_failed"},
		{"not found / not owned", domain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"llm upstream", domain.ErrLLM, http.StatusBadGateway, "llm_failure"},
		{"internal", errors.New("boom"), http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeSvc{
				generateCoachReportFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
					return domain.CoachReport{}, tt.svcErr
				},
			}
			h := NewInterviewHandler(svc, discardLogger())

			req := withUserID(httptest.NewRequest(http.MethodPost, "/api/v1/interviews/mock-abc/coach-report", nil), "user_1")
			rec := httptest.NewRecorder()
			interviewRouter(h).ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Contains(t, rec.Body.String(), tt.wantCode)
		})
	}
}

// --- GetCoachReport (GET /interviews/{mockId}/coach-report) ---

func TestGetCoachReport_Unauthorized(t *testing.T) {
	h := NewInterviewHandler(&fakeSvc{}, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-abc/coach-report", nil)
	rec := httptest.NewRecorder()

	interviewRouter(h).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetCoachReport_HappyPath_Returns200WithBody(t *testing.T) {
	want := sampleCoachReport()
	svc := &fakeSvc{
		getCoachReportFunc: func(_ context.Context, userID, mockID string) (domain.CoachReport, error) {
			require.Equal(t, "user_1", userID)
			require.Equal(t, "mock-abc", mockID)
			return want, nil
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-abc/coach-report", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got coachReportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, want.MockID, got.MockID)
	require.Equal(t, want.Content, got.Content)
}

func TestGetCoachReport_NotFound(t *testing.T) {
	svc := &fakeSvc{
		getCoachReportFunc: func(_ context.Context, _, _ string) (domain.CoachReport, error) {
			return domain.CoachReport{}, domain.ErrNotFound
		},
	}
	h := NewInterviewHandler(svc, discardLogger())

	req := withUserID(httptest.NewRequest(http.MethodGet, "/api/v1/interviews/mock-abc/coach-report", nil), "user_1")
	rec := httptest.NewRecorder()
	interviewRouter(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "not_found")
}
