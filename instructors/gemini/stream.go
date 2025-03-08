package gemini

import (
	"context"
	"encoding/json"
	"log"

	gemini "github.com/google/generative-ai-go/genai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *Request,
	response *gemini.GenerateContentResponse,
) (stream <-chan string, err error) {
	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System

	if i.Verbose() {
		modelBytes, _ := json.MarshalIndent(model, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Model: %s\n`, i.Provider(), string(bs), string(modelBytes))
	}

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
