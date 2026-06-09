# Spec: 多 Agent 共享记忆与长期记忆系统技术规格

更新时间：2026-06-09

---

## 1. 模块一：Blackboard 共享修复

### 1.1 BlackboardRegistry（全局单例注册表）

**目的**：保证同一 taskID 的所有 Agent 调用拿到同一个 `MemoryBlackboard` 实例，防止内存模式下每次 `NewBlackboard()` 返回空实例导致状态丢失。

**接口规格**：

```go
// internal/state/blackboard.go 新增

// BlackboardRegistry 管理 MemoryBlackboard 的生命周期。
// Redis 模式无需此 Registry（Redis 本身是全局共享的）。
type BlackboardRegistry struct {
    mu     sync.RWMutex
    boards map[string]*MemoryBlackboard
}

var globalRegistry = &BlackboardRegistry{
    boards: make(map[string]*MemoryBlackboard),
}

// GetOrCreate 返回 taskID 对应的 MemoryBlackboard，不存在则创建。
func (r *BlackboardRegistry) GetOrCreate(taskID string) *MemoryBlackboard

// Release 任务完成/失败时清理，释放内存。
func (r *BlackboardRegistry) Release(taskID string)

// GlobalRegistry 获取全局单例。
func GlobalRegistry() *BlackboardRegistry
```

**`NewBlackboard` 修改规格**：

```go
// 修改前（每次返回新实例）：
//   return NewMemoryBlackboard()

// 修改后（从 Registry 获取共享实例）：
func NewBlackboard(useRedis bool, redisClient redis.Cmdable, taskID string) Blackboard {
    if useRedis && redisClient != nil {
        return NewRedisBlackboard(redisClient)
    }
    return GlobalRegistry().GetOrCreate(taskID)
}
```

**生命周期约束**：

- 任务完成（`TaskStatusCompleted`）后 → `GlobalRegistry().Release(taskID)`
- 任务失败（`TaskStatusFailed`）后 → `GlobalRegistry().Release(taskID)`
- 任务取消（`TaskStatusCancelled`）后 → `GlobalRegistry().Release(taskID)`
- Registry 内最大驻留数不超过 1000 个 task（防 OOM），可用 LRU 策略淘汰老任务

---

### 1.2 WorkflowState 乐观锁

**目的**：防止多 Worker 并发修改 `workflow:state` 时相互覆盖。

**数据结构规格**：

```go
// internal/state/types.go 修改

type WorkflowState struct {
    // 现有字段保持不变
    TraceID               string `json:"traceId"`
    CurrentAgent          string `json:"currentAgent"`
    NextAgent             string `json:"nextAgent"`
    Round                 int    `json:"round"`
    MaxRounds             int    `json:"maxRounds"`
    RejectionCount        int    `json:"rejectionCount"`
    NeedsClarification    bool   `json:"needsClarification"`
    ClarificationResolved bool   `json:"clarificationResolved"`
    QAQueryAttempts       int    `json:"qaQueryAttempts"`

    // 新增：版本号，用于乐观锁
    Version int64 `json:"version"`
}
```

**乐观锁接口规格（`TaskStore` 扩展）**：

```go
// internal/state/task_board.go 新增方法

type TaskStore interface {
    // ... 现有方法保持不变 ...

    // SaveWorkflowStateVersioned 带版本检查的写入。
    // expectedVersion=0 表示首次写入（允许任意写）。
    // 若当前版本 != expectedVersion，返回 ErrVersionConflict。
    SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error
}

// ErrVersionConflict 版本冲突错误。
var ErrVersionConflict = errors.New("workflow state version conflict")
```

**Redis 实现规格**（使用 Lua 脚本保证原子性）：

```lua
-- 乐观锁 Lua 脚本
local key = KEYS[1]
local expected = tonumber(ARGV[1])
local newValue = ARGV[2]
local current = redis.call('GET', key)
if current == false then
    -- key 不存在，允许首次写入
    redis.call('SET', key, newValue, 'EX', 86400)
    return 1
end
local parsed = cjson.decode(current)
if parsed.version ~= expected then
    return 0  -- 版本冲突
end
redis.call('SET', key, newValue, 'EX', 86400)
return 1
```

**内存实现规格**：

- `MemoryBlackboard` 内部对 `workflow:state` 的写操作加互斥锁
- 比较 JSON 中的 `version` 字段，不匹配返回 `ErrVersionConflict`

---

