package llm

import (
	"fmt"
	"strings"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
)

// MaxFollowUpsPerQuestion caps how many follow-ups the LLM may ask before
// the service forces a move-on. Surfaced here so the prompt can reference
// the same bound the service enforces.
const MaxFollowUpsPerQuestion = 2

// QuestionCount is the fixed number of questions generated per interview.
// Surfaced as a constant because the schema's MinItems/MaxItems uses the
// same number; changing one without the other corrupts the contract.
const QuestionCount = 5

func buildQuestionPrompt(in domain.InterviewSeed) string {
	resumeSection := ""
	resumeConstraint := ""
	if strings.TrimSpace(in.ResumeText) != "" {
		resumeSection = fmt.Sprintf(`
Candidate's resume:
%s
`, in.ResumeText)
		resumeConstraint = "\n- At least 2 questions MUST reference specific, concrete details from the candidate's resume (a named project, employer, technology, or claim) — e.g. \"You wrote that you led a database migration at X — walk me through it.\" Quote or paraphrase the resume detail so the candidate knows exactly what you mean. Do not invent details that are not in the resume."
	}

	return fmt.Sprintf(`You are an experienced technical interviewer.

Generate exactly %d distinct interview questions and their model answers for the following candidate context. Questions must be open-ended (no yes/no questions), ordered from easier to harder, and tightly scoped to the role and experience level.

Job position: %s
Years of experience: %d
Job description:
%s
%s
Constraints:
- Each question must be answerable verbally in 2–4 minutes by a competent candidate.
- The "answer" field must be a strong reference answer the interviewer would consider excellent.
- Avoid questions that depend on company-internal context the candidate cannot know.%s
- Do NOT include numbering, prefaces, or any text outside the structured JSON output.`,
		QuestionCount,
		in.JobPosition,
		in.YearsExperience,
		in.JobDescription,
		resumeSection,
		resumeConstraint,
	)
}

func buildFollowUpPrompt(in domain.FollowUpSeed) string {
	var prior strings.Builder
	if len(in.PriorTurns) == 0 {
		prior.WriteString("(none yet — this is the first follow-up decision)")
	} else {
		for i, t := range in.PriorTurns {
			fmt.Fprintf(&prior, "Follow-up %d question: %s\nFollow-up %d answer: %s\n", i+1, t.Question, i+1, t.Answer)
		}
	}

	return fmt.Sprintf(`You are an experienced interviewer running a live interview for the role of %s. You have just heard the candidate's answer to a main question, plus any follow-ups already exchanged. Decide whether to ask one more probing follow-up or to move on.

Main question:
%s

Candidate's main answer:
%s

Follow-ups so far:
%s

Decide:
- If the candidate's answer (including any follow-ups) has clear depth, addresses the question, and there is no obvious gap or hand-wavy claim left, return an EMPTY string for "followUp" and a brief "reason".
- Otherwise, return ONE specific follow-up question that probes the most important gap, vague claim, or missing detail. Do not ask multiple questions in one. Keep it short and direct, the way a real interviewer would.

Constraints:
- "followUp" MUST be empty when the answer is good enough. Do not invent low-value probes.
- Never repeat a follow-up that has already been asked.
- "reason" is one short sentence explaining why you chose to probe or move on.
- Output ONLY the structured JSON specified by the response schema — no preamble, no markdown.`,
		in.JobPosition,
		in.MainQuestion,
		in.MainAnswer,
		prior.String(),
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
