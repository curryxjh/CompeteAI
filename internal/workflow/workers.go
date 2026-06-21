package workflow

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"context"
)

// RegisterWorkers 兼容旧入口；企业架构请用 orchestrator.Runtime.RegisterHandlers。
func RegisterWorkers(e *Engine) {
	topics := bus.AgentRoleTopics("all")
	for _, topic := range topics {
		t := topic
		e.bus.Register(t, func(ctx context.Context, d bus.Delivery) error {
			return e.HandleDelivery(ctx, t, d.Envelope)
		})
	}
}

// HandleDelivery Worker 消费消息后调用（API 不得调用）。
func (e *Engine) HandleDelivery(ctx context.Context, topic string, msg domain.MessageEnvelope) error {
	return e.withIdempotent(ctx, msg, func() error {
		topic = bus.NormalizeTopic(topic)
		switch topic {
		case bus.TopicTaskCreate:
			return e.handleTaskCreated(ctx, msg)
		case bus.TopicAgentCoordinator, bus.TopicAgentCollector, bus.TopicAgentAnalyst, bus.TopicAgentWriter, bus.TopicAgentQA:
			agentName, ok := agentForInputTopic(topic)
			if !ok {
				return nil
			}
			return e.handleAgentInput(ctx, agentName, msg)
		case bus.TopicQAQuery:
			return e.handleQAQuery(ctx, msg)
		case bus.TopicAnalystReply:
			return e.handleAnalystResponse(ctx, msg)
		case bus.TopicTaskClarify:
			return e.ProcessClarification(ctx, msg)
		case bus.TopicTaskCancel:
			return e.ProcessCancel(ctx, msg.TaskID)
		default:
			return nil
		}
	})
}

// Start API 进程禁止调用；Worker 使用 bus.Run。
func (e *Engine) Start(ctx context.Context) {}
