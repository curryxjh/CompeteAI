package workflow

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/event"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"context"
	"testing"
)

func testEngineWithBus() *Engine {
	b := bus.NewMemoryBus()
	return &Engine{
		hub:    event.NewHybridHub(nil),
		bus:    b,
		router: bus.NewRouter(b),
	}
}

func TestTryQAQueryLoopDetectsUnsupportedClaim(t *testing.T) {
	e := testEngineWithBus()
	store := state.ForTask(state.NewMemoryBlackboard(), "t1")
	wf := state.WorkflowState{QAQueryAttempts: 0}
	qa := domain.QAResultPayload{
		Issues: []domain.Issue{{Category: domain.IssueUnsupportedClaim, Problem: "摘要无来源"}},
	}
	if !e.tryQAQueryLoop(context.Background(), "t1", "tr1", store, wf, qa) {
		t.Fatal("expected qa query loop on unsupported claim")
	}
	wf.QAQueryAttempts = 1
	if e.tryQAQueryLoop(context.Background(), "t1", "tr1", store, wf, qa) {
		t.Fatal("should not loop twice")
	}
}

func TestTryQAQueryLoopSkipsOtherIssues(t *testing.T) {
	e := testEngineWithBus()
	store := state.ForTask(state.NewMemoryBlackboard(), "t1")
	qa := domain.QAResultPayload{
		Issues: []domain.Issue{{Category: domain.IssueSourceMissing}},
	}
	if e.tryQAQueryLoop(context.Background(), "t1", "tr1", store, state.WorkflowState{}, qa) {
		t.Fatal("should not query for source_missing only")
	}
}
