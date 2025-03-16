package yaml

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return new(Encoder)
}

func (e *Encoder) Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func (e *Encoder) Unmarshal(bs []byte, ret any) error {
	data := cleanup(bs)
	return yaml.Unmarshal(data, ret)
}

func (e *Encoder) Validate(req any) error {
	return nil
}

func (e *Encoder) Context() []byte {
	bs, err := e.Marshal(nil)
	if err != nil {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("\nPlease respond with YAML in the following YAML schema:\n")
	b.WriteString("```yaml\n")
	b.Write(bs)
	b.WriteString("\n```")
	b.WriteString("Make sure to return an instance of the YAML, not the schema itself\n")
	return b.Bytes()
}

// cleanup the JSON by trimming prefixes and postfixes
func cleanup(bs []byte) []byte {
	return bytes.TrimSpace(bs)
}
