package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/invopop/jsonschema"
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) Chat(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) error {
	return chat.Handler(i, ctx, request, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *Request, response *gemini.GenerateContentResponse) (string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCall(ctx, *request, response)
	case instructor.ModeJSONStrict:
		return i.completion(ctx, *request, response, true)
	default:
		return i.completion(ctx, *request, response, false)
	}
}

func (i *Instructor) chatToolCall(ctx context.Context, request Request, response *gemini.GenerateContentResponse) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}

	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
		Tools:             createTools(schema),
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
	}

	var (
		resp *gemini.GenerateContentResponse
		err  error
	)
	if len(request.History) > 0 {
		cs, err := i.Chats.Create(ctx, request.Model, &cfg, request.History)
		if err != nil {
			return "", err
		}
		parts := make([]gemini.Part, 0, len(request.Parts))
		for _, part := range request.Parts {
			parts = append(parts, *part)
		}
		resp, err = cs.SendMessage(ctx, parts...)
	} else {
		resp, err = i.Models.GenerateContent(ctx, request.Model, []*gemini.Content{
			{
				Parts: request.Parts,
				Role:  "user",
			},
		}, &cfg)
	}

	if err != nil {
		return "", err
	}
	if response != nil {
		*response = *resp
	}

	var toolCalls []gemini.FunctionCall
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if toolCall := part.FunctionCall; toolCall != nil {
				toolCalls = append(toolCalls, *toolCall)
				break
			}
		}
	}

	numTools := len(toolCalls)

	if numTools < 1 {
		i.EmptyResponseWithResponseUsage(response, resp)
		return "", errors.New("received no tool calls from model, expected at least 1")
	}

	if numTools == 1 {
		if response != nil {
			*response = *resp
		}
		resultJSON, err := json.Marshal(toolCalls[0].Args)
		if err != nil {
			i.EmptyResponseWithResponseUsage(response, resp)
			return "", err
		}
		return string(resultJSON), nil
	}

	jsonArray := make([]map[string]any, len(toolCalls))

	for idx, toolCall := range toolCalls {
		jsonArray[idx] = toolCall.Args
	}

	resultJSON, err := json.Marshal(jsonArray)
	if err != nil {
		i.EmptyResponseWithResponseUsage(response, resp)
		return "", err
	}

	if response != nil {
		*response = *resp
	}
	return string(resultJSON), nil
}

func (i *Instructor) completion(ctx context.Context, request Request, response *gemini.GenerateContentResponse, strict bool) (string, error) {
	if bs := i.Encoder().Context(); bs != nil {
		request.Parts = append(request.Parts, &gemini.Part{Text: string(bs)})
	}

	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	enc, isJSON := i.Encoder().(*jsonenc.Encoder)
	if isJSON {
		cfg.ResponseMIMEType = "application/json"
	} else {
		cfg.ResponseMIMEType = "text/plain"
	}
	if strict {
		if !isJSON {
			return "", errors.New("encoder must be JSON Encoder")
		}
		schema := enc.Schema()
		cfg.ResponseSchema = new(gemini.Schema)
		convertSchema(schema.Schema, cfg.ResponseSchema)
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
	}

	var (
		resp *gemini.GenerateContentResponse
		err  error
	)
	if len(request.History) > 0 {
		cs, err := i.Chats.Create(ctx, request.Model, &cfg, request.History)
		if err != nil {
			return "", err
		}
		parts := make([]gemini.Part, 0, len(request.Parts))
		for _, part := range request.Parts {
			parts = append(parts, *part)
		}
		resp, err = cs.SendMessage(ctx, parts...)
	} else {
		resp, err = i.Models.GenerateContent(ctx, request.Model, []*gemini.Content{
			{
				Parts: request.Parts,
				Role:  "user",
			},
		}, &cfg)
	}

	if err != nil {
		return "", err
	}
	if response != nil {
		*response = *resp
	}
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if text := part.Text; text != "" {
				return text, nil
			}
		}
	}
	return "", nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *gemini.GenerateContentResponse, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = gemini.GenerateContentResponse{
		UsageMetadata: &gemini.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     internal.ToPtr(int32(usage.InputTokens)),
			CandidatesTokenCount: internal.ToPtr(int32(usage.OutputTokens)),
			TotalTokenCount:      int32(usage.TotalTokens),
		},
	}
}

func (i *Instructor) EmptyResponseWithResponseUsage(ret *gemini.GenerateContentResponse, response *gemini.GenerateContentResponse) {
	if ret == nil {
		return
	}
	var resp gemini.GenerateContentResponse
	if response == nil {
		*ret = resp
		return
	}
	resp.UsageMetadata = new(gemini.GenerateContentResponseUsageMetadata)
	*resp.UsageMetadata = *response.UsageMetadata
	*ret = resp
}

func (i *Instructor) SetUsageSumToResponse(response *gemini.GenerateContentResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	if response.UsageMetadata == nil {
		response.UsageMetadata = new(gemini.GenerateContentResponseUsageMetadata)
	}
	response.UsageMetadata.PromptTokenCount = internal.ToPtr(int32(usage.InputTokens))
	response.UsageMetadata.CandidatesTokenCount = internal.ToPtr(int32(usage.OutputTokens))
	response.UsageMetadata.TotalTokenCount = int32(usage.TotalTokens)
}

func (i *Instructor) CountUsageFromResponse(response *gemini.GenerateContentResponse, usage *instructor.UsageSum) {
	if response == nil || response.UsageMetadata == nil || usage == nil {
		return
	}
	if v := response.UsageMetadata.PromptTokenCount; v != nil {
		usage.InputTokens += int(*v)
	}
	if v := response.UsageMetadata.CandidatesTokenCount; v != nil {
		usage.OutputTokens += int(*v)
	}
	usage.TotalTokens += int(response.UsageMetadata.TotalTokenCount)
}

func convertSchema(src *jsonschema.Schema, dist *gemini.Schema) {
	dist.Type = gemini.TypeObject
	dist.Format = src.Format
	dist.Description = src.Description
	dist.Enum = make([]string, 0, len(src.Enum))
	for _, v := range src.Enum {
		if str, ok := v.(string); ok {
			dist.Enum = append(dist.Enum, str)
		}
	}
	dist.Properties = make(map[string]*gemini.Schema, src.Properties.Len())
	for pair := src.Properties.Oldest(); pair != nil; pair.Next() {
		schema := new(gemini.Schema)
		convertSchema(pair.Value, schema)
		dist.Properties[pair.Key] = schema
	}
	dist.Required = src.Required
}

func createTools(schema *instructor.Schema) []*gemini.Tool {
	tools := make([]*gemini.Tool, 0, len(schema.Functions))
	for _, function := range schema.Functions {
		f := gemini.FunctionDeclaration{
			Name:        function.Name,
			Description: function.Description,
		}
		f.Parameters = new(gemini.Schema)
		convertSchema(function.Parameters, f.Parameters)
		t := gemini.Tool{
			FunctionDeclarations: []*gemini.FunctionDeclaration{&f},
		}
		tools = append(tools, &t)
	}
	return tools
}
