package llm

import (
	"strings"
	"testing"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildQuestionPrompt_OmitsResumeSectionWhenAbsent(t *testing.T) {
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build Go services",
		YearsExperience: 5,
	})
	require.NotContains(t, p, "Candidate's resume:")
	require.NotContains(t, p, "MUST reference specific")
}

func TestBuildQuestionPrompt_IncludesResumeAndConstraintWhenPresent(t *testing.T) {
	resume := "Led a Postgres migration at Acme Corp in 2024."
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build Go services",
		YearsExperience: 5,
		ResumeText:      resume,
	})
	require.Contains(t, p, "Candidate's resume:")
	require.Contains(t, p, resume)
	require.Contains(t, p, "MUST reference specific")
}

func TestBuildQuestionPrompt_BlankResumeTreatedAsAbsent(t *testing.T) {
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build Go services",
		YearsExperience: 5,
		ResumeText:      "   \n\t  ",
	})
	require.NotContains(t, p, "Candidate's resume:")
	require.False(t, strings.Contains(p, "MUST reference specific"))
}

func TestBuildCoachReportPrompt_IncludesContextAndStructure(t *testing.T) {
	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition:     "Frontend Engineer",
		YearsExperience: 3,
		Answers: []domain.UserAnswer{
			{
				QuestionIndex: 0,
				QuestionText:  "Explain React hooks",
				UserAnswer:    "Hooks let you use state in function components",
				Rating:        7,
				Feedback:      "Good basic understanding, missing useEffect cleanup",
			},
			{
				QuestionIndex: 1,
				QuestionText:  "What is useMemo?",
				UserAnswer:    "It memoizes values",
				Rating:        5,
				Feedback:      "Too shallow; no discussion of dependencies",
			},
		},
	})

	// Role context must be present so the LLM tailors the report.
	require.Contains(t, p, "Frontend Engineer")
	require.Contains(t, p, "3")

	// Each answer's question, response and rating must be in the prompt so
	// the LLM has the full evidence base to narrate from.
	require.Contains(t, p, "Explain React hooks")
	require.Contains(t, p, "Hooks let you use state")
	require.Contains(t, p, "7")
	require.Contains(t, p, "useMemo")

	// The fixed section headings anchor the markdown output. The frontend
	// renders by these headings so they must appear in the request.
	require.Contains(t, p, "Overall Performance")
	require.Contains(t, p, "Strengths")
	require.Contains(t, p, "Areas to Improve")
	require.Contains(t, p, "Recommended Next Steps")
	require.Contains(t, p, "Speaking & Delivery")

	// Explicit format guard so the model returns markdown, not JSON.
	require.Contains(t, p, "markdown")
}

func TestBuildCoachReportPrompt_IncludesSpeechMetricsWhenPresent(t *testing.T) {
	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition:     "Backend Engineer",
		YearsExperience: 4,
		Answers: []domain.UserAnswer{
			{
				QuestionIndex:  0,
				QuestionText:   "Describe Postgres MVCC",
				UserAnswer:     "Multi-version concurrency control",
				Rating:         6,
				Feedback:       "Surface-level, no mention of vacuum",
				FillerCount:    12,
				WordsPerMinute: 165,
				LongPauseCount: 3,
			},
		},
	})

	// Speech metrics must reach the LLM so the Speaking & Delivery section
	// is grounded in real data, not hallucinated.
	require.Contains(t, p, "12")
	require.Contains(t, p, "165")
	require.Contains(t, p, "3")
}

func TestBuildCoachReportPrompt_HandlesSingleAnswer(t *testing.T) {
	// Sanity: a single-answer interview must still produce a valid prompt
	// without panics or empty placeholders.
	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition:     "Data Analyst",
		YearsExperience: 1,
		Answers: []domain.UserAnswer{
			{QuestionIndex: 0, QuestionText: "What is a window function?", UserAnswer: "It runs per row", Rating: 4, Feedback: "Vague"},
		},
	})
	require.Contains(t, p, "Data Analyst")
	require.Contains(t, p, "window function")
	require.NotContains(t, p, "%!s") // no fmt formatting leaks
	require.NotContains(t, p, "%!d")
}
