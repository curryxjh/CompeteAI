// Package domain 定义所有核心领域模型。
// 本文件包含消息协议常量、信封结构、工件引用及路由规则（原 protocol 包合并）。
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────
// 消息类型（§5）
// ──────────────────────────────────────────────────────

// MessageType 固定消息类型（§5）。
type MessageType string

const (
	MsgTaskCreated           MessageType = "task_created"
	MsgClarificationRequired MessageType = "clarification_required"
	MsgClarificationAnswered MessageType = "clarification_answered"
	MsgPlanReady             MessageType = "plan_ready"
	MsgMaterialsReady        MessageType = "materials_ready"
	MsgAnalysisReady         MessageType = "analysis_ready"
	MsgReportReady           MessageType = "report_ready"
	MsgQAPass                MessageType = "qa_pass"
	MsgQAReject              MessageType = "qa_reject"
	MsgQAQuery               MessageType = "qa_query"
	MsgAnalystResponse       MessageType = "analyst_response"
	MsgTaskCompleted         MessageType = "task_completed"
	MsgTaskFailed            MessageType = "task_failed"
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
	IssueUnsupportedClaim   = "unsupported_claim"
	IssueMissingSourceRef   = "missing_source_ref"
)

// ──────────────────────────────────────────────────────
// 消息载荷结构（§7）
// ──────────────────────────────────────────────────────

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
	Score       int        `json:"score"`
	Result      string     `json:"result"`
	Issues      []Issue    `json:"issues"`
	TargetAgent AgentName  `json:"target_agent,omitempty"`
	Reason      string     `json:"reason,omitempty"`
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

// ──────────────────────────────────────────────────────
// 消息信封（§4 + 企业蓝图 §5.3）
// ──────────────────────────────────────────────────────

