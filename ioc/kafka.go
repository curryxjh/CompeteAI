package ioc

import (
	"CompeteAI/internal/bus"

	"github.com/redis/go-redis/v9"
)

func InitKafkaBus(redisClient redis.Cmdable) bus.Bus {
	return bus.InitBus(redisClient)
}

func InitKafkaRouter(b bus.Bus) *bus.Router {
	return bus.NewRouter(b)
}