package workflow

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/eventlog"
	"CompeteAI/internal/kafka"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
	"testing"
)

func testEngineWithBus() *Engine {
	b := bus.NewMemoryBus()
	return &Engine{
		hub:    eventlog.NewHybridHub(nil),
		bus:    b,
		router: kafka.NewRouter(b),
	}
}

func TestTryQAQueryLoopDetectsUnsupportedClaim(t *testing.T) {
	e := testEngineWithBus()
	store := state.ForTask(state.NewMemoryBlackboard(), "t1")
	wf := state.WorkflowState{QAQueryAttempts: 0}
	qa := protocol.QAResultPayload{
		Issues: []protocol.Issue{{Category: protocol.IssueUnsupportedClaim, Problem: "摘要无来源"}},
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
	qa := protocol.QAResultPayload{
		Issues: []protocol.Issue{{Category: protocol.IssueSourceMissing}},
	}
	if e.tryQAQueryLoop(context.Background(), "t1", "tr1", store, state.WorkflowState{}, qa) {
		t.Fatal("should not query for source_missing only")
	}
}
