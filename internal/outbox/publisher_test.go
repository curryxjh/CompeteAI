package outbox

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"encoding/json"
	"testing"
)

func TestOutboxPayloadRoundtrip(t *testing.T) {
	task := domain.Task{ID: "t1", Title: "test", Status: domain.TaskStatusQueued}
	msg := protocol.NewTaskCreatedMessage(task, "tr1")
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var back protocol.MessageEnvelope
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.TaskID != "t1" {
		t.Fatalf("task id mismatch")
	}
}
