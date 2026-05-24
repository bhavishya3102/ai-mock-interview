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

func buildCoachReportPrompt(in domain.CoachReportSeed) string {
	var answers strings.Builder
	for _, a := range in.Answers {
		fmt.Fprintf(&answers,
			"Q%d: %s\nCandidate's answer: %s\nPer-question rating: %d/10\nPer-question feedback: %s\nSpeech metrics: %d fillers, %d words/min, %d long pauses\n\n",
			a.QuestionIndex+1,
			a.QuestionText,
			a.UserAnswer,
			a.Rating,
			a.Feedback,
			a.FillerCount,
			a.WordsPerMinute,
			a.LongPauseCount,
		)
	}

	// Memory enrichment is purely additive: when the user has no prior
	// interviews and no recurring weak hits the historicalSection and
	// progressTrackingDirective stay empty and the prompt is byte-identical
	// to the pre-memory version — no regression risk for first-time users.
	historicalSection, progressTrackingDirective := buildCoachHistorySections(in)

	return fmt.Sprintf(`You are an experienced interview coach writing a personalised post-interview review for a candidate. You are NOT re-grading the answers — the per-question ratings and feedback are already final and given to you as evidence. Your job is to synthesise them into a single narrative report the candidate can act on.

Candidate context:
- Role applied for: %s
- Years of experience: %d

Per-question evidence (already evaluated):
%s%sWrite the report in markdown using EXACTLY these section headings, in this order:

## Overall Performance
One short paragraph naming the candidate's overall level for this role and the 1–2 most important takeaways.

### Strengths
Bulleted list (2–5 items). Cite specific evidence from the answers above (quote or paraphrase a phrase, name the topic). Generic praise is not allowed.

### Areas to Improve
Bulleted list (2–5 items). Cite specific evidence. For each item, say what was missing or wrong and what "good" would look like.
%s
### Recommended Next Steps
Numbered list (3 items). Each item is one concrete action the candidate can take this week — a topic to study, a pattern to practice, a resource type to read (no specific URLs). Tie each step to a weakness named above.

### Speaking & Delivery
One short paragraph. Use the speech metrics (fillers, words/minute, long pauses) to comment on pace and confidence. If metrics are all zero, say "no audio metrics captured for this interview" and skip pace commentary.

Tone: direct, supportive, second person ("you"). No preamble before the first heading, no closing pep talk, no emojis.

Output the markdown only — no JSON, no code fences around the whole report.`,
		in.JobPosition,
		in.YearsExperience,
		answers.String(),
		historicalSection,
		progressTrackingDirective,
	)
}

// buildCoachHistorySections renders the optional memory-enrichment blocks
// of the coach report prompt. Returns two strings:
//
//   - historicalSection: a "HISTORICAL CONTEXT" preamble injected before
//     the report structure instructions. Empty when there is nothing
//     historical to report.
//   - progressTrackingDirective: the `### Progress Tracking` instruction
//     block that slots between Areas to Improve and Recommended Next
//     Steps. Empty when there is no history (so the report keeps the
//     original five-section structure).
//
// Both strings are empty when the seed has no past interviews AND no
// recurring weak hits — i.e. for a first-time user. That keeps the
// pre-memory prompt byte-identical for new users.
func buildCoachHistorySections(in domain.CoachReportSeed) (string, string) {
	hasHistory := len(in.PastInterviews) > 1 // current interview counts as 1
	hasWeak := len(in.RecurringWeak) > 0
	if !hasHistory && !hasWeak {
		return "", ""
	}

	var b strings.Builder
	b.WriteString("\nHISTORICAL CONTEXT (use this in the Progress Tracking section below):\n")

	if hasHistory {
		b.WriteString("Past interviews (newest first):\n")
		for _, h := range in.PastInterviews {
			fmt.Fprintf(&b, "- %s  %s  avg %.1f/10  (%d answered)\n",
				h.CreatedAt.Format("2006-01-02"),
				h.JobPosition,
				h.AvgRating,
				h.AnsweredCount,
			)
		}
		b.WriteString("\n")
	}

	if hasWeak {
		b.WriteString("Recurring weak answers from prior interviews on topics similar to this role:\n")
		for _, w := range in.RecurringWeak {
			fmt.Fprintf(&b, "- %q — got %d/10, feedback: %q\n",
				w.QuestionText, w.Rating, w.Feedback,
			)
		}
		b.WriteString("\n")
	}

	directive := `
### Progress Tracking
One short paragraph plus optional bullets. Cite specific numerical movement from the past interviews above (e.g. "5.8 → 6.4 → 7.1 on Frontend"), and call out recurring weak topics — if the candidate finally answered one of them well this time, name it as a win. Skip this section entirely if the historical evidence is too thin to draw a trend.
`

	return b.String(), directive
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
