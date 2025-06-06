package anthropic

import (
	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*anthropic.Client
	instructor.Options
}

func (i *Instructor) SetMemory(m *instructor.Memory) {
	instructor.WithMemory(m)(&i.Options)
}

var (
	_ instructor.ChatInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse]         = (*Instructor)(nil)
	_ instructor.SchemaStreamInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse] = (*Instructor)(nil)
	_ instructor.StreamInstructor[anthropic.MessagesRequest, anthropic.MessagesResponse]       = (*Instructor)(nil)
)

func New(client *anthropic.Client, opts ...instructor.Option) *Instructor {
	i := &Instructor{
		Client: client,
	}
	instructor.WithProvider(instructor.ProviderAnthropic)
	for _, opt := range opts {
		opt(&i.Options)
	}
	return i
}
