package domain

// 与 frontend/src/types/index.ts 对齐的 DTO

type TaskStatus string

const (
	TaskStatusPending           TaskStatus = "pending"
	TaskStatusQueued            TaskStatus = "queued"
	TaskStatusRunning           TaskStatus = "running"
	TaskStatusClarifying        TaskStatus = "clarifying"
	TaskStatusReworking         TaskStatus = "reworking"
	TaskStatusWaitingReply      TaskStatus = "waiting_reply"
	TaskStatusCompleted         TaskStatus = "completed"
	TaskStatusFailed            TaskStatus = "failed"
	TaskStatusCancelled         TaskStatus = "cancelled"
	TaskStatusAttentionRequired TaskStatus = "attention_required"
)

type AgentName string

const (
	AgentCoordinator AgentName = "coordinator"
	AgentCollector   AgentName = "collector"
	AgentAnalyst     AgentName = "analyst"
	AgentWriter      AgentName = "writer"
	AgentQA          AgentName = "qa"
)

type AgentRunStatus string

const (
	AgentRunPending   AgentRunStatus = "pending"
	AgentRunLeased    AgentRunStatus = "leased"
	AgentRunRunning   AgentRunStatus = "running"
	AgentRunCompleted AgentRunStatus = "completed"
	AgentRunFailed    AgentRunStatus = "failed"
	AgentRunRejected  AgentRunStatus = "rejected"
	AgentRunRetrying  AgentRunStatus = "retrying"
	AgentRunSkipped   AgentRunStatus = "skipped"
	AgentRunTimedOut  AgentRunStatus = "timed_out"
)

type AgentState struct {
	Name       AgentName      `json:"name"`
	Status     AgentRunStatus `json:"status"`
	Progress   *int           `json:"progress,omitempty"`
	Message    string         `json:"message,omitempty"`
	StartedAt  string         `json:"startedAt,omitempty"`
	FinishedAt string         `json:"finishedAt,omitempty"`
}

type Task struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Competitors  []string     `json:"competitors"`
	Dimensions   []string     `json:"dimensions"`
	Status       TaskStatus   `json:"status"`
	Progress     int          `json:"progress"`
	AgentStates  []AgentState `json:"agentStates"`
	CreatedAt    string       `json:"createdAt"`
	UpdatedAt    string       `json:"updatedAt,omitempty"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
	// UserID 创建任务的用户 ID，用于记忆作用域关联
	UserID int64 `json:"userId,omitempty"`
}

type CreateTaskPayload struct {
	Competitors []string `json:"competitors" binding:"required,min=1"`
	Dimensions  []string `json:"dimensions" binding:"required,min=1"`
	Title       string   `json:"title"`
}

type ClarifyPayload struct {
	Answer string `json:"answer" binding:"required"`
}

type SWOTItem struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"sourceIds,omitempty"`
}

type SWOTAnalysis struct {
	Strengths     []SWOTItem `json:"strengths"`
	Weaknesses    []SWOTItem `json:"weaknesses"`
	Opportunities []SWOTItem `json:"opportunities"`
	Threats       []SWOTItem `json:"threats"`
}

type FeatureRow struct {
	Feature   string                       `json:"feature"`
	Values    map[string]interface{}       `json:"values"`
	SourceIDs map[string][]string            `json:"sourceIds,omitempty"`
}

// FeatureTreeNode 竞品功能树节点（层级结构）。
type FeatureTreeNode struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Supported   *bool             `json:"supported,omitempty"`
	SourceIDs   []string          `json:"sourceIds,omitempty"`
	Children    []FeatureTreeNode `json:"children,omitempty"`
}

type PricingTier struct {
	Name     string   `json:"name"`
	Price    string   `json:"price"`
	Features []string `json:"features"`
}

type PricingInfo struct {
	Competitor string        `json:"competitor"`
	Tiers      []PricingTier `json:"tiers"`
	SourceIDs  []string      `json:"sourceIds,omitempty"`
}

type UserPersona struct {
	Competitor  string   `json:"competitor"`
	Segments    []string `json:"segments"`
	PainPoints  []string `json:"painPoints"`
	UseCases    []string `json:"useCases"`
	SourceIDs   []string `json:"sourceIds,omitempty"`
}

type SourceRef struct {
	URL         string `json:"url"`
	Excerpt     string `json:"excerpt"`
	CollectedAt string `json:"collectedAt"`
	Title       string `json:"title,omitempty"`
}

type Report struct {
	TaskID      string                       `json:"taskId"`
	Title       string                       `json:"title"`
	GeneratedAt string                       `json:"generatedAt"`
	QAScore     int                          `json:"qaScore"`
	Summary     string                       `json:"summary"`
	SWOT        map[string]SWOTAnalysis      `json:"swot"`
	Features    []FeatureRow                 `json:"features"`
	FeatureTree map[string][]FeatureTreeNode `json:"featureTree,omitempty"`
	Pricing     []PricingInfo                `json:"pricing"`
	Personas    []UserPersona                `json:"personas"`
	Sources     map[string]SourceRef         `json:"sources"`
}

// TraceStep Agent 运行内的细粒度步骤（thinking / tool / note 等）。
type TraceStep struct {
	Kind     string `json:"kind"`
	Content  string `json:"content"`
	Status   string `json:"status,omitempty"`
	ToolName string `json:"toolName,omitempty"`
}

type TraceNode struct {
	ID          string                 `json:"id"`
	Agent       AgentName              `json:"agent"`
	Label       string                 `json:"label"`
	Status      AgentRunStatus         `json:"status"`
	DurationMs  int                    `json:"durationMs"`
	TokenCount  int                    `json:"tokenCount"`
	Input       string                 `json:"input,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	IsRetry     bool                   `json:"isRetry,omitempty"`
	IsRejection bool                   `json:"isRejection,omitempty"`
	ParentID    string                 `json:"parentId,omitempty"`
	Steps       []TraceStep            `json:"steps,omitempty"`
}

type Trace struct {
	TaskID string      `json:"taskId"`
	Nodes  []TraceNode `json:"nodes"`
}

func DefaultAgentStates() []AgentState {
	return []AgentState{
		{Name: AgentCoordinator, Status: AgentRunPending},
		{Name: AgentCollector, Status: AgentRunPending},
		{Name: AgentAnalyst, Status: AgentRunPending},
		{Name: AgentWriter, Status: AgentRunPending},
		{Name: AgentQA, Status: AgentRunPending},
	}
}
