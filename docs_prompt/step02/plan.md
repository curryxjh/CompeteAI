# Plan: Writer/QA LLM 升级 + 基础设施补全实施计划

更新时间：2026-06-09

---

## 总体策略

分三个 Phase，优先级：**基础设施先行 → Agent 质量提升 → 测试覆盖**。

- **Phase 1（必做，1 天）**：init.sql 补全 + WorkflowConfig 扩展 + BlackboardRegistry 配置绑定
- **Phase 2（核心，2~3 天）**：Writer/QA Agent 接入 LLM + domain.Report 字段扩展
- **Phase 3（质量，1 天）**：WorkflowState 乐观锁完整实现 + 单元测试

---

## Phase 1：基础设施补全（1 天）

### P1-1：创建 scripts/init.sql

**文件**：`scripts/init.sql`（新建）

包含项目全部表的 DDL，逻辑分三段：

**段一：核心业务表**（users / tasks / reports / traces / outbox / message_logs / event_logs / checkpoints / dead_letters / worker_leases）

```sql
CREATE TABLE IF NOT EXISTS `tasks` (
  `id`            varchar(36)  NOT NULL,
  `title`         varchar(255) DEFAULT '',
  `competitors`   json         DEFAULT NULL,
  `dimensions`    json         DEFAULT NULL,
  `status`        varchar(20)  DEFAULT '' COMMENT 'pending/queued/running/...',
  `progress`      int          NOT NULL DEFAULT 0,
  `agent_states`  json         DEFAULT NULL,
  `error_message` text,
  `user_id`       bigint       NOT NULL DEFAULT 0 COMMENT '创建任务的用户 ID',
  `created_at`    datetime(3)  DEFAULT NULL,
  `updated_at`    datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tasks_status` (`status`),
  KEY `idx_tasks_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**段二：长期记忆表**（memory_scopes / memory_entities / memory_facts / memory_sources / memory_chunks / memory_embeddings / memory_claims / memory_episodes / memory_preferences / memory_events / memory_evidence_links）

关键索引（性能影响大的）：
- `memory_facts`：联合索引 `(scope_id, status, verification_status)`、`(subject, predicate)`
- `memory_chunks`：`(source_id)`、`(task_id)`
- `memory_embeddings`：`(object_type, object_id)`、`(scope_type, scope_key)`
- `memory_episodes`：`(scope_id)`

**段三：辅助表**（reports / traces）

写法要求：
- 所有表加 `IF NOT EXISTS`，幂等可重复执行
- 所有表 `CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
- 不含外键约束（GORM 不使用外键，保持性能）

### P1-2：WorkflowConfig + BlackboardRegistry 配置绑定

**文件**：`settings/settings.go`

```go
type WorkflowConfig struct {
    MaxAgentRetries           int  `mapstructure:"max_agent_retries"`
    MaxRounds                 int  `mapstructure:"max_rounds"`
    UseRedisBlackboard        bool `mapstructure:"use_redis_blackboard"`
    BlackboardRegistryMaxSize int  `mapstructure:"blackboard_registry_max_size"` // ← 新增
}
```

**文件**：`internal/app/bootstrap.go`，在 `BuildSharedDeps` 开头添加：

```go
// 设置内存 Blackboard 注册表上限（仅影响非 Redis 模式）
if cfg := settings.Conf.WorkflowConfig; cfg != nil && cfg.BlackboardRegistryMaxSize > 0 {
    state.GlobalRegistry().SetMaxSize(cfg.BlackboardRegistryMaxSize)
}
```

**文件**：`config/dev.yaml`，`workflow` 段新增：

```yaml
workflow:
  blackboard_registry_max_size: 1000
```

### P1-3：验证 P1

```bash
# docker-compose 启动 MySQL + Redis
docker-compose up mysql redis -d

# 等待健康检查通过后验证表是否存在
docker exec compete-ai-mysql mysql -uroot -pcompete_ai_2026 compete_ai \
  -e "SHOW TABLES;" | grep memory_facts

# go build 确认编译通过
cd /path/to/CompeteAI && go build ./...
```

---

## Phase 2：Writer / QA Agent LLM 化（2~3 天）

### P2-1：domain.Report 字段扩展

**文件**：`internal/domain/analysis.go`

