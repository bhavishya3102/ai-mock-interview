package llm

import (
	"fmt"

	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/domain"
)

// QuestionCount is the fixed number of questions generated per interview.
// Surfaced as a constant because the schema's MinItems/MaxItems uses the
// same number; changing one without the other corrupts the contract.
const QuestionCount = 5

func buildQuestionPrompt(in domain.InterviewSeed) string {
	return fmt.Sprintf(`You are an experienced technical interviewer.

Generate exactly %d distinct interview questions and their model answers for the following candidate context. Questions must be open-ended (no yes/no questions), ordered from easier to harder, and tightly scoped to the role and experience level.

Job position: %s
Years of experience: %d
Job description:
%s

Constraints:
- Each question must be answerable verbally in 2–4 minutes by a competent candidate.
- The "answer" field must be a strong reference answer the interviewer would consider excellent.
- Avoid questions that depend on company-internal context the candidate cannot know.
- Do NOT include numbering, prefaces, or any text outside the structured JSON output.`,
		QuestionCount,
		in.JobPosition,
		in.YearsExperience,
		in.JobDescription,
	)
}

func buildAnswerEvalPrompt(in domain.AnswerSeed) string {
	return fmt.Sprintf(`You are evaluating a candidate's answer in a technical interview for the role of %s.

Question:
%s

Reference answer (what a strong candidate would say):
%s

Candidate's answer:
%s

Rate the candidate's answer on a 1–10 scale where 1 is "completely wrong or non-answer" and 10 is "exceptional, exceeds the reference". Provide concise, specific, actionable feedback (at least 20 characters) addressing factual accuracy, depth, structure, and any missed points. Address the candidate in second person ("you").

Output ONLY the structured JSON specified by the response schema — no preamble, no markdown.`,
		in.JobPosition,
		in.QuestionText,
		in.CorrectAnswer,
		in.UserAnswer,
	)
}
