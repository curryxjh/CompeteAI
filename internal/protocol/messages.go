package protocol

import (
	"encoding/json"
	"time"

	"CompeteAI/internal/domain"

	"github.com/google/uuid"
)

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
	Metadata      map[string]any  `json:"metadata,omitempty"`
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
func NewAgentMessage(taskID, traceID string, from domain.AgentName, msgType MessageType, payload any, artifacts []ArtifactRef) MessageEnvelope {
	to := DefaultReceiver(msgType, payload)
	env := NewEnvelope(taskID, traceID, string(from), string(to), msgType, payload)
	env.Status = MessageStatusCompleted
	env.Artifacts = artifacts
	return env
}

// NewTaskCreatedMessage API 创建任务消息。
func NewTaskCreatedMessage(task domain.Task, traceID string) MessageEnvelope {
	payload := TaskCreatedPayload{
		Title:       task.Title,
		Competitors: task.Competitors,
		Dimensions:  task.Dimensions,
	}
	env := NewEnvelope(task.ID, traceID, "api", string(domain.AgentCoordinator), MsgTaskCreated, payload)
	env.Artifacts = []ArtifactRef{NewArtifactRef(ArtifactTaskBrief, "task:"+task.ID+":task:meta", 1)}
	return env
}
