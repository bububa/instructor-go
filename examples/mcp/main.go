package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/examples/mcp/mock"
	"github.com/bububa/instructor-go/instructors"
)

func main() {
	ctx := context.Background()
	mcpClt := new(mock.MockMCPClient)
	toolListResult, err := mcpClt.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalln(err)
		return
	}
	tools := make([]instructor.MCPTool, 0, len(toolListResult.Tools))
	for _, v := range toolListResult.Tools {
		tools = append(tools, instructor.MCPTool{
			ServerName: "mock",
			Client:     mcpClt,
			Tool:       &v,
		})
	}

  clt := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), option.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	client := instructors.FromOpenAI(
  &clt,
		instructor.WithMode(instructor.ModeJSON),
		instructor.WithVerbose(),
		instructor.WithExtraBody(map[string]any{
			"enable_thinking": false,
		}),
		instructor.WithMCPTools(tools...),
	)

	type Result struct {
		Weather string `json:"weather,omitempty"`
	}
	var result Result
	err = client.Chat(ctx, &openai.ChatCompletionNewParams{
		Model: os.Getenv("OPENAI_MODEL"),
		Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(`你是一个很有帮助的助手。如果用户提问关于天气的问题，调用 ‘get_weather_data’ 函数;
     请以友好的语气回答问题。`),
				openai.UserMessage("上海现在天气怎么样？"),
		},
	},
		&result,
		nil,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}
