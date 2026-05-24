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

// Model returns the configured Gemini model identifier (e.g. "gemini-2.5-flash").
// Stored alongside generated content so reports remain traceable to the
// model version that produced them.
func (c *Client) Model() string { return c.model }

// embeddingModel is the Gemini text-embedding model used by Embed below.
// Pinned here rather than in config because the database column dimension
// (vector(768)) is coupled to this exact model; changing the model
// without a migration would corrupt the index.
const embeddingModel = "models/text-embedding-004"

// Embed turns text into a 768-dimensional vector via Gemini's
// text-embedding-004 model. Empty input returns an empty slice and no
// error — the caller can decide whether that is a problem (usually it
// means there is nothing meaningful to index).
//
// Errors are wrapped with ErrLLM so the caller can errors.Is() the
// failure class without caring about the underlying SDK error type.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, nil
	}
	resp, err := c.sdk.Models.EmbedContent(
		ctx,
		embeddingModel,
		genai.Text(text),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", errors.Join(domain.ErrLLM, err))
	}
	if len(resp.Embeddings) == 0 || resp.Embeddings[0] == nil {
		return nil, fmt.Errorf("gemini embed: empty response: %w", domain.ErrLLM)
	}
	values := resp.Embeddings[0].Values
	if len(values) == 0 {
		return nil, fmt.Errorf("gemini embed: empty vector: %w", domain.ErrLLM)
	}
	return values, nil
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

// ExtractResumeText sends a resume PDF to Gemini and returns its text content
// as clean plain text. Run once at upload time; the stored text is reused for
// every interview the candidate creates.
//
// mimeType is expected to be "application/pdf". Empty input is rejected before
// the API round-trip to save quota.
func (c *Client) ExtractResumeText(ctx context.Context, pdf []byte, mimeType string) (string, error) {
	if len(pdf) == 0 {
		return "", fmt.Errorf("gemini extract resume: empty file: %w", domain.ErrValidation)
	}
	if mimeType == "" {
		return "", fmt.Errorf("gemini extract resume: empty mime type: %w", domain.ErrValidation)
	}

	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.0),
		ResponseMIMEType: "application/json",
		ResponseSchema:   resumeExtractSchema,
	}

	prompt := "Extract the full text content of this resume/CV PDF as clean, " +
		"readable plain text. Preserve section headings, role titles, company " +
		"names, dates, and bullet points. Do NOT summarize, reword, or omit " +
		"anything. No markdown, no commentary. Return JSON with field " +
		"\"resumeText\" containing the extracted text (empty string if the file " +
		"has no readable resume content)."

	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
			genai.NewPartFromBytes(pdf, mimeType),
		}, genai.RoleUser),
	}

	resp, err := c.sdk.Models.GenerateContent(ctx, c.model, contents, cfg)
	if err != nil {
		return "", fmt.Errorf("gemini extract resume: %w", errors.Join(domain.ErrLLM, err))
	}

	raw := resp.Text()
	if raw == "" {
		return "", fmt.Errorf("gemini empty response: %w", domain.ErrLLM)
	}

	var out struct {
		ResumeText string `json:"resumeText"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", fmt.Errorf("gemini decode resume: %w", errors.Join(domain.ErrLLM, err))
	}
	return out.ResumeText, nil
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

// GenerateCoachReportStream is the streaming variant of GenerateCoachReport.
// Each model-produced chunk is passed to onChunk as it arrives; the total
// token count from the final UsageMetadata is returned once the stream
// closes cleanly.
//
// If onChunk returns a non-nil error, streaming aborts immediately with
// that error wrapped — the caller is the typical "client disconnected,
// stop spending tokens" path. The returned token count in that case is
// 0 and the error is NOT tagged ErrLLM (it is a transport failure).
//
// Empty chunks (the SDK occasionally emits keepalive responses with no
// text but with UsageMetadata) are not forwarded to onChunk.
func (c *Client) GenerateCoachReportStream(
	ctx context.Context,
	in domain.CoachReportSeed,
	onChunk func(text string) error,
) (int, error) {
	cfg := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		TopP:        genai.Ptr[float32](0.95),
	}

	var tokens int
	for resp, iterErr := range c.sdk.Models.GenerateContentStream(
		ctx,
		c.model,
		genai.Text(buildCoachReportPrompt(in)),
		cfg,
	) {
		if iterErr != nil {
			return 0, fmt.Errorf("gemini stream coach report: %w", errors.Join(domain.ErrLLM, iterErr))
		}
		if text := resp.Text(); text != "" {
			if err := onChunk(text); err != nil {
				return 0, fmt.Errorf("gemini stream coach report: chunk callback: %w", err)
			}
		}
		if resp.UsageMetadata != nil {
			tokens = int(resp.UsageMetadata.TotalTokenCount)
		}
	}
	return tokens, nil
}

// GenerateCoachReport asks the model for a free-form markdown coaching
// report aggregating the per-question evaluations. Unlike the other LLM
// calls there is no ResponseSchema — the report is consumed as markdown
// by the frontend renderer.
//
// Returns the markdown content, the total tokens used (prompt + completion;
// 0 if the SDK omitted UsageMetadata), and a wrapped error tagged with
// ErrLLM on any failure.
func (c *Client) GenerateCoachReport(ctx context.Context, in domain.CoachReportSeed) (string, int, error) {
	cfg := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.7),
		TopP:        genai.Ptr[float32](0.95),
	}

	resp, err := c.sdk.Models.GenerateContent(
		ctx,
		c.model,
		genai.Text(buildCoachReportPrompt(in)),
		cfg,
	)
	if err != nil {
		return "", 0, fmt.Errorf("gemini generate coach report: %w", errors.Join(domain.ErrLLM, err))
	}

	content := resp.Text()
	if content == "" {
		return "", 0, fmt.Errorf("gemini empty coach report: %w", domain.ErrLLM)
	}

	tokens := 0
	if resp.UsageMetadata != nil {
		tokens = int(resp.UsageMetadata.TotalTokenCount)
	}
	return content, tokens, nil
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
