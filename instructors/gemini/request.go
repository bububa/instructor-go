package gemini

import (
	gemini "google.golang.org/genai"
)

type Request struct {
	Model   string
	System  *gemini.Content
	Parts   []*gemini.Part
	History []*gemini.Content
}
