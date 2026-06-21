package bus

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/pkg/metrics"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
)

const MaxAttempts = 5

// DLQHandler 超过重试阈值后写入 dead_letters 并投递 DLQ topic。
type DLQHandler struct {
	dao *dao.DeadLetterDao
	bus Bus
}

func NewDLQHandler(d *dao.DeadLetterDao, b Bus) *DLQHandler {
	return &DLQHandler{dao: d, bus: b}
}

func (h *DLQHandler) MoveToDLQ(ctx context.Context, agent domain.AgentName, topic string, env domain.MessageEnvelope, reason string) error {
	metrics.IncMessageDLQ()
	payload, _ := json.Marshal(env)
	_ = h.dao.Create(ctx, dao.DeadLetterEntity{
		TaskID: env.TaskID, MessageID: env.MessageID, Topic: topic,
		Agent: string(agent), Reason: reason, PayloadJSON: string(payload),
		Attempt: env.Attempt,
	})
	return PublishDLQ(ctx, h.bus, agent, env, reason)
}