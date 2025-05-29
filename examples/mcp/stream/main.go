package main

import (
	"context"
	"encoding/json"
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
		instructor.WithMode(instructor.ModePlainText),
		instructor.WithVerbose(),
		instructor.WithoutThinking(),
		instructor.WithExtraBody(map[string]any{
			"enable_thinking": false,
		}),
		instructor.WithMCPTools(tools...),
	)

	var result string
	ch, err := client.Stream(ctx, &openai.ChatCompletionNewParams{
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
	var thinkingStart bool
	for part := range ch {
		switch part.Type {
		case instructor.ThinkingStream:
			if !thinkingStart {
				fmt.Println("thinking...")
			}
			thinkingStart = true
			fmt.Print(part.Content)
		case instructor.ContentStream:
			if thinkingStart {
				fmt.Println()
				fmt.Println("thinking end")
				thinkingStart = false
			}
			fmt.Print(part.Content)
		case instructor.ToolCallStream:
			bs, _ := json.MarshalIndent(part.ToolCall, "", "  ")
			fmt.Println(string(bs))
		}
	}
}
