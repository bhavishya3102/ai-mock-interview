package llm

import (
	"regexp"
	"strings"
)

// fillerPatterns matches obvious filler words and discourse-marker tics
// against a lowercased transcript. Each regex match counts as one filler.
//
// Why programmatic and not LLM-judged: Gemini was inconsistent and
// under-counted (the prompt previously asked it to "be conservative").
// Counting against the verbatim transcript is deterministic and reviewable.
var fillerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bu+m+\b`),
	regexp.MustCompile(`\bu+h+\b`),
	regexp.MustCompile(`\be+r+\b`),
	regexp.MustCompile(`\ba+h+\b`),
	regexp.MustCompile(`\blike\b`),
	regexp.MustCompile(`\byou know\b`),
	regexp.MustCompile(`\bsort of\b`),
	regexp.MustCompile(`\bkind of\b`),
	regexp.MustCompile(`\bbasically\b`),
	regexp.MustCompile(`\bliterally\b`),
	regexp.MustCompile(`\bi mean\b`),
	// "so" only counts as a filler when it opens an utterance/sentence.
	// Counting every "so" would flag legitimate conjunctions ("I did X so Y happened").
	regexp.MustCompile(`(?:^|[.!?]\s+)so\b`),
}

// CountFillers returns the number of filler-word occurrences in transcript.
// Case-insensitive; punctuation is ignored at word boundaries.
func CountFillers(transcript string) int {
	if transcript == "" {
		return 0
	}
	s := strings.ToLower(transcript)
	n := 0
	for _, p := range fillerPatterns {
		n += len(p.FindAllStringIndex(s, -1))
	}
	return n
}
