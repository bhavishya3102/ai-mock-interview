package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/auth"
	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/bhavishya3102/ai-mock-interview/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// maxTranscribeAudioBytes caps the audio upload at 10 MiB (~10 minutes of
// Opus-encoded speech), well above any single interview answer.
const maxTranscribeAudioBytes int64 = 10 << 20

// allowedAudioMimeTypes are the MediaRecorder containers the browsers we
// support actually emit. We accept the base type and ignore codec parameters
// (e.g. "audio/webm;codecs=opus") — Gemini parses the bytes regardless.
var allowedAudioMimeTypes = map[string]struct{}{
	"audio/webm": {},
	"audio/ogg":  {},
	"audio/mp4":  {},
	"audio/mpeg": {},
	"audio/wav":  {},
}

// InterviewService is the consumer-side interface implemented by
// service.InterviewService. Tests inject a fake.
type InterviewService interface {
	CreateInterview(ctx context.Context, clerkUserID string, in service.CreateInterviewInput) (domain.MockInterview, error)
	ListInterviews(ctx context.Context, clerkUserID string, limit int) ([]domain.InterviewSummary, error)
	GetInterview(ctx context.Context, clerkUserID, mockID string) (domain.MockInterview, error)
	SubmitAnswer(ctx context.Context, clerkUserID string, in service.SubmitAnswerInput) (domain.UserAnswer, error)
	ListFeedback(ctx context.Context, clerkUserID, mockID string) ([]domain.UserAnswer, error)
	TranscribeAudio(ctx context.Context, clerkUserID, mockID string, audio []byte, mimeType string, longPauseCount int) (domain.TranscriptResult, error)
	JudgeFollowUp(ctx context.Context, clerkUserID string, in service.JudgeFollowUpInput) (string, error)
}

// InterviewHandler holds dependencies for the 5 interview endpoints.
type InterviewHandler struct {
	svc InterviewService
	log *slog.Logger
}

func NewInterviewHandler(svc InterviewService, log *slog.Logger) *InterviewHandler {
	return &InterviewHandler{svc: svc, log: log}
}

type createInterviewRequest struct {
	JobPosition     string `json:"jobPosition"     validate:"required,min=2,max=120"`
	JobDescription  string `json:"jobDescription"  validate:"required,min=10,max=4000"`
	YearsExperience int    `json:"yearsExperience" validate:"gte=0,lte=60"`
}

type interviewResponse struct {
	MockID          string               `json:"mockId"`
	JobPosition     string               `json:"jobPosition"`
	JobDescription  string               `json:"jobDescription"`
	YearsExperience int                  `json:"yearsExperience"`
	Questions       []domain.GeneratedQA `json:"questions"`
	CreatedAt       time.Time            `json:"createdAt"`
}

func toInterviewResponse(m domain.MockInterview) interviewResponse {
	return interviewResponse{
		MockID:          m.MockID,
		JobPosition:     m.JobPosition,
		JobDescription:  m.JobDescription,
		YearsExperience: m.YearsExperience,
		Questions:       m.Questions,
		CreatedAt:       m.CreatedAt,
	}
}

// Create handles POST /api/v1/interviews.
func (h *InterviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	var req createInterviewRequest
	if err := decodeAndValidate(r, &req); err != nil {
		h.respondErr(w, r, err, nil)
		return
	}

	mi, err := h.svc.CreateInterview(r.Context(), userID, service.CreateInterviewInput{
		JobPosition:     req.JobPosition,
		JobDescription:  req.JobDescription,
		YearsExperience: req.YearsExperience,
	})
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusCreated, toInterviewResponse(mi))
}

// List handles GET /api/v1/interviews.
func (h *InterviewHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	limit := 0
	if s := r.URL.Query().Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			h.respondErr(w, r, errors.New("invalid limit"), domain.ErrValidation)
			return
		}
		limit = v
	}

	items, err := h.svc.ListInterviews(r.Context(), userID, limit)
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, map[string]any{
		"items": items,
	})
}

// Get handles GET /api/v1/interviews/{mockId}.
func (h *InterviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	mockID := chi.URLParam(r, "mockId")
	mi, err := h.svc.GetInterview(r.Context(), userID, mockID)
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, toInterviewResponse(mi))
}

