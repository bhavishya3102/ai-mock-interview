package domain

import "time"

// CoachReport is the persisted narrative coaching report generated for a
// finished interview. The Content is markdown rendered directly by the
// frontend. TokensUsed and Model are captured for usage analytics and cost
// attribution — neither is shown to the candidate today.
type CoachReport struct {
	MockID     string    `json:"-"`
	Content    string    `json:"content"`
	TokensUsed int       `json:"tokensUsed"`
	Model      string    `json:"model"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CoachReportSeed is the input the LLM uses to draft the report. Answers
// are the already-evaluated per-question rows (rating, feedback, speech
// metrics) so the LLM does not re-grade — it only narrates.
type CoachReportSeed struct {
	JobPosition     string
	YearsExperience int
	Answers         []UserAnswer
}