### 1.3 Blackboard 事件通知（可选，第三阶段）

**目的**：Agent 可订阅特定 key 的变更，替代轮询。

规格：

```go
// 不在本期实现，记录于此作为后续扩展方向
type BlackboardWatcher interface {
    Watch(ctx context.Context, keyPattern string) (<-chan BlackboardEvent, error)
}
```

---

## 2. 模块二：真实 Embedding 接口

### 2.1 VolcanoEmbedder（对接豆包/火山引擎文本向量化）

**接口保持不变**（`Embedder` 接口已有，仅新增实现）：

```go
// internal/memory/embedder.go 新增

type VolcanoEmbedder struct {
    apiKey   string
    endpoint string // 豆包 embedding 接口地址
    model    string // 模型名，如 "doubao-embedding"
    dim      int    // 向量维度，如 1024
    client   *http.Client
}

func NewVolcanoEmbedder(apiKey, endpoint, model string, dim int) *VolcanoEmbedder

// Embed 调用火山引擎 embedding API，返回文本向量列表。
// 单批次最多 32 条文本，超出自动分批。
func (e *VolcanoEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error)

func (e *VolcanoEmbedder) Dim() int
```

**HTTP 请求规格**：

```
POST {endpoint}/api/v3/embeddings
Authorization: Bearer {apiKey}
Content-Type: application/json

{
  "model": "doubao-embedding",
  "input": ["text1", "text2", ...]
}
```

**响应解析**：

```json
{
  "data": [
    {"index": 0, "embedding": [0.1, 0.2, ...], "object": "embedding"},
    ...
  ]
}
```

**降级策略**：

- Embedding API 调用失败时，降级为 `HashEmbedder` 保证流程不中断
- 失败记录到 metrics：`memory_embed_error_total`

---

### 2.2 余弦相似度向量检索（MySQLRetriever 增强）

**当前问题**：`MySQLRetriever` 用 LIKE 关键词搜索，没有语义相似度。

**改进规格**：

```go
// internal/memory/retriever.go 修改

// cosineSimilarity 计算两个向量的余弦相似度（CPU 端）。
func cosineSimilarity(a, b []float32) float64

// RetrieveFacts 改进逻辑：
// 1. 将 req.Query embed 成向量
// 2. 从 MySQL 拉取候选 Fact（先通过 subject/predicate 关键词粗筛，最多 200 条）
// 3. 读取每条 Fact 对应的 memory_embeddings 表中的向量
// 4. 计算 cosine similarity，按分数降序排列
// 5. 取 top-K（req.Limit，默认 10）返回
```

**数据库查询规格**：

```sql
-- 粗筛（保留关键词命中 OR 近期活跃）
SELECT f.*, e.embedding_json
FROM memory_facts f
LEFT JOIN memory_embeddings e ON e.object_type = 'fact' AND e.object_id = f.id
WHERE f.scope_id IN (?)
  AND f.status = 'active'
  AND f.verification_status IN ('verified', 'pending')
ORDER BY f.last_seen_at DESC
LIMIT 200
```

**性能约束**：

- 200 条候选 × 1024 维向量的 cosine 计算耗时 < 5ms（Go CPU 端）
- 若候选超过 200 条，优先取 freshness_score 最高的 200 条

---

## 3. 模块三：Agent Prompt 记忆注入

### 3.1 记忆格式化规格

`memory.FormatForPrompt()` 已在 `internal/memory/format.go` 存在，规格如下（确认符合要求）：

```
## 历史知识（来自长期记忆）

### 偏好设置
- report_language: zh-CN

### 相关事实（置信度排序）
- [competitor: Cursor] 支持 AI 代码补全，置信度 0.95 ✓已验证
- [competitor: GitHub Copilot] 月费 $10/用户，置信度 0.87

### 相关经验（历史任务）
- 任务「Cursor vs Copilot」已完成，QA 评分 85 分
  关键教训：SWOT 弱点需要引用数据来源

### 证据片段
- [github.com] "GitHub Copilot now supports..."
```

**Token 预算约束**（已在 `AgentTokenBudget` 中定义，确认生效）：

| Agent | Token 预算 |
|---|---|
| coordinator | 400 |
| collector | 500 |
| analyst | 1800 |
| writer | 1200 |
| qa | 1500 |

### 3.2 各 Agent 注入位置规格

每个 Agent 的 `Run()` 方法中，在构建 system prompt 时追加记忆后缀：

