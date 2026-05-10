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