type submitAnswerRequest struct {
	QuestionIndex int    `json:"questionIndex" validate:"gte=0,lte=4"`
	UserAnswer    string `json:"userAnswer"    validate:"required,min=1,max=8000"`
}

type submitAnswerResponse struct {
	QuestionIndex int       `json:"questionIndex"`
	Rating        int       `json:"rating"`
	Feedback      string    `json:"feedback"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toSubmitAnswerResponse(a domain.UserAnswer) submitAnswerResponse {
	return submitAnswerResponse{
		QuestionIndex: a.QuestionIndex,
		Rating:        a.Rating,
		Feedback:      a.Feedback,
		CreatedAt:     a.CreatedAt,
	}
}

// feedbackResponse is the per-answer payload returned by the feedback list
// endpoint. The review screen needs the full triplet (question, user's answer,
// reference answer) plus rating/feedback to be useful — submitAnswerResponse
// stays minimal because the create-answer caller already has those locally.
type feedbackResponse struct {
	QuestionIndex int       `json:"questionIndex"`
	Question      string    `json:"question"`
	UserAnswer    string    `json:"userAnswer"`
	CorrectAnswer string    `json:"correctAnswer"`
	Rating        int       `json:"rating"`
	Feedback      string    `json:"feedback"`
	CreatedAt     time.Time `json:"createdAt"`
}

func toFeedbackResponse(a domain.UserAnswer) feedbackResponse {
	return feedbackResponse{
		QuestionIndex: a.QuestionIndex,
		Question:      a.QuestionText,
		UserAnswer:    a.UserAnswer,
		CorrectAnswer: a.CorrectAnswer,
		Rating:        a.Rating,
		Feedback:      a.Feedback,
		CreatedAt:     a.CreatedAt,
	}
}

// SubmitAnswer handles POST /api/v1/interviews/{mockId}/answers.
func (h *InterviewHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	mockID := chi.URLParam(r, "mockId")

	var req submitAnswerRequest
	if err := decodeAndValidate(r, &req); err != nil {
		h.respondErr(w, r, err, nil)
		return
	}

	a, err := h.svc.SubmitAnswer(r.Context(), userID, service.SubmitAnswerInput{
		MockID:        mockID,
		QuestionIndex: req.QuestionIndex,
		UserAnswer:    req.UserAnswer,
	})
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusCreated, toSubmitAnswerResponse(a))
}

type speechAnalysisResponse struct {
	FillerCount    int `json:"fillerCount"`
	WordsPerMinute int `json:"wordsPerMinute"`
	LongPauseCount int `json:"longPauseCount"`
}

type transcribeResponse struct {
	Transcript string                 `json:"transcript"`
	Analysis   speechAnalysisResponse `json:"analysis"`
}

type followUpTurnDTO struct {
	Question string `json:"question" validate:"required,min=1,max=2000"`
	Answer   string `json:"answer"   validate:"required,min=1,max=8000"`
}

type judgeFollowUpRequest struct {
	QuestionIndex int               `json:"questionIndex" validate:"gte=0,lte=4"`
	MainAnswer    string            `json:"mainAnswer"    validate:"required,min=1,max=8000"`
	PriorTurns    []followUpTurnDTO `json:"priorTurns"    validate:"dive"`
}

type judgeFollowUpResponse struct {
	FollowUp string `json:"followUp"`
}

// JudgeFollowUp handles POST /api/v1/interviews/{mockId}/follow-up.
//
// Decides whether the candidate's current answer needs one more probing
// follow-up. Empty followUp string means "move on". Hard cap and ownership
// are enforced server-side regardless of what the client sends.
func (h *InterviewHandler) JudgeFollowUp(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	mockID := chi.URLParam(r, "mockId")

	var req judgeFollowUpRequest
	if err := decodeAndValidate(r, &req); err != nil {
		h.respondErr(w, r, err, nil)
		return
	}

	turns := make([]domain.FollowUpTurn, 0, len(req.PriorTurns))
	for _, t := range req.PriorTurns {
		turns = append(turns, domain.FollowUpTurn{Question: t.Question, Answer: t.Answer})
	}

	follow, err := h.svc.JudgeFollowUp(r.Context(), userID, service.JudgeFollowUpInput{
		MockID:        mockID,
		QuestionIndex: req.QuestionIndex,
		MainAnswer:    req.MainAnswer,
		PriorTurns:    turns,
	})
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, judgeFollowUpResponse{FollowUp: follow})
}

// Transcribe handles POST /api/v1/interviews/{mockId}/transcribe.
//
// Accepts a multipart upload with the audio blob in the "audio" field.
// Returns the transcribed text. The frontend then forwards that text through
// the normal SubmitAnswer flow — keeping rating/feedback logic untouched.
func (h *InterviewHandler) Transcribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	mockID := chi.URLParam(r, "mockId")

	// Cap the entire request body before parsing — protects against a client
	// that sets a tiny audio part but pads the multipart envelope.
	r.Body = http.MaxBytesReader(w, r.Body, maxTranscribeAudioBytes+(1<<20))
	if err := r.ParseMultipartForm(maxTranscribeAudioBytes); err != nil {
		h.respondErr(w, r, errors.Join(domain.ErrValidation, err), nil)
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		h.respondErr(w, r, errors.Join(domain.ErrValidation, err), nil)
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		h.respondErr(w, r, errors.New("empty audio"), domain.ErrValidation)
		return
	}
	if header.Size > maxTranscribeAudioBytes {
		h.respondErr(w, r, errors.New("audio too large"), domain.ErrValidation)
		return
	}

	mimeType := normalizeAudioMime(header.Header.Get("Content-Type"))
	if mimeType == "" {
		h.respondErr(w, r, errors.New("unsupported audio mime type"), domain.ErrValidation)
		return
	}

	audio, err := io.ReadAll(io.LimitReader(file, maxTranscribeAudioBytes+1))
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	if int64(len(audio)) > maxTranscribeAudioBytes {
		h.respondErr(w, r, errors.New("audio too large"), domain.ErrValidation)
		return
	}

	// longPauseCount is measured client-side by the recorder's VAD; missing or
	// malformed values silently default to 0 — pauses are advisory, not gating.
	longPauseCount := 0
	if raw := r.FormValue("longPauseCount"); raw != "" {
		if n, perr := strconv.Atoi(raw); perr == nil && n >= 0 {
			longPauseCount = n
		}
	}

	result, err := h.svc.TranscribeAudio(r.Context(), userID, mockID, audio, mimeType, longPauseCount)
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}
	writeJSON(w, h.log, http.StatusOK, transcribeResponse{
		Transcript: result.Transcript,
		Analysis: speechAnalysisResponse{
			FillerCount:    result.Analysis.FillerCount,
			WordsPerMinute: result.Analysis.WordsPerMinute,
			LongPauseCount: result.Analysis.LongPauseCount,
		},
	})
}

// normalizeAudioMime strips codec parameters and validates against the
// allowed container list. Returns "" for unsupported types.
func normalizeAudioMime(raw string) string {
	base := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.Index(base, ";"); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	if _, ok := allowedAudioMimeTypes[base]; !ok {
		return ""
	}
	return base
}

// ListFeedback handles GET /api/v1/interviews/{mockId}/feedback.
func (h *InterviewHandler) ListFeedback(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondErr(w, r, errors.New("missing user id"), domain.ErrUnauthorized)
		return
	}

	mockID := chi.URLParam(r, "mockId")

	answers, err := h.svc.ListFeedback(r.Context(), userID, mockID)
	if err != nil {
		h.respondErr(w, r, err, nil)
		return
	}

	items := make([]feedbackResponse, 0, len(answers))
	for _, a := range answers {
		items = append(items, toFeedbackResponse(a))
	}
	writeJSON(w, h.log, http.StatusOK, map[string]any{
		"items": items,
	})
}

// respondErr is the single place that maps error -> HTTP and emits the slog
// line. Service/repo layers MUST NOT log errors themselves (single-handling
// rule). force lets callers force a sentinel mapping (e.g. when ctx auth is
// missing but we don't have a wrapped error).
func (h *InterviewHandler) respondErr(w http.ResponseWriter, r *http.Request, err error, force error) {
	target := err
	if force != nil {
		target = force
	}
	mapped := mapErrToHTTP(target)
	h.log.LogAttrs(r.Context(), slog.LevelInfo, "request error",
		slog.String("request_id", RequestIDFromContext(r.Context())),
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.Int("status", mapped.Status),
		slog.String("error", err.Error()),
	)
	writeError(w, h.log, mapped.Status, mapped.Code, mapped.Message)
}
