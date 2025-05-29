package instructor

import (
	"encoding/json"
	"reflect"
  "fmt"
	// "strconv"
	"strings"
	"sync"

	// "github.com/cespare/xxhash/v2"
	"github.com/invopop/jsonschema"
)

var reflectorPool = sync.Pool{
	New: func() any {
		return &jsonschema.Reflector{
      AllowAdditionalProperties: false,
		  DoNotReference:            false,
    }
	},
}

type SchemaNamer func(t reflect.Type) string

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

func NewSchema(t reflect.Type, namer SchemaNamer) (*Schema, error) {
	schema := JSONSchema(t, true, namer)

	str, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}

	// funcs := ToFunctionSchema(t, schema)
  // funcSchema := JSONSchema(t, true, namer)

	s := &Schema{
		Schema: schema,
		String: string(str),

		Functions: []FunctionDefinition{
    {
      Name: "instructor-go-func",
      Description: schema.Description,
      Parameters: schema,
      },
    },
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
  arr := strings.Split(s.Ref, "/")
	return arr[len(arr) - 1] // ex: '#/$defs/MyStruct'
}

// JSONSchema return the json schema of the configuration
func JSONSchema(t reflect.Type, doNotReference bool, namer SchemaNamer) *jsonschema.Schema {
	r := reflectorPool.Get().(*jsonschema.Reflector)
	defer reflectorPool.Put(r)
  r.DoNotReference = doNotReference

	if namer != nil {
		r.Namer = namer
	} else {
		// The Struct name could be same, but the package name is different
		// For example, all of the notification plugins have the same struct name - `NotifyConfig`
		// This would cause the json schema to be wrong `$ref` to the same name.
		// the following code is to fix this issue by adding the package name to the struct name
		// p.s. this issue has been reported in: https://github.com/invopop/jsonschema/issues/42
		r.Namer = func(t reflect.Type) string {
      if t.Kind() == reflect.Pointer {
        t = t.Elem()
      }
			name := t.Name()
			// if t.Kind() == reflect.Struct {
			// 	v := reflect.New(t)
			// 	vt := v.Elem().Type()
			// 	name = vt.PkgPath() + "/" + vt.Name()
			// 	name = strconv.FormatUint(xxhash.Sum64String(name), 10)
			// }
			return name
		}
	}

  ret := r.ReflectFromType(t)
  ret.Ref = fmt.Sprintf("#/$defs/%s", r.Namer(t))
  return ret
}
