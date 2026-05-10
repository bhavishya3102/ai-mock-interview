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
