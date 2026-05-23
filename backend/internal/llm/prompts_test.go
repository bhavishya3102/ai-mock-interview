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
