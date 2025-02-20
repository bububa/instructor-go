package instructor

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/invopop/jsonschema"
)

var reflectorPool = sync.Pool{
	New: func() any {
		return new(jsonschema.Reflector)
	},
}

type Schema struct {
	*jsonschema.Schema
	String string

	Functions []FunctionDefinition
}

type Function struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters"`
}

func NewSchema(t reflect.Type) (*Schema, error) {
	schema := JSONSchema(t)

	str, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}

	funcs := ToFunctionSchema(t, schema)

	s := &Schema{
		Schema: schema,
		String: string(str),

		Functions: funcs,
	}

	return s, nil
}

func ToFunctionSchema(tType reflect.Type, tSchema *jsonschema.Schema) []FunctionDefinition {
	fds := []FunctionDefinition{}

	for name, def := range tSchema.Definitions {

		parameters := &jsonschema.Schema{
			Type:       "object",
			Properties: def.Properties,
			Required:   def.Required,
		}

		fd := FunctionDefinition{
			Name:        name,
			Description: def.Description,
			Parameters:  parameters,
		}

		fds = append(fds, fd)
	}

	return fds
}

func (s *Schema) NameFromRef() string {
	return strings.Split(s.Ref, "/")[2] // ex: '#/$defs/MyStruct'
}

// JSONSchema return the json schema of the configuration
func JSONSchema(t reflect.Type) *jsonschema.Schema {
	r := reflectorPool.Get().(*jsonschema.Reflector)
	defer reflectorPool.Put(r)

	// The Struct name could be same, but the package name is different
	// For example, all of the notification plugins have the same struct name - `NotifyConfig`
	// This would cause the json schema to be wrong `$ref` to the same name.
	// the following code is to fix this issue by adding the package name to the struct name
	// p.s. this issue has been reported in: https://github.com/invopop/jsonschema/issues/42
	r.Namer = func(t reflect.Type) string {
		name := t.Name()
		if t.Kind() == reflect.Struct {
			v := reflect.New(t)
			vt := v.Elem().Type()
			name = vt.PkgPath() + "/" + vt.Name()
			name = strconv.FormatUint(xxhash.Sum64String(name), 10)
		}
		return name
	}

	return r.ReflectFromType(t)
}
