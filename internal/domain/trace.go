// Package domain 定义所有核心领域模型。
// 本文件包含追踪聚合根及其值对象，用于记录 Agent 执行过程的可观测性数据。
package domain

// ──────────────────────────────────────────────────────
// Trace 值对象
// ──────────────────────────────────────────────────────

// TraceStep Agent 运行内的细粒度步骤，记录 thinking / tool_call / note 等子操作。
// 用于前端 Trace 回放页面展示 Agent 的内部推理过程。
type TraceStep struct {
	Kind     string `json:"kind"`                // 步骤类型：thinking, tool_call, tool_result, note
	Content  string `json:"content"`              // 步骤内容（thinking 的推理文本、tool_call 的参数等）
	Status   string `json:"status,omitempty"`      // 步骤状态：running, completed, failed
	ToolName string `json:"toolName,omitempty"`   // 工具名称（仅 tool_call 类型）
}

// TraceNode 单个 Agent 执行节点，记录一次 Agent 运行的起止、耗时、Token 消耗等信息。
// 多个 TraceNode 组成一次任务执行的完整追踪链。
type TraceNode struct {
	ID          string                 `json:"id"`                    // 节点唯一标识（UUID）
	Agent       AgentName              `json:"agent"`                 // 执行的 Agent 名称
	Label       string                 `json:"label"`                 // 节点标签（如"Collector 执行"、"QA 评分"）
	Status      AgentRunStatus         `json:"status"`                // Agent 运行状态
	DurationMs  int                    `json:"durationMs"`             // 执行耗时（毫秒）
	TokenCount  int                    `json:"tokenCount"`             // LLM Token 消耗估算
	Input       string                 `json:"input,omitempty"`       // Agent 输入摘要
	Output      string                 `json:"output,omitempty"`      // Agent 输出摘要
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // 扩展元数据
	IsRetry     bool                   `json:"isRetry,omitempty"`     // 是否为重试执行
	IsRejection bool                   `json:"isRejection,omitempty"`  // 是否为 QA 打回后的重做
	ParentID    string                 `json:"parentId,omitempty"`    // 父节点 ID（QA 打回时关联原始节点）
	Steps       []TraceStep            `json:"steps,omitempty"`        // 细粒度步骤列表
}

// Trace 追踪聚合根，记录一次任务中所有 Agent 执行节点的完整链路。
// 前端通过 SSE 实时追踪，或通过 GET /api/traces/:taskId 回放。
type Trace struct {
	TaskID string      `json:"taskId"` // 关联的任务 ID
	Nodes  []TraceNode `json:"nodes"`  // Agent 执行节点列表，按执行顺序排列
}