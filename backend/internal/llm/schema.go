package llm

import "google.golang.org/genai"

// questionGenSchema constrains the LLM output for question generation. The
// model is forced to emit exactly QuestionCount items via MinItems/MaxItems.
var questionGenSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"questions": {
			Type:     genai.TypeArray,
			MinItems: genai.Ptr[int64](QuestionCount),
			MaxItems: genai.Ptr[int64](QuestionCount),
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"question": {Type: genai.TypeString, MinLength: genai.Ptr[int64](10)},
					"answer":   {Type: genai.TypeString, MinLength: genai.Ptr[int64](20)},
				},
				Required:         []string{"question", "answer"},
				PropertyOrdering: []string{"question", "answer"},
			},
		},
	},
	Required:         []string{"questions"},
	PropertyOrdering: []string{"questions"},
}

// followUpSchema constrains the LLM output for the follow-up judge. An
// empty followUp string means "good enough — no further probe needed".
var followUpSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"followUp": {Type: genai.TypeString},
		"reason":   {Type: genai.TypeString},
	},
	Required:         []string{"followUp", "reason"},
	PropertyOrdering: []string{"followUp", "reason"},
}

// transcribeSchema constrains TranscribeAudio so Gemini returns the verbatim
// transcript and the speaking-rate estimate. Filler and long-pause counts are
// no longer LLM-judged: fillers are counted programmatically from the
// transcript (see CountFillers), and pauses come from client-side VAD.
// wordsPerMinute is capped at 400 (auctioneer-speed) so a hallucinated huge
// value can't poison the UI.
var transcribeSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"transcript": {Type: genai.TypeString},
		"wordsPerMinute": {
			Type:    genai.TypeInteger,
			Minimum: genai.Ptr(0.0),
			Maximum: genai.Ptr(400.0),
		},
	},
	Required:         []string{"transcript", "wordsPerMinute"},
	PropertyOrdering: []string{"transcript", "wordsPerMinute"},
}

// resumeExtractSchema constrains ExtractResumeText: Gemini reads the uploaded
// PDF and returns its text content as a single plain-text field.
var resumeExtractSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"resumeText": {Type: genai.TypeString},
	},
	Required:         []string{"resumeText"},
	PropertyOrdering: []string{"resumeText"},
}

// answerEvalSchema constrains the LLM output for answer evaluation. The
// rating is bounded to 1..10 to match the user_answers.rating CHECK constraint.
var answerEvalSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"rating": {
			Type:    genai.TypeInteger,
			Minimum: genai.Ptr(1.0),
			Maximum: genai.Ptr(10.0),
		},
		"feedback": {
			Type:      genai.TypeString,
			MinLength: genai.Ptr[int64](20),
		},
	},
	Required:         []string{"rating", "feedback"},
	PropertyOrdering: []string{"rating", "feedback"},
}
