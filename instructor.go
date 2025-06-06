package instructor

import (
	"context"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

type Instructor interface {
	Provider() Provider
	Mode() Mode
	SetEncoder(enc Encoder)
	Encoder() Encoder
	SetStreamEncoder(enc StreamEncoder)
	StreamEncoder() StreamEncoder
	SchemaNamer() SchemaNamer
	MCPTools() []MCPTool
	Memory() *Memory
	MaxRetries() int
	Validate() bool
	Verbose() bool
}

type ChatInstructor[T any, RESP any] interface {
	Instructor
	Chat(
		ctx context.Context,
		request *T,
		responseType any,
		response *RESP,
	) error
	Handler(
		ctx context.Context,
		request *T,
		response *RESP,
	) (string, error)

	// Usage counting
	EmptyResponseWithUsageSum(*RESP, *UsageSum)
	EmptyResponseWithResponseUsage(*RESP, *RESP)
	SetUsageSumToResponse(response *RESP, usage *UsageSum)
	CountUsageFromResponse(response *RESP, usage *UsageSum)
}

type SchemaStreamInstructor[T any, RESP any] interface {
	Instructor
	SchemaStream(
		ctx context.Context,
		request *T,
		responseType any,
		response *RESP,
	) (<-chan any, <-chan StreamData, error)
	SchemaStreamHandler(
		ctx context.Context,
		request *T,
		response *RESP,
	) (<-chan StreamData, error)
}

type StreamInstructor[T any, RESP any] interface {
	Instructor
	Stream(
		ctx context.Context,
		request *T,
		responseType any,
		response *RESP,
	) (<-chan StreamData, error)
}

type StreamDataType int

const (
	ContentStream StreamDataType = iota
	ThinkingStream
	ToolCallStream
	ErrorStream
)

type StreamData struct {
	Type     StreamDataType `json:"type,omitempty"`
	Content  string         `json:"content,omitempty"`
	ToolCall *ToolCall      `json:"tool_call,omitempty"`
	Err      error          `json:"error,omitempty"`
}
