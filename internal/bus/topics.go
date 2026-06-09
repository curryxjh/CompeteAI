package bus

import "CompeteAI/internal/domain"

// Enterprise topic names (blueprint §5.2).
const (
	TopicTaskCreate       = "task.command.create"
	TopicTaskCancel       = "task.command.cancel"
	TopicTaskClarify      = "task.command.clarify"
	TopicAgentCoordinator = "agent.command.coordinator"
	TopicAgentCollector   = "agent.command.collector"
	TopicAgentAnalyst     = "agent.command.analyst"
	TopicAgentWriter      = "agent.command.writer"
	TopicAgentQA          = "agent.command.qa"
	TopicQAQuery          = "agent.query.qa_to_analyst"
	TopicAnalystReply     = "agent.reply.analyst_to_qa"
	TopicEventLifecycle   = "task.event.lifecycle"
	TopicEventProgress    = "task.event.progress"
	TopicEventTrace       = "task.event.trace"
	TopicEventAudit       = "task.event.audit"
)

func DLQTopic(agent domain.AgentName) string {
	return "task.dlq." + string(agent)
}

func AgentCommandTopic(name domain.AgentName) string {
	switch name {
	case domain.AgentCoordinator:
		return TopicAgentCoordinator
	case domain.AgentCollector:
		return TopicAgentCollector
	case domain.AgentAnalyst:
		return TopicAgentAnalyst
	case domain.AgentWriter:
		return TopicAgentWriter
	case domain.AgentQA:
		return TopicAgentQA
	default:
		return ""
	}
}

func AgentRoleTopics(role string) []string {
	switch role {
	case "coordinator":
		return []string{TopicAgentCoordinator, TopicTaskCreate}
	case "collector":
		return []string{TopicAgentCollector}
	case "analyst":
		return []string{TopicAgentAnalyst, TopicQAQuery}
	case "writer":
		return []string{TopicAgentWriter}
	case "qa":
		return []string{TopicAgentQA, TopicAnalystReply}
	case "all":
		return []string{
			TopicTaskCreate, TopicTaskCancel, TopicTaskClarify,
			TopicAgentCoordinator, TopicAgentCollector, TopicAgentAnalyst,
			TopicAgentWriter, TopicAgentQA, TopicQAQuery, TopicAnalystReply,
		}
	default:
		return nil
	}
}

// LegacyTopicMap maps old kafka topic names to enterprise topics.
var LegacyTopicMap = map[string]string{
	"task.create":         TopicTaskCreate,
	"coordinator.input":   TopicAgentCoordinator,
	"collector.input":     TopicAgentCollector,
	"analyst.input":       TopicAgentAnalyst,
	"writer.input":        TopicAgentWriter,
	"qa.input":            TopicAgentQA,
	"qa.query":            TopicQAQuery,
	"analyst.response":    TopicAnalystReply,
	"user.clarify":        TopicTaskClarify,
}

func NormalizeTopic(topic string) string {
	if t, ok := LegacyTopicMap[topic]; ok {
		return t
	}
	return topic
}
