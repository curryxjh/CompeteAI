package protocol

import "CompeteAI/internal/domain"

// RouteDecision 路由决策（§11）。
type RouteDecision struct {
	ToAgent domain.AgentName
	Topic   string // Kafka topic 提示，本地执行器可忽略
}

// DefaultReceiver 根据消息类型返回默认接收方。
func DefaultReceiver(msgType MessageType, payload any) domain.AgentName {
	switch msgType {
	case MsgTaskCreated, MsgClarificationAnswered, MsgQAReject:
		return domain.AgentCoordinator
	case MsgClarificationRequired:
		return domain.AgentName("user")
	case MsgPlanReady:
		return domain.AgentCollector
	case MsgMaterialsReady:
		return domain.AgentAnalyst
	case MsgAnalysisReady:
		return domain.AgentWriter
	case MsgReportReady:
		return domain.AgentQA
	case MsgQAPass, MsgTaskCompleted, MsgTaskFailed:
		return domain.AgentName("api")
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
func ReworkTarget(payload any) domain.AgentName {
	switch p := payload.(type) {
	case QAResultPayload:
		if p.TargetAgent != "" {
			return domain.AgentName(p.TargetAgent)
		}
	case *QAResultPayload:
		if p != nil && p.TargetAgent != "" {
			return domain.AgentName(p.TargetAgent)
		}
	case QAResult:
		if p.TargetAgent != "" {
			return p.TargetAgent
		}
	}
	return domain.AgentAnalyst
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
