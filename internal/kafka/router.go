package kafka

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"context"
)

// Router 根据协议消息路由到 Topic。
type Router struct {
	bus bus.Bus
}

func NewRouter(b bus.Bus) *Router {
	return &Router{bus: b}
}

func (r *Router) Publish(ctx context.Context, msg protocol.MessageEnvelope) error {
	topic := topicForMessage(msg)
	if topic == "" {
		return nil
	}
	return r.bus.Publish(ctx, topic, msg)
}

func (r *Router) PublishTaskCreated(ctx context.Context, task domain.Task, traceID string) error {
	msg := protocol.NewTaskCreatedMessage(task, traceID).WithStatus(protocol.MessageStatusPending)
	return r.bus.Publish(ctx, bus.TopicTaskCreate, msg)
}

func (r *Router) PublishAgentOutput(ctx context.Context, taskID, traceID string, from domain.AgentName, out protocol.MessageEnvelope) error {
	return r.Publish(ctx, out)
}

func (r *Router) PublishQAReject(ctx context.Context, taskID, traceID string, payload protocol.QAResultPayload) error {
	msg := protocol.NewEnvelope(
		taskID, traceID,
		string(domain.AgentQA), string(domain.AgentCoordinator),
		protocol.MsgQAReject, payload,
	).WithStatus(protocol.MessageStatusRejected)
	return r.bus.Publish(ctx, bus.TopicAgentCoordinator, msg)
}

func (r *Router) PublishTaskCompleted(ctx context.Context, taskID, traceID string, payload protocol.TaskCompletedPayload) error {
	msg := protocol.NewEnvelope(taskID, traceID, "router", "api", protocol.MsgTaskCompleted, payload).
		WithStatus(protocol.MessageStatusCompleted)
	return r.bus.Publish(ctx, bus.TopicEventLifecycle, msg)
}

func (r *Router) PublishTaskFailed(ctx context.Context, taskID, traceID, agent, message string) error {
	payload := protocol.TaskFailedPayload{TaskID: taskID, Message: message, Agent: agent}
	msg := protocol.NewEnvelope(taskID, traceID, "router", "api", protocol.MsgTaskFailed, payload).
		WithStatus(protocol.MessageStatusFailed)
	return r.bus.Publish(ctx, bus.TopicEventLifecycle, msg)
}

func (r *Router) PublishClarification(ctx context.Context, taskID, traceID string, payload protocol.ClarificationPayload) error {
	msg := protocol.NewEnvelope(
		taskID, traceID,
		string(domain.AgentCoordinator), "user",
		protocol.MsgClarificationRequired, payload,
	).WithStatus(protocol.MessageStatusPending)
	return r.bus.Publish(ctx, bus.TopicEventProgress, msg)
}

func (r *Router) PublishQAQuery(ctx context.Context, taskID, traceID string, payload protocol.QAQueryPayload) error {
	msg := protocol.NewEnvelope(
		taskID, traceID,
		string(domain.AgentQA), string(domain.AgentAnalyst),
		protocol.MsgQAQuery, payload,
	).WithStatus(protocol.MessageStatusPending)
	return r.bus.Publish(ctx, bus.TopicQAQuery, msg)
}

func topicForMessage(msg protocol.MessageEnvelope) string {
	if t := topicByType(msg.MessageType); t != "" {
		return t
	}
	if dec := protocol.RouteNext(msg.MessageType, nil); dec.Topic != "" {
		return bus.NormalizeTopic(dec.Topic)
	}
	return ""
}

func topicByType(msgType protocol.MessageType) string {
	switch msgType {
	case protocol.MsgTaskCreated:
		return bus.TopicTaskCreate
	case protocol.MsgClarificationRequired:
		return bus.TopicTaskClarify
	case protocol.MsgClarificationAnswered:
		return bus.TopicAgentCoordinator
	case protocol.MsgPlanReady:
		return bus.TopicAgentCollector
	case protocol.MsgMaterialsReady:
		return bus.TopicAgentAnalyst
	case protocol.MsgAnalysisReady:
		return bus.TopicAgentWriter
	case protocol.MsgReportReady:
		return bus.TopicAgentQA
	case protocol.MsgQAPass:
		return bus.TopicEventLifecycle
	case protocol.MsgQAReject:
		return bus.TopicAgentCoordinator
	case protocol.MsgQAQuery:
		return bus.TopicQAQuery
	case protocol.MsgAnalystResponse:
		return bus.TopicAnalystReply
	case protocol.MsgTaskCompleted, protocol.MsgTaskFailed:
		return bus.TopicEventLifecycle
	default:
		return ""
	}
}
