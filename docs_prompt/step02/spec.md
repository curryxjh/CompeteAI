# Spec: Writer/QA LLM 升级 + 基础设施补全技术规格

更新时间：2026-06-09

---

## 1. Writer Agent LLM 化规格

### 1.1 结构体变更

```go
// internal/agent/writer.go

// 修改前
type Writer struct{}

// 修改后
type Writer struct {
    chat ChatClient
}

func NewWriter(chat ChatClient) *Writer {
    return &Writer{chat: chat}
}
```

### 1.2 LLM 调用 System Prompt

Writer 的 system prompt 固定如下，要求输出严格 JSON：

```
你是竞品分析报告撰写专家。基于提供的分析结果，撰写高质量竞品分析报告。
输出严格 JSON，字段：
{
  "summary": "200-400字执行摘要，综合全部竞品对比的核心洞察",
  "keyFindings": ["发现1", "发现2", "发现3"],
  "recommendations": ["建议1", "建议2"]
}
不要 markdown 代码块，直接输出 JSON。
```

### 1.3 User Prompt 构建规格

```go
userPrompt := fmt.Sprintf(`
任务：%s
竞品：%s

分析结果：
- SWOT 覆盖：%d 个竞品
- 功能矩阵：%d 行
- 定价对比：%d 组
- 用户画像：%d 组

执行摘要草稿：%s

SWOT 摘要：
%s

来源数量：%d 个
%s`,
    meta.Title,
    strings.Join(meta.Competitors, " vs "),
    len(analysis.SWOT), len(analysis.Features),
    len(analysis.Pricing), len(analysis.Personas),
    analysis.Summary,
    formatSWOTBrief(analysis.SWOT),
    len(coll.Sources),
    MemoryPromptSuffix(input),
)
```

### 1.4 输出合并规格

LLM 生成的字段**叠加**到已有 `domain.Report`，不替换分析字段：

```go
// 解析 LLM 输出
var llmOut struct {
    Summary         string   `json:"summary"`
    KeyFindings     []string `json:"keyFindings"`
    Recommendations []string `json:"recommendations"`
}
if err := parseJSONFromLLM(raw, &llmOut); err == nil {
    if llmOut.Summary != "" {
        report.Summary = llmOut.Summary
    }
    if len(llmOut.KeyFindings) > 0 {
        report.KeyFindings = llmOut.KeyFindings
    }
    if len(llmOut.Recommendations) > 0 {
        report.Recommendations = llmOut.Recommendations
    }
}
// LLM 失败时用 analysis.Summary 作为 fallback，不阻塞流程
```

### 1.5 domain.Report 字段扩展

```go
// internal/domain/analysis.go
type Report struct {
    // 现有字段不变
    TaskID      string                         `json:"taskId"`
    Title       string                         `json:"title"`
    GeneratedAt string                         `json:"generatedAt"`
    QAScore     int                            `json:"qaScore"`
    Summary     string                         `json:"summary"`
    SWOT        map[string]SWOTAnalysis        `json:"swot"`
    Features    []FeatureRow                   `json:"features"`
    Pricing     []PricingInfo                  `json:"pricing"`
    Personas    []UserPersona                  `json:"personas"`
    Sources     map[string]SourceRef           `json:"sources"`

    // 新增：LLM 生成的高质量洞察字段
    KeyFindings     []string `json:"keyFindings,omitempty"`
    Recommendations []string `json:"recommendations,omitempty"`
}
```

---

## 2. QA Agent LLM 辅助验证规格

### 2.1 结构体变更

```go
// internal/agent/qa.go

// 修改后
type QA struct {
    chat ChatClient // 可为 nil，nil 时退化为纯规则模式
}

func NewQA(chat ChatClient) *QA {
    return &QA{chat: chat}
}
```

### 2.2 LLM Claim 验证逻辑

仅当 `a.chat != nil` 且规则评分 >= 60 分时触发（避免明显不合格的报告浪费 LLM 调用）：

