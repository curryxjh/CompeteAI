package state

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

// BlackboardRegistry 全局 MemoryBlackboard 单例注册表。
// 保证同一 taskID 的所有调用方共享同一 MemoryBlackboard 实例，
// 避免非 Redis 模式下每次 NewBlackboard() 创建空实例导致状态丢失。
type BlackboardRegistry struct {
	mu      sync.RWMutex
	boards  map[string]*MemoryBlackboard
	order   []string // 按插入顺序，用于 LRU 淘汰
	maxSize int
}

var globalRegistry = &BlackboardRegistry{
	boards:  make(map[string]*MemoryBlackboard),
	maxSize: 1000,
}

// GlobalRegistry 返回全局单例注册表。
func GlobalRegistry() *BlackboardRegistry { return globalRegistry }

// SetMaxSize 允许在初始化时调整最大 task 容量（测试用）。
func (r *BlackboardRegistry) SetMaxSize(n int) {
	r.mu.Lock()
	r.maxSize = n
	r.mu.Unlock()
}

// GetOrCreate 返回 taskID 对应的 MemoryBlackboard，不存在则创建。
func (r *BlackboardRegistry) GetOrCreate(taskID string) *MemoryBlackboard {
	// fast path
	r.mu.RLock()
	if bb, ok := r.boards[taskID]; ok {
		r.mu.RUnlock()
		return bb
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// double-check
	if bb, ok := r.boards[taskID]; ok {
		return bb
	}
	// LRU 淘汰：超出 maxSize 时移除最旧的条目
	if len(r.boards) >= r.maxSize && r.maxSize > 0 {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.boards, oldest)
	}
	bb := NewMemoryBlackboard()
	r.boards[taskID] = bb
	r.order = append(r.order, taskID)
	return bb
}

// Release 任务完成/失败/取消时清理，释放内存。
func (r *BlackboardRegistry) Release(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.boards, taskID)
	for i, id := range r.order {
		if id == taskID {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

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
