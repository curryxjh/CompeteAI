package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/memory"
	"context"
)

// Deps Agent 运行时依赖。
type Deps struct {
	Chat  ChatClient
	Tools ToolInvoker
}

// ChatClient LLM 调用抽象，便于测试。
type ChatClient interface {
	Chat(ctx context.Context, system, user string) (string, error)
	StreamChat(ctx context.Context, system, user string, onChunk func(eventType, content string)) (string, error)
}

// ToolInvoker Firecrawl 等工具调用抽象。
type ToolInvoker interface {
	Enabled() bool
	Invoke(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// MemoryPromptSuffix 将长期记忆格式化为 prompt 后缀。
func MemoryPromptSuffix(input domain.RunInput) string {
	if ctx, ok := input.Memory.(memory.AgentMemoryContext); ok {
		return memory.FormatForPrompt(ctx)
	}
	return ""
}