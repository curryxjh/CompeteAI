# Checklist: Writer/QA LLM 升级 + 基础设施补全实施清单

更新时间：2026-06-09

---

## Phase 1：基础设施补全

### P1-1 scripts/init.sql 创建

- [ ] 创建 `scripts/` 目录
- [ ] 创建 `scripts/init.sql` 文件
  - [ ] 文件头：`SET NAMES utf8mb4; USE compete_ai;`
  - [ ] `users` 表：id(bigint auto_increment), email(varchar 128 unique), username(varchar 64), password(varchar 255), created_at, updated_at
  - [ ] `tasks` 表：包含 step01 新增的 `user_id bigint default 0`，INDEX(user_id)，INDEX(status)
  - [ ] `reports` 表：task_id(varchar 36 unique), payload(longtext), created_at, updated_at
  - [ ] `traces` 表：task_id(varchar 36 unique), payload(longtext), created_at, updated_at
  - [ ] `outbox_messages` 表：id(bigint auto_increment), aggregate_type, aggregate_id, payload(longtext), status, created_at, updated_at
  - [ ] `message_logs` 表
  - [ ] `event_logs` 表
  - [ ] `task_checkpoints` 表
  - [ ] `worker_leases` 表
  - [ ] `dead_letters` 表
  - [ ] `memory_scopes` 表：id(varchar 36 pk), scope_type(varchar 32), scope_key(varchar 128), display_name, owner_user_id(bigint), project_id(varchar 64), UNIQUE(scope_type, scope_key)
  - [ ] `memory_entities` 表：id(varchar 36 pk), canonical_name(varchar 255), entity_type(varchar 64), normalized_name(varchar 255), aliases_json(json), confidence_score(decimal 5,4), status(varchar 32 default 'active'), UNIQUE(entity_type, normalized_name)
  - [ ] `memory_facts` 表：id(varchar 36 pk), scope_id(varchar 36), entity_id(varchar 36), fact_type(varchar 64), subject(varchar 255), predicate(varchar 128), object_value_json(json), object_text(text), normalized_hash(varchar 64), summary(text), confidence_score(decimal 5,4), freshness_score(decimal 5,4), importance_score(decimal 5,4), source_count(int), inference_type(varchar 32), verification_status(varchar 32), status(varchar 32 default 'active'), created_by_task_id(varchar 64), updated_by_task_id(varchar 64), UNIQUE(scope_id, normalized_hash), INDEX(scope_id, status, verification_status), INDEX(subject, predicate)
  - [ ] `memory_sources` 表：id(varchar 36 pk), task_id(varchar 64), source_url(text), source_domain(varchar 255), title(varchar 512), excerpt(text), content_hash(varchar 64 unique), collected_at(datetime), reliability_tier(varchar 32), metadata_json(json), INDEX(task_id), INDEX(source_domain)
  - [ ] `memory_chunks` 表：id(varchar 36 pk), source_id(varchar 36), task_id(varchar 64), chunk_index(int), content_text(text), token_count(int), metadata_json(json), embedding_json(mediumtext), INDEX(source_id), INDEX(task_id)
  - [ ] `memory_embeddings` 表：id(varchar 36 pk), object_type(varchar 64), object_id(varchar 36), scope_type(varchar 32), scope_key(varchar 128), content_text(text), embedding_json(mediumtext), INDEX(object_type, object_id), INDEX(scope_type, scope_key)
  - [ ] `memory_claims` 表：id(varchar 36 pk), task_id(varchar 64), agent_name(varchar 64), claim_text(text), claim_hash(varchar 64), claim_type(varchar 64), verification_status(varchar 32 default 'pending'), qa_result(varchar 32), fact_id(varchar 36), INDEX(task_id), UNIQUE(claim_hash)
  - [ ] `memory_episodes` 表：id(varchar 36 pk), scope_id(varchar 36), task_id(varchar 64), title(varchar 512), query_text(text), summary(text), competitors_json(json), dimensions_json(json), outcome(varchar 32), qa_score(int), lessons_json(json), INDEX(scope_id), INDEX(task_id)
  - [ ] `memory_preferences` 表：id(varchar 36 pk), scope_id(varchar 36), pref_key(varchar 255), pref_value_json(json), priority(int default 100), source_type(varchar 64), status(varchar 32 default 'active'), INDEX(scope_id), UNIQUE(scope_id, pref_key)
  - [ ] `memory_events` 表：id(varchar 36 pk), aggregate_type(varchar 64), aggregate_id(varchar 36), event_type(varchar 64), payload_json(json), created_by_task_id(varchar 64), created_at(datetime), INDEX(aggregate_type, aggregate_id)
  - [ ] `memory_evidence_links` 表：id(varchar 36 pk), fact_id(varchar 36), source_id(varchar 36), chunk_id(varchar 36), support_score(decimal 5,4), quote_text(text), INDEX(fact_id), INDEX(source_id)
  - [ ] 所有表加 `IF NOT EXISTS`，`ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

- [ ] 验证：`docker-compose up mysql -d` 后，`SHOW TABLES` 显示所有表

### P1-2 WorkflowConfig 扩展

- [ ] `settings/settings.go` 中 `WorkflowConfig` 新增 `BlackboardRegistryMaxSize int mapstructure:"blackboard_registry_max_size"`
- [ ] `config/dev.yaml` 中 `workflow` 段新增 `blackboard_registry_max_size: 1000`
- [ ] `internal/app/bootstrap.go` `BuildSharedDeps` 开头添加：
  ```go
  if cfg := settings.Conf.WorkflowConfig; cfg != nil && cfg.BlackboardRegistryMaxSize > 0 {
      state.GlobalRegistry().SetMaxSize(cfg.BlackboardRegistryMaxSize)
  }
  ```
- [ ] 验证：`go build ./...` 无错误

### P1-3 docker-compose 挂载验证

- [ ] `docker-compose.yaml` 中 `init.sql` 挂载路径与文件位置一致：`./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql:ro`
- [ ] 启动成功：`docker-compose up mysql redis -d && docker-compose ps` 显示 healthy

---

## Phase 2：Writer / QA Agent LLM 化

### P2-1 domain.Report 字段扩展

- [ ] `internal/domain/analysis.go` 中 `Report` 结构体末尾新增：
  ```go
  KeyFindings     []string `json:"keyFindings,omitempty"`
  Recommendations []string `json:"recommendations,omitempty"`
  ```
- [ ] 确认现有 `json.Marshal(report)` 在字段为 nil 时仍正常序列化（omitempty 保证）
- [ ] `go build ./...` 无错误

### P2-2 Writer Agent LLM 化

- [ ] `internal/agent/writer.go` 修改 `Writer` 结构体：添加 `chat ChatClient` 字段
- [ ] `NewWriter()` 签名改为 `NewWriter(chat ChatClient) *Writer`
- [ ] 添加常量 `writerSystemPrompt`（spec.md §1.2 内容）
- [ ] 添加 `writerLLMOutput` 结构体（summary/keyFindings/recommendations）
- [ ] 添加 `buildWriterUserPrompt(meta, analysis, coll, input) string` 函数
- [ ] 添加 `formatSWOTBrief(swot map[string]domain.SWOTAnalysis) string` 辅助函数
- [ ] 在 `Run()` 中 `store.SaveReportFinal(ctx, report)` **之前**，调用 LLM 生成增强内容：
  - [ ] `a.chat != nil` 才执行（保证无 LLM 环境下流程不中断）
  - [ ] LLM 失败 / JSON 解析失败时：用 `analysis.Summary` 作为 fallback，不返回 error
  - [ ] LLM 成功时：覆盖 report.Summary，填充 KeyFindings/Recommendations
- [ ] 在 `Run()` 末尾 EmitProgress 输出 `keyFindings` 条数和 `recommendations` 条数
- [ ] 验证：`chat=nil` 时 Writer 行为与修改前完全一致（regression 无影响）

### P2-3 QA Agent LLM 化

- [ ] `internal/agent/qa.go` 修改 `QA` 结构体：添加 `chat ChatClient` 字段
- [ ] `NewQA()` 签名改为 `NewQA(chat ChatClient) *QA`
- [ ] 添加常量 `qaLLMSystemPrompt`（spec.md §2.2 内容）
- [ ] 添加 `qaLLMOutput` 结构体（llm_score int, issues []qaLLMIssue, overall string）
- [ ] 添加 `buildQAUserPrompt(report, meta, ruleScore) string` 函数
- [ ] 添加 `formatSourcesBrief(sources map[string]domain.SourceRef) string` 辅助函数
- [ ] 实现 `runLLMCheck(ctx, input, report, meta, ruleScore) (int, error)` 方法：
  - [ ] 调用 `a.chat.Chat()`（非流式，控制延迟）
  - [ ] 解析 JSON，取 `llm_score`
  - [ ] 将 LLM 发现的 issues 追加到主 issues 列表（去重：同 location + problem）
  - [ ] 返回 LLM 分数
- [ ] 在 `Run()` 规则评分后插入 LLM 调用：
  - [ ] 条件：`a.chat != nil && score >= 60`（低分快速失败，不浪费 LLM 调用）
  - [ ] 加权：`finalScore = int(ruleScore*0.6 + llmScore*0.4)`
  - [ ] LLM 调用失败时：保留规则分，不阻塞流程
- [ ] 验证：`chat=nil` 时 QA 行为与修改前完全一致

### P2-4 Registry 更新

- [ ] `internal/agent/registry.go` 中 `NewRegistry(deps Deps)` 函数：
  - [ ] `NewWriter(deps.Chat)` 替换原 `NewWriter()`
  - [ ] `NewQA(deps.Chat)` 替换原 `NewQA()`
- [ ] 检查是否有其他地方直接调用 `NewWriter()` 或 `NewQA()`（如 wire_gen.go / agent_wire.go）
  - [ ] 若有，同步更新签名调用
- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无警告

### P2-5 端到端验证

- [ ] 启动服务（配置有效 ARK_API_KEY）
- [ ] 创建竞品分析任务，等待完成
- [ ] 检查 `GET /api/reports/:id`，response 中 `keyFindings` 不为空
- [ ] 检查 `GET /api/reports/:id`，response 中 `recommendations` 不为空
- [ ] 检查 `summary` 不是机械拼接（长度 > 100 字，包含洞察性语言）
- [ ] 触发一次 QA 打回（制造缺少 summary 的报告），验证打回原因包含 LLM 生成的细节
- [ ] 无 LLM 场景（`ARK_API_KEY` 清空）：任务仍能正常跑完，无 panic

---

## Phase 3：乐观锁 + 单元测试

### P3-1 WorkflowState 乐观锁

- [ ] `internal/state/task_board.go` 中 `TaskStore` 接口新增方法：
  ```go
  SaveWorkflowStateVersioned(ctx context.Context, wf WorkflowState, expectedVersion int64) error
  ```
- [ ] `taskStore` 实现 `SaveWorkflowStateVersioned`（spec.md §5.2 内存 CAS 逻辑）：
  - [ ] expectedVersion=0 且 key 不存在 → 直接写入（首次写）
  - [ ] 读取当前 version，不匹配 → 返回 `ErrVersionConflict`
  - [ ] 匹配 → 新 version = expectedVersion+1，写入
- [ ] `internal/workflow/dispatch.go` `handleQAReject` 方法：
  - [ ] 读取当前 wf，记录 `curVersion = wf.Version`
  - [ ] 用 `SaveWorkflowStateVersioned(ctx, newWF, int64(curVersion))` 替换 `SaveWorkflowState`
  - [ ] 失败时最多重试 3 次（重新读取 curVersion）
  - [ ] 3 次均失败：记录警告日志，回退到原逻辑 `SaveWorkflowState`（不阻塞流程）
- [ ] `go build ./...` 无错误

### P3-2 BlackboardRegistry 单元测试

- [ ] 新建 `internal/state/blackboard_test.go`
  - [ ] `TestBlackboardRegistry_ShareBetweenCalls`：
    - 同 taskID 两次 `GetOrCreate` 返回指针相等
  - [ ] `TestBlackboardRegistry_Release`：
    - `Release` 后再 `GetOrCreate` 返回指针不相等（新实例）
  - [ ] `TestBlackboardRegistry_LRUEviction`：
    - `SetMaxSize(2)`，插入 task1/task2 后插入 task3
    - 验证 task1 已被淘汰（GetOrCreate 返回新实例，Put/Get 数据不存在）
  - [ ] `TestBlackboardRegistry_Concurrent`：
    - 100 个并发 goroutine `GetOrCreate("same-task")`
    - 用 `sync.Map` 收集返回的指针地址
    - 验证所有地址相同（只创建了 1 个实例）
- [ ] `go test ./internal/state/... -v -race` 全部通过

### P3-3 VolcanoEmbedder 单元测试

- [ ] 新建 `internal/memory/embedder_test.go`
  - [ ] `TestVolcanoEmbedder_NilConfig_ReturnsHashEmbedder`：
    - `NewVolcanoEmbedder(nil)` 返回的 Embedder 类型为 `*HashEmbedder`
  - [ ] `TestVolcanoEmbedder_EmptyAPIKey_ReturnsHashEmbedder`：
    - `NewVolcanoEmbedder(&EmbedderConfig{APIKey: ""})` 返回 HashEmbedder
  - [ ] `TestVolcanoEmbedder_FallbackOnAPIError`：
    - 启动 `httptest.NewServer` 返回 500
    - 调用 `Embed(ctx, []string{"hello", "world"})`
    - 验证返回 2 个向量（降级到 HashEmbedder），err == nil
  - [ ] `TestHashEmbedder_DimConsistency`：
    - 不同长度文本 Embed，所有输出向量长度 == dim(128)
- [ ] `go test ./internal/memory/... -v` 全部通过

### P3-4 全量回归

- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无警告
- [ ] `go test ./internal/state/... -v -race` 通过
- [ ] `go test ./internal/memory/... -v` 通过
- [ ] `go test ./internal/workflow/... -v` 通过（现有测试无回归）

---

## 全局验收标准

- [ ] `docker-compose up -d` 一键启动，MySQL + Redis 健康，API + Worker 正常运行
- [ ] 创建任务（无 LLM）：5 个 Agent 流水线完整执行，报告生成
- [ ] 创建任务（有 LLM）：报告中包含 keyFindings 和 recommendations
- [ ] 相同竞品第二次任务：Analyst 的 systemPrompt 中包含 `## 历史知识` 段落
- [ ] QA 打回后重跑：Blackboard 数据不丢失（Registry 同实例共享验证）
- [ ] 并发 10 个任务同时运行：无 data race，Registry 正常工作
