package workflow

import (
	"CompeteAI/internal/domain"
)

func buildTraceNode(id string, name domain.AgentName, label string, status domain.AgentRunStatus, output string, meta map[string]any) domain.TraceNode {
	node := domain.TraceNode{
		ID: id, Agent: name, Label: label, Status: status,
		Output: truncate(output, 300), Metadata: meta,
	}
	if meta == nil {
		return node
	}
	if v, ok := meta["duration_ms"].(int); ok {
		node.DurationMs = v
	}
	if v, ok := meta["token_count"].(int); ok {
		node.TokenCount = v
	}
	if steps, ok := meta["trace_steps"].([]domain.TraceStep); ok {
		node.Steps = steps
	}
	return node
}
