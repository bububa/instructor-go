package mock

import (
	"context"
	"fmt"
	"strings"

	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MockMCPClient 模拟 MCP 客户端
type MockMCPClient mcpClient.Client

func (c *MockMCPClient) ListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{
			{
				Name:        "get_weather_data",
				Description: "通过 MCP 协议查询天气信息",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]any{
						"location": map[string]string{ // 动态参数，模型会根据 query_type 提供不同的 JSON 字符串
							"type":        "string",
							"description": "查询城市",
						},
					},
					Required: []string{"location"},
				},
			},
		},
	}, nil
}

// simulateMCPQuery 模拟调用 MCP 库进行查询操作
func (c *MockMCPClient) CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	location := strings.TrimSpace(req.GetString("location", ""))
	fmt.Printf("MCP Client: 模拟查询天气操作，参数: %s\n", location)

	// 实际这里会调用 mcp-go 库的函数，并返回其 ToolResult
	// 例如：mcpClient.CallTool(...)

	if location == "北京" {
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: `{"location": "北京", "temperature": "26", "unit": "摄氏度", "forecast": "晴朗"}`},
			},
		}, nil
	} else if location == "上海" {
		return &mcp.CallToolResult{
			IsError: false,
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: `{"location": "上海", "temperature": "28", "unit": "摄氏度", "forecast": "多云"}`},
			},
		}, nil
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: `{"error": "未找到指定地点的天气信息"}`},
		},
	}, fmt.Errorf("未找到指定地点的天气信息")
}
