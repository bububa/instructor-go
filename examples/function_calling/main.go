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

type SearchType string

const (
	Web   SearchType = "web"
	Image SearchType = "image"
	Video SearchType = "video"
)

type Search struct {
	Topic string     `json:"topic" jsonschema:"title=Topic,description=Topic of the search,example=golang"`
	Query string     `json:"query" jsonschema:"title=Query,description=Query to search for relevant content,example=what is golang"`
	Type  SearchType `json:"type"  jsonschema:"title=Type,description=Type of search,default=web,enum=web,enum=image,enum=video"`
}

func (s *Search) execute() {
	fmt.Printf("Searching for `%s` with query `%s` using `%s`\n", s.Topic, s.Query, s.Type)
}

type Searches struct {
  Items []Search `json:"items"`
}

func segment(ctx context.Context, data string) *Searches {
	clt := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), option.WithBaseURL(os.Getenv("OPENAI_API_BASE_URL")))
	client := instructors.FromOpenAI(
    &clt,
		instructor.WithMode(instructor.ModeToolCall),
		instructor.WithMaxRetries(3),
    instructor.WithVerbose(),
	)

	var searches Searches
	err := client.Chat(ctx, &openai.ChatCompletionNewParams{
		Model: os.Getenv("OPENAI_MODEL"),
		Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(fmt.Sprintf("Consider the data below: '\n%s' and segment it into multiple search queries", data)),
		},
	},
		&searches,
		nil,
	)
	if err != nil {
		panic(err)
	}

	return &searches
}

func main() {
	ctx := context.Background()

	q := "Search for a picture of a cat, a video of a dog, and the taxonomy of each"
  searches := *segment(ctx, q)
	for _, search := range searches.Items {
		search.execute()
	}
	/*
		Searching for `cat` with query `picture of a cat` using `image`
		Searching for `dog` with query `video of a dog` using `video`
		Searching for `cat` with query `taxonomy of a cat` using `web`
		Searching for `dog` with query `taxonomy of a dog` using `web`
	*/
}
