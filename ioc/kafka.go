package ioc

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/kafka"

	"github.com/redis/go-redis/v9"
)

// InitKafkaBus 初始化消息总线（API 仅 Publish，Worker 才 Run）。
func InitKafkaBus(redisClient redis.Cmdable) bus.Bus {
	return bus.InitBus(redisClient)
}

func InitKafkaRouter(b bus.Bus) *kafka.Router {
	return kafka.NewRouter(b)
}
