package instructor

type ToolCall struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}
