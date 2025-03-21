package anthropic

import (
	"context"
	"fmt"

	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go/encoding"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *anthropic.MessagesRequest,
	responseType any,
	response *anthropic.MessagesResponse,
) (stream <-chan string, err error) {
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
			if req.System == "" {
				req.System = string(bs)
			} else {
				req.System = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", req.System, bs)
			}
		}
	}
	stream, err = i.createStream(ctx, &req, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}
