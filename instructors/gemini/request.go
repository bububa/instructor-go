package gemini

import (
	gemini "github.com/google/generative-ai-go/genai"
)

type Request struct {
	Model   string
	System  *gemini.Content
	Parts   []gemini.Part
	History []*gemini.Content
}
