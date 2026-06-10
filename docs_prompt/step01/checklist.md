# Checklist: 多 Agent 共享记忆与长期记忆实施检查清单

更新时间：2026-06-09

---

## Phase 1：修复 MemoryBlackboard 共享

### P1-1 BlackboardRegistry 实现

- [ ] `internal/state/blackboard.go` 新增 `BlackboardRegistry` 结构体
  - [ ] 字段：`mu sync.RWMutex`、`boards map[string]*MemoryBlackboard`、`order []string`、`maxSize int`
  - [ ] 实现 `GetOrCreate(taskID string) *MemoryBlackboard`（含 double-check locking）
  - [ ] 实现 `Release(taskID string)`（从 boards 和 order 中删除）
  - [ ] 全局变量 `globalRegistry`，`maxSize` 默认 1000
  - [ ] 导出函数 `GlobalRegistry() *BlackboardRegistry`

- [ ] `internal/state/blackboard.go` 中 `MemoryBlackboard` 保持不变（接口兼容）

### P1-2 NewBlackboard 修改

- [ ] `internal/state/redis.go` 中 `NewBlackboard` 函数：
  - [ ] Redis 模式路径不变（`useRedis && redisClient != nil` → `NewRedisBlackboard`）
  - [ ] 内存模式改为 `GlobalRegistry().GetOrCreate(taskID)` 而非 `NewMemoryBlackboard()`
  - [ ] `taskID` 参数从 `_` 改为实际使用

### P1-3 Engine 释放 Registry

- [ ] `internal/workflow/engine.go` 的 `completeTask` 方法末尾：
  - [ ] 调用 `state.GlobalRegistry().Release(taskID)`（仅当 `!e.useRedisBB` 时）

- [ ] `internal/workflow/engine.go` 的 `failTask` 方法末尾：
  - [ ] 调用 `state.GlobalRegistry().Release(taskID)`（仅当 `!e.useRedisBB` 时）

- [ ] `internal/workflow/engine.go` 的 `cancelTask` 方法末尾：
  - [ ] 调用 `state.GlobalRegistry().Release(taskID)`（仅当 `!e.useRedisBB` 时）

### P1-4 测试验证

- [ ] `go test ./internal/state/...` 全部通过（无回归）
- [ ] 新增测试：`TestBlackboardRegistry_ShareBetweenCalls`
  - [ ] 同 taskID 两次 `GetOrCreate` 返回同一实例
  - [ ] `Release` 后再 `GetOrCreate` 返回新实例
  - [ ] maxSize=2 时插入第 3 个 taskID 会淘汰最旧的
- [ ] 新增集成测试（或手动验证）：不启用 Redis，创建任务 → Coordinator 写入 → Collector 能读取 `task:meta`
- [ ] `go test ./internal/workflow/...` 全部通过

---

## Phase 2：长期记忆真正生效

### P2-1 VolcanoEmbedder 实现

- [ ] `internal/memory/embedder.go` 新增 `VolcanoEmbedder` 结构体
  - [ ] 字段：`apiKey`、`endpoint`、`model`、`dim`、`client *http.Client`、`fallback Embedder`
  - [ ] 构造函数 `NewVolcanoEmbedder(cfg *settings.EmbedderConfig) Embedder`：cfg 为 nil 或 apiKey 空时返回 `HashEmbedder`
  - [ ] `Embed()` 方法：分批（每批 ≤ 32 条），单批失败时降级为 fallback
  - [ ] `embedBatch()` 私有方法：调用 POST `/api/v3/embeddings`，解析响应
  - [ ] `Dim()` 返回配置的维度
  - [ ] HTTP timeout = 10 秒

- [ ] `internal/memory/embedder.go` 保留原 `HashEmbedder`（不删除）

- [ ] 新增单元测试 `TestVolcanoEmbedder_FallbackOnError`：
  - [ ] Mock HTTP 返回 500 时，降级为 HashEmbedder，函数不报错
  - [ ] 返回向量数量 == 输入文本数量

### P2-2 MySQLRetriever 余弦相似度

- [ ] `internal/memory/retriever.go` 新增 `cosineSimilarity(a, b []float32) float64`
  - [ ] a/b 长度不一致返回 0
  - [ ] a 或 b 为零向量返回 0
  - [ ] 标准余弦公式：dot(a,b) / (||a|| × ||b||)

- [ ] `RetrieveFacts` 方法改进：
  - [ ] 先 embed req.Query 为 queryVec（失败时 fallback 到关键词搜索）
  - [ ] SQL 粗筛：按 `freshness_score DESC LIMIT 200`，JOIN memory_embeddings
  - [ ] 对每条候选计算 cosineSimilarity(queryVec, factVec)
  - [ ] 按相似度降序排列，取 top `req.Limit`（默认 10）
  - [ ] factVec 为空（未 embed）时相似度设为 0.1（不完全淘汰）

- [ ] `RetrieveEvidence` 方法同步改进（对 memory_chunks 表做相似度检索）

- [ ] 新增测试 `TestCosineSimilarity`：
  - [ ] 相同向量 → 1.0
  - [ ] 正交向量 → 0.0
  - [ ] 零向量 → 0.0

### P2-3 NewService 使用 VolcanoEmbedder

- [ ] `internal/memory/service.go` 中 `NewService`：
  - [ ] 根据 `cfg.Embedder.Type == "volcano"` 条件选择 `NewVolcanoEmbedder`
  - [ ] 其他情况保持 `NewHashEmbedder(128)`

### P2-4 Agent 注入 MemoryPromptSuffix

