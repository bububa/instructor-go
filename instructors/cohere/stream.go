package cohere

import (
	"context"
	"fmt"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go/encoding"
	"github.com/bububa/instructor-go/internal"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan string, <-chan string, error) {
	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType); err != nil {
				return nil, nil, err
			} else {
				i.SetEncoder(enc)
			}
		}
		if bs := i.Encoder().Context(); bs != nil {
			if system := req.Preamble; system == nil {
				req.Preamble = internal.ToPtr(string(bs))
			} else {
				req.Preamble = internal.ToPtr(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", *system, string(bs)))
			}
		}
	}
	stream, thinking, err := i.createStream(ctx, &req, response)
	if err != nil {
		return nil, nil, err
	}

	return stream, thinking, err
}
