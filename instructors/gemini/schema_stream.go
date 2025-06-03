package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log"
	"strings"

	"google.golang.org/api/iterator"
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
	"github.com/mark3labs/mcp-go/mcp"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (<-chan any, <-chan instructor.StreamData, error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *Request, response *gemini.GenerateContentResponse) (<-chan instructor.StreamData, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	case instructor.ModeJSON:
		return i.chatJSONStream(ctx, *request, response, false)
	case instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, response, true)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse) (<-chan instructor.StreamData, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
	}
	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
		Tools:             createTools(schema),
	}
	return i.stream(ctx, cfg, request, response, false)
}

func (i *Instructor) chatJSONStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse, strict bool) (<-chan instructor.StreamData, error) {
	if bs := i.StreamEncoder().Context(); bs != nil {
		request.Parts = append(request.Parts, &gemini.Part{Text: string(bs)})
	}
	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
	}

	enc, isJSON := i.Encoder().(*jsonenc.Encoder)
	if isJSON {
		cfg.ResponseMIMEType = "application/json"
	} else {
		cfg.ResponseMIMEType = "text/plain"
	}

	if strict {
		cfg.ResponseSchema = new(gemini.Schema)
		schema := enc.Schema()
		convertSchema(schema.Schema, cfg.ResponseSchema)
	}
	return i.stream(ctx, cfg, request, response, false)
}

func (i *Instructor) stream(ctx context.Context, cfg gemini.GenerateContentConfig, request Request, response *gemini.GenerateContentResponse, reRun bool) (<-chan instructor.StreamData, error) {
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: thinkingConfig.Enabled,
			ThinkingBudget:  internal.ToPtr(int32(thinkingConfig.Budget)),
		}
	}
	if !reRun {
		i.InjectMCP(ctx, &cfg)
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
	}
	contents := make([]*gemini.Content, len(request.History)+1)
	contents = append(contents, request.History...)
	contents = append(contents, gemini.NewContentFromParts(request.Parts, gemini.RoleUser))
	iter := i.Models.GenerateContentStream(ctx, request.Model, contents, &cfg)
	outCh := make(chan instructor.StreamData)
	go func() {
		defer close(outCh)
		var toolRequest Request
		defer func() {
			oldMessagesCount := len(toolRequest.History)
			if oldMessagesCount > 0 {
				request.History = append(request.History, toolRequest.History...)
				tmpCh, err := i.stream(ctx, cfg, request, response, true)
				if err != nil {
					return
				}
				if newMessagesCount := len(request.History); newMessagesCount > oldMessagesCount && i.memory != nil {
					i.memory.Add(request.History[oldMessagesCount:newMessagesCount]...)
				}
				for v := range tmpCh {
					outCh <- v
				}
			}
		}()
		var usage gemini.GenerateContentResponseUsageMetadata
		if response != nil && response.UsageMetadata != nil {
			usage = *response.UsageMetadata
		}
		defer func() {
			if response != nil {
				if response.UsageMetadata == nil {
					response.UsageMetadata = &usage
				} else {
					response.UsageMetadata.PromptTokenCount += usage.PromptTokenCount
					response.UsageMetadata.CandidatesTokenCount += usage.CandidatesTokenCount
					response.UsageMetadata.TotalTokenCount += usage.TotalTokenCount
				}
			}
		}()
		ch, err := i.createStream(ctx, &cfg, iter, response, &toolRequest)
		if err != nil {
			return
		}
		var bs strings.Builder
		for part := range ch {
			if !reRun && part.Type == instructor.ContentStream {
				bs.WriteString(part.Content)
			}
			outCh <- part
		}
		if text := bs.String(); text != "" && i.memory != nil {
			i.memory.Add(gemini.NewContentFromText(text, gemini.RoleModel))
		}
	}()
	return outCh, nil
}

func (i *Instructor) createStream(ctx context.Context, cfg *gemini.GenerateContentConfig, iter iter.Seq2[*gemini.GenerateContentResponse, error], response *gemini.GenerateContentResponse, toolRequest *Request) (<-chan instructor.StreamData, error) {
	ch := make(chan instructor.StreamData)

	go func() {
		defer close(ch)
		sb := new(bytes.Buffer)
		if i.Verbose() {
			defer func() {
				log.Printf("%s Response: %s\n", i.Provider(), sb.String())
			}()
		}
		toolCallMap := make(map[int32]gemini.FunctionCall)
		toolCallInput := make(map[int32]string)
		defer func() {
			toolCalls := make([]gemini.FunctionCall, 0, len(toolCallMap))
			parts := make([]*gemini.Part, 0, len(toolCallMap))
			for idx, toolCall := range toolCallMap {
				if input, ok := toolCallInput[idx]; ok {
					if err := json.Unmarshal([]byte(input), &toolCall.Args); err != nil {
						toolCalls = append(toolCalls, toolCall)
						parts = append(parts, gemini.NewPartFromFunctionCall(toolCall.Name, toolCall.Args))
					}
				}
			}
			if len(toolCalls) == 0 {
				return
			}
			toolRequest.History = append(toolRequest.History, gemini.NewContentFromParts(parts, gemini.RoleModel))
			contents := make([]*gemini.Part, 0, len(toolCalls))
			for _, toolCall := range toolCalls {
				part, call := i.CallMCP(ctx, &toolCall)
				if call != nil {
					ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: call}
				} else {
					var shouldReturn bool
					for _, tool := range cfg.Tools {
						for _, fn := range tool.FunctionDeclarations {
							if fn.Name == toolCall.Name {
								callReq := new(mcp.CallToolRequest)
								callReq.Params.Name = toolCall.Name
								callReq.Params.Arguments = toolCall.Args
								call := instructor.ToolCall{
									Request: callReq,
								}
								ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: &call}
								shouldReturn = true
							}
						}
					}
					if shouldReturn {
						toolRequest.History = nil
						return
					}
				}
				contents = append(contents, part)
			}
			toolRequest.History = append(toolRequest.History, gemini.NewContentFromParts(contents, "function"))
		}()
		for resp, err := range iter {
			if err == iterator.Done {
				return
			}
			if err != nil {
				ch <- instructor.StreamData{Type: instructor.ErrorStream, Err: err}
				return
			}
			if response != nil {
				response.UsageMetadata = resp.UsageMetadata
			}
			for _, cand := range resp.Candidates {
				if cand.Content == nil {
					continue
				}
				for _, part := range cand.Content.Parts {
					if fcCall := part.FunctionCall; fcCall != nil {
						if _, ok := toolCallMap[cand.Index]; !ok {
							toolCallMap[cand.Index] = gemini.FunctionCall{
								ID:   fcCall.ID,
								Name: fcCall.Name,
							}
						}
						if fcCall.Args != nil {
							bs, _ := json.Marshal(fcCall.Args)
							toolCallInput[cand.Index] += string(bs)
						}
					}
					if part.Thought {
						ch <- instructor.StreamData{Type: instructor.ThinkingStream, Content: part.Text}
					} else if text := part.Text; text != "" {
						if i.Verbose() {
							sb.WriteString(text)
						}
						ch <- instructor.StreamData{Type: instructor.ContentStream, Content: text}
					}
				}
			}
		}
	}()
	return ch, nil
}
