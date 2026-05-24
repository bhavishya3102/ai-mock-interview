package llm

import (
	"strings"
	"testing"
	"time"

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

func TestBuildQuestionPrompt_FirstTimeUser_NoAdaptiveDirective(t *testing.T) {
	// First-time user: no PastInterviews, no WeakAreas → directive omitted
	// and the prompt is byte-identical to the pre-adaptive output.
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build Go services",
		YearsExperience: 5,
	})
	require.NotContains(t, p, "ADAPTIVE DIFFICULTY DIRECTIVE")
	require.NotContains(t, p, "Calibrate the 5 questions")
}

func TestBuildQuestionPrompt_ReturningUser_InjectsAdaptiveDirective(t *testing.T) {
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Backend Engineer",
		JobDescription:  "Build Go microservices with Postgres",
		YearsExperience: 3,
		PastInterviews: []domain.PastInterviewSummary{
			{MockID: "mock-1", JobPosition: "Backend Engineer", AvgRating: 5.4, AnsweredCount: 5},
			{MockID: "mock-2", JobPosition: "Backend Engineer", AvgRating: 6.2, AnsweredCount: 5},
		},
		WeakAreas: []domain.WeakAnswerHit{
			{MockID: "mock-1", QuestionText: "Explain goroutine cleanup", Rating: 4, Feedback: "confused mount and unmount"},
		},
	})

	require.Contains(t, p, "ADAPTIVE DIFFICULTY DIRECTIVE")
	// Historical context: count + computed average reach the LLM so it can
	// calibrate (5.4 + 6.2)/2 = 5.8.
	require.Contains(t, p, "2 time(s)")
	require.Contains(t, p, "5.8/10")
	// Weak evidence cited verbatim — LLM is told NOT to repeat verbatim.
	require.Contains(t, p, "Explain goroutine cleanup")
	require.Contains(t, p, "confused mount and unmount")
	require.Contains(t, p, "Do NOT ask the verbatim past question")
	// Question-shape directive present so the LLM knows what to do with
	// the historical context it just received.
	require.Contains(t, p, "Q1: medium warm-up")
	require.Contains(t, p, "Q3: a topic on the JD/resume the candidate has NOT been asked")
}

func TestBuildQuestionPrompt_OnlyWeakAreas_NoHistory_StillInjects(t *testing.T) {
	// Edge: weak hits exist but ListUserInterviewSummaries failed (warn
	// dropped) so PastInterviews is empty. The directive should still
	// render — weak topics alone are signal enough to calibrate.
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Frontend Engineer",
		JobDescription:  "React + TypeScript",
		YearsExperience: 2,
		WeakAreas: []domain.WeakAnswerHit{
			{MockID: "mock-x", QuestionText: "What does useEffect cleanup do?", Rating: 3, Feedback: "fundamental misunderstanding"},
		},
	})

	require.Contains(t, p, "ADAPTIVE DIFFICULTY DIRECTIVE")
	require.NotContains(t, p, "historical avg") // no past-interview line when history empty
	require.Contains(t, p, "useEffect cleanup")
}

func TestBuildQuestionPrompt_PastHistoryOnly_NoWeakHits_StillInjects(t *testing.T) {
	// Symmetric to above: prior interviews exist but no rating was low
	// enough to qualify as a weak hit (or embedding failed). Still
	// inject the directive so the LLM at least respects the history.
	p := buildQuestionPrompt(domain.InterviewSeed{
		JobPosition:     "Data Scientist",
		JobDescription:  "ML pipelines on GCP",
		YearsExperience: 4,
		PastInterviews: []domain.PastInterviewSummary{
			{MockID: "mock-a", JobPosition: "Data Scientist", AvgRating: 8.4, AnsweredCount: 5},
		},
	})

	require.Contains(t, p, "ADAPTIVE DIFFICULTY DIRECTIVE")
	require.Contains(t, p, "1 time(s)")
	require.Contains(t, p, "8.4/10")
	// No weak-topic bullets when WeakAreas is empty.
	require.NotContains(t, p, "Prior recurring weak topics:")
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

func TestBuildCoachReportPrompt_FirstTimeUser_NoHistoricalSection(t *testing.T) {
	// First-time user (no enrichment): prompt must NOT include the
	// historical-context preamble or the Progress Tracking directive.
	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition:     "Backend Engineer",
		YearsExperience: 2,
		Answers:         []domain.UserAnswer{{QuestionIndex: 0, QuestionText: "Q", UserAnswer: "A", Rating: 7, Feedback: "ok"}},
		// PastInterviews + RecurringWeak both nil.
	})
	require.NotContains(t, p, "HISTORICAL CONTEXT")
	require.NotContains(t, p, "Progress Tracking")
}