- [ ] `internal/agent/coordinator.go`：
  - [ ] 在 `Run()` 中找到构建 systemPrompt 的位置
  - [ ] 追加 `agent.MemoryPromptSuffix(input)`（非空时追加，空时跳过）
  - [ ] 已有的硬编码 prompt 字符串不修改，仅在末尾 concat

- [ ] `internal/agent/collector.go`：
  - [ ] 同上，在 LLM 调用前注入记忆后缀

- [ ] `internal/agent/analyst.go`：
  - [ ] 同上

- [ ] `internal/agent/writer.go`：
  - [ ] 同上

- [ ] `internal/agent/qa.go`：
  - [ ] 同上

- [ ] 确认 `agent.MemoryPromptSuffix` 调用 `memory.FormatForPrompt(input.Memory)` 正确
  - [ ] `input.Memory` 为空（`AgentMemoryContext{}`) 时 `FormatForPrompt` 返回空字符串
  - [ ] 空字符串不会追加到 systemPrompt

### P2-5 开启 MemoryConfig 默认配置

- [ ] 找到项目配置文件（`config.yaml` 或 `config/config.yaml`）
  - [ ] 新增 `memory` 段，`enabled: true`
  - [ ] 新增 `embedder` 子配置，`type: volcano`，`api_key` 读自环境变量 `${VOLCANO_API_KEY}`
  - [ ] 新增 `write_policy`：`allow_pending_inference: true`，`episode_on_completion_only: true`

- [ ] `settings/config.go`（或 `settings/` 目录下对应文件）中：
  - [ ] `MemoryConfig` 结构体新增 `Embedder *EmbedderConfig` 字段
  - [ ] 新增 `EmbedderConfig` 结构体（Type、APIKey、Endpoint、Model、Dim）
  - [ ] mapstructure tag 与 YAML 键名对齐

### P2-6 Phase 2 测试验证

- [ ] `go test ./internal/memory/... -v` 全部通过
- [ ] 集成验证（需要真实 MySQL）：
  - [ ] 创建任务，Collector 跑完后查询 `SELECT COUNT(*) FROM memory_facts WHERE status='active'` > 0
  - [ ] QA 通过后查询 `SELECT COUNT(*) FROM memory_episodes` > 0
- [ ] DEBUG 日志验证：
  - [ ] 开启 DEBUG 后，能在 Analyst systemPrompt 中看到 `## 历史知识` 段落（第二次相同竞品任务时）
- [ ] Embedding API 不可用时的降级验证：
  - [ ] 将 `VOLCANO_API_KEY` 设为无效值，任务仍能正常完成（只是记忆召回变为关键词匹配）

---

## Phase 3：乐观锁 + 用户级作用域（可选）

### P3-1 WorkflowState 乐观锁

- [ ] `internal/state/types.go` 中 `WorkflowState` 新增 `Version int64 json:"version"`
- [ ] `internal/state/task_board.go` 中 `TaskStore` 接口新增：
  - [ ] `SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error`
- [ ] `internal/state/task_board.go` 的 `taskStore` 实现 `SaveWorkflowStateVersioned`：
  - [ ] 内存模式：读取当前，比较 version，不符返回 `ErrVersionConflict`，符合则写新值（version+1）
  - [ ] Redis 模式：执行 Lua 脚本（原子 CAS）
- [ ] `internal/state/errors.go` 新增 `ErrVersionConflict`
- [ ] `internal/workflow/dispatch.go` 的 `handleQAReject` 中用 `SaveWorkflowStateVersioned` 替换 `SaveWorkflowState`
  - [ ] 失败时重试最多 3 次，超出记日志后继续原逻辑（不阻塞流程）
- [ ] 测试：`TestWorkflowStateVersioned`：
  - [ ] 并发 10 个 goroutine 写 version=0，只有 1 个成功
  - [ ] 成功后 version 变为 1

### P3-2 UserID 关联记忆作用域

- [ ] `internal/web/task.go` 中创建任务的 handler 从 JWT 中提取 `userID` 并注入 context
  - [ ] 使用 `state.WithUserID(ctx, userID)` 或类似函数
- [ ] `internal/workflow/dispatch.go` 的 `PrepareTask`：
  - [ ] 从 context 中取 `userID`
  - [ ] 传入 `memory.EnsureScopes(ctx, EnsureScopesRequest{UserID: userID, ...})`
- [ ] `internal/workflow/dispatch.go` 的 `buildMemoryContext`：
  - [ ] 从 context 取 `userID`，传入 `BuildContextRequest.UserID`
- [ ] 验证：创建两个任务（同一用户），第二个任务时 `BuildAgentContext` 返回的 `Preferences` 不为空

### P3-3 配置结构完善

- [ ] `settings/config.go` 中 `WorkflowConfig` 新增 `BlackboardRegistryMaxSize int`
- [ ] `BlackboardRegistry.maxSize` 从配置读取（默认 1000）
- [ ] `settings/config.go` 中 `RetrievalConfig` 结构体（FactLimit、EvidenceLimit、EpisodeLimit、CandidatePool）
- [ ] `MySQLRetriever` 中的硬编码 `200`（候选池大小）改为读配置

---

## 全局回归验证

- [ ] `go build ./...` 无编译错误
- [ ] `go vet ./...` 无警告
- [ ] `go test ./...` 全部通过（在无 Redis 环境下通过，无 Redis 的测试用内存 BB）
- [ ] 人工端到端测试：
  - [ ] 创建竞品分析任务（无 Redis）→ 5 个 Agent 依次运行 → 报告生成成功
  - [ ] 创建竞品分析任务（有 Redis）→ 5 个 Agent 依次运行 → 报告生成成功
  - [ ] 相同竞品第二次任务 → systemPrompt 中包含历史知识
  - [ ] QA 打回后重跑 → Blackboard 数据不丢失
