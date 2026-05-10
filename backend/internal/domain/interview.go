package domain

import "time"

// GeneratedQA is the question/answer pair produced by the LLM and persisted in
// mock_interviews.questions (JSONB).
type GeneratedQA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// InterviewSeed is the input passed to the LLM to produce questions. Kept
// separate from CreateInterviewInput so the LLM layer doesn't depend on the
// HTTP DTO.
type InterviewSeed struct {
	JobPosition     string
	JobDescription  string
	YearsExperience int
}

// MockInterview is the persisted aggregate.
type MockInterview struct {
	MockID          string        `json:"mockId"`
	ClerkUserID     string        `json:"-"`
	JobPosition     string        `json:"jobPosition"`
	JobDescription  string        `json:"jobDescription"`
	YearsExperience int           `json:"yearsExperience"`
	Questions       []GeneratedQA `json:"questions"`
	CreatedAt       time.Time     `json:"createdAt"`
}

// InterviewSummary is the list-view shape: excludes Questions to keep
// payloads small.
type InterviewSummary struct {
	MockID          string    `json:"mockId"`
	JobPosition     string    `json:"jobPosition"`
	JobDescription  string    `json:"jobDescription"`
	YearsExperience int       `json:"yearsExperience"`
	CreatedAt       time.Time `json:"createdAt"`
}