func TestBuildCoachReportPrompt_ReturningUser_IncludesHistoryAndProgressDirective(t *testing.T) {
	// Past interviews list has TWO entries (one being the current
	// interview); historical-section logic triggers only when there are
	// more than just the current attempt.
	past := []domain.PastInterviewSummary{
		{
			MockID: "mock-current", JobPosition: "Backend Engineer",
			AvgRating: 7.1, AnsweredCount: 5,
			CreatedAt: mustParse("2026-05-10T10:00:00Z"),
		},
		{
			MockID: "mock-mar", JobPosition: "Backend Engineer",
			AvgRating: 5.8, AnsweredCount: 5,
			CreatedAt: mustParse("2026-03-12T10:00:00Z"),
		},
	}
	weak := []domain.WeakAnswerHit{
		{
			MockID: "mock-mar", QuestionText: "Explain MVCC",
			Rating: 4, Feedback: "no mention of vacuum",
		},
	}

	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition:     "Backend Engineer",
		YearsExperience: 4,
		Answers:         []domain.UserAnswer{{QuestionIndex: 0, QuestionText: "Q", UserAnswer: "A", Rating: 7, Feedback: "ok"}},
		PastInterviews:  past,
		RecurringWeak:   weak,
	})

	// Historical preamble + both past dates surfaced for the LLM to cite.
	require.Contains(t, p, "HISTORICAL CONTEXT")
	require.Contains(t, p, "2026-05-10")
	require.Contains(t, p, "2026-03-12")
	require.Contains(t, p, "5.8") // historical avg shows up
	// Recurring weak evidence — verbatim quote so the LLM can paraphrase
	// without inventing details.
	require.Contains(t, p, "Explain MVCC")
	require.Contains(t, p, "no mention of vacuum")
	// Progress Tracking section instruction is injected between Areas to
	// Improve and Recommended Next Steps.
	require.Contains(t, p, "### Progress Tracking")
	areasIdx := strings.Index(p, "### Areas to Improve")
	progressIdx := strings.Index(p, "### Progress Tracking")
	nextStepsIdx := strings.Index(p, "### Recommended Next Steps")
	require.Greater(t, progressIdx, areasIdx, "Progress Tracking should follow Areas to Improve")
	require.Greater(t, nextStepsIdx, progressIdx, "Recommended Next Steps should follow Progress Tracking")
}

func TestBuildCoachReportPrompt_OnlyCurrentInterviewInHistory_NoProgressSection(t *testing.T) {
	// Edge: history contains exactly one row (the current interview).
	// That's not enough context to draw a trend — skip enrichment.
	p := buildCoachReportPrompt(domain.CoachReportSeed{
		JobPosition: "Frontend Engineer",
		Answers:     []domain.UserAnswer{{QuestionIndex: 0, QuestionText: "Q", UserAnswer: "A", Rating: 7, Feedback: "ok"}},
		PastInterviews: []domain.PastInterviewSummary{
			{MockID: "current", JobPosition: "Frontend Engineer", AvgRating: 7.0, AnsweredCount: 1, CreatedAt: mustParse("2026-05-10T10:00:00Z")},
		},
		// RecurringWeak intentionally empty.
	})
	require.NotContains(t, p, "HISTORICAL CONTEXT")
	require.NotContains(t, p, "Progress Tracking")
}

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
