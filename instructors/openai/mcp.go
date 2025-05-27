package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bububa/instructor-go"
	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/sashabaranov/go-openai"
)

func (i *Instructor) InjectMCP(ctx context.Context, req *openai.ChatCompletionRequest) {
	req.Tools = make([]openai.Tool, 0, len(i.MCPTools()))
	for _, v := range i.MCPTools() {
		f := openai.FunctionDefinition{
			Name:        fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()),
			Description: v.Tool.Description,
			Parameters:  v.Tool.InputSchema,
		}
		tool := openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &f,
		}
		req.Tools = append(req.Tools, tool)
	}
	req.ToolChoice = "auto"
}

func (i *Instructor) CallMCP(ctx context.Context, toolUse *openai.ToolCall, req *openai.ChatCompletionRequest) *instructor.ToolCall {
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
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       toolUse.Function.Name,
				ToolCallID: toolUse.ID,
				Content:    toolContent,
			})
			return ret
		}
	}
	toolRet := mcp.NewToolResultError("no mcp tool found")
	if bs, err := json.Marshal(toolRet); err == nil {
		toolContent = string(bs)
	}
	req.Messages = append(req.Messages, openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Name:       toolUse.Function.Name,
		ToolCallID: toolUse.ID,
		Content:    toolContent,
	})
	return nil
}
