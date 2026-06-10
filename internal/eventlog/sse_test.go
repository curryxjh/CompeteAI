package eventlog

import "testing"

func TestSSEEventNameMapping(t *testing.T) {
	cases := map[string]string{
		"task_completed":         "task_complete",
		"agent_progress":         "agent_state",
		"clarification_required": "clarification",
		"qa_rejected":            "rejection",
		"custom_event":           "custom_event",
	}
	for in, want := range cases {
		if got := SSEEventName(in); got != want {
			t.Fatalf("%s: want %s got %s", in, want, got)
		}
	}
}

func TestMapLegacyEvent(t *testing.T) {
	if MapLegacyEvent("task_complete") != "task_completed" {
		t.Fatal("legacy task_complete mapping")
	}
	if MapLegacyEvent("agent_state") != "agent_progress" {
		t.Fatal("legacy agent_state mapping")
	}
}
