package bus

import (
	"CompeteAI/internal/domain"
	"context"
)

// Delivery 消费侧消息投递（含 stream entry id 供 ACK）。
type Delivery struct {
	EntryID  string
	Topic    string
	Envelope domain.MessageEnvelope
}

// Handler 返回 nil 表示成功可 ACK；非 nil 表示 Nack（不 ACK，可重试）。
type Handler func(ctx context.Context, d Delivery) error

// Bus 企业级消息总线：Publish 只写入，Consume 才执行。
type Bus interface {
	Publish(ctx context.Context, topic string, env domain.MessageEnvelope) error
	Register(topic string, handler Handler)
	Run(ctx context.Context) error
	Close() error
}
