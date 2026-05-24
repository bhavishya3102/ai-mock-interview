package domain

import "time"

// Evaluation is the LLM's rating + feedback for a user's answer to a question.
type Evaluation struct {
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}

// UserAnswer is the persisted aggregate for a single answered question.
// FillerCount / WordsPerMinute / LongPauseCount are speech-delivery metrics
// aggregated across all recordings for the question (main + follow-ups).
// Zero values are valid — e.g. WordsPerMinute=0 when audio was too short to
// estimate, or when the answer was typed.
type UserAnswer struct {
	MockID         string    `json:"-"`
	ClerkUserID    string    `json:"-"`
	QuestionIndex  int       `json:"questionIndex"`
	QuestionText   string    `json:"-"`
	CorrectAnswer  string    `json:"-"`
	UserAnswer     string    `json:"-"`
	Rating         int       `json:"rating"`
	Feedback       string    `json:"feedback"`
	FillerCount    int       `json:"fillerCount"`
	WordsPerMinute int       `json:"wordsPerMinute"`
	LongPauseCount int       `json:"longPauseCount"`
	CreatedAt      time.Time `json:"createdAt"`
}

// AnswerSeed is the input passed to the LLM to evaluate a user's answer.
type AnswerSeed struct {
	JobPosition   string
	QuestionText  string
	CorrectAnswer string
	UserAnswer    string
}

// FollowUpTurn is a single Q→A pair from a follow-up exchange (excluding the
// main question, which lives in FollowUpSeed.MainQuestion).
type FollowUpTurn struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// FollowUpSeed is the input passed to the LLM to decide whether the
// candidate needs a follow-up probe. PriorTurns is empty for the first
// follow-up decision after the main answer.
type FollowUpSeed struct {
	JobPosition  string
	MainQuestion string
	MainAnswer   string
	PriorTurns   []FollowUpTurn
}

// SpeechAnalysis is the delivery metrics Gemini extracts from a single
// recorded answer in the same call that produces the transcript. Zero
// values are valid — e.g. WordsPerMinute=0 when the clip is too short.
type SpeechAnalysis struct {
	FillerCount    int `json:"fillerCount"`
	WordsPerMinute int `json:"wordsPerMinute"`
	LongPauseCount int `json:"longPauseCount"`
}

// TranscriptResult bundles the transcript and the delivery analysis returned
// by a single TranscribeAudio call.
type TranscriptResult struct {
	Transcript string         `json:"transcript"`
	Analysis   SpeechAnalysis `json:"analysis"`
}

// PastInterviewSummary is one row of a user's interview history. Used by
// the coach report to surface trend lines ("3rd Frontend interview, avg
// rising 5.8 → 6.4 → 7.1"). Computed via JOIN — no separate aggregate
// table is maintained.
type PastInterviewSummary struct {
	MockID        string    `json:"mockId"`
	JobPosition   string    `json:"jobPosition"`
	AvgRating     float64   `json:"avgRating"`
	AnsweredCount int       `json:"answeredCount"`
	CreatedAt     time.Time `json:"createdAt"`
}

// WeakAnswerHit is a low-scoring past answer that is semantically close
// to the current interview's role. The coach report uses these to point
// out recurring weaknesses without making the user re-explain context.
type WeakAnswerHit struct {
	MockID       string    `json:"mockId"`
	QuestionText string    `json:"questionText"`
	UserAnswer   string    `json:"userAnswer"`
	Rating       int       `json:"rating"`
	Feedback     string    `json:"feedback"`
	CreatedAt    time.Time `json:"createdAt"`
}
