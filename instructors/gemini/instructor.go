package gemini

import (
	gemini "github.com/google/generative-ai-go/genai"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*gemini.Client
	instructor.Options
}

func (i *Instructor) SetClient(clt *gemini.Client) {
	i.Client = clt
}

var (
	_ instructor.ChatInstructor[Request, gemini.GenerateContentResponse]         = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[Request, gemini.GenerateContentResponse] = (*Instructor)(nil)
	_ instructor.StreamInstructor[Request, gemini.GenerateContentResponse]       = (*Instructor)(nil)
)

func New(client *gemini.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	for _, opt := range opts {
		opt(&i.Options)
	}
	instructor.WithProvider(instructor.ProviderGemini)
	return i
}
