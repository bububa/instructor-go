package anthropic

import (
	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
)

type Instructor struct {
	*anthropic.Client
	instructor.Options
	memory *instructor.Memory[anthropic.Message]
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
	i.memory = instructor.NewMemory[anthropic.Message](10)
	return i
}

func (i Instructor) Memory() *instructor.Memory[anthropic.Message] {
	return i.memory
}
