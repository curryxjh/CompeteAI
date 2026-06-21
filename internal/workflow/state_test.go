package workflow

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"testing"
)

func TestCanTransition(t *testing.T) {
	if !CanTransition(domain.TaskStatusRunning, domain.TaskStatusReworking) {
		t.Fatal("running -> reworking should be allowed")
	}
	if CanTransition(domain.TaskStatusCompleted, domain.TaskStatusRunning) {
		t.Fatal("completed -> running should be denied")
	}
}

func TestShouldRunAgentRework(t *testing.T) {
	if !ShouldRunAgent(domain.AgentCoordinator, domain.AgentAnalyst, true) {
		t.Fatal("coordinator always runs on rework")
	}
	if ShouldRunAgent(domain.AgentCollector, domain.AgentAnalyst, true) {
		t.Fatal("collector should skip when target is analyst")
	}
	if !ShouldRunAgent(domain.AgentAnalyst, domain.AgentAnalyst, true) {
		t.Fatal("analyst should run when target is analyst")
	}
}

func TestTargetAgentForIssues(t *testing.T) {
	target := TargetAgentForIssues([]domain.Issue{
		{Category: domain.IssueReportStructure},
		{Category: domain.IssueSourceMissing},
	})
	if target != domain.AgentCollector {
		t.Fatalf("expected collector, got %s", target)
	}
}

func TestRouteQARejectMaxRounds(t *testing.T) {
	out := RouteQAReject(&domain.QAResultPayload{
		Result: domain.QAResultReject, TargetAgent: "analyst",
	}, state.WorkflowState{Round: 3, MaxRounds: 3})
	if !out.Failed {
		t.Fatal("should fail when round >= maxRounds")
	}
}

func TestNextAgentInPipeline(t *testing.T) {
	next, ok := NextAgentInPipeline(domain.AgentWriter)
	if !ok || next != domain.AgentQA {
		t.Fatalf("next=%s ok=%v", next, ok)
	}
}
