package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
)

func (i *Instructor) InjectMCP(ctx context.Context, req *gemini.GenerateContentConfig) {
	req.Tools = make([]*gemini.Tool, 0, len(i.MCPTools()))
	for _, v := range i.MCPTools() {
		f := gemini.FunctionDeclaration{
			Name:        fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()),
			Description: v.Tool.Description,
			Parameters:  translateToGeminiSchema(v.Tool.InputSchema),
		}
		t := gemini.Tool{
			FunctionDeclarations: []*gemini.FunctionDeclaration{&f},
		}
		req.Tools = append(req.Tools, &t)
	}
}

func (i *Instructor) CallMCP(ctx context.Context, toolUse *gemini.FunctionCall) (*gemini.Part, *instructor.ToolCall) {
	toolContent := make(map[string]any)
	for _, v := range i.MCPTools() {
		if name := fmt.Sprintf("%s_%s", v.ServerName, v.Tool.GetName()); name == toolUse.Name {
			ret := new(instructor.ToolCall)
			ret.Request = new(mcp.CallToolRequest)
			ret.Request.Params.Name = v.Tool.GetName()
			ret.Request.Params.Arguments = toolUse.Args
			if result, err := v.Client.CallTool(ctx, *ret.Request); err != nil {
				ret.Result = mcp.NewToolResultError(fmt.Sprintf("tool call error: %v", err))
			} else {
				ret.Result = result
			}
			if bs, err := json.Marshal(ret.Result); err == nil {
				json.Unmarshal(bs, &toolContent)
			}
			return gemini.NewPartFromFunctionResponse(toolUse.Name, toolContent), ret
		}
	}
	toolRet := mcp.NewToolResultError("no mcp tool found")
	if bs, err := json.Marshal(toolRet); err == nil {
		json.Unmarshal(bs, &toolContent)
	}
	return gemini.NewPartFromFunctionResponse(toolUse.Name, toolContent), nil
}

func translateToGeminiSchema(schema mcp.ToolInputSchema) *gemini.Schema {
	s := &gemini.Schema{
		Type:       toType(schema.Type),
		Required:   schema.Required,
		Properties: make(map[string]*gemini.Schema),
	}

	for name, prop := range schema.Properties {
		m, ok := prop.(map[string]any)
		if !ok || len(m) == 0 {
			continue
		}
		s.Properties[name] = propertyToGeminiSchema(m)
	}

	if len(s.Properties) == 0 {
		// Functions that don't take any arguments have an object-type schema with 0 properties.
		// Google/Gemini does not like that: Error 400: * GenerateContentRequest properties: should be non-empty for OBJECT type.
		// To work around this issue, we'll just inject some unused, nullable property with a primitive type.
		s.Nullable = internal.ToPtr(true)
		s.Properties["unused"] = &gemini.Schema{
			Type:     gemini.TypeInteger,
			Nullable: internal.ToPtr(true),
		}
	}
	return s
}

func propertyToGeminiSchema(properties map[string]any) *gemini.Schema {
	typ, ok := properties["type"].(string)
	if !ok {
		return nil
	}
	s := &gemini.Schema{Type: toType(typ)}
	if desc, ok := properties["description"].(string); ok {
		s.Description = desc
	}

	// Objects and arrays need to have their properties recursively mapped.
	if s.Type == gemini.TypeObject {
		objectProperties := properties["properties"].(map[string]any)
		s.Properties = make(map[string]*gemini.Schema)
		for name, prop := range objectProperties {
			s.Properties[name] = propertyToGeminiSchema(prop.(map[string]any))
		}
	} else if s.Type == gemini.TypeArray {
		itemProperties := properties["items"].(map[string]any)
		s.Items = propertyToGeminiSchema(itemProperties)
	}

	return s
}

func toType(typ string) gemini.Type {
	switch typ {
	case "string":
		return gemini.TypeString
	case "number":
		return gemini.TypeNumber
	case "integer":
		return gemini.TypeInteger
	case "boolean":
		return gemini.TypeBoolean
	case "object":
		return gemini.TypeObject
	case "array":
		return gemini.TypeArray
	default:
		return gemini.TypeUnspecified
	}
}
