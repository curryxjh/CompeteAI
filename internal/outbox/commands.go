package outbox

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
)

// EnqueueClarify 澄清命令入 outbox。
func (p *Publisher) EnqueueClarify(ctx context.Context, taskID, traceID, answer string) error {
	msg := domain.NewEnvelope(
		taskID, traceID, "user", string(domain.AgentCoordinator),
		domain.MsgClarificationAnswered,
		domain.ClarificationPayload{Answer: answer},
	).WithStatus(domain.MessageStatusCompleted)
	raw, _ := json.Marshal(msg)
	return p.dao.Enqueue(ctx, dao.OutboxMessageEntity{
		TaskID: taskID, MessageID: msg.MessageID,
		Topic: bus.TopicTaskClarify, PayloadJSON: string(raw), Status: "pending",
	})
}

// EnqueueCancel 取消命令入 outbox。
func (p *Publisher) EnqueueCancel(ctx context.Context, taskID, traceID string) error {
	msg := domain.NewEnvelope(
		taskID, traceID, "user", "system",
		domain.MsgTaskFailed,
		domain.TaskFailedPayload{TaskID: taskID, Message: "cancelled by user"},
	).WithStatus(domain.MessageStatusFailed)
	raw, _ := json.Marshal(msg)
	return p.dao.Enqueue(ctx, dao.OutboxMessageEntity{
		TaskID: taskID, MessageID: msg.MessageID,
		Topic: bus.TopicTaskCancel, PayloadJSON: string(raw), Status: "pending",
	})
}

// ReplayEnvelope 死信重放入 outbox。
func (p *Publisher) ReplayEnvelope(ctx context.Context, topic string, env domain.MessageEnvelope) error {
	raw, _ := json.Marshal(env)
	return p.dao.Enqueue(ctx, dao.OutboxMessageEntity{
		TaskID: env.TaskID, MessageID: env.MessageID + "-replay",
		Topic: bus.NormalizeTopic(topic), PayloadJSON: string(raw), Status: "pending",
	})
}
