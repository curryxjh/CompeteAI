package bus

import (
	"CompeteAI/internal/domain"
	"context"
)

// Router 根据协议消息路由到 Topic。
type Router struct {
	bus Bus
}

func NewRouter(b Bus) *Router {
	return &Router{bus: b}
}

func (r *Router) Publish(ctx context.Context, msg domain.MessageEnvelope) error {
	topic := topicForMessage(msg)
	if topic == "" {
		return nil
	}
	return r.bus.Publish(ctx, topic, msg)
}

func (r *Router) PublishTaskCreated(ctx context.Context, task domain.Task, traceID string) error {
	msg := domain.NewTaskCreatedMessage(task, traceID).WithStatus(domain.MessageStatusPending)
	return r.bus.Publish(ctx, TopicTaskCreate, msg)
}

func (r *Router) PublishAgentOutput(ctx context.Context, taskID, traceID string, from domain.AgentName, out domain.MessageEnvelope) error {
	return r.Publish(ctx, out)
}

func (r *Router) PublishQAReject(ctx context.Context, taskID, traceID string, payload domain.QAResultPayload) error {
	msg := domain.NewEnvelope(
		taskID, traceID,
		string(domain.AgentQA), string(domain.AgentCoordinator),
		domain.MsgQAReject, payload,
	).WithStatus(domain.MessageStatusRejected)
	return r.bus.Publish(ctx, TopicAgentCoordinator, msg)
}

func (r *Router) PublishTaskCompleted(ctx context.Context, taskID, traceID string, payload domain.TaskCompletedPayload) error {
	msg := domain.NewEnvelope(taskID, traceID, "router", "api", domain.MsgTaskCompleted, payload).
		WithStatus(domain.MessageStatusCompleted)
	return r.bus.Publish(ctx, TopicEventLifecycle, msg)
}

func (r *Router) PublishTaskFailed(ctx context.Context, taskID, traceID, agent, message string) error {
	payload := domain.TaskFailedPayload{TaskID: taskID, Message: message, Agent: agent}
	msg := domain.NewEnvelope(taskID, traceID, "router", "api", domain.MsgTaskFailed, payload).
		WithStatus(domain.MessageStatusFailed)
	return r.bus.Publish(ctx, TopicEventLifecycle, msg)
}

func (r *Router) PublishClarification(ctx context.Context, taskID, traceID string, payload domain.ClarificationPayload) error {
	msg := domain.NewEnvelope(
		taskID, traceID,
		string(domain.AgentCoordinator), "user",
		domain.MsgClarificationRequired, payload,
	).WithStatus(domain.MessageStatusPending)
	return r.bus.Publish(ctx, TopicEventProgress, msg)
}

func (r *Router) PublishQAQuery(ctx context.Context, taskID, traceID string, payload domain.QAQueryPayload) error {
	msg := domain.NewEnvelope(
		taskID, traceID,
		string(domain.AgentQA), string(domain.AgentAnalyst),
		domain.MsgQAQuery, payload,
	)
	return r.bus.Publish(ctx, TopicQAQuery, msg)
}

func topicForMessage(msg domain.MessageEnvelope) string {
	if t := topicByType(msg.MessageType); t != "" {
		return t
	}
	if dec := domain.RouteNext(msg.MessageType, nil); dec.Topic != "" {
		return NormalizeTopic(dec.Topic)
	}
	return ""
}

func topicByType(msgType domain.MessageType) string {
	switch msgType {
	case domain.MsgTaskCreated:
		return TopicTaskCreate
	case domain.MsgClarificationRequired:
		return TopicTaskClarify
	case domain.MsgClarificationAnswered:
		return TopicAgentCoordinator
	case domain.MsgPlanReady:
		return TopicAgentCollector
	case domain.MsgMaterialsReady:
		return TopicAgentAnalyst
	case domain.MsgAnalysisReady:
		return TopicAgentWriter
	case domain.MsgReportReady:
		return TopicAgentQA
	case domain.MsgQAPass:
		return TopicEventLifecycle
	case domain.MsgQAReject:
		return TopicAgentCoordinator
	case domain.MsgQAQuery:
		return TopicQAQuery
	case domain.MsgAnalystResponse:
		return TopicAnalystReply
	case domain.MsgTaskCompleted, domain.MsgTaskFailed:
		return TopicEventLifecycle
	default:
		return ""
	}
}