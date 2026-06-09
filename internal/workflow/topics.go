package workflow

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
)

func inputTopicForAgent(name domain.AgentName) string {
	return bus.AgentCommandTopic(name)
}

func agentForInputTopic(topic string) (domain.AgentName, bool) {
	switch bus.NormalizeTopic(topic) {
	case bus.TopicAgentCoordinator, bus.TopicTaskCreate:
		return domain.AgentCoordinator, true
	case bus.TopicAgentCollector:
		return domain.AgentCollector, true
	case bus.TopicAgentAnalyst, bus.TopicQAQuery:
		return domain.AgentAnalyst, true
	case bus.TopicAgentWriter:
		return domain.AgentWriter, true
	case bus.TopicAgentQA, bus.TopicAnalystReply:
		return domain.AgentQA, true
	default:
		return "", false
	}
}
