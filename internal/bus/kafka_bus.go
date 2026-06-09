package bus

import (
	"CompeteAI/internal/protocol"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaBus Kafka 实现：手动 commit offset（handler 成功后才 CommitMessages）。
type KafkaBus struct {
	brokers  []string
	groupID  string
	mu       sync.RWMutex
	handlers map[string][]Handler
	readers  []*kafka.Reader
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewKafkaBus(brokers []string, groupID string) *KafkaBus {
	if groupID == "" {
		groupID = "compete-ai"
	}
	return &KafkaBus{
		brokers:  brokers,
		groupID:  groupID,
		handlers: make(map[string][]Handler),
	}
}

func (b *KafkaBus) Publish(ctx context.Context, topic string, env protocol.MessageEnvelope) error {
	topic = NormalizeTopic(topic)
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	w := &kafka.Writer{
		Addr:     kafka.TCP(b.brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
	}
	defer w.Close()
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(env.TaskID),
		Value: raw,
	})
}

func (b *KafkaBus) Register(topic string, handler Handler) {
	topic = NormalizeTopic(topic)
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
}

func (b *KafkaBus) Run(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)
	b.mu.RLock()
	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	b.mu.RUnlock()

	for _, topic := range topics {
		t := topic
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  b.brokers,
			GroupID:  b.groupID,
			Topic:    t,
			MinBytes: 1,
			MaxBytes: 10e6,
		})
		b.readers = append(b.readers, r)
		b.wg.Add(1)
		go func(reader *kafka.Reader) {
			defer b.wg.Done()
			b.consumeLoop(ctx, t, reader)
		}(r)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (b *KafkaBus) consumeLoop(ctx context.Context, topic string, reader *kafka.Reader) {
	for {
		m, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[kafka-bus] fetch %s: %v", topic, err)
			time.Sleep(time.Second)
			continue
		}
		var env protocol.MessageEnvelope
		if err := json.Unmarshal(m.Value, &env); err != nil {
			_ = reader.CommitMessages(ctx, m)
			continue
		}
		d := Delivery{EntryID: fmt.Sprintf("%d-%d", m.Partition, m.Offset), Topic: topic, Envelope: env}
		b.mu.RLock()
		handlers := append([]Handler{}, b.handlers[topic]...)
		b.mu.RUnlock()

		var lastErr error
		for _, h := range handlers {
			if h != nil {
				if err := h(ctx, d); err != nil {
					lastErr = err
					break
				}
			}
		}
		if lastErr != nil {
			log.Printf("[kafka-bus] handler failed: %v", lastErr)
			continue // 不 commit
		}
		_ = reader.CommitMessages(ctx, m)
	}
}

func (b *KafkaBus) Close() error {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	for _, r := range b.readers {
		_ = r.Close()
	}
	return nil
}
