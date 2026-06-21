package outbox

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
	"log"
	"time"
)

const defaultPollInterval = 2 * time.Second

// Publisher 可靠投递：DB outbox → Bus（蓝图 §7.3）。
type Publisher struct {
	dao *dao.OutboxDao
	bus bus.Bus
}

func NewPublisher(dao *dao.OutboxDao, b bus.Bus) *Publisher {
	return &Publisher{dao: dao, bus: b}
}

// EnqueueTaskCreated 仅写 outbox，不直接 Publish。
func (p *Publisher) EnqueueTaskCreated(ctx context.Context, task domain.Task, traceID string) error {
	msg := domain.NewTaskCreatedMessage(task, traceID).WithStatus(domain.MessageStatusPending)
	raw, _ := json.Marshal(msg)
	return p.dao.Enqueue(ctx, dao.OutboxMessageEntity{
		TaskID: task.ID, MessageID: msg.MessageID,
		Topic: bus.TopicTaskCreate, PayloadJSON: string(raw),
	})
}

// EnqueueRaw 通用 outbox 入队。
func (p *Publisher) EnqueueRaw(ctx context.Context, topic string, env domain.MessageEnvelope) error {
	raw, _ := json.Marshal(env)
	return p.dao.Enqueue(ctx, dao.OutboxMessageEntity{
		TaskID: env.TaskID, MessageID: env.MessageID,
		Topic: bus.NormalizeTopic(topic), PayloadJSON: string(raw),
	})
}

// Run 轮询 pending 并发布到总线。
func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.flush(ctx)
		}
	}
}

func (p *Publisher) flush(ctx context.Context) {
	rows, err := p.dao.ListPending(ctx, 50)
	if err != nil {
		return
	}
	for _, row := range rows {
		var env domain.MessageEnvelope
		if json.Unmarshal([]byte(row.PayloadJSON), &env) != nil {
			continue
		}
		if err := p.bus.Publish(ctx, row.Topic, env); err != nil {
			log.Printf("[outbox] publish %s failed: %v", row.MessageID, err)
			continue
		}
		_ = p.dao.MarkSent(ctx, row.ID)
	}
}
