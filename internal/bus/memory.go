package bus

import (
	"CompeteAI/internal/protocol"
	"context"
	"sync"
	"time"
)

// MemoryBus 开发降级：Publish 只入队，Run 才消费（API 进程不应 Run）。
type MemoryBus struct {
	mu       sync.Mutex
	handlers map[string][]Handler
	queues   map[string][]Delivery
	closed   bool
}

func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[string][]Handler),
		queues:   make(map[string][]Delivery),
	}
}

func (b *MemoryBus) Publish(_ context.Context, topic string, env protocol.MessageEnvelope) error {
	topic = NormalizeTopic(topic)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return context.Canceled
	}
	id := env.MessageID
	if id == "" {
		id = "mem-" + topic
	}
	b.queues[topic] = append(b.queues[topic], Delivery{EntryID: id, Topic: topic, Envelope: env})
	return nil
}

func (b *MemoryBus) Register(topic string, handler Handler) {
	topic = NormalizeTopic(topic)
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
}

func (b *MemoryBus) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		b.mu.Lock()
		var work []struct {
			topic string
			d     Delivery
		}
		for topic, q := range b.queues {
			if len(q) > 0 {
				work = append(work, struct {
					topic string
					d     Delivery
				}{topic, q[0]})
				b.queues[topic] = q[1:]
			}
		}
		handlersCopy := make(map[string][]Handler, len(b.handlers))
		for k, v := range b.handlers {
			handlersCopy[k] = append([]Handler{}, v...)
		}
		b.mu.Unlock()

		if len(work) == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timeAfter(50 * time.Millisecond):
			}
			continue
		}
		for _, item := range work {
			for _, h := range handlersCopy[item.topic] {
				if h != nil {
					_ = h(ctx, item.d)
				}
			}
		}
	}
}

func (b *MemoryBus) Close() error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	return nil
}

func timeAfter(d time.Duration) <-chan time.Time {
	return time.NewTimer(d).C
}
