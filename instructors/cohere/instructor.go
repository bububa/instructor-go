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

var (
	instance                                                                                           = new(Instructor)
	_        instructor.ChatInstructor[cohere.ChatRequest, cohere.NonStreamedChatResponse]             = instance
	_        instructor.JSONStreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse] = instance
	_        instructor.StreamInstructor[cohere.ChatStreamRequest, cohere.NonStreamedChatResponse]     = instance
)

func New(client *cohereClient.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	for _, opt := range opts {
		opt(&i.Options)
	}
	instructor.WithProvider(instructor.ProviderCohere)
	return i
}
