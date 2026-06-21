// Package domain 定义所有核心领域模型。
// 本文件包含 Agent 聚合接口及运行时输入/输出结构（原 agent 包提炼）。
package domain

import "context"

// Agent 统一 Agent 接口（§5.1）。
type Agent interface {
	Name() AgentName
	Run(ctx context.Context, input RunInput, bb any) (RunOutput, error)
}

// RunInput 统一运行时输入（§5.2）。
type RunInput struct {
	TaskID      string
	TraceID     string
	Attempt     int
	TriggerType string
	Reason      string
	Payload     map[string]any
	Memory      any // 具体类型为 memory.AgentMemoryContext，用 any 解除 domain→memory 循环依赖
}

// RunOutput 统一运行时输出（§5.3）。
type RunOutput struct {
	Status      AgentRunStatus
	NextAgent   *AgentName
	MessageType string
	Payload     any
	Summary     string
	Artifacts   []ArtifactRef
	NeedsRetry  bool
	Retryable   bool
	Metadata    map[string]any
}