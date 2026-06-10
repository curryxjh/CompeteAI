# Plan: 多 Agent 共享记忆与长期记忆实施计划

更新时间：2026-06-09

---

## 总体策略

分三个阶段，优先级：**稳定性 > 召回质量 > 功能扩展**。

- **Phase 1（必做，阻塞性 Bug 修复）**：修复 MemoryBlackboard 共享问题，确保非 Redis 环境下多 Agent 能正确传递状态
- **Phase 2（核心功能，长期记忆真正生效）**：引入真实 Embedding、注入记忆 Prompt
- **Phase 3（质量提升，可选）**：乐观锁、用户级作用域、配置完善

---

## Phase 1：修复 MemoryBlackboard 共享（1~2 天）

### P1-1：新增 BlackboardRegistry

**文件**：`internal/state/blackboard.go`

```go
// 在文件末尾新增

// BlackboardRegistry 全局 MemoryBlackboard 注册表。
// 保证同一 taskID 的所有调用方共享同一 MemoryBlackboard 实例。
type BlackboardRegistry struct {
    mu     sync.RWMutex
    boards map[string]*MemoryBlackboard
    order  []string    // 按插入顺序记录，用于 LRU 淘汰
    maxSize int
}

var globalRegistry = &BlackboardRegistry{
    boards:  make(map[string]*MemoryBlackboard),
    maxSize: 1000,
}

func GlobalRegistry() *BlackboardRegistry { return globalRegistry }

func (r *BlackboardRegistry) GetOrCreate(taskID string) *MemoryBlackboard {
    r.mu.RLock()
    if bb, ok := r.boards[taskID]; ok {
        r.mu.RUnlock()
        return bb
    }
    r.mu.RUnlock()
    
    r.mu.Lock()
    defer r.mu.Unlock()
    if bb, ok := r.boards[taskID]; ok { // double-check
        return bb
    }
    // LRU 淘汰
    if len(r.boards) >= r.maxSize {
        oldest := r.order[0]
        r.order = r.order[1:]
        delete(r.boards, oldest)
    }
    bb := NewMemoryBlackboard()
    r.boards[taskID] = bb
    r.order = append(r.order, taskID)
    return bb
}

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
```

### P1-2：修改 NewBlackboard 使用 Registry

**文件**：`internal/state/redis.go`

```go
// 修改 NewBlackboard：
func NewBlackboard(useRedis bool, redisClient redis.Cmdable, taskID string) Blackboard {
    if settings.Conf != nil && settings.Conf.Mode == "prod" && (!useRedis || redisClient == nil) {
        panic("prod mode requires Redis blackboard; memory forbidden")
    }
    if useRedis && redisClient != nil {
        return NewRedisBlackboard(redisClient)
    }
    // 使用全局 Registry，保证同 taskID 拿到同一实例
    return GlobalRegistry().GetOrCreate(taskID)
}
```

### P1-3：Engine 任务结束时释放 Registry

**文件**：`internal/workflow/engine.go`（`completeTask`、`failTask`、`cancelTask` 方法末尾）

```go
// 在每个终态处理方法末尾添加：
if !e.useRedisBB {
    state.GlobalRegistry().Release(taskID)
}
```

### P1-4：验证

运行现有测试，确保 `state_test.go`、`dispatch_test.go` 全部通过：

```bash
go test ./internal/state/... -v
go test ./internal/workflow/... -v
```

**预期结果**：非 Redis 环境下，5 个 Agent 能依次从 Blackboard 读取上游产物，流程完整运行。

---

## Phase 2：长期记忆真正生效（3~4 天）

### P2-1：新增 VolcanoEmbedder

**文件**：`internal/memory/embedder.go`（新增实现，不删除 HashEmbedder）

```go
type VolcanoEmbedder struct {
    apiKey   string
    endpoint string
    model    string
    dim      int
    client   *http.Client
    fallback Embedder // 降级为 HashEmbedder
}

func NewVolcanoEmbedder(cfg *settings.EmbedderConfig) Embedder {
    if cfg == nil || cfg.APIKey == "" {
        return NewHashEmbedder(128) // 无配置时降级
    }
    return &VolcanoEmbedder{
        apiKey:   cfg.APIKey,
        endpoint: cfg.Endpoint,
        model:    cfg.Model,
        dim:      cfg.Dim,
        client:   &http.Client{Timeout: 10 * time.Second},
        fallback: NewHashEmbedder(cfg.Dim),
    }
}

func (e *VolcanoEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    // 分批处理，每批最多 32 条
    const batchSize = 32
    var result [][]float32
    for i := 0; i < len(texts); i += batchSize {
        end := i + batchSize
        if end > len(texts) { end = len(texts) }
        batch, err := e.embedBatch(ctx, texts[i:end])
        if err != nil {
            // 降级：对这批用 fallback
            fb, _ := e.fallback.Embed(ctx, texts[i:end])
            result = append(result, fb...)
            continue
        }
        result = append(result, batch...)
    }
    return result, nil
}

func (e *VolcanoEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    body, _ := json.Marshal(map[string]any{
        "model": e.model,
        "input": texts,
    })
    req, _ := http.NewRequestWithContext(ctx, "POST",
        e.endpoint+"/api/v3/embeddings", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+e.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := e.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    
    var result struct {
        Data []struct {
            Index     int       `json:"index"`
            Embedding []float32 `json:"embedding"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    vecs := make([][]float32, len(texts))
    for _, d := range result.Data {
        if d.Index < len(vecs) {
            vecs[d.Index] = d.Embedding
        }
    }
    return vecs, nil
}