// MessageEnvelope 统一消息信封（§4 + 企业蓝图 §5.3）。
type MessageEnvelope struct {
	MessageID     string          `json:"message_id"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	TaskID        string          `json:"task_id"`
	TraceID       string          `json:"trace_id"`
	FromAgent     string          `json:"from_agent"`
	ToAgent       string          `json:"to_agent"`
	MessageType   MessageType     `json:"message_type"`
	Kind          string          `json:"kind,omitempty"` // command | event | query | reply
	Name          string          `json:"name,omitempty"`
	Status        MessageStatus   `json:"status"`
	Attempt       int             `json:"attempt,omitempty"`
	DedupKey      string          `json:"dedup_key,omitempty"`
	SchemaVersion int             `json:"schema_version,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Artifacts     []ArtifactRef   `json:"artifacts,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

// NewEnvelope 构造消息信封。
func NewEnvelope(taskID, traceID, from, to string, msgType MessageType, payload any) MessageEnvelope {
	var raw json.RawMessage
	if payload != nil {
		raw, _ = json.Marshal(payload)
	}
	return MessageEnvelope{
		MessageID:     uuid.NewString(),
		CorrelationID: traceID,
		DedupKey:      uuid.NewString(),
		TaskID:        taskID,
		TraceID:       traceID,
		FromAgent:     from,
		ToAgent:       to,
		MessageType:   msgType,
		Kind:          "command",
		Name:          string(msgType),
		Status:        MessageStatusPending,
		SchemaVersion: 1,
		Payload:       raw,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
}

// WithStatus 返回带状态副本。
func (e MessageEnvelope) WithStatus(status MessageStatus) MessageEnvelope {
	e.Status = status
	return e
}

// WithArtifacts 返回带工件引用副本。
func (e MessageEnvelope) WithArtifacts(refs ...ArtifactRef) MessageEnvelope {
	e.Artifacts = refs
	return e
}

// DecodePayload 反序列化载荷。
func (e MessageEnvelope) DecodePayload(dest any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, dest)
}

// NewAgentMessage 由 Agent 输出构造已完成的消息信封。
func NewAgentMessage(taskID, traceID string, from AgentName, msgType MessageType, payload any, artifacts []ArtifactRef) MessageEnvelope {
	to := DefaultReceiver(msgType, payload)
	env := NewEnvelope(taskID, traceID, string(from), string(to), msgType, payload)
	env.Status = MessageStatusCompleted
	env.Artifacts = artifacts
	return env
}

// NewTaskCreatedMessage API 创建任务消息。
func NewTaskCreatedMessage(task Task, traceID string) MessageEnvelope {
	payload := TaskCreatedPayload{
		Title:       task.Title,
		Competitors: task.Competitors,
		Dimensions:  task.Dimensions,
	}
	env := NewEnvelope(task.ID, traceID, "api", string(AgentCoordinator), MsgTaskCreated, payload)
	env.Artifacts = []ArtifactRef{NewArtifactRef(ArtifactTaskBrief, "task:"+task.ID+":task:meta", 1)}
	return env
}

// ──────────────────────────────────────────────────────
// 工件引用（§9）
// ──────────────────────────────────────────────────────

// 工件类型（§9）。
const (
	ArtifactTaskBrief         = "task_brief"
	ArtifactPlan              = "plan"
	ArtifactSourceRef         = "source_ref"
	ArtifactCollectedMaterial = "collected_material"
	ArtifactAnalysisResult   = "analysis_result"
	ArtifactReportDraft      = "report_draft"
	ArtifactReportFinal      = "report_final"
	ArtifactQAReview         = "qa_review"
)

// ArtifactRef 工件引用，大数据走 Blackboard。
type ArtifactRef struct {
	Type      string `json:"type"`
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

// NewArtifactRef 构造工件引用。
func NewArtifactRef(artifactType, key string, version int) ArtifactRef {
	return ArtifactRef{
		Type:      artifactType,
		Key:       key,
		Version:   version,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

// ──────────────────────────────────────────────────────
// 路由决策（§11）
// ──────────────────────────────────────────────────────

// RouteDecision 路由决策（§11）。
type RouteDecision struct {
	ToAgent AgentName
	Topic   string // Kafka topic 提示，本地执行器可忽略
}

// DefaultReceiver 根据消息类型返回默认接收方。
func DefaultReceiver(msgType MessageType, payload any) AgentName {
	switch msgType {
	case MsgTaskCreated, MsgClarificationAnswered, MsgQAReject:
		return AgentCoordinator
	case MsgClarificationRequired:
		return AgentName("user")
	case MsgPlanReady:
		return AgentCollector
	case MsgMaterialsReady:
		return AgentAnalyst
	case MsgAnalysisReady:
		return AgentWriter
	case MsgReportReady:
		return AgentQA
	case MsgQAPass, MsgTaskCompleted, MsgTaskFailed:
		return AgentName("api")
	default:
		return ""
	}
}

// RouteNext 根据输入消息类型决定下一跳（§11 路由规则表）。
func RouteNext(msgType MessageType, payload any) RouteDecision {
	dec := RouteDecision{ToAgent: DefaultReceiver(msgType, payload)}

	switch msgType {
	case MsgTaskCreated:
		dec.Topic = "coordinator.input"
	case MsgPlanReady:
		dec.Topic = "collector.input"
	case MsgMaterialsReady:
		dec.Topic = "analyst.input"
	case MsgAnalysisReady:
		dec.Topic = "writer.input"
	case MsgReportReady:
		dec.Topic = "qa.input"
	case MsgQAPass:
		dec.Topic = "report.final"
	case MsgQAReject:
		dec.ToAgent = ReworkTarget(payload)
		dec.Topic = "coordinator.input"
	case MsgClarificationRequired:
		dec.Topic = "user.clarify"
	case MsgTaskCompleted:
		dec.Topic = "report.final"
	case MsgTaskFailed:
		dec.Topic = "task.failed"
	}
	return dec
}

// ReworkTarget 从 QA 打回载荷解析重做目标 Agent。
func ReworkTarget(payload any) AgentName {
	switch p := payload.(type) {
	case QAResultPayload:
		if p.TargetAgent != "" {
			return AgentName(p.TargetAgent)
		}
	case *QAResultPayload:
		if p != nil && p.TargetAgent != "" {
			return AgentName(p.TargetAgent)
		}
	case QAResult:
		if p.TargetAgent != "" {
			return p.TargetAgent
		}
	}
	return AgentAnalyst
}

// TraceAction 消息对应的 Trace 动作（§13）。
func TraceAction(msgType MessageType, phase string) string {
	switch phase {
	case "created":
		return "message.created"
	case "started":
		return "agent.started"
	case "completed":
		return "agent.completed"
	case "failed":
		return "agent.failed"
	}
	if msgType == MsgQAReject {
		return "qa.rejected"
	}
	if msgType == MsgTaskCompleted {
		return "task.completed"
	}
	return "message." + string(msgType)
}