在 `Report` 结构体末尾添加：
```go
KeyFindings     []string `json:"keyFindings,omitempty"`
Recommendations []string `json:"recommendations,omitempty"`
```

这两个字段是 `omitempty`，不影响已有测试和前端展示（前端可按需读取）。

### P2-2：Writer Agent LLM 化

**文件**：`internal/agent/writer.go`

**步骤 2-2-a**：修改结构体

```go
type Writer struct {
    chat ChatClient
}

func NewWriter(chat ChatClient) *Writer {
    return &Writer{chat: chat}
}
```

**步骤 2-2-b**：在 `Run()` 方法中，报告组装完成后，追加 LLM 调用：

```go
// 当 chat 可用时，调用 LLM 生成增强摘要和建议
if a.chat != nil {
    systemPrompt := writerSystemPrompt  // 常量，见 spec.md §1.2
    userPrompt := buildWriterUserPrompt(meta, analysis, coll, input)
    raw, err := a.chat.StreamChat(ctx, systemPrompt, userPrompt, func(et, c string) {
        if et == "content" { EmitProgress(ctx, "output", c, "running") }
    })
    if err == nil {
        var llmOut writerLLMOutput
        if parseJSONFromLLM(raw, &llmOut) == nil {
            if llmOut.Summary != ""          { report.Summary = llmOut.Summary }
            if len(llmOut.KeyFindings) > 0   { report.KeyFindings = llmOut.KeyFindings }
            if len(llmOut.Recommendations) > 0 { report.Recommendations = llmOut.Recommendations }
        }
    }
    // LLM 失败时 fallback 到 analysis.Summary，不阻塞
}
```

**步骤 2-2-c**：新增辅助函数

```go
const writerSystemPrompt = `你是竞品分析报告撰写专家。...（同 spec.md §1.2）`

type writerLLMOutput struct {
    Summary         string   `json:"summary"`
    KeyFindings     []string `json:"keyFindings"`
    Recommendations []string `json:"recommendations"`
}

func buildWriterUserPrompt(meta state.TaskMeta, analysis state.AnalysisOutput,
    coll state.CollectorOutput, input RunInput) string { ... }

func formatSWOTBrief(swot map[string]domain.SWOTAnalysis) string {
    // 每个竞品输出 2-3 个 strength keywords
}
```

### P2-3：QA Agent LLM 化

**文件**：`internal/agent/qa.go`

**步骤 2-3-a**：修改结构体

```go
type QA struct {
    chat ChatClient // nil = 纯规则模式
}

func NewQA(chat ChatClient) *QA {
    return &QA{chat: chat}
}
```

**步骤 2-3-b**：在 `Run()` 中，规则评分完成后，追加 LLM 辅助验证：

```go
llmScore := score  // 默认等于规则分
if a.chat != nil && score >= 60 {
    if ls, err := a.runLLMCheck(ctx, input, report, meta, score); err == nil {
        // 加权平均：规则 60% + LLM 40%
        llmScore = int(float64(score)*0.6 + float64(ls)*0.4)
    }
    score = llmScore
}
```

**步骤 2-3-c**：新增 `runLLMCheck` 方法：

```go
func (a *QA) runLLMCheck(ctx context.Context, input RunInput,
    report domain.Report, meta state.TaskMeta, ruleScore int) (int, error) {
    
    systemPrompt := qaLLMSystemPrompt  // 常量，见 spec.md §2.2
    userPrompt := buildQAUserPrompt(report, meta, ruleScore)
    
    raw, err := a.chat.Chat(ctx, systemPrompt, userPrompt)
    if err != nil {
        return ruleScore, err
    }
    var result qaLLMOutput
    if err := parseJSONFromLLM(raw, &result); err != nil {
        return ruleScore, err
    }
    // 将 LLM 发现的问题追加到 issues
    // ...
    return result.LLMScore, nil
}
```

### P2-4：更新 Agent Registry

**文件**：`internal/agent/registry.go`

```go
// 查找现有 NewRegistry 调用，将 NewWriter() 和 NewQA() 改为传入 deps.Chat
func NewRegistry(deps Deps) *Registry {
    r := &Registry{agents: make(map[domain.AgentName]Agent)}
    r.Register(NewCoordinator())
    r.Register(NewCollector(deps.Tools))
    r.Register(NewAnalyst(deps.Chat))
    r.Register(NewWriter(deps.Chat))   // ← 传入 Chat
    r.Register(NewQA(deps.Chat))       // ← 传入 Chat
    return r
}
```

