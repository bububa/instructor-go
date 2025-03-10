package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"reflect"

	"github.com/go-playground/validator/v10"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/ljson"
)

func Handler[T any, RESP any](i instructor.ChatInstructor[T, RESP], ctx context.Context, request *T, responseType any, response *RESP) error {
	var err error

	t := reflect.TypeOf(responseType)

	schema, err := instructor.NewSchema(t)
	if err != nil {
		return err
	}

	// keep a running total of usage
	usage := &instructor.UsageSum{}

	for attempt := 0; attempt <= i.MaxRetries(); attempt++ {
		if i.Verbose() {
			if bs, err := json.MarshalIndent(request, "", "  "); err != nil {
				log.Printf("%s Request(attempt:%d) MarshalError: %v\n", i.Provider(), attempt, err)
			} else {
				log.Printf("%s Request(attempt:%d): %s\n", i.Provider(), attempt, string(bs))
			}
		}

		resp := new(RESP)
		text, err := i.Handler(ctx, request, schema, resp)
		if err != nil {
			// no retry on non-marshalling/validation errors
			i.EmptyResponseWithResponseUsage(response, resp)
			return err
		}

		jsText := internal.ExtractJSON(&text)
		if i.Verbose() {
			log.Printf("%s Response(attempt:%d): %s\n", i.Provider(), attempt, text)
		}
		i.CountUsageFromResponse(resp, usage)

		if err := ljson.Unmarshal([]byte(jsText), &responseType); err != nil {
			// TODO:
			// add more sophisticated retry logic (send back json and parse error for model to fix).
			//
			if i.Verbose() {
				log.Printf("Err(attempt:%d): %+v\n", attempt, err)
			}
			// Currently, its just recalling with no new information
			// or attempt to fix the error with the last generated JSON
			continue
		}

		if i.Validate() {
			validate := validator.New()
			// Validate the response structure against the defined model using the validator
			if err := validate.Struct(responseType); err != nil {
				// TODO:
				// add more sophisticated retry logic (send back validator error and parse error for model to fix).
				if i.Verbose() {
					log.Printf("Err(attempt:%d): %+v\n", attempt, err)
				}

				continue
			}
		}

		i.SetUsageSumToResponse(response, usage)
		return nil
	}
	i.EmptyResponseWithUsageSum(response, usage)
	return errors.New("hit max retry attempts")
}
