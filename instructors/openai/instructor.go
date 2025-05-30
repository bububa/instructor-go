package openai

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*openai.Client
	instructor.Options
	tools map[string]*mcp.Tool
}

func (i *Instructor) SetClient(clt *openai.Client) {
	i.Client = clt
}

var (
	_ instructor.ChatInstructor[openai.ChatCompletionNewParams, openai.ChatCompletion]         = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[openai.ChatCompletionNewParams, openai.ChatCompletion] = (*Instructor)(nil)
	_ instructor.StreamInstructor[openai.ChatCompletionNewParams, openai.ChatCompletion]       = (*Instructor)(nil)
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
