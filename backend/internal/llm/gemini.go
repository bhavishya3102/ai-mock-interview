package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"google.golang.org/genai"
)

// Client is the Gemini-backed LLM implementation. Per-request session: every
// call constructs a fresh GenerateContentConfig — no shared mutable state
// between requests.
type Client struct {
	sdk   *genai.Client
	model string
}

// NewClient constructs a Gemini client. The api key is captured here and never
// logged.
func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("llm: empty api key")
	}
	if model == "" {
		return nil, errors.New("llm: empty model")
	}
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai new client: %w", err)
	}
	return &Client{sdk: sdk, model: model}, nil
}

// GenerateQuestions asks the model for QuestionCount question/answer pairs.
// The response is structured JSON validated by questionGenSchema, so we can
// json.Unmarshal directly without string-mutation hacks.
func (c *Client) GenerateQuestions(ctx context.Context, in domain.InterviewSeed) ([]domain.GeneratedQA, error) {
	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](1.0),
		TopP:             genai.Ptr[float32](0.95),
		ResponseMIMEType: "application/json",
		ResponseSchema:   questionGenSchema,
	}

	resp, err := c.sdk.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(buildQuestionPrompt(in)),
		cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("gemini generate questions: %w", errors.Join(domain.ErrLLM, err))
	}

	raw := resp.Text()
	if raw == "" {
		return nil, fmt.Errorf("gemini empty response: %w", domain.ErrLLM)
	}

	var out struct {
		Questions []domain.GeneratedQA `json:"questions"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("gemini decode questions: %w", errors.Join(domain.ErrLLM, err))
	}
	if len(out.Questions) != QuestionCount {
		return nil, fmt.Errorf("gemini wrong question count %d: %w", len(out.Questions), domain.ErrLLM)
	}
	return out.Questions, nil
}

// TranscribeAudio sends audio bytes to Gemini and returns the verbatim
// transcript plus delivery metrics (filler count, words/min, long pauses).
// All four come back in one structured call — cheaper than a follow-up
// analysis round-trip.
//
// mimeType must match what the browser's MediaRecorder produced (typically
// "audio/webm" with Opus on Chromium/Firefox, "audio/mp4" on Safari). Empty
// audio is rejected before the API round-trip to save quota.
func (c *Client) TranscribeAudio(ctx context.Context, audio []byte, mimeType string) (domain.TranscriptResult, error) {
	if len(audio) == 0 {
		return domain.TranscriptResult{}, fmt.Errorf("gemini transcribe: empty audio: %w", domain.ErrValidation)
	}
	if mimeType == "" {
		return domain.TranscriptResult{}, fmt.Errorf("gemini transcribe: empty mime type: %w", domain.ErrValidation)
	}

	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.0),
		ResponseMIMEType: "application/json",
		ResponseSchema:   transcribeSchema,
	}

	prompt := "Transcribe the spoken audio verbatim into plain text. " +
		"Return JSON with exactly these fields:\n" +
		"- transcript: the verbatim words spoken, INCLUDING every filler (um, uh, er, ah, like, you know, etc.). " +
		"No commentary, speaker labels, timestamps, or markdown. Empty string if silent or unintelligible.\n" +
		"- wordsPerMinute: rate of actual spoken words across audible speech only (exclude long silent gaps). Integer. 0 if no speech."

	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
			genai.NewPartFromBytes(audio, mimeType),
		}, genai.RoleUser),
	}

	resp, err := c.sdk.Models.GenerateContent(ctx, c.model, contents, cfg)
	if err != nil {
		return domain.TranscriptResult{}, fmt.Errorf("gemini transcribe audio: %w", errors.Join(domain.ErrLLM, err))
	}

	raw := resp.Text()
	if raw == "" {
		return domain.TranscriptResult{}, fmt.Errorf("gemini empty response: %w", domain.ErrLLM)
	}

	// Gemini's schema is flat (transcript + wordsPerMinute at top level), but
	// our domain type nests delivery metrics under Analysis. Decode flat, then
	// map. Filler counting is deterministic and runs server-side here so the
	// LLM doesn't need to count or moderate.
	var flat struct {
		Transcript     string `json:"transcript"`
		WordsPerMinute int    `json:"wordsPerMinute"`
	}
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return domain.TranscriptResult{}, fmt.Errorf("gemini decode transcript: %w", errors.Join(domain.ErrLLM, err))
	}
	return domain.TranscriptResult{
		Transcript: flat.Transcript,
		Analysis: domain.SpeechAnalysis{
			FillerCount:    CountFillers(flat.Transcript),
			WordsPerMinute: flat.WordsPerMinute,
			// LongPauseCount is set by the service from the client's VAD count.
		},
	}, nil
}

// JudgeFollowUp asks the model whether the candidate needs one more
// probing follow-up. Returns the follow-up question string, or empty string
// if the model judges the answer good enough. Output is constrained by
// followUpSchema.
//
// The hard cap on number of follow-ups is enforced by the service layer —
// this method always calls the LLM regardless of how many prior turns are
// supplied.
func (c *Client) JudgeFollowUp(ctx context.Context, in domain.FollowUpSeed) (string, error) {
	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.3),
		TopP:             genai.Ptr[float32](0.9),
		ResponseMIMEType: "application/json",
		ResponseSchema:   followUpSchema,
	}

	resp, err := c.sdk.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(buildFollowUpPrompt(in)),
		cfg,
	)
	if err != nil {
		return "", fmt.Errorf("gemini judge follow-up: %w", errors.Join(domain.ErrLLM, err))
	}

	raw := resp.Text()
	if raw == "" {
		return "", fmt.Errorf("gemini empty response: %w", domain.ErrLLM)
	}

	var out struct {
		FollowUp string `json:"followUp"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", fmt.Errorf("gemini decode follow-up: %w", errors.Join(domain.ErrLLM, err))
	}
	return out.FollowUp, nil
}

// EvaluateAnswer asks the model to rate the candidate's answer 1..10 with
// feedback. Output is constrained by answerEvalSchema.
func (c *Client) EvaluateAnswer(ctx context.Context, in domain.AnswerSeed) (domain.Evaluation, error) {
	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.4),
		TopP:             genai.Ptr[float32](0.9),
		ResponseMIMEType: "application/json",
		ResponseSchema:   answerEvalSchema,
	}

	resp, err := c.sdk.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(buildAnswerEvalPrompt(in)),
		cfg,
	)
	if err != nil {
		return domain.Evaluation{}, fmt.Errorf("gemini evaluate answer: %w", errors.Join(domain.ErrLLM, err))
	}

	raw := resp.Text()
	if raw == "" {
		return domain.Evaluation{}, fmt.Errorf("gemini empty response: %w", domain.ErrLLM)
	}

	var out domain.Evaluation
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return domain.Evaluation{}, fmt.Errorf("gemini decode evaluation: %w", errors.Join(domain.ErrLLM, err))
	}
	if out.Rating < 1 || out.Rating > 10 {
		return domain.Evaluation{}, fmt.Errorf("gemini rating out of range %d: %w", out.Rating, domain.ErrLLM)
	}
	return out, nil
}
