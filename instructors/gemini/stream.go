package gemini

import (
	"context"
	"encoding/json"
	"log"

	"github.com/bububa/instructor-go/encoding"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	gemini "github.com/google/generative-ai-go/genai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *Request,
	responseType any,
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

	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType); err != nil {
				return nil, err
			} else {
				i.SetEncoder(enc)
			}
		}
		if bs := i.Encoder().Context(); bs != nil {
			req.Parts = append(req.Parts, gemini.Text(string(bs)))
		}
		if _, isJSON := i.Encoder().(*jsonenc.Encoder); isJSON {
			model.ResponseMIMEType = "application/json"
		} else {
			model.ResponseMIMEType = "text/plain"
		}
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
