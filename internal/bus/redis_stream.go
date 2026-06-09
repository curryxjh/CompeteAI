package bus

import (
	"CompeteAI/internal/protocol"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisStreamPrefix = "compete:bus:"

// RedisStreamBus Redis Stream 实现：Publish 不本地 dispatch，ACK 由 handler 成功后提交。
type RedisStreamBus struct {
	redis    redis.Cmdable
	group    string
	consumer string

	mu       sync.RWMutex
	handlers map[string][]Handler
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewRedisStreamBus(client redis.Cmdable, group, consumer string) *RedisStreamBus {
	if group == "" {
		group = "compete-ai"
	}
	if consumer == "" {
		consumer = "worker-1"
	}
	return &RedisStreamBus{
		redis:    client,
		group:    group,
		consumer: consumer,
		handlers: make(map[string][]Handler),
	}
}

func (b *RedisStreamBus) streamKey(topic string) string {
	return redisStreamPrefix + NormalizeTopic(topic)
}

func (b *RedisStreamBus) Publish(ctx context.Context, topic string, env protocol.MessageEnvelope) error {
	topic = NormalizeTopic(topic)
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: b.streamKey(topic),
		Values: map[string]interface{}{"payload": string(raw)},
	}).Err()
}

func (b *RedisStreamBus) Register(topic string, handler Handler) {
	topic = NormalizeTopic(topic)
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
}

func (b *RedisStreamBus) Run(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)
	b.mu.RLock()
	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	b.mu.RUnlock()

	for _, topic := range topics {
		b.ensureGroup(ctx, topic)
		t := topic
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.consumeLoop(ctx, t)
		}()
	}
	<-ctx.Done()
	return ctx.Err()
}

func (b *RedisStreamBus) ensureGroup(ctx context.Context, topic string) {
	stream := b.streamKey(topic)
	_ = b.redis.XGroupCreateMkStream(ctx, stream, b.group, "0").Err()
}

func (b *RedisStreamBus) consumeLoop(ctx context.Context, topic string) {
	stream := b.streamKey(topic)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := b.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.group,
			Consumer: b.consumer,
			Streams:  []string{stream, ">"},
			Count:    10,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				continue
			}
			log.Printf("[bus] read %s: %v", topic, err)
			time.Sleep(time.Second)
			continue
		}
		for _, s := range streams {
			for _, x := range s.Messages {
				b.handleEntry(ctx, topic, stream, x)
			}
		}
	}
}

func (b *RedisStreamBus) handleEntry(ctx context.Context, topic, stream string, x redis.XMessage) {
	raw, _ := x.Values["payload"].(string)
	var env protocol.MessageEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		log.Printf("[bus] bad payload on %s: %v", topic, err)
		_ = b.redis.XAck(ctx, stream, b.group, x.ID).Err()
		return
	}
	d := Delivery{EntryID: x.ID, Topic: topic, Envelope: env}

	b.mu.RLock()
	handlers := append([]Handler{}, b.handlers[topic]...)
	b.mu.RUnlock()

	var lastErr error
	for _, h := range handlers {
		if h == nil {
			continue
		}
		if err := h(ctx, d); err != nil {
			lastErr = err
			break
		}
	}
	if lastErr != nil {
		log.Printf("[bus] handler failed topic=%s msg=%s: %v", topic, env.MessageID, lastErr)
		return // 不 ACK → pending → recovery 可 reclaim
	}
	_ = b.redis.XAck(ctx, stream, b.group, x.ID).Err()
}

func (b *RedisStreamBus) Close() error {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	return nil
}

// ReclaimPending 认领超时 pending 消息（recovery 用）。
func (b *RedisStreamBus) ReclaimPending(ctx context.Context, topic string, minIdle time.Duration, count int64) (int, error) {
	stream := b.streamKey(topic)
	messages, _, err := b.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    b.group,
		Consumer: b.consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    count,
	}).Result()
	if err != nil {
		return 0, err
	}
	reclaimed := 0
	for _, x := range messages {
		b.handleEntry(ctx, topic, stream, x)
		reclaimed++
	}
	return reclaimed, nil
}

func DedupKey(messageID string) string {
	return fmt.Sprintf("compete:dedup:%s", messageID)
}
