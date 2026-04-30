package llm

type StreamEventType string

const (
	StreamContent          StreamEventType = "content"
	StreamToolCall         StreamEventType = "tool_call"
	StreamToolResult       StreamEventType = "tool_result"
	StreamDone             StreamEventType = "done"
	StreamError            StreamEventType = "error"
	StreamApprovalRequired StreamEventType = "approval_required"
)

type StreamEvent struct {
	Type       StreamEventType
	Content    string
	ToolName   string
	ToolArgs   map[string]interface{}
	ToolResult string
	ToolCall   *ToolCallState
	Error      error
}
