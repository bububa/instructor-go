package main

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/instructors"
)

type Person struct {
	Name string `json:"name"          jsonschema:"title=the name,description=The name of the person,example=joe,example=lucy"`
	Age  int    `json:"age,omitempty" jsonschema:"title=the age,description=The age of the person,example=25,example=67"`
}

func main() {
	ctx := context.Background()

	clt := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), option.WithBaseURL(os.Getenv("OPENAI_API_BASE_URL")))
	client := instructors.FromOpenAI(
    &clt,
		instructor.WithMode(instructor.ModeJSON),
		instructor.WithMaxRetries(3),
    instructor.WithVerbose(),
	)

	var (
		person Person
		resp   = new(openai.ChatCompletion)
	)
	err := client.Chat(
		ctx,
		&openai.ChatCompletionNewParams{
			Model: os.Getenv("OPENAI_MODEL"),
			Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage( "Extract Robby is 22 years old."),
			},
		},
		&person,
		resp,
	)
	_ = resp // sends back original response so no information loss from original API
	if err != nil {
		panic(err)
	}

	fmt.Printf(`
Name: %s
Age:  %d
`, person.Name, person.Age)
	/*
		Name: Robby
		Age:  22
	*/
}
