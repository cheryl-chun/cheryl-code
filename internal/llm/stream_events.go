package llm

type StreamEventType string

const (
	StreamContent    StreamEventType = "content"
	StreamToolCall   StreamEventType = "tool_call"
	StreamToolResult StreamEventType = "tool_result"
	StreamDone       StreamEventType = "done"
	StreamError      StreamEventType = "error"
)

type StreamEvent struct {
	Type       StreamEventType
	Content    string
	ToolName   string
	ToolArgs   map[string]interface{}
	ToolResult string
	Error      error
}
