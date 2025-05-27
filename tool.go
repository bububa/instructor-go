package instructor

import (
	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type ToolCall struct {
	Request *mcp.CallToolRequest `json:"request,omitempty"`
	Result  *mcp.CallToolResult  `json:"result,omitempty"`
}

type MCPTool struct {
	Client     mcpClient.MCPClient
	ServerName string
	Tool       *mcp.Tool
}