**文件**：`internal/service/agent_wire.go`（如果有独立的 wire 逻辑，同步更新）

### P2-5：验证 P2

手动端到端验证（需配置 ARK_API_KEY）：

1. 创建任务 → 查看 Writer 输出的 `report.summary` 是否是 LLM 生成的洞察性文字（而非机械拼接）
2. QA 通过后，查看 `report.keyFindings` 和 `report.recommendations` 字段是否有内容
3. 人为制造低质量报告（清空 sources），验证 QA 的 LLM 检查能给出比规则更细致的问题描述

---

## Phase 3：乐观锁 + 单元测试（1 天）

### P3-1：TaskStore 接口扩展 + 实现

**文件**：`internal/state/task_board.go`

1. `TaskStore` 接口新增 `SaveWorkflowStateVersioned`（见 spec.md §5.1）
2. `taskStore` 实现内存版 CAS（见 spec.md §5.2）
3. 在 `internal/workflow/dispatch.go` 的 `handleQAReject` 中替换 `SaveWorkflowState` 为 `SaveWorkflowStateVersioned`，加最多 3 次重试

### P3-2：单元测试

**文件**：`internal/state/blackboard_test.go`（新建）

```go
package state_test
// 4 个测试用例：ShareBetweenCalls / Release / LRUEviction / Concurrent
```

**文件**：`internal/memory/embedder_test.go`（新建）

```go
package memory_test
// 4 个测试用例：FallbackOnAPIError / NilConfig / EmptyAPIKey / DimConsistency
```

**文件**：`internal/state/state_test.go`（追加）

```go
// 2 个测试用例：ConflictDetected / Sequential
```

### P3-3：全量回归

```bash
go test ./internal/state/... -v -race
go test ./internal/memory/... -v
go build ./...
go vet ./...
```

---

## 各 Phase 风险与应对

| Phase | 风险 | 应对策略 |
|---|---|---|
| P1 | init.sql 与 AutoMigrate 字段名不一致 | 以 GORM 字段名为准（snake_case），init.sql 同步对齐 |
| P1 | docker-compose init.sql 不执行（DB 已有数据） | init.sql 只在空库首次初始化，已有库用 AutoMigrate |
| P2 | Writer LLM 输出 JSON 解析失败 | `parseJSONFromLLM` 已有容错逻辑；失败时 fallback 到规则模式 |
| P2 | QA LLM 调用增加延迟（+3~8秒） | 仅当 `score >= 60` 时触发 LLM，低质量报告快速失败 |
| P2 | `NewWriter(nil)` / `NewQA(nil)` 兼容性 | chat 为 nil 时退化到原逻辑，所有测试用 nil chat 不依赖 LLM |
| P3 | 乐观锁重试风暴 | 最多 3 次，超出后继续原逻辑（不阻塞） |

---

## 交付物汇总

| Phase | 文件 | 变更描述 |
|---|---|---|
| P1 | `scripts/init.sql` | 新建，全量建表 DDL |
| P1 | `settings/settings.go` | WorkflowConfig 新增 BlackboardRegistryMaxSize |
| P1 | `internal/app/bootstrap.go` | 从配置设置 Registry maxSize |
| P1 | `config/dev.yaml` | 新增 blackboard_registry_max_size: 1000 |
| P2 | `internal/domain/analysis.go` | Report 新增 KeyFindings/Recommendations |
| P2 | `internal/agent/writer.go` | 接入 LLM 生成增强摘要和建议 |
| P2 | `internal/agent/qa.go` | 接入 LLM 辅助验证，加权评分 |
| P2 | `internal/agent/registry.go` | NewWriter/NewQA 传入 Chat |
| P3 | `internal/state/task_board.go` | SaveWorkflowStateVersioned |
| P3 | `internal/state/blackboard_test.go` | 新建，Registry 4 个测试 |
| P3 | `internal/memory/embedder_test.go` | 新建，Embedder 4 个测试 |
| P3 | `internal/state/state_test.go` | 追加乐观锁 2 个测试 |
