package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountFillers(t *testing.T) {
	tests := []struct {
		name       string
		transcript string
		want       int
	}{
		{"empty", "", 0},
		{"no fillers", "I built the service end to end.", 0},
		{"single um", "um I think so.", 1}, // only "um" — trailing "so" is adverbial, not an opener
		{"elongated um/uh", "uhhh okay ummm right", 2},
		{"phrasal fillers", "you know what I mean? like, basically yes.", 4},
		{
			"mixed",
			"Um, so I uh built the API. Basically, we sort of had to refactor.",
			4, // um, uh, basically, sort of — "so" only counts at sentence start
		},
		{"so as conjunction not counted", "I tested it so it works.", 0},
		{"so at sentence start counted", "It failed. So I retried.", 1},
		{"like in middle counts", "I like apples and oranges.", 1},
		{"case insensitive", "UM well LIKE basically", 3},
		{"no false positive on substrings", "humbly, alike, history", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, CountFillers(tt.transcript))
		})
	}
}
