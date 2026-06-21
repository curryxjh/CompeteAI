// Package domain 定义所有核心领域模型。
// 本文件包含任务聚合根及其值对象、枚举常量。
package domain

// ──────────────────────────────────────────────────────
// 任务状态枚举
// ──────────────────────────────────────────────────────

// TaskStatus 任务生命周期状态，对应工作流状态机的合法节点。
// 合法转换由 workflow/state.go 的 taskTransitions 定义。
type TaskStatus string

const (
	TaskStatusPending           TaskStatus = "pending"            // 初始状态，任务刚创建
	TaskStatusQueued            TaskStatus = "queued"             // 已入队，等待 Worker 消费
	TaskStatusRunning           TaskStatus = "running"            // 执行中，某个 Agent 正在运行
	TaskStatusClarifying        TaskStatus = "clarifying"         // 等待用户澄清回答
	TaskStatusReworking         TaskStatus = "reworking"          // QA 打回后重做中
	TaskStatusWaitingReply      TaskStatus = "waiting_reply"      // 等待外部回复（预留）
	TaskStatusCompleted         TaskStatus = "completed"          // 已完成，报告已生成
	TaskStatusFailed            TaskStatus = "failed"             // 执行失败（超过重试上限等）
	TaskStatusCancelled         TaskStatus = "cancelled"           // 用户主动取消
	TaskStatusAttentionRequired TaskStatus = "attention_required"  // 需要人工介入（DLQ 超限等）
)

// ──────────────────────────────────────────────────────
// Agent 枚举
// ──────────────────────────────────────────────────────

// AgentName 五大专职 Agent 标识，对应流水线固定顺序：
// Coordinator → Collector → Analyst → Writer → QA。
type AgentName string

const (
	AgentCoordinator AgentName = "coordinator" // 任务分解、澄清对话、工作流规划
	AgentCollector   AgentName = "collector"   // Firecrawl 搜索+抓取、素材组装
	AgentAnalyst     AgentName = "analyst"     // SWOT / 功能矩阵 / 定价 / 画像分析
	AgentWriter      AgentName = "writer"      // LLM 报告生成、源引用嵌入
	AgentQA          AgentName = "qa"           // 质量打分、源覆盖检查、证据验证
)

// ──────────────────────────────────────────────────────
// Agent 运行状态枚举
// ──────────────────────────────────────────────────────

// AgentRunStatus 单个 Agent 在一次任务中的运行状态。
type AgentRunStatus string

const (
	AgentRunPending   AgentRunStatus = "pending"   // 等待执行
	AgentRunLeased    AgentRunStatus = "leased"    // 已获取租约，即将执行
	AgentRunRunning   AgentRunStatus = "running"   // 执行中
	AgentRunCompleted AgentRunStatus = "completed" // 执行成功
	AgentRunFailed    AgentRunStatus = "failed"    // 执行失败
	AgentRunRejected  AgentRunStatus = "rejected"  // QA 打回
	AgentRunRetrying  AgentRunStatus = "retrying"  // 重试中
	AgentRunSkipped   AgentRunStatus = "skipped"   // 跳过（重做时不需要重新执行的 Agent）
	AgentRunTimedOut  AgentRunStatus = "timed_out" // 执行超时
)

// ──────────────────────────────────────────────────────
// 聚合根 & 值对象
// ──────────────────────────────────────────────────────

// AgentState 单个 Agent 在任务中的运行状态快照，嵌入 Task.AgentStates。
type AgentState struct {
	Name       AgentName      `json:"name"`                // Agent 名称
	Status     AgentRunStatus `json:"status"`               // 运行状态
	Progress   *int           `json:"progress,omitempty"`     // 进度百分比（0-100）
	Message    string         `json:"message,omitempty"`     // 状态描述或错误信息
	StartedAt  string         `json:"startedAt,omitempty"`   // 开始时间（ISO 3339）
	FinishedAt string         `json:"finishedAt,omitempty"`  // 结束时间（ISO 3339）
}

// Task 竞品分析任务聚合根。
// 代表用户提交的一次完整的竞品分析请求，从创建到完成/失败的全生命周期。
type Task struct {
	ID           string       `json:"id"`                     // 任务唯一标识（UUID）
	Title        string       `json:"title"`                  // 任务标题（用户输入或 Coordinator 生成）
	Competitors  []string     `json:"competitors"`             // 待分析竞品列表
	Dimensions   []string     `json:"dimensions"`              // 分析维度（如"功能"、"定价"）
	Status       TaskStatus   `json:"status"`                  // 当前任务状态
	Progress     int          `json:"progress"`                 // 整体进度百分比（0-100）
	AgentStates  []AgentState `json:"agentStates"`             // 各 Agent 运行状态快照
	CreatedAt    string       `json:"createdAt"`               // 创建时间
	UpdatedAt    string       `json:"updatedAt,omitempty"`     // 最后更新时间
	ErrorMessage string       `json:"errorMessage,omitempty"`  // 失败时的错误信息
	UserID       int64        `json:"userId,omitempty"`        // 创建者用户 ID，用于记忆作用域关联
}

// CreateTaskPayload 创建任务的请求载荷。
type CreateTaskPayload struct {
	Competitors []string `json:"competitors" binding:"required,min=1"` // 竞品列表，至少 1 个
	Dimensions  []string `json:"dimensions" binding:"required,min=1"`  // 分析维度，至少 1 个
	Title       string   `json:"title"`                                  // 任务标题（可选）
}

// ClarifyPayload 用户回答澄清问题的请求载荷。
type ClarifyPayload struct {
	Answer string `json:"answer" binding:"required"` // 用户对澄清问题的回答
}

// DefaultAgentStates 返回流水线 5 个 Agent 的初始状态（全部 pending），
// 用于创建新任务时初始化 Task.AgentStates。
func DefaultAgentStates() []AgentState {
	return []AgentState{
		{Name: AgentCoordinator, Status: AgentRunPending},
		{Name: AgentCollector, Status: AgentRunPending},
		{Name: AgentAnalyst, Status: AgentRunPending},
		{Name: AgentWriter, Status: AgentRunPending},
		{Name: AgentQA, Status: AgentRunPending},
	}
}