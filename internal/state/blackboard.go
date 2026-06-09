package state

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

// Blackboard 共享上下文接口（§8）。
type Blackboard interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, key string) error
	ListByPrefix(ctx context.Context, prefix string) ([]string, error)
}

// MemoryBlackboard 内存实现（§12 第一阶段）。
type MemoryBlackboard struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMemoryBlackboard() *MemoryBlackboard {
	return &MemoryBlackboard{data: make(map[string][]byte)}
}

func (b *MemoryBlackboard) Put(_ context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.data[key] = raw
	b.mu.Unlock()
	return nil
}

func (b *MemoryBlackboard) Get(_ context.Context, key string, dest any) error {
	b.mu.RLock()
	raw, ok := b.data[key]
	b.mu.RUnlock()
	if !ok {
		return ErrKeyNotFound
	}
	return json.Unmarshal(raw, dest)
}

func (b *MemoryBlackboard) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	delete(b.data, key)
	b.mu.Unlock()
	return nil
}

func (b *MemoryBlackboard) ListByPrefix(_ context.Context, prefix string) ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var keys []string
	for k := range b.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// Snapshot 导出全部 KV（调试用）。
func (b *MemoryBlackboard) Snapshot(_ context.Context) map[string]any {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make(map[string]any, len(b.data))
	for k, raw := range b.data {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			out[k] = v
		}
	}
	return out
}

// GetJSON / PutJSON 兼容辅助。
func GetJSON(ctx context.Context, bb Blackboard, key string, dest any) error {
	return bb.Get(ctx, key, dest)
}

func PutJSON(ctx context.Context, bb Blackboard, key string, value any) error {
	return bb.Put(ctx, key, value)
}
