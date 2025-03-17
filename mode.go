package instructor

type Mode = string

const (
	ModeToolCall       Mode = "tool_call_mode"
	ModeToolCallStrict Mode = "tool_call_strict_mode"
	ModeJSON           Mode = "json_mode"
	ModeJSONStrict     Mode = "json_strict_mode"
	ModeJSONSchema     Mode = "json_schema_mode"
	ModeYAML           Mode = "yaml_mode"
	ModeTOML           Mode = "toml_mode"
	ModePlainText      Mode = "plain_text_mode"
	ModeCustom         Mode = "custom_mode"
	ModeDefault        Mode = ModeJSONSchema
)
