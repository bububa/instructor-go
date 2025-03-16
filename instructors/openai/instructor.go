package openai

import (
	openai "github.com/sashabaranov/go-openai"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*openai.Client
	instructor.Options
}

func (i *Instructor) SetClient(clt *openai.Client) {
	i.Client = clt
}

var (
	_ instructor.ChatInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse]         = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse] = (*Instructor)(nil)
	_ instructor.StreamInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse]       = (*Instructor)(nil)
)

func New(client *openai.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	instructor.WithProvider(instructor.ProviderOpenAI)
	for _, opt := range opts {
		opt(&i.Options)
	}
	return i
}