```go
// LLM 辅助验证的 system prompt
llmSystemPrompt := `你是竞品分析质检专家。评估报告质量，输出严格 JSON：
{
  "llm_score": 85,           // 0-100 主观质量分
  "issues": [               // 发现的问题（可为空数组）
    {
      "location": "swot.Cursor.strengths[0]",
      "problem": "结论缺少数据支撑",
      "severity": "medium"  // high/medium/low
    }
  ],
  "overall": "报告整体评价"
}`
```

### 2.3 最终分数计算规格

```
finalScore = ruleScore * 0.6 + llmScore * 0.4
```

- `ruleScore`：现有规则评分（字段完整性、SWOT 覆盖等）
- `llmScore`：LLM 主观质量分
- 当 `chat == nil` 时：`finalScore = ruleScore`

### 2.4 LLM User Prompt 规格

```go
userPrompt := fmt.Sprintf(`
报告标题：%s
竞品：%s

执行摘要：%s

SWOT（共 %d 个竞品）：
%s

功能矩阵（%d 行）：略

来源（%d 个）：
%s

请评估此报告的质量。`,
    report.Title,
    strings.Join(meta.Competitors, ", "),
    report.Summary,
    len(report.SWOT),
    formatSWOTBrief(report.SWOT),  // 复用 Writer 中的辅助函数
    len(report.Features),
    len(report.Sources),
    formatSourcesBrief(report.Sources),
)
```

---

## 3. scripts/init.sql 规格

文件路径：`scripts/init.sql`

包含以下表的 `CREATE TABLE IF NOT EXISTS`，字符集 `utf8mb4`：

```sql
-- 业务表
CREATE TABLE IF NOT EXISTS `users` (...);
CREATE TABLE IF NOT EXISTS `tasks` (
  `id` varchar(36) NOT NULL,
  `title` varchar(255) DEFAULT '',
  `competitors` json DEFAULT NULL,
  `dimensions` json DEFAULT NULL,
  `status` varchar(20) DEFAULT '',
  `progress` int DEFAULT 0,
  `agent_states` json DEFAULT NULL,
  `error_message` text,
  `user_id` bigint DEFAULT 0,           -- step01 新增
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tasks_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- reports, traces, event_logs, message_logs 等...

-- 记忆表（9张）
CREATE TABLE IF NOT EXISTS `memory_scopes` (...);
CREATE TABLE IF NOT EXISTS `memory_entities` (...);
CREATE TABLE IF NOT EXISTS `memory_facts` (...);
CREATE TABLE IF NOT EXISTS `memory_sources` (...);
CREATE TABLE IF NOT EXISTS `memory_chunks` (...);
CREATE TABLE IF NOT EXISTS `memory_embeddings` (...);
CREATE TABLE IF NOT EXISTS `memory_claims` (...);
CREATE TABLE IF NOT EXISTS `memory_episodes` (...);
CREATE TABLE IF NOT EXISTS `memory_preferences` (...);
CREATE TABLE IF NOT EXISTS `memory_events` (...);
```

**原则**：init.sql 是 docker-compose 首次启动时的初始化 SQL，线上环境用 GORM AutoMigrate 追加列（两者不冲突）。

---

## 4. WorkflowConfig 扩展规格

```go
// settings/settings.go
type WorkflowConfig struct {
    MaxAgentRetries           int  `mapstructure:"max_agent_retries"`
    MaxRounds                 int  `mapstructure:"max_rounds"`
    UseRedisBlackboard        bool `mapstructure:"use_redis_blackboard"`
    BlackboardRegistryMaxSize int  `mapstructure:"blackboard_registry_max_size"` // ← 新增
}
```

**bootstrap.go 设置规格**：

```go
// internal/app/bootstrap.go BuildSharedDeps 函数内
if cfg := settings.Conf.WorkflowConfig; cfg != nil {
    if cfg.BlackboardRegistryMaxSize > 0 {
        state.GlobalRegistry().SetMaxSize(cfg.BlackboardRegistryMaxSize)
    }
}
```

---

## 5. WorkflowState 乐观锁完整规格

### 5.1 TaskStore 接口新增方法

```go
// internal/state/task_board.go
type TaskStore interface {
    // ... 现有方法不变 ...
    
    // SaveWorkflowStateVersioned 带版本校验的写入。
    // expectedVersion 为 0 表示首次写入（任意写）。
    // 当前版本 ≠ expectedVersion 时返回 ErrVersionConflict。
    SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error
}
```

