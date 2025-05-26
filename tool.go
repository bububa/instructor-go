package instructor

import (
  "context"

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolCall struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

type MCPTool struct {
	Client MCPClient
  ServerName string
	Tool   *mcp.Tool
}

type MCPClient interface {
  CallTool(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}
