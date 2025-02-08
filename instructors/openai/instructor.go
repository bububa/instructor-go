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
	instance                                                                                              = new(Instructor)
	_        instructor.ChatInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse]       = instance
	_        instructor.JSONStreamInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse] = instance
	_        instructor.StreamInstructor[openai.ChatCompletionRequest, openai.ChatCompletionResponse]     = instance
)

func New(client *openai.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	for _, opt := range opts {
		opt(&i.Options)
	}
	instructor.WithProvider(instructor.ProviderOpenAI)
	return i
}