### 5.2 内存实现规格

```go
func (s *taskStore) SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error {
    // 读取当前版本
    var cur WorkflowState
    err := s.bb.Get(ctx, WorkflowStateKey(s.taskID), &cur)
    if err != nil && expectedVersion != 0 {
        return ErrVersionConflict
    }
    if err == nil && int64(cur.Version) != expectedVersion {
        return ErrVersionConflict
    }
    // 版本号自增后写入
    wf.Version = int(expectedVersion + 1)
    if wf.MaxRounds == 0 {
        wf.MaxRounds = defaultMaxRounds
    }
    return s.bb.Put(ctx, WorkflowStateKey(s.taskID), wf)
}
```

### 5.3 Redis 实现规格（Lua 原子 CAS）

`RedisBlackboard` 新增方法 `PutVersioned`，用 Lua 保证原子读-比较-写：

```lua
local key = KEYS[1]
local expected = tonumber(ARGV[1])
local newValue = ARGV[2]
local ttl = tonumber(ARGV[3])
local raw = redis.call('GET', key)
if raw == false then
    if expected ~= 0 then return 0 end
    redis.call('SET', key, newValue, 'EX', ttl)
    return 1
end
local ok, obj = pcall(cjson.decode, raw)
if not ok then return 0 end
if (obj.version or 0) ~= expected then return 0 end
redis.call('SET', key, newValue, 'EX', ttl)
return 1
```

---

## 6. 单元测试规格

### 6.1 BlackboardRegistry 测试

```go
// internal/state/blackboard_test.go

func TestBlackboardRegistry_ShareBetweenCalls(t *testing.T)
// 验证：同 taskID 两次 GetOrCreate 返回同一实例

func TestBlackboardRegistry_Release(t *testing.T)
// 验证：Release 后再 GetOrCreate 返回新实例（指针不同）

func TestBlackboardRegistry_LRUEviction(t *testing.T)
// 验证：maxSize=2 时插入第 3 个 taskID 淘汰最旧的

func TestBlackboardRegistry_Concurrent(t *testing.T)
// 验证：100 个并发 GetOrCreate 同一 taskID，只创建 1 个实例
```

### 6.2 VolcanoEmbedder 测试

```go
// internal/memory/embedder_test.go

func TestVolcanoEmbedder_FallbackOnAPIError(t *testing.T)
// Mock HTTP 返回 500，验证降级为 HashEmbedder，结果向量数 == 输入数

func TestVolcanoEmbedder_NilConfig_ReturnsHashEmbedder(t *testing.T)
// cfg 为 nil 时构造函数返回 HashEmbedder

func TestVolcanoEmbedder_EmptyAPIKey_ReturnsHashEmbedder(t *testing.T)
// apiKey 为空串时构造函数返回 HashEmbedder

func TestHashEmbedder_DimConsistency(t *testing.T)
// 不同文本 Embed 结果维度一致
```

### 6.3 WorkflowState 乐观锁测试

```go
// internal/state/state_test.go（新增用例）

func TestSaveWorkflowStateVersioned_ConflictDetected(t *testing.T)
// 并发 10 goroutine 写 version=0，只有 1 个成功

func TestSaveWorkflowStateVersioned_Sequential(t *testing.T)
// 顺序写入：v0→v1→v2，每次成功后 version 自增
```

---

## 7. 接口兼容约束

1. `NewWriter(chat ChatClient)` / `NewQA(chat ChatClient)` 签名变更，需同步更新 `registry.go` 和 `agent_wire.go`
2. `chat` 参数允许为 `nil`（Writer fallback 到无 LLM 模式，QA 退化为纯规则）— 保证测试环境可不依赖 LLM 服务
3. `domain.Report` 新增 `KeyFindings` / `Recommendations` 为 `omitempty`，不影响现有 JSON 序列化
4. `TaskStore.SaveWorkflowStateVersioned` 为新增方法，现有代码不调用它，不产生接口破坏
5. `scripts/init.sql` 与 GORM AutoMigrate 共存：init.sql 用于 docker-compose 首次初始化，AutoMigrate 用于增量迁移
