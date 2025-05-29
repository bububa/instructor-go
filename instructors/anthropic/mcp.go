package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/bububa/instructor-go"
)

func (i *Instructor) InjectMCP(ctx context.Context, req *anthropic.MessagesRequest) {
	req.Tools = make([]anthropic.ToolDefinition, 0, len(i.MCPTools()))
	for _, v := range i.MCPTools() {
		tool := anthropic.ToolDefinition{
			Name:        fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()),
			Description: v.Tool.Description,
			InputSchema: v.Tool.InputSchema,
		}
		req.Tools = append(req.Tools, tool)
	}
}

func (i *Instructor) CallMCP(ctx context.Context, toolUse *anthropic.MessageContentToolUse) (anthropic.MessageContent, *instructor.ToolCall) {
	var toolContent string
	for _, v := range i.MCPTools() {
		if name := fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()); name == toolUse.Name {
			var (
				toolArgs map[string]any
				isError  bool
			)
			ret := new(instructor.ToolCall)
			if err := json.Unmarshal([]byte(toolUse.Input), &toolArgs); err != nil {
				ret.Result = mcp.NewToolResultError(fmt.Sprintf("error parsing tool arguments: %v", err))
				if bs, err := json.Marshal(ret.Result); err == nil {
					toolContent = string(bs)
				}
				isError = true
			} else {
				ret.Request = new(mcp.CallToolRequest)
				ret.Request.Params.Name = v.Tool.GetName()
				ret.Request.Params.Arguments = toolArgs
				ret.Result, err = v.Client.CallTool(ctx, *ret.Request)
				if err != nil {
					ret.Result = mcp.NewToolResultError(fmt.Sprintf("tool call error: %v", err))
					isError = true
				}
				if bs, err := json.Marshal(ret.Result); err == nil {
					toolContent = string(bs)
				}
			}
			return anthropic.NewToolResultMessageContent(toolUse.ID, toolContent, isError), ret
		}
	}
	toolRet := mcp.NewToolResultError("invalid tool name")
	if bs, err := json.Marshal(toolRet); err == nil {
		toolContent = string(bs)
	}
	return anthropic.NewToolResultMessageContent(toolUse.ID, toolContent, true), nil
}
