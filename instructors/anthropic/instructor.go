package anthropic

import (
	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*anthropic.Client
	instructor.Options
}

var (
	instance                                                                                        = new(Instructor)
	_        instructor.ChatInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse]       = instance
	_        instructor.JSONStreamInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse] = instance
	_        instructor.StreamInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse]     = instance
)

func New(client *anthropic.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	for _, opt := range opts {
		opt(&i.Options)
	}
	instructor.WithProvider(instructor.ProviderAnthropic)
	return i
}
