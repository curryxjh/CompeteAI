package state

import (
	"context"
	"testing"

	"CompeteAI/internal/domain"
)

func TestTaskStorePartitions(t *testing.T) {
	ctx := context.Background()
	bb := NewMemoryBlackboard()
	store := ForTask(bb, "t1")

	meta := TaskMeta{ID: "t1", Title: "test", Competitors: []string{"A", "B"}, Dimensions: []string{"pricing"}}
	if err := store.SaveTaskMeta(ctx, meta); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadTaskMeta(ctx)
	if err != nil || loaded.Title != "test" {
		t.Fatalf("meta: %+v err=%v", loaded, err)
	}

	_ = store.SaveWorkflowState(ctx, WorkflowState{Round: 1, MaxRounds: 2, TraceID: "tr1"})
	coll := CollectorOutput{
		Query: "q", URLs: []string{"http://a.com"},
		Sources: map[string]domain.SourceRef{"s1": {URL: "http://a.com"}},
		Materials: "body", Summary: "ok",
	}
	if err := store.SaveCollectorOutput(ctx, coll); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadCollectorOutput(ctx)
	if err != nil || got.Query != "q" {
		t.Fatalf("collector: %+v err=%v", got, err)
	}

	keys, err := store.ListKeys(ctx)
	if err != nil || len(keys) < 3 {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
}

func TestAnalysisHistoryOnRework(t *testing.T) {
	ctx := context.Background()
	store := ForTask(NewMemoryBlackboard(), "t2")
	_ = store.SaveWorkflowState(ctx, WorkflowState{Round: 1})

	v1 := AnalysisOutput{Summary: "v1", SWOT: map[string]domain.SWOTAnalysis{}}
	_ = store.SaveAnalysisOutput(ctx, v1)
	v2 := AnalysisOutput{Summary: "v2", SWOT: map[string]domain.SWOTAnalysis{}}
	_ = store.SaveAnalysisOutput(ctx, v2)

	cur, _ := store.LoadAnalysisOutput(ctx)
	if cur.Summary != "v2" || cur.Version != 2 {
		t.Fatalf("current=%+v", cur)
	}

	var history AnalysisHistory
	_ = GetJSON(ctx, store.(*taskStore).bb, AnalysisHistoryKey("t2"), &history)
	if len(history.Entries) != 1 || history.Entries[0].Summary != "v1" {
		t.Fatalf("history=%+v", history)
	}
}

func TestQAHistory(t *testing.T) {
	ctx := context.Background()
	store := ForTask(NewMemoryBlackboard(), "t3")
	_ = store.SaveWorkflowState(ctx, WorkflowState{Round: 1})

	_ = store.SaveQAResult(ctx, QARecord{Score: 60, Result: "reject", Issues: []string{"missing sources"}})
	_ = store.SaveQAResult(ctx, QARecord{Score: 90, Result: "pass"})

	hist, _ := store.LoadQAHistory(ctx)
	if len(hist.Entries) != 2 {
		t.Fatalf("history=%+v", hist)
	}
}

func TestListByPrefixMemory(t *testing.T) {
	ctx := context.Background()
	bb := NewMemoryBlackboard()
	_ = bb.Put(ctx, TaskMetaKey("x"), TaskMeta{ID: "x"})
	_ = bb.Put(ctx, CollectorQueryKey("x"), CollectorQuery{Query: "q"})
	keys, err := bb.ListByPrefix(ctx, TaskPrefix("x"))
	if err != nil || len(keys) != 2 {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
}
