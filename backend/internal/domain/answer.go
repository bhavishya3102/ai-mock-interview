package domain

import "time"

// Evaluation is the LLM's rating + feedback for a user's answer to a question.
type Evaluation struct {
	Rating   int    `json:"rating"`
	Feedback string `json:"feedback"`
}

// UserAnswer is the persisted aggregate for a single answered question.
type UserAnswer struct {
	MockID        string    `json:"-"`
	ClerkUserID   string    `json:"-"`
	QuestionIndex int       `json:"questionIndex"`
	QuestionText  string    `json:"-"`
	CorrectAnswer string    `json:"-"`
	UserAnswer    string    `json:"-"`
	Rating        int       `json:"rating"`
	Feedback      string    `json:"feedback"`
	CreatedAt     time.Time `json:"createdAt"`
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
