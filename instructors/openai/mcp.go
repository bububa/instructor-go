package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bububa/instructor-go"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
)

func (i *Instructor) InjectMCP(ctx context.Context, req *openai.ChatCompletionNewParams) {
  l := len(i.MCPTools())
  if l == 0 {
    return
  }
  if req.Tools == nil {
	  req.Tools = make([]openai.ChatCompletionToolParam, 0, l)
  }
	for _, v := range i.MCPTools() {
		tool := openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()),
				Description: openai.String(v.Tool.Description),
				Parameters: openai.FunctionParameters{
					"type":       v.Tool.InputSchema.Type,
					"required":   v.Tool.InputSchema.Required,
					"properties": v.Tool.InputSchema.Properties,
				},
			},
		}
		req.Tools = append(req.Tools, tool)
	}
}

func (i *Instructor) CallMCP(ctx context.Context, toolUse *openai.ChatCompletionMessageToolCall, req *openai.ChatCompletionNewParams) *instructor.ToolCall {
	var toolContent string
	for _, v := range i.MCPTools() {
		if name := fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()); name == toolUse.Function.Name {
			var toolArgs map[string]any
			ret := new(instructor.ToolCall)
			if err := json.Unmarshal([]byte(toolUse.Function.Arguments), &toolArgs); err != nil {
				ret.Result = mcp.NewToolResultError(fmt.Sprintf("error parsing tool arguments: %v", err))
				if bs, err := json.Marshal(ret.Result); err == nil {
					toolContent = string(bs)
				}
			} else {
				ret.Request = new(mcp.CallToolRequest)
				ret.Request.Params.Name = v.Tool.GetName()
				ret.Request.Params.Arguments = toolArgs
				ret.Result, err = v.Client.CallTool(ctx, *ret.Request)
				if err != nil {
					ret.Result = mcp.NewToolResultError(fmt.Sprintf("tool call error: %v", err))
				}
				if bs, err := json.Marshal(ret.Result); err == nil {
					toolContent = string(bs)
				}
			}
			req.Messages = append(req.Messages, openai.ToolMessage(toolContent, toolUse.ID))
			return ret
		}
	}
	toolRet := mcp.NewToolResultError("invalid tool name")
	if bs, err := json.Marshal(toolRet); err == nil {
		toolContent = string(bs)
	}
	req.Messages = append(req.Messages, openai.ToolMessage(toolContent, toolUse.ID))
	return nil
}
