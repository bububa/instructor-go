package instructors

import (
	"github.com/bububa/instructor-go/instructors/anthropic"
	"github.com/bububa/instructor-go/instructors/cohere"
	"github.com/bububa/instructor-go/instructors/gemini"
	"github.com/bububa/instructor-go/instructors/openai"
)

var (
	FromOpenAI    = openai.New
	FromAnthropic = anthropic.New
	FromCohere    = cohere.New
	FromGemini    = gemini.New
)
