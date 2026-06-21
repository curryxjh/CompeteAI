package bus

import (
	"CompeteAI/internal/domain"
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"CompeteAI/settings"

	"github.com/redis/go-redis/v9"
)

func InitBus(redisClient redis.Cmdable) Bus {
	validateProdBus(redisClient)
	cfg := settings.Conf.KafkaConfig
	if cfg != nil && cfg.Enabled && len(cfg.Brokers) > 0 {
		log.Printf("[bus] Kafka brokers=%v", cfg.Brokers)
		return NewKafkaBus(cfg.Brokers, cfg.GroupID)
	}
	if redisClient != nil {
		consumer := "worker-" + hostname()
		log.Printf("[bus] Redis Stream consumer=%s", consumer)
		return NewRedisStreamBus(redisClient, "compete-ai", consumer)
	}
	log.Printf("[bus] memory (dev only — start worker separately)")
	return NewMemoryBus()
}

func InitPublisher(redisClient redis.Cmdable) Bus {
	// API 进程只需要 Publish，不 Run
	return InitBus(redisClient)
}

func InitConsumer(redisClient redis.Cmdable, consumerID string) Bus {
	validateProdBus(redisClient)
	cfg := settings.Conf.KafkaConfig
	if cfg != nil && cfg.Enabled && len(cfg.Brokers) > 0 {
		return NewKafkaBus(cfg.Brokers, cfg.GroupID+"-"+consumerID)
	}
	if redisClient != nil {
		if consumerID == "" {
			consumerID = "worker-" + hostname()
		}
		return NewRedisStreamBus(redisClient, "compete-ai", consumerID)
	}
	return NewMemoryBus()
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "local"
	}
	return h
}

// PublishDLQ 将超过重试阈值的消息转入死信 topic。
func PublishDLQ(ctx context.Context, b Bus, agent domain.AgentName, env domain.MessageEnvelope, reason string) error {
	meta := map[string]any{"dlq_reason": reason, "original_topic": env.MessageType}
	raw, _ := json.Marshal(meta)
	env.Metadata = meta
	env.Payload = raw
	return b.Publish(ctx, DLQTopic(agent), env)
}

// RetryBackoff 指数退避序列（§8.4）。
func RetryBackoff(attempt int) time.Duration {
	backoffs := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}
	if attempt <= 0 {
		return backoffs[0]
	}
	if attempt > len(backoffs) {
		return backoffs[len(backoffs)-1]
	}
	return backoffs[attempt-1]
}

func validateProdBus(redisClient redis.Cmdable) {
	if settings.Conf == nil || settings.Conf.Mode != "prod" {
		return
	}
	if redisClient == nil {
		panic("prod mode requires Redis bus; memory bus forbidden")
	}
}
