package cohere

import (
	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereClient "github.com/cohere-ai/cohere-go/v2/client"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*cohereClient.Client
	instructor.Options
	memory *instructor.Memory[cohere.Message]
}

var (
	_ instructor.ChatInstructor[cohere.ChatRequest, cohere.NonStreamedChatResponse, cohere.Message]               = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse, cohere.Message] = (*Instructor)(nil)
	_ instructor.StreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse, cohere.Message]       = (*Instructor)(nil)
)

func New(client *cohereClient.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	instructor.WithProvider(instructor.ProviderCohere)
	for _, opt := range opts {
		opt(&i.Options)
	}
	i.memory = instructor.NewMemory[cohere.Message](10)
	return i
}

func (i Instructor) Memory() *instructor.Memory[cohere.Message] {
	return i.memory
}
