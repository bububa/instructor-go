package main

import (
	"context"
	"fmt"
	"log"
	"os"
  "encoding/json"

	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/instructors"
)

func main() {
	ctx := context.Background()
	envs := []string{fmt.Sprintf("FIRECRAWL_API_KEY=%s", os.Getenv("FIRECRAWL_API_KEY"))}
	mcpClt, err := mcpClient.NewStdioMCPClient("npx", envs, "-y", "firecrawl-mcp")
	if err != nil {
		log.Fatalln(err)
		return
	}
	// if err := mcpClt.Start(ctx); err != nil {
	// 	log.Fatalf("Failed to start client: %v", err)
	// }
	// Initialize the client
	fmt.Println("Initializing client...")
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "MCP-Go Client for instructor-go",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	serverInfo, err := mcpClt.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Display server information
	fmt.Printf("Connected to server: %s (version %s)\n",
		serverInfo.ServerInfo.Name,
		serverInfo.ServerInfo.Version)
	fmt.Printf("Server capabilities: %+v\n", serverInfo.Capabilities)
	toolListResult, err := mcpClt.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Fatalln(err)
		return
	}
	tools := make([]instructor.MCPTool, 0, len(toolListResult.Tools))
	for _, v := range toolListResult.Tools {
		tools = append(tools, instructor.MCPTool{
			ServerName: serverInfo.ServerInfo.Name,
			Client:     mcpClt,
			Tool:       &v,
		})
	}

  clt := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), option.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	client := instructors.FromOpenAI(
    &clt,
		instructor.WithMode(instructor.ModePlainText),
		instructor.WithVerbose(),
		instructor.WithThinking(2048),
		instructor.WithExtraBody(map[string]any{
			"enable_thinking": true,
		}),
		instructor.WithMCPTools(tools...),
	)

	var result string
	ch, err := client.Stream(ctx, &openai.ChatCompletionNewParams{
		Model: os.Getenv("OPENAI_MODEL"),
		Messages: []openai.ChatCompletionMessageParamUnion{
      openai.SystemMessage("你是一个很有帮助的助手。如果需要搜索请使用工具。"),
				openai.UserMessage("搜索目前比较受欢迎的扫地机"),
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
		case instructor.ErrorStream:
      fmt.Println("ERROR", part.Err)
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
