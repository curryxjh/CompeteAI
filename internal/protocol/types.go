package protocol

import "CompeteAI/internal/domain"

// MessageType 固定消息类型（§5）。
type MessageType string

const (
	MsgTaskCreated            MessageType = "task_created"
	MsgClarificationRequired  MessageType = "clarification_required"
	MsgClarificationAnswered  MessageType = "clarification_answered"
	MsgPlanReady              MessageType = "plan_ready"
	MsgMaterialsReady         MessageType = "materials_ready"
	MsgAnalysisReady          MessageType = "analysis_ready"
	MsgReportReady            MessageType = "report_ready"
	MsgQAPass                 MessageType = "qa_pass"
	MsgQAReject               MessageType = "qa_reject"
	MsgQAQuery                MessageType = "qa_query"
	MsgAnalystResponse        MessageType = "analyst_response"
	MsgTaskCompleted          MessageType = "task_completed"
	MsgTaskFailed             MessageType = "task_failed"
)

// 兼容旧常量名
const (
	MessagePlanReady      = string(MsgPlanReady)
	MessageMaterialsReady = string(MsgMaterialsReady)
	MessageAnalysisReady  = string(MsgAnalysisReady)
	MessageReportReady    = string(MsgReportReady)
	MessageQAPass         = string(MsgQAPass)
	MessageQAReject       = string(MsgQAReject)
)

// MessageStatus 消息处理状态（§6）。
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusRunning   MessageStatus = "running"
	MessageStatusCompleted MessageStatus = "completed"
	MessageStatusFailed    MessageStatus = "failed"
	MessageStatusRejected  MessageStatus = "rejected"
)

// 触发类型（工作流引擎内部使用，与 MessageType 对齐）。
const (
	TriggerTaskCreated           = "task_created"
	TriggerQAReject              = "qa_reject"
	TriggerClarificationAnswered = "clarification_answered"
)

// QA 结果枚举
const (
	QAResultPass   = "pass"
	QAResultReject = "reject"
)

// IssueCategory QA 问题分类（§8）。
const (
	IssueSourceMissing       = "source_missing"
	IssueAnalysisIncomplete  = "analysis_incomplete"
	IssuePricingMissing      = "pricing_missing"
	IssueReportStructure     = "report_structure"
	IssueUnsupportedClaim    = "unsupported_claim"
)

// Issue QA 打回结构化问题。
type Issue struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Location   string `json:"location"`
	Problem    string `json:"problem"`
	Suggestion string `json:"suggestion"`
	Severity   string `json:"severity"`
}

// TaskCreatedPayload §7.1
type TaskCreatedPayload struct {
	Title       string   `json:"title"`
	Competitors []string `json:"competitors"`
	Dimensions  []string `json:"dimensions"`
}

// ClarificationPayload §7.2
type ClarificationPayload struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Answer   string   `json:"answer,omitempty"`
}

// PlanPayload §7.3
type PlanPayload struct {
	Summary      string   `json:"summary"`
	Competitors  []string `json:"competitors"`
	Dimensions   []string `json:"dimensions"`
	Deliverables []string `json:"deliverables"`
}

// MaterialsPayload §7.4
type MaterialsPayload struct {
	Query       string   `json:"query"`
	SourceCount int      `json:"sourceCount"`
	SourceIDs   []string `json:"sourceIds"`
	Summary     string   `json:"summary"`
}

// AnalysisPayload §7.5
type AnalysisPayload struct {
	Summary      string   `json:"summary"`
	SWOTKeys     []string `json:"swotKeys"`
	FeatureCount int      `json:"featureCount"`
	PricingCount int      `json:"pricingCount"`
	PersonaCount int      `json:"personaCount"`
}

// ReportPayload §7.6
type ReportPayload struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	SourceCount int    `json:"sourceCount"`
}

// QAResultPayload §7.7
type QAResultPayload struct {
	Score       int     `json:"score"`
	Result      string  `json:"result"`
	Issues      []Issue `json:"issues,omitempty"`
	TargetAgent string  `json:"targetAgent,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

// TaskCompletedPayload 任务完成通知。
type TaskCompletedPayload struct {
	TaskID  string `json:"taskId"`
	Title   string `json:"title,omitempty"`
	QAScore int    `json:"qaScore,omitempty"`
}

// AnalystResponsePayload QA 追问后 Analyst 的局部回复。
type AnalystResponsePayload struct {
	Claim       string `json:"claim"`
	Evidence    string `json:"evidence"`
	Explanation string `json:"explanation"`
}

// QAQueryPayload QA 对单条结论的追问。
type QAQueryPayload struct {
	Claim    string `json:"claim"`
	Question string `json:"question"`
}

// TaskFailedPayload 任务失败通知。
type TaskFailedPayload struct {
	TaskID  string `json:"taskId"`
	Message string `json:"message"`
	Agent   string `json:"agent,omitempty"`
}

// AgentErrorKind 错误分类。
const (
	ErrorKindRetryable             = "retryable"
	ErrorKindFatal                 = "fatal"
	ErrorKindClarificationRequired = "clarification_required"
	ErrorKindRejectedByQA          = "rejected_by_qa"
)

// AgentError 带类别的 Agent 错误。
type AgentError struct {
	Kind      string         `json:"kind"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (e *AgentError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// QAResult Blackboard 持久化的 QA 结果。
type QAResult struct {
	Score       int              `json:"score"`
	Result      string           `json:"result"`
	Issues      []Issue          `json:"issues"`
	TargetAgent domain.AgentName `json:"target_agent,omitempty"`
	Reason      string           `json:"reason,omitempty"`
}

// ToPayload 转为消息协议载荷。
func (q QAResult) ToPayload() QAResultPayload {
	return QAResultPayload{
		Score:       q.Score,
		Result:      q.Result,
		Issues:      q.Issues,
		TargetAgent: string(q.TargetAgent),
		Reason:      q.Reason,
	}
}
