package gemini

import (
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*gemini.Client
	instructor.Options
	memory *instructor.Memory[*gemini.Content]
}

func (i *Instructor) SetClient(clt *gemini.Client) {
	i.Client = clt
}

var (
	_ instructor.ChatInstructor[Request, gemini.GenerateContentResponse, *gemini.Content]         = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[Request, gemini.GenerateContentResponse, *gemini.Content] = (*Instructor)(nil)
	_ instructor.StreamInstructor[Request, gemini.GenerateContentResponse, *gemini.Content]       = (*Instructor)(nil)
)

func New(client *gemini.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	for _, opt := range opts {
		opt(&i.Options)
	}
	instructor.WithProvider(instructor.ProviderGemini)
	i.memory = instructor.NewMemory[*gemini.Content](10)
	return i
}

func (i Instructor) Memory() *instructor.Memory[*gemini.Content] {
	return i.memory
}
