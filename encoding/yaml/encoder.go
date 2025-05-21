package yaml

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/bububa/instructor-go"
)

type CommentStyle int

const (
	NoComment CommentStyle = iota
	HeadComment
	LineComment
	FootComment
)

var (
	IGNORE_PREFIX = []byte("```yaml")
	IGNORE_SUFFIX = []byte("```")
)

type Encoder struct {
	reqType      reflect.Type
	commentStyle CommentStyle
}

func NewEncoder(req any) *Encoder {
	t := reflect.TypeOf(req)
	return &Encoder{
		reqType:      t,
		commentStyle: NoComment,
	}
}

func (e *Encoder) Marshal(v any) ([]byte, error) {
	if e.commentStyle == NoComment {
		return yaml.Marshal(v)
	}
	node, err := e.structToYAMLWithComments(v)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(node)
}

func (e *Encoder) Unmarshal(bs []byte, ret any) error {
	data := cleanup(bs)
	return yaml.Unmarshal(data, ret)
}

func (e *Encoder) Validate(req any) error {
	validate := validator.New()
	return validate.Struct(req)
}

func (e *Encoder) WithCommentStyle(style CommentStyle) *Encoder {
	e.commentStyle = style
	return e
}

func (e *Encoder) Context() []byte {
	tValue := reflect.New(e.reqType)
	instance := tValue.Interface()
	if f, ok := tValue.Elem().Interface().(instructor.Faker); ok {
		instance = f.Fake()
	} else {
		gofakeit.Struct(instance)
	}
	bs, err := e.Marshal(instance)
	if err != nil {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("\nPlease respond with YAML in the following YAML schema without comments:\n\n")
	b.WriteString("```yaml\n")
	b.Write(bs)
	b.WriteString("```")
	b.WriteString("\nDo not forget quote the field value which contains special characters for YAML.\n")
	b.WriteString("\nMake sure to return an instance of the YAML, not the schema itself\n")
	return b.Bytes()
}

// cleanup the JSON by trimming prefixes and postfixes
func cleanup(bs []byte) []byte {
	// 找到 "```yaml" 的位置
	startIndex := bytes.Index(bs, IGNORE_PREFIX)
	if startIndex == -1 {
		// 如果没有找到起始标记，直接返回原始字符串
		return bs
	}

	// 计算移除起始标记及其之前的内容后的字符串
	contentAfterStart := bs[startIndex+len(IGNORE_PREFIX):]

	// 找到最后一个 "```" 的位置
	endIndex := bytes.LastIndex(contentAfterStart, IGNORE_SUFFIX)
	if endIndex == -1 {
		// 如果没有找到结束标记，直接返回从起始标记之后的内容
		return contentAfterStart
	}

	// 截取中间的有效内容
	result := contentAfterStart[:endIndex]

	return bytes.TrimSpace(result)
}

// 解析 struct 并转换为带注释的 YAML Node
func (e *Encoder) structToYAMLWithComments(v any) (*yaml.Node, error) {
	val := reflect.ValueOf(v)

	val = dereference(val)
	if !val.IsValid() {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: "null", Tag: "!!null"}, nil
	}

	typ := val.Type()

	// 确保是 struct
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}

	// 创建 YAML 根节点
	root := &yaml.Node{Kind: yaml.MappingNode}

	// 遍历结构体字段
	for i := range val.NumField() {
		field := typ.Field(i)

		// 获取 yaml key
		yamlKey := field.Tag.Get("yaml")
		if yamlKey == "" || yamlKey == "-" {
			continue // 跳过未导出的字段
		}

		// 获取注释
		comment := field.Tag.Get("comment")
		if comment == "" {
			comment = extractDescription(field.Tag.Get("jsonschema"))
		}

		// 添加 key
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: yamlKey}
		if comment != "" {
			switch e.commentStyle {
			case HeadComment:
				keyNode.HeadComment = comment
			case LineComment:
				keyNode.LineComment = comment
			case FootComment:
				keyNode.FootComment = comment
			}
		}

		// 添加 value
		valueNode := e.getValueNode(val.Field(i))

		// 组装 YAML 结构
		root.Content = append(root.Content, keyNode, valueNode)
	}

	return root, nil
}

// 递归解析值，支持指针和 interface{}
func (e *Encoder) getValueNode(v reflect.Value) *yaml.Node {
	// 处理指针
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return &yaml.Node{Kind: yaml.ScalarNode, Value: "null", Tag: "!!null"}
		}
		v = v.Elem()
	}

	// 处理 interface{}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return &yaml.Node{Kind: yaml.ScalarNode, Value: "null", Tag: "!!null"}
		}
		v = reflect.ValueOf(v.Interface()) // 获取真实值
	}

	var node *yaml.Node
	// 处理基本类型
	switch v.Kind() {
	case reflect.String:
		node = &yaml.Node{Kind: yaml.ScalarNode, Value: v.String()}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		node = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", v.Int()), Tag: "!!int"}
	case reflect.Float32, reflect.Float64:
		node = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%f", v.Float()), Tag: "!!float"}
	case reflect.Bool:
		node = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%t", v.Bool()), Tag: "!!bool"}
	case reflect.Map:
		node = e.mapToYAMLNode(v)
	case reflect.Struct:
		node, _ = e.structToYAMLWithComments(v.Interface()) // 递归解析 struct
	case reflect.Slice, reflect.Array:
		node = e.sliceToYAMLNode(v)
	default:
		node = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v.Interface())}
	}
	return node
}

// 处理 map 类型
func (e *Encoder) mapToYAMLNode(v reflect.Value) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, key := range v.MapKeys() {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", key.Interface())}
		valueNode := e.getValueNode(v.MapIndex(key))
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node
}

// 处理 slice/array 类型
func (e *Encoder) sliceToYAMLNode(v reflect.Value) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for i := range v.Len() {
		node.Content = append(node.Content, e.getValueNode(v.Index(i)))
	}
	return node
}

// 从 jsonschema 解析 description
func extractDescription(tag string) string {
	re := regexp.MustCompile(`description=([^,]+)`)
	matches := re.FindStringSubmatch(tag)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// 递归解引用指针，直到 `v` 不是指针类型
func dereference(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{} // 直接返回一个空值，防止 nil pointer dereference
		}
		v = v.Elem()
	}
	return v
}
