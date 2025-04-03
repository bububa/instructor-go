package gemini

import (
	"context"
	"encoding/json"
	"log"

	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (<-chan instructor.StreamData, error) {
	cfg := gemini.GenerateContentConfig{
		SystemInstruction: request.System,
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
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
			req.Parts = append(req.Parts, &gemini.Part{Text: string(bs)})
		}
		if _, isJSON := i.Encoder().(*jsonenc.Encoder); isJSON {
			cfg.ResponseMIMEType = "application/json"
		} else {
			cfg.ResponseMIMEType = "text/plain"
		}
	}

	contents := make([]*gemini.Content, len(request.History)+1)
	contents = append(contents, request.History...)
	contents = append(contents, &gemini.Content{
		Parts: request.Parts,
		Role:  "user",
	})
	iter := i.Models.GenerateContentStream(ctx, request.Model, contents, &cfg)

	return i.createStream(ctx, iter, response)
}
