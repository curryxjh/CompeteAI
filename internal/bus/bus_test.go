package bus

import (
	"CompeteAI/internal/domain"
	"context"
	"testing"
)

func TestMemoryBusPublishDoesNotAutoConsume(t *testing.T) {
	b := NewMemoryBus()
	got := false
	b.Register(TopicTaskCreate, func(ctx context.Context, d Delivery) error {
		got = true
		return nil
	})
	env := domain.NewEnvelope("t1", "tr1", "api", "coordinator", domain.MsgTaskCreated, nil)
	_ = b.Publish(context.Background(), TopicTaskCreate, env)
	if got {
		t.Fatal("publish must not invoke handler before Run")
	}
}

func TestNormalizeTopic(t *testing.T) {
	if NormalizeTopic("coordinator.input") != TopicAgentCoordinator {
		t.Fatalf("legacy map failed")
	}
}
