package cohere

import (
	"encoding/json"
	"errors"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go"
)

func ConvertMessageFrom(src *instructor.Message, dist *cohere.Message) {
	if src.Role == instructor.SystemRole {
		if src.Text != "" {
			dist.Role = "SYSTEM"
			dist.System = &cohere.ChatMessage{
				Message: src.Text,
			}
		}
		return
	}
	if len(src.ToolResults) > 0 {
		list := make([]*cohere.ToolResult, 0, len(src.ToolResults))
		for _, v := range src.ToolResults {
			args := make(map[string]any)
			if err := json.Unmarshal([]byte(v.Content), &args); err != nil {
				continue
			}
			msg := cohere.ToolResult{
				Call: &cohere.ToolCall{
					Name: v.Name,
				},
				Outputs: []map[string]any{args},
			}
			list = append(list, &msg)
		}
		dist.Role = "USER"
		dist.Tool = &cohere.ToolMessage{
			ToolResults: list,
		}
		return
	}
	if len(src.ToolUses) > 0 {
		list := make([]*cohere.ToolCall, 0, len(src.ToolUses))
		for _, v := range src.ToolUses {
			args := make(map[string]any)
			if err := json.Unmarshal([]byte(v.Arguments), &args); err != nil {
				continue
			}
			call := cohere.ToolCall{
				Name:       v.Name,
				Parameters: args,
			}
			list = append(list, &call)
		}
		dist.Role = "CHATBOT"
		dist.Chatbot = &cohere.ChatMessage{
			ToolCalls: list,
		}
		return
	}
	switch src.Role {
	case instructor.AssistantRole:
		dist.Role = "CHATBOT"
		dist.Chatbot = &cohere.ChatMessage{
			Message: src.Text,
		}
	case instructor.UserRole:
		dist.Role = "CHATBOT"
		dist.Chatbot = &cohere.ChatMessage{
			Message: src.Text,
		}
	}
}

func ConvertMessageTo(src *cohere.Message, dist *instructor.Message) error {
	if msg := src.Chatbot; msg != nil {
		dist.Role = instructor.AssistantRole
		if toolCalls := msg.ToolCalls; len(toolCalls) > 0 {
			for _, v := range toolCalls {
				bs, _ := json.Marshal(v.Parameters)
				dist.ToolUses = append(dist.ToolUses, instructor.ToolUse{
					Name:      v.Name,
					Arguments: string(bs),
				})
			}
		} else {
			dist.Text = msg.Message
		}
	} else if msg := src.User; msg != nil {
		dist.Role = instructor.UserRole
		dist.Text = msg.Message
	} else if msg := src.Tool; msg != nil {
		dist.Role = instructor.ToolRole
		for _, v := range msg.ToolResults {
			bs, _ := json.Marshal(v.Outputs[0])
			dist.ToolResults = append(dist.ToolResults, instructor.ToolResult{
				Name:    v.Call.Name,
				Content: string(bs),
			})
		}
	}
	return errors.New("role not support")
}
