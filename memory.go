package instructor

type Memory struct {
	list []Message
}

func NewMemory(cap int) *Memory {
	if cap > 0 {
		return &Memory{
			list: make([]Message, 0, cap),
		}
	}
	return new(Memory)
}

func (m *Memory) Set(list []Message) {
	m.list = make([]Message, len(list))
	copy(m.list, list)
}

func (m *Memory) Add(v ...Message) {
	m.list = append(m.list, v...)
}

func (m *Memory) List() []Message {
	return m.list
}

type Role string

const (
	SystemRole    Role = "system"
	UserRole      Role = "user"
	AssistantRole Role = "assistant"
	ToolRole      Role = "tool"
)

type Message struct {
	Role        Role         `json:"role,omitempty"`
	Text        string       `json:"text,omitempty"`
	Images      []Image      `json:"images,omitempty"`
	Audios      []Audio      `json:"audios,omitempty"`
	Files       []File       `json:"files,omitempty"`
	ToolUses    []ToolUse    `json:"tool_uses,omitempty"`
	ToolResults []ToolResult `json:"tool_result,omitempty"`
	ResponseID  string       `json:"response_id,omitempty"`
}

type Image struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type Audio struct {
	ID     string `json:"id,omitempty"`
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
}

type File struct {
	ID   string `json:"id,omitempty"`
	Data string `json:"data,omitempty"`
	Name string `json:"name,omitempty"`
}

type ToolUse struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolResult struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}
