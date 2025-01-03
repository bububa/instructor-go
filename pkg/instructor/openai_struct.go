package instructor

import (
	openai "github.com/sashabaranov/go-openai"
)

type InstructorOpenAI struct {
	*openai.Client

	provider   Provider
	mode       Mode
	maxRetries int
	validate   bool
	verbose    bool
}

var _ Instructor = &InstructorOpenAI{}

func FromOpenAI(client *openai.Client, opts ...Options) *InstructorOpenAI {
	options := mergeOptions(opts...)

	i := &InstructorOpenAI{
		Client: client,

		provider:   ProviderOpenAI,
		mode:       *options.Mode,
		maxRetries: *options.MaxRetries,
		validate:   *options.validate,
		verbose:    *options.verbose,
	}
	return i
}

func (i *InstructorOpenAI) Provider() Provider {
	return i.provider
}

func (i *InstructorOpenAI) Mode() Mode {
	return i.mode
}

func (i *InstructorOpenAI) MaxRetries() int {
	return i.maxRetries
}

func (i *InstructorOpenAI) Validate() bool {
	return i.validate
}

func (i *InstructorOpenAI) Verbose() bool {
	return i.verbose
}
