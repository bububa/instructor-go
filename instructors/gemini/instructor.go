package gemini

import (
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*gemini.Client
	instructor.Options
}

func (i *Instructor) SetClient(clt *gemini.Client) {
	i.Client = clt
}

func (i *Instructor) SetMemory(m *instructor.Memory) {
	instructor.WithMemory(m)(&i.Options)
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
	if i.Memory() == nil {
		i.SetMemory(instructor.NewMemory(-1))
	}
	instructor.WithProvider(instructor.ProviderGemini)(&i.Options)
	return i
}
