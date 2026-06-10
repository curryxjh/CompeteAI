package workflow

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"testing"
)

func TestClarificationTriggerConstant(t *testing.T) {
	if protocol.TriggerClarificationAnswered != "clarification_answered" {
		t.Fatalf("unexpected trigger constant")
	}
}

func TestRouteClarificationAnswered(t *testing.T) {
	out := Route(RouteInput{MessageType: protocol.MsgClarificationAnswered})
	if out.NextAgent != domain.AgentCoordinator || out.TaskStatus != domain.TaskStatusRunning {
		t.Fatalf("route=%+v", out)
	}
}
