package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
)

// Agent 统一 Agent 接口（§5.1）。
type Agent interface {
	Name() domain.AgentName
	Card() AgentCard
	Run(ctx context.Context, input RunInput, bb state.Blackboard) (RunOutput, error)
}

// RunInput 统一运行时输入（§5.2）。
type RunInput struct {
	TaskID      string
	TraceID     string
	Attempt     int
	TriggerType string
	Reason      string
	Payload     map[string]any
	Memory      memory.AgentMemoryContext
}

// RunOutput 统一运行时输出（§5.3）。
type RunOutput struct {
	Status      domain.AgentRunStatus
	NextAgent   *domain.AgentName
	MessageType string
	Payload     any
	Summary     string
	Artifacts   []protocol.ArtifactRef
	NeedsRetry  bool
	Retryable   bool
	Metadata    map[string]any
}

// Deps Agent 运行时依赖。
type Deps struct {
	Chat  ChatClient
	Tools ToolInvoker
}

// ChatClient LLM 调用抽象，便于测试。
type ChatClient interface {
	Chat(ctx context.Context, system, user string) (string, error)
	// StreamChat 流式调用；onChunk 的 eventType 为 thinking 或 content。
	StreamChat(ctx context.Context, system, user string, onChunk func(eventType, content string)) (string, error)
}

// ToolInvoker Firecrawl 等工具调用抽象。
type ToolInvoker interface {
	Enabled() bool
	Invoke(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// MemoryPromptSuffix 将长期记忆格式化为 prompt 后缀。
func MemoryPromptSuffix(input RunInput) string {
	return memory.FormatForPrompt(input.Memory)
}
