package workflow

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"testing"
)

func TestRouteQAPassFromRouterTest(t *testing.T) {
	out := Route(RouteInput{MessageType: protocol.MsgQAPass, FromAgent: domain.AgentQA})
	if !out.Done {
		t.Fatal("expected done on qa pass")
	}
}
