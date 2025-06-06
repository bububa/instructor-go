package cohere

import (
	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereClient "github.com/cohere-ai/cohere-go/v2/client"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*cohereClient.Client
	instructor.Options
}

func (i *Instructor) SetMemory(m *instructor.Memory) {
	instructor.WithMemory(m)(&i.Options)
}

var (
	_ instructor.ChatInstructor[cohere.ChatRequest, cohere.NonStreamedChatResponse]               = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse] = (*Instructor)(nil)
	_ instructor.StreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse]       = (*Instructor)(nil)
)

func New(client *cohereClient.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	instructor.WithProvider(instructor.ProviderCohere)
	for _, opt := range opts {
		opt(&i.Options)
	}
	return i
}
