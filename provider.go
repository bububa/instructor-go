package instructor

import (
	cohere "github.com/cohere-ai/cohere-go/v2/client"
	"github.com/liushuangls/go-anthropic/v2"
	"github.com/sashabaranov/go-openai"
)

type Provider = string

const (
	ProviderOpenAI    Provider = "OpenAI"
	ProviderAnthropic Provider = "Anthropic"
	ProviderCohere    Provider = "Cohere"
)

type ProviderClient interface {
	openai.Client | cohere.Client | anthropic.Client
}

func ProviderFromClient(clt any) Provider {
	switch clt.(type) {
	case *openai.Client:
		return ProviderOpenAI
	case *anthropic.Client:
		return ProviderAnthropic
	case *cohere.Client:
		return ProviderCohere
	}
	return ProviderOpenAI
}
