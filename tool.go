package instructor

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolCall struct {
	Request *mcp.CallToolRequest `json:"request,omitempty"`
	Result  *mcp.CallToolResult  `json:"result,omitempty"`
}

type MCPTool struct {
	Client     MCPClient
	ServerName string
	Tool       *mcp.Tool
}

type MCPClient interface {
	CallTool(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}
