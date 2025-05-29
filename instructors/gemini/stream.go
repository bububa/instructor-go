package gemini

import (
	"context"

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

	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
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
	return i.stream(ctx, cfg, req, response, false)
}
