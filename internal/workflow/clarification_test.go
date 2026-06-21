package workflow

import (
	"CompeteAI/internal/domain"
	"testing"
)

func TestClarificationTriggerConstant(t *testing.T) {
	if domain.TriggerClarificationAnswered != "clarification_answered" {
		t.Fatalf("unexpected trigger constant")
	}
}

func TestRouteClarificationAnswered(t *testing.T) {
	out := Route(RouteInput{MessageType: domain.MsgClarificationAnswered})
	if out.NextAgent != domain.AgentCoordinator || out.TaskStatus != domain.TaskStatusRunning {
		t.Fatalf("route=%+v", out)
	}
}
