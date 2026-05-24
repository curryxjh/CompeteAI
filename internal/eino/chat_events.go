package eino

// StreamEventType SSE 事件类型
type StreamEventType string

const (
	StreamEventThinking   StreamEventType = "thinking"
	StreamEventToolCall   StreamEventType = "tool_call"
	StreamEventToolResult StreamEventType = "tool_result"
	StreamEventContent    StreamEventType = "content"
)

// StreamEvent 聊天流式事件，供前端展示思考过程与工具调用。
type StreamEvent struct {
	Type       StreamEventType `json:"type"`
	Content    string          `json:"content,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolArgs   string          `json:"tool_args,omitempty"`
	ToolResult string          `json:"tool_result,omitempty"`
	Status     string          `json:"status,omitempty"`
}
