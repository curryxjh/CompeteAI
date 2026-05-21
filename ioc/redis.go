package ioc

import (
	"CompeteAI/internal/pkg/ratelimit"
	"CompeteAI/settings"
	"time"

	"github.com/redis/go-redis/v9"
)

func InitRedis() redis.Cmdable {
	cfg := settings.Conf.RedisConfig
	if cfg == nil {
		panic("redis config is nil, check config file")
	}

	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
}

func NewRateLimiter(redisClient redis.Cmdable, interval time.Duration, rate int) ratelimit.Limiter {
	return ratelimit.NewRedisSlidingWindowLimiter(redisClient, interval, rate)
}
