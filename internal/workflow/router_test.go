package workflow

import (
	"CompeteAI/internal/domain"
	"testing"
)

func TestRouteQAPassFromRouterTest(t *testing.T) {
	out := Route(RouteInput{MessageType: domain.MsgQAPass, FromAgent: domain.AgentQA})
	if !out.Done {
		t.Fatal("expected done on qa pass")
	}
}
