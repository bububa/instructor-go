package gemini

import (
	"context"

	gemini "github.com/google/generative-ai-go/genai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *Request,
	response *gemini.GenerateContentResponse,
) (stream <-chan string, err error) {
	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System
	var iter *gemini.GenerateContentResponseIterator

	if len(request.History) > 0 {
		cs := model.StartChat()
		cs.History = request.History
		iter = cs.SendMessageStream(ctx, request.Parts...)
	} else {
		iter = model.GenerateContentStream(ctx, request.Parts...)
	}
	return i.createStream(ctx, iter, response)
}
