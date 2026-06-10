package state

import (
	"CompeteAI/settings"
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const blackboardTTL = 24 * time.Hour

// RedisBlackboard Redis 实现（§12 第二阶段）。
type RedisBlackboard struct {
	client redis.Cmdable
}

func NewRedisBlackboard(client redis.Cmdable) *RedisBlackboard {
	return &RedisBlackboard{client: client}
}

func (b *RedisBlackboard) Put(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return b.client.Set(ctx, key, raw, blackboardTTL).Err()
}

func (b *RedisBlackboard) Get(ctx context.Context, key string, dest any) error {
	raw, err := b.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrKeyNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func (b *RedisBlackboard) Delete(ctx context.Context, key string) error {
	return b.client.Del(ctx, key).Err()
}

func (b *RedisBlackboard) ListByPrefix(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	iter := b.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

// ClearTask 按任务前缀清理 Blackboard 数据。
func (b *RedisBlackboard) ClearTask(ctx context.Context, taskID string) error {
	prefix := TaskPrefix(taskID)
	keys, err := b.ListByPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return b.client.Del(ctx, keys...).Err()
}

// ClearTaskMemory 内存实现任务级清理。
func ClearTaskMemory(ctx context.Context, bb *MemoryBlackboard, taskID string) error {
	keys, err := bb.ListByPrefix(ctx, TaskPrefix(taskID))
	if err != nil {
		return err
	}
	for _, k := range keys {
		bb.Delete(ctx, k)
	}
	return nil
}

// NewBlackboard 按配置选择 Redis 或全局共享 MemoryBlackboard（§12）。
// 修复：内存模式下通过 GlobalRegistry 确保同一 taskID 共享同一实例，
// 避免每次调用返回空实例导致多 Agent 状态传递失效。
func NewBlackboard(useRedis bool, redisClient redis.Cmdable, taskID string) Blackboard {
	if settings.Conf != nil && settings.Conf.Mode == "prod" && (!useRedis || redisClient == nil) {
		panic("prod mode requires Redis blackboard; memory forbidden")
	}
	if useRedis && redisClient != nil {
		return NewRedisBlackboard(redisClient)
	}
	return GlobalRegistry().GetOrCreate(taskID)
}
