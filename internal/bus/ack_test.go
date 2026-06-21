package bus

import (
	"CompeteAI/internal/domain"
	"context"
	"testing"
	"time"
)

func TestMemoryBusRunConsumesPublished(t *testing.T) {
	b := NewMemoryBus()
	done := make(chan struct{}, 1)
	b.Register(TopicTaskCreate, func(ctx context.Context, d Delivery) error {
		done <- struct{}{}
		return nil
	})
	env := domain.NewEnvelope("t1", "tr1", "api", "coordinator", domain.MsgTaskCreated, nil)
	if err := b.Publish(context.Background(), TopicTaskCreate, env); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("handler not invoked after Run")
	}
}

func TestRetryBackoff(t *testing.T) {
	if RetryBackoff(1) <= 0 {
		t.Fatal("backoff must be positive")
	}
	if RetryBackoff(99) <= 0 {
		t.Fatal("backoff must cap to positive value")
	}
}