```go
// 统一模式（每个 Agent Run() 方法内）：
memorySuffix := agent.MemoryPromptSuffix(input)
systemPrompt := buildBaseSystemPrompt(...) + memorySuffix
```

**注入规则**：

- `memorySuffix` 为空时不追加（避免冗余文本）
- 记忆内容放在 system prompt 末尾，base prompt 在前
- 不修改 user prompt 内容

**各 Agent 注入时机**：

| Agent | 注入时机 | 主要使用的记忆类型 |
|---|---|---|
| Coordinator | 任务规划时 | 偏好（报告语言、输出格式） + 经验（历史任务成功经验） |
| Collector | 搜索前 | 实体信息（竞品已知别名）+ 历史采集经验 |
| Analyst | 分析前 | 事实（竞品已知数据）+ 历史分析结论 |
| Writer | 撰写前 | 偏好（报告风格）+ 历史报告质量教训 |
| QA | 质检前 | 历史常见问题类型 + 事实核验基准 |

---

## 4. 模块四：用户级记忆作用域关联

### 4.1 UserID 传递链路规格

```
HTTP 请求 → TaskController → TaskService.Create(userID)
                              → Engine.PrepareTask(ctx, task, traceID)
                                → memory.EnsureScopes(ctx, EnsureScopesRequest{
                                      UserID: userID,  // ← 当前缺失，需补全
                                      ...
                                  })
```

**`TaskService.Create` 签名扩展**：

```go
// internal/service/task_service.go
func (s *TaskService) Create(ctx context.Context, userID int64, req CreateTaskRequest) (domain.Task, error)
```

**`Engine.PrepareTask` 扩展**：

```go
// internal/workflow/dispatch.go
func (e *Engine) PrepareTask(ctx context.Context, task domain.Task, traceID string, userID int64) (string, error)
```

### 4.2 BuildContextRequest 补全规格

```go
// 在 buildMemoryContext() 中补全用户信息
req := memory.BuildContextRequest{
    TaskID:      taskID,
    Agent:       agentName,
    TriggerType: trigger,
    Competitors: meta.Competitors,
    Dimensions:  meta.Dimensions,
    Query:       meta.Title,
    ProjectID:   defaultMemoryProjectID(),
    UserID:      userIDFromCtx(ctx),  // ← 从 context 中取，需注入
}
```

---

## 5. 模块五：配置规格

### 5.1 MemoryConfig 完整 YAML 规格

```yaml
memory:
  enabled: true                  # 是否启用长期记忆
  project_id: "compete-ai"       # 项目级作用域标识
  workspace_id: "default"        # 工作空间标识
  explicit_file: ""              # 显式偏好文件路径（可选）
  
  # Embedding 配置
  embedder:
    type: "volcano"              # volcano | hash (降级)
    api_key: "${VOLCANO_API_KEY}"
    endpoint: "https://ark.cn-beijing.volces.com"
    model: "doubao-embedding-large"
    dim: 1024
  
  # 写入策略
  write_policy:
    allow_pending_inference: true        # 是否保存 pending 推断事实
    episode_on_completion_only: true     # 仅任务完成时记录 episode
    min_confidence_threshold: 0.5        # 最低置信度
  
  # 检索策略
  retrieval:
    fact_limit: 10               # 每次最多检索多少条事实
    evidence_limit: 5            # 每次最多检索多少条证据
    episode_limit: 3             # 每次最多检索多少条经验
    candidate_pool: 200          # 粗筛候选池大小
  
  # Blackboard 配置（与 workflow 配置对齐）
  # 见 workflow.use_redis_blackboard
```

### 5.2 WorkflowConfig 补全规格

```yaml
workflow:
  max_agent_retries: 3
  max_rounds: 2
  use_redis_blackboard: true     # 生产必须 true；false 时使用全局 Registry
  blackboard_registry_max_size: 1000  # ← 新增：内存注册表最大 task 数
```

---

## 6. 接口兼容性约束

1. `Blackboard` 接口定义**不变**（`Put/Get/Delete/ListByPrefix`）
2. `TaskStore` 接口**向后兼容**：新方法 `SaveWorkflowStateVersioned` 为可选实现，`taskStore` 结构体实现默认行为
3. `MemoryService` 接口**不变**
4. `RunInput.Memory` 字段已存在，无需变更 Agent 接口
5. 所有现有单元测试必须通过（不得破坏现有 `state_test.go`、`memory_test.go`）