func (e *VolcanoEmbedder) Dim() int { return e.dim }
```

### P2-2：MySQLRetriever 增加余弦相似度

**文件**：`internal/memory/retriever.go`

```go
// 新增余弦相似度函数
func cosineSimilarity(a, b []float32) float64 {
    if len(a) != len(b) { return 0 }
    var dot, normA, normB float64
    for i := range a {
        dot  += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }
    if normA == 0 || normB == 0 { return 0 }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// RetrieveFacts 改进：
// 1. embed query → queryVec
// 2. DB 粗筛 top-200 候选 facts
// 3. 读取 memory_embeddings 表中各 fact 的向量
// 4. 余弦相似度排序
// 5. 返回 top-Limit
```

### P2-3：NewService 使用 VolcanoEmbedder

**文件**：`internal/memory/service.go`

```go
func NewService(dbDao *dao.MemoryDao, cfg *settings.MemoryConfig) MemoryService {
    if cfg == nil || !cfg.Enabled {
        return NoopService{}
    }
    // 根据配置选择 Embedder
    var emb Embedder
    if cfg.Embedder != nil && cfg.Embedder.Type == "volcano" {
        emb = NewVolcanoEmbedder(cfg.Embedder)
    } else {
        emb = NewHashEmbedder(128)
    }
    // ... 其余不变
}
```

### P2-4：各 Agent 注入 MemoryPromptSuffix

**文件**：`internal/agent/collector.go`、`analyst.go`、`writer.go`、`qa.go`、`coordinator.go`

在每个 Agent 的 `Run()` 方法中，在构建 systemPrompt 前添加：

```go
func (a *Collector) Run(ctx context.Context, input RunInput, bb state.Blackboard) (RunOutput, error) {
    // ... 现有逻辑 ...
    
    // 在调用 LLM 之前，将记忆追加到 systemPrompt
    memorySuffix := MemoryPromptSuffix(input)  // 已有函数，直接调用
    systemPrompt := collectorSystemPrompt + memorySuffix
    
    // response, err := chat.Chat(ctx, systemPrompt, userPrompt)
    // ...
}
```

**注意**：若当前 Agent 的 system prompt 是硬编码字符串常量，拆出 `buildSystemPrompt(input RunInput) string` 函数，在其中追加 `MemoryPromptSuffix`。

### P2-5：开启 MemoryConfig 默认配置

**文件**：配置文件（`config.yaml` 或 `settings/`）

```yaml
memory:
  enabled: true
  project_id: "compete-ai"
  embedder:
    type: "volcano"
    api_key: "${VOLCANO_API_KEY}"
    endpoint: "https://ark.cn-beijing.volces.com"
    model: "doubao-embedding-large"
    dim: 1024
  write_policy:
    allow_pending_inference: true
    episode_on_completion_only: true
```

### P2-6：验证

```bash
# 1. 单元测试
go test ./internal/memory/... -v

# 2. 集成测试：创建任务，完成后查询 memory_facts 表
# 期望：memory_facts 表中有新增事实，confidence_score > 0
# 期望：memory_episodes 表中有新增 episode

# 3. 验证 Agent prompt 中有记忆内容
# 打开 DEBUG 日志，检查 systemPrompt 中是否包含 "历史知识" 段落
```

---

## Phase 3：乐观锁 + 用户作用域（1~2 天，可选）

### P3-1：WorkflowState 版本号 + 乐观锁

**文件**：`internal/state/types.go`（WorkflowState 新增 `Version int64`）

**文件**：`internal/state/task_board.go`（新增 `SaveWorkflowStateVersioned` 方法）

**文件**：`internal/state/redis.go`（Redis Lua 脚本实现原子 CAS）

```go
// taskStore 内 SaveWorkflowStateVersioned 实现：
func (s *taskStore) SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error {
    // Redis 模式：执行 Lua 脚本
    // 内存模式：加锁比较 version 字段
    wf.Version = expectedVersion + 1
    return s.bb.PutVersioned(ctx, s.key(keyWorkflowState), wf, expectedVersion)
}
```

**在 Engine 关键路径中替换**：

- `handleQAReject` 中修改 `wf.Round++` 后的写入
- `applyClarification` 中的写入

### P3-2：UserID 传入记忆作用域

**文件**：`internal/web/task.go`（从 JWT 中提取 userID，传入 TaskService）

**文件**：`internal/service/task_service.go`（签名扩展，传入 userID）

**文件**：`internal/workflow/dispatch.go`（`PrepareTask` 接收 userID，传入 `EnsureScopes`）

```go
// context key
type contextKeyUserID struct{}
func WithUserID(ctx context.Context, userID int64) context.Context {
    return context.WithValue(ctx, contextKeyUserID{}, userID)
}
func UserIDFromCtx(ctx context.Context) int64 {
    v, _ := ctx.Value(contextKeyUserID{}).(int64)
    return v
}
```

**在 `buildMemoryContext` 中取出 userID**：

```go
req := memory.BuildContextRequest{
    // ... 现有字段 ...
    UserID: state.UserIDFromCtx(ctx),
}
```

### P3-3：完善配置文件 settings 结构

**文件**：`settings/config.go`（或对应配置结构体文件）

新增字段：

```go
type MemoryConfig struct {
    Enabled       bool           `mapstructure:"enabled"`
    ProjectID     string         `mapstructure:"project_id"`
    WorkspaceID   string         `mapstructure:"workspace_id"`
    ExplicitFile  string         `mapstructure:"explicit_file"`
    Embedder      *EmbedderConfig `mapstructure:"embedder"`
    WritePolicy   WritePolicyConfig `mapstructure:"write_policy"`
    Retrieval     RetrievalConfig   `mapstructure:"retrieval"`
}

type EmbedderConfig struct {
    Type     string `mapstructure:"type"`      // "volcano" | "hash"
    APIKey   string `mapstructure:"api_key"`
    Endpoint string `mapstructure:"endpoint"`
    Model    string `mapstructure:"model"`
    Dim      int    `mapstructure:"dim"`
}

type RetrievalConfig struct {
    FactLimit     int `mapstructure:"fact_limit"`
    EvidenceLimit int `mapstructure:"evidence_limit"`
    EpisodeLimit  int `mapstructure:"episode_limit"`
    CandidatePool int `mapstructure:"candidate_pool"`
}
```

---

## 各 Phase 风险与应对

| Phase | 风险 | 应对策略 |
|---|---|---|
| P1 | Registry 内存泄漏（长任务不释放） | 实现 LRU 淘汰 + Release 在任务终态时调用 |
| P1 | 并发 GetOrCreate 竞态 | double-check locking（已包含） |
| P2 | Embedding API 限流/不可用 | 降级为 HashEmbedder，流程不中断 |
| P2 | 向量余弦计算 CPU 耗时 | 限制候选池 200 条，实测 < 5ms |
| P2 | 记忆内容过长导致 LLM 上下文超限 | TokenBudget 已有限制，FormatForPrompt 会截断 |
| P3 | 乐观锁导致重试风暴 | 最多重试 3 次，超出按原逻辑处理（不阻塞） |

---

## 交付物汇总

| Phase | 文件 | 变更描述 |
|---|---|---|
| P1 | `internal/state/blackboard.go` | 新增 `BlackboardRegistry` |
| P1 | `internal/state/redis.go` | `NewBlackboard` 使用 Registry |
| P1 | `internal/workflow/engine.go` | 任务终态调用 `Registry.Release` |
| P2 | `internal/memory/embedder.go` | 新增 `VolcanoEmbedder` |
| P2 | `internal/memory/retriever.go` | 余弦相似度检索 |
| P2 | `internal/memory/service.go` | 使用 VolcanoEmbedder |
| P2 | `internal/agent/collector.go` | 注入 MemoryPromptSuffix |
| P2 | `internal/agent/analyst.go` | 注入 MemoryPromptSuffix |
| P2 | `internal/agent/writer.go` | 注入 MemoryPromptSuffix |
| P2 | `internal/agent/qa.go` | 注入 MemoryPromptSuffix |
| P2 | `internal/agent/coordinator.go` | 注入 MemoryPromptSuffix |
| P2 | `config.yaml` | 启用 memory 配置 |
| P3 | `internal/state/types.go` | WorkflowState 版本号 |
| P3 | `internal/state/task_board.go` | 乐观锁写方法 |
| P3 | `internal/workflow/dispatch.go` | UserID 注入 context |
| P3 | `settings/config.go` | 完善配置结构体 |
