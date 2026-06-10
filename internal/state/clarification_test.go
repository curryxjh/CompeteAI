package state

import "testing"

func TestMergeClarificationIntoMeta(t *testing.T) {
	meta := TaskMeta{Competitors: []string{}, Dimensions: []string{"pricing"}}
	ans := ClarificationAnswer{Answer: "Notion, Airtable"}
	q := ClarificationQuestion{Question: "请提供至少一个竞品名称"}

	got := MergeClarificationIntoMeta(meta, ans, q)
	if len(got.Competitors) != 2 || got.Competitors[0] != "Notion" {
		t.Fatalf("competitors=%v", got.Competitors)
	}
}

func TestSaveWorkflowReworkRejectionCountOnce(t *testing.T) {
	ctx := t.Context()
	bb := NewMemoryBlackboard()
	store := ForTask(bb, "t1")
	_ = store.SaveWorkflowState(ctx, WorkflowState{Round: 1, RejectionCount: 0})

	wf, _ := store.LoadWorkflowState(ctx)
	wf.RejectionCount++
	wf.Round++
	_ = store.SaveWorkflowState(ctx, wf)
	_ = store.SaveWorkflowRework(ctx, WorkflowRework{TargetAgent: "analyst", Reason: "test"})

	wf, _ = store.LoadWorkflowState(ctx)
	if wf.RejectionCount != 1 {
		t.Fatalf("rejection_count=%d want 1", wf.RejectionCount)
	}
}
