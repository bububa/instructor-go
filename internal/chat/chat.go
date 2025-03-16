package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

func Handler[T any, RESP any](i instructor.ChatInstructor[T, RESP], ctx context.Context, request *T, responseType any, response *RESP) error {
	var (
		enc = i.Encoder()
		err error
	)
	if enc == nil {
		if enc, err = encoding.PredefinedEncoder(i.Mode(), responseType); err != nil {
			return err
		}
		i.SetEncoder(enc)
	}

	// keep a running total of usage
	usage := &instructor.UsageSum{}
	retErr := errors.New("hit max retry attempts")

	for attempt := 0; attempt <= i.MaxRetries(); attempt++ {
		if i.Verbose() {
			if bs, err := json.MarshalIndent(request, "", "  "); err != nil {
				log.Printf("%s Request(attempt:%d) MarshalError: %v\n", i.Provider(), attempt, err)
			} else {
				log.Printf("%s Request(attempt:%d): %s\n", i.Provider(), attempt, string(bs))
			}
		}

		resp := new(RESP)
		text, err := i.Handler(ctx, request, resp)
		if err != nil {
			// no retry on non-marshalling/validation errors
			i.EmptyResponseWithResponseUsage(response, resp)
			return errors.Join(retErr, err)
		}

		if i.Verbose() {
			log.Printf("%s Response(attempt:%d): %s\n", i.Provider(), attempt, text)
		}
		i.CountUsageFromResponse(resp, usage)

		if err := enc.Unmarshal([]byte(text), &responseType); err != nil {
			if i.Verbose() {
				log.Printf("Err(attempt:%d): %+v\n", attempt, err)
			}
			retErr = errors.Join(retErr, err)
			// Currently, its just recalling with no new information
			// or attempt to fix the error with the last generated JSON
			continue
		}

		if i.Validate() {
			if validator, ok := enc.(instructor.Validator); ok {
				// Validate the response structure against the defined model using the validator
				if err := validator.Validate(responseType); err != nil {
					if i.Verbose() {
						log.Printf("Err(attempt:%d): %+v\n", attempt, err)
					}
					retErr = errors.Join(retErr, err)
					continue
				}
			}
		}

		i.SetUsageSumToResponse(response, usage)
		return nil
	}
	i.EmptyResponseWithUsageSum(response, usage)
	return retErr
}
