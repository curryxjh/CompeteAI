package dlq

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/metrics"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
)

const MaxAttempts = 5

// Handler 超过重试阈值后写入 dead_letters 并投递 DLQ topic。
type Handler struct {
	dao *dao.DeadLetterDao
	bus bus.Bus
}

func NewHandler(d *dao.DeadLetterDao, b bus.Bus) *Handler {
	return &Handler{dao: d, bus: b}
}

func (h *Handler) MoveToDLQ(ctx context.Context, agent domain.AgentName, topic string, env protocol.MessageEnvelope, reason string) error {
	metrics.IncMessageDLQ()
	payload, _ := json.Marshal(env)
	_ = h.dao.Create(ctx, dao.DeadLetterEntity{
		TaskID: env.TaskID, MessageID: env.MessageID, Topic: topic,
		Agent: string(agent), Reason: reason, PayloadJSON: string(payload),
		Attempt: env.Attempt,
	})
	return bus.PublishDLQ(ctx, h.bus, agent, env, reason)
}
