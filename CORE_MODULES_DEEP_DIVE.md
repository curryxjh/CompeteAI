# CompeteAI 核心模块答辩详解

> 本文档对 CompeteAI 项目中五个核心模块进行深入的答辩级讲解。每个模块从"是什么→怎么实现→为什么这样设计→代码走读→面试问答"五个维度展开。

---

## 目录

1. [记忆系统：如何跨 Agent 共享](#1-记忆系统如何跨-agent-共享)
2. [整个 Agent 编排](#2-整个-agent-编排)
3. [可追溯链路](#3-可追溯链路)
4. [Outbox 模式详解](#4-outbox-模式详解)
5. [Workflow 引擎详解](#5-workflow-引擎详解)

---

## 1. 记忆系统：如何跨 Agent 共享

### 1.1 是什么

CompeteAI 的记忆系统不是简单的 KV 缓存，而是一个**四层分级记忆架构**，让每个 Agent 在执行时都能获取到"跨任务积累的经验"和"当前任务的上游产物"。

四层从下到上：

| 层 | 名称 | 存储位置 | 生命周期 | 内容 |
|----|------|---------|---------|------|
| L0 | 执行记忆 | Blackboard (Redis) | 单次任务 | 当前任务的中间产物（CollectorOutput、AnalysisOutput 等） |
| L1 | 显式记忆 | MySQL preferences | 项目级 | COMPETE.md 规则、用户手动偏好 |
| L2 | 事实记忆 | MySQL + Milvus | 跨任务 | 从历史任务中提取的结构化事实 |
| L3 | 经验记忆 | MySQL + Milvus | 跨任务 | 任务经验摘要（Episode） |

### 1.2 "跨 Agent 共享"到底怎么实现的

这是面试中最容易被追问的点。跨 Agent 共享分为两种机制：

#### 机制一：同任务内——通过 Blackboard（L0 执行记忆）

这是最直接的共享方式。每个 Agent 执行完后将产出写入 Blackboard，下一个 Agent 从 Blackboard 读取。

```
Coordinator → 写 workflow:state + workflow:plan
Collector   → 读 workflow:plan         → 写 collector:output
Analyst     → 读 collector:output       → 写 analysis:output
Writer      → 读 analysis:output        → 写 report:draft
QA          → 读 report:draft + collector:output → 写 qa:result
```

代码入口：`dispatch.go:handleAgentInput` → `buildRunInput` → 直接从 Blackboard `store.LoadCollectorOutput()` 等。

**关键设计**：Blackboard 是 KV 存储，每个分区（`task:ID:scope:field`）有单一写入者，其他 Agent 只读。版本号（`Versioned.Round`/`Version`）支持 QA 打回后的数据覆盖。

#### 机制二：跨任务——通过长期记忆系统（L1+L2+L3）

这才是"记忆"的核心价值：**Collector 在任务 A 中抓取的竞品 X 的网页内容，在任务 B 中分析竞品 X 时可以直接复用。**

跨任务共享的完整链路：

```
                         摄取阶段（写入）                              检索阶段（读取）
                         
任务 A:                                                    任务 B:
Collector 执行完毕                                         Analyst 执行前
    │                                                         │
    ▼                                                         ▼
ingestAfterAgent()                          buildMemoryContext()
    │                                                         │
    ▼                                                         ▼
mem.IngestCollectorOutput()                 mem.BuildAgentContext()
    │                                                         │
    ├─→ 1. 源 URL+网页内容写入 memory_sources                  │
    ├─→ 2. 内容分块→Embedding→写入 memory_chunks         Assembler.Build()
    ├─→ 3. 抽取事实候选→去重→合并→写入 memory_facts           │
    └─→ 4. 向量化双写 MySQL + Milvus                    ├─→ 1. ScopeResolver 解析多层作用域
                                                         ├─→ 2. HybridRetriever 向量+关键词检索
                                                         ├─→ 3. Ranker 综合评分排序
                                                         ├─→ 4. trimByAgent 按 Token 预算裁剪
                                                         └─→ 5. 注入 RunInput.Memory
                                                              │
                                                              ▼
                                                     Agent 的 LLM Prompt 后缀
```

**五种作用域贯穿始终**：

```
ScopeType  │ ScopeKey 示例           │ 用途
───────────┼─────────────────────────┼──────────────────────
global     │ "default"               │ 全局共享事实
workspace  │ "default"               │ 工作空间级
project    │ "compete-ai"            │ 项目级偏好+事实
user       │ "user:42"               │ 用户私有记忆
entity     │ "entity:notion"         │ 竞品实体级（如 "Notion"）
```

`ScopeResolver` 在检索时把这五种 ScopeID 全部收集起来，Milvus 用 ScopeKey 做标量预过滤，MySQL 用 ScopeID 做 IN 查询。这意味着 Analyst 分析 "Notion" 时，能同时命中全局事实、项目偏好、用户偏好、以及所有关于 Notion 这个实体的事实。

### 1.3 关键代码走读

#### 写入端：`memory_hook.go` — 每个_agent 执行完毕后触发摄取

```go
// workflow/memory_hook.go:47
func (e *Engine) ingestAfterAgent(ctx context.Context, agentName, taskID string, store state.TaskStore) {
    mem := e.memorySvc()
    if mem == nil || !mem.Enabled() { return }
    
    meta, _ := store.LoadTaskMeta(ctx)
    switch agentName {
    case "collector":
        out, _ := store.LoadCollectorOutput(ctx)
        mem.IngestCollectorOutput(ctx, IngestCollectorRequest{
            TaskID: taskID, Sources: out.Sources, Materials: out.Materials, ...
        })
    case "analyst":
        analysis, _ := store.LoadAnalysisOutput(ctx)
        mem.IngestAnalysisOutput(ctx, IngestAnalysisRequest{
            Analysis: report, Sources: sources, ...
        })
    case "qa":
        qa, _ := store.LoadQAResult(ctx)
        mem.IngestQAResult(ctx, IngestQARequest{
            Score: qa.Score, Passed: passed, Issues: issues, ...
        })
    }
}
```

这个 hook 在 `dispatch.go:274` 被调用——Agent 执行成功后、发下一条消息前。

#### 读取端：`memory_hook.go` — Agent 执行前构建记忆上下文

```go
// workflow/memory_hook.go:28
func (e *Engine) buildMemoryContext(ctx context.Context, taskID, agentName string, store state.TaskStore, trigger string) memory.AgentMemoryContext {
    mem := e.memorySvc()
    if mem == nil || !mem.Enabled() { return memory.AgentMemoryContext{} }
    
    meta, _ := store.LoadTaskMeta(ctx)
    return mem.BuildAgentContext(ctx, BuildContextRequest{
        TaskID: taskID, Agent: agentName, TriggerType: trigger,
        Competitors: meta.Competitors, Dimensions: meta.Dimensions,
        Query: meta.Title, UserID: UserIDFromCtx(ctx),
    })
}
```

#### Assembler：按 Agent 类型差异化裁剪

```go
// memory/assembler.go:153
func (a *Assembler) trimByAgent(agent string, budget int, ctx AgentMemoryContext) AgentMemoryContext {
    switch agent {
    case "coordinator":  // 规划阶段不需要历史事实和证据
        ctx.Facts = nil
        ctx.Evidence = nil
    case "collector":    // 搜索阶段保留少量经验
        ctx.Episodes = trimEpisodes(ctx.Episodes, 2)
    case "writer":      // 写作阶段需要事实，但不需要原始证据
        ctx.Evidence = nil
        ctx.Facts = trimFacts(ctx.Facts, 5)
    case "analyst":      // 分析阶段需要全部记忆
        // keep all layers
    case "qa":           // 质检需要事实+证据做交叉验证
        // keep facts + evidence
    }
    // Token 预算溢出时按优先级裁剪：Evidence → Facts → Episodes
    for estimateContextTokens(ctx) > budget && len(ctx.Evidence) > 0 { ... }
    for estimateContextTokens(ctx) > budget && len(ctx.Facts) > 0 { ... }
}
```

Agent Token 预算差异：

| Agent | Token 预算 | 原因 |
|-------|-----------|------|
| Coordinator | 400 | 规划阶段主要看结构化偏好，不需要大量上下文 |
| Collector | 500 | 需要经验但不需要事实 |
| Analyst | 1800 | 核心分析层，需要最多历史事实和证据 |
| Writer | 1200 | 需要事实做引用，但不需要原始网页证据 |
| QA | 1500 | 需要事实和证据做交叉验证 |

### 1.4 为什么这样设计

**1. 为什么不直接把所有 Agent 的上下文都做全量检索？**

因为不同 Agent 的认知任务不同：
- Coordinator 做规划决策，不需要原始网页内容，只需要规则偏好
- Analyst 做事实对比，需要最丰富的上下文
- Writer 做报告生成，需要事实做引用但不自己搜索
- QA 做质量检查，需要证据做事实核查

一刀切的全量注入会浪费 Token，且噪声干扰 LLM 判断。

**2. 为什么用 Milvus + MySQL 双写而不是只用一个？**

- MySQL 全文检索：准确但召回率低，无法做语义相似度匹配
- Milvus ANN 检索：语义相似度好但需要额外维护向量
- 双写检索：`HybridRetriever` 同时查两个数据源，Milvus 基于向量距离返回 semantically similar 的候选，MySQL 基于关键词返回 lexically matching 的候选，`Ranker` 综合两者评分重排

**3. 降级策略**

```
Milvus 可用 → HybridRetriever (Milvus ANN + MySQL 关键词)
Milvus 不可用 → MySQLRetriever (纯关键词，自动降级)
Embedding API 可用 → VolcanoEmbedder (火山引擎)
Embedding API 不可用 → HashEmbedder (FNV-1a 哈希，本地降级)
MCP 不可用 → 纯 LLM 模式（无外部工具）
```

### 1.5 面试问答

> Q: 不同 Agent 看到的记忆不一样吗？
>
> A: 是的。`trimByAgent` 方法对每个 Agent 类型裁剪策略不同。Coordinator 不需要历史事实，Analyst 需要全部。这不只是 Token 节省，更是减少噪声。如果给 Coordinator 注入 1800 Token 的历史事实，它的规划反而会被干扰。

> Q: 跨任务记忆的准确性如何保证？事实会不会越积累越脏？
>
> A: 三个治理机制：
> 1. **去重**：`Governance.Dedup()` 用 claim hash 去重，同一事实不会重复写入
> 2. **合并**：`Governance.Merge()` 冲突时用 `ConfidenceScore` 和 `FreshnessScore` 综合评分决定保留谁
> 3. **失效**：QA 打回时如果标记了 `IssueUnsupportedClaim`，`Governance.Invalidate()` 将对应 fact 的 status 改为 `invalidated`

> Q: 如果 QA Agent 发现报告中的某个结论没有来源支撑，记忆系统会怎么处理？
>
> A: 这是 QA 摄取的关键逻辑。`IngestQAResult` 会遍历当前任务的所有 Claims，对于 QA 标记为 `unsupported` 的 Claim：
> 1. 如果该 Claim 关联了 FactID，调用 `Governance.Invalidate` 将 fact 状态设为 `invalidated`
> 2. 更新 Claim 的 `verification_status` 为 `rejected`
> 3. 如果 QA 通过，则将 Claim 对应的 Fact 状态设为 `verified`

---

## 2. 整个 Agent 编排

### 2.1 是什么

Agent 编排是指**从用户创建任务到报告产出全过程中，5 个 Agent 是如何被调度、串接、打断回、询问澄清的**。CompeteAI 的编排不是中心化控制器轮询每个 Agent，而是**事件驱动（消息总线）+ 状态机 + 路由表**的组合。

### 2.2 两种编排模式并存

CompeteAI 使用两种模式：

| 模式 | 适用场景 | 实现覆盖 |
|------|---------|---------|
| 事件驱动（Bus 异步消息） | 正常 Agent 间流转、QA 打回、澄清 | 90% 场景，生产部署 |
| 直接函数调用（同步） | QA 追问 Analyst | 10% 场景，同进程优化 |

### 2.3 编排全链路图

```
用户 POST /api/tasks
    │
    ▼
TaskService.Create
    │
    ├─→ DB.Transaction: INSERT tasks + INSERT outbox (同一事务)
    │
    ▼
Outbox.Poller (2s 轮询)
    │
    ├─→ SELECT pending FROM outbox LIMIT 50
    ├─→ Bus.Publish(TopicTaskCreate, MessageEnvelope)
    ├─→ UPDATE outbox SET status='sent'
    │
    ▼
Bus (Redis Stream / Kafka / Memory)
    │
    ▼
Worker.consumeLoop
    │
    ▼
orchestrator.Runtime.wrapHandler
    │
    ├─→ 1. Idempotent: Redis SetNX(messageID, 7天) → 已处理则跳过
    ├─→ 2. AcquireLease: Redis SetNX(lease:task:ID:agent, 30s) → 并发控制
    ├─→ 3. engine.HandleDelivery(topic, msg)
    │       │
    │       ├─ topic=TopicTaskCreate → handleTaskCreated
    │       ├─ topic=TopicAgentCoordinator → handleAgentInput(coordinator)
    │       ├─ topic=TopicQAQuery → handleQAQuery (同步调 Analyst)
    │       └─ topic=TopicAnalystReply → handleAnalystResponse (同步调 QA)
    │
    ▼
handleAgentInput(name, msg)
    │
    ├─→ 1. 检查取消状态 (Redis Get)
    ├─→ 2. 获取 Blackboard + TaskStore
    ├─→ 3. 加载或初始化 Trace
    ├─→ 4. 检查是否 rework (Round > 1 或 MsgQAReject)
    │      └─→ 重跑时跳过 target 之前的 Agent (ShouldRunAgent)
    ├─→ 5. 从 Registry 获取 Agent 实例
    ├─→ 6. 设置 Agent 状态为 Running
    ├─→ 7. buildRunInput (含记忆上下文注入)
    ├─→ 8. executeAgent → runAgentWithRetry
    │       │
    │       └─→ ag.Run(ctx, input, bb)
    │              │
    │              ├─→ 调 LLM (maybe with Tool Calling)
    │              ├─→ 读 Blackboard
    │              └─→ 写 Blackboard
    │
    ├─→ 9. 构建 TraceNode + 保存 Trace
    ├─→ 10. 构建 Agent 下游消息 (NewAgentMessage)
    ├─→ 11. 更新 Task.AgentStates + WorkflowState
    ├─→ 12. ingestAfterAgent (记忆摄取)
    │
    └─→ 13. 路由决策
           │
           ├─ QA Reject + 未超轮次 → handleQAReject
           │     └─→ Router.PublishQAReject → Bus → 回到 Coordinator
           │
           ├─ QA Pass → completeTask
           │     └─→ 保存 Report + Trace → Bus → 通知完成
           │
           ├─ Clarification required → 转为 Clarifying 状态
           │     └─→ 等待用户 POST /clarify → Outbox → Bus → 回到 Coordinator
           │
           └─ 正常下一跳 → forwardToAgent
                 └─→ Bus.Publish(topic, envelope) → 消息驱动下一个 Agent
```

### 2.4 QA 打回的完整机制

QA 打回是编排中最复杂的路径：

```go
// dispatch.go:276
if name == domain.AgentQA && out.MessageType == string(domain.MsgQAReject) {
    qaPayload := out.Payload.(domain.QAResultPayload)
    
    // 先尝试 QA 追问循环（只追问 1 次）
    if e.tryQAQueryLoop(ctx, taskID, traceID, store, wf, qaPayload) {
        return nil // 进入追问子循环
    }
    
    // 追问不够或已用过，执行正式打回
    cont, err := e.handleQAReject(ctx, store, &task, &trace, traceID, out.Payload)
    if !cont {
        return e.failTask(ctx, store, task, trace, traceID, "qa", "超过最大打回轮次")
    }
    return nil
}
```

QA 拒绝后的路由规则：

```go
// workflow/router.go:60
func RouteQAReject(qa *QAResultPayload, wf WorkflowState) RouteOutput {
    // 1. 检查是否超过最大轮次
    if wf.Round >= maxRounds {
        return RouteOutput{Failed: true, Reason: "超过最大打回轮次"}
    }
    
    // 2. 从 QA 的 Issue 列表中按优先级选择目标 Agent
    target := qa.TargetAgent
    if target == "" {
        target = TargetAgentForIssues(qa.Issues)
    }
    
    return RouteOutput{NextAgent: target, TaskStatus: TaskStatusReworking}
}

// Issue 优先级路由表
func TargetAgentForIssues(issues []Issue) AgentName {
    priority := []struct{ category string; agent AgentName }{
        {IssueSourceMissing,      AgentCollector},  // 缺素材 → Collector
        {IssueMissingSourceRef,   AgentAnalyst},     // 缺引用 → Analyst
        {IssueAnalysisIncomplete, AgentAnalyst},    // 分析不完整 → Analyst
        {IssuePricingMissing,     AgentAnalyst},    // 缺定价 → Analyst
        {IssueUnsupportedClaim,   AgentAnalyst},    // 无支撑结论 → Analyst
        {IssueReportStructure,    AgentWriter},      // 报告结构问题 → Writer
    }
    // 按优先级匹配第一个命中的 Issue
}
```

重跑时哪些 Agent 需要执行：

```go
// domain/task.go:185 — ShouldRunAgent
func ShouldRunAgent(name, target AgentName, isRework bool) bool {
    if !isRework || name == AgentCoordinator { return true }
    return AgentIndex(name) >= AgentIndex(target)
    // 如果 target 是 Analyst（index=2），则 Analyst(2)、Writer(3)、QA(4) 重跑
    // Collector(1) 被跳过
}
```

### 2.5 QA 追问子循环（同步调用）

与通过 Bus 异步消息不同，QA 追问是一个**同进程同步调用**的子循环：

```
QA 发现 "某结论缺来源"
    │
    ├─→ tryQAQueryLoop: 检查 QAQueryAttempts < 1
    │
    ▼
PublishQAQuery → Bus → TopicQAQuery
    │
    ▼
Worker 收到 → handleQAQuery
    │
    ├─→ 直接执行 Analyst.Run (同步，不经过 Bus)
    │
    ▼
Analyst 补充证据 → AnalystResponsePayload
    │
    ▼
Publish → TopicAnalystReply
    │
    ▼
handleAnalystResponse
    │
    ├─→ 将证据追加到 Report.Summary
    └─→ 重新调用 QA.Run (handleAgentInput(qa, msg))
```

这是唯一的"同步跳过 Bus"路径。为什么？因为追问通常只需要 Analyst 局部操作（补充一个引用），不值得走完整的 Bus 异步流程。

### 2.6 面试问答

> Q: 为什么要同步调用 Analyst 而不是全走 Bus？
>
> A: QA 追问是一个"快速修复"机制。如果每次追问都走完整的 Bus 流程（序列化→入队→消费→租约→执行→序列化→入队→消费），延迟会累加到 5-10 秒。同步调用在同进程内直接执行 Analyst 的 `runQAQuery` 方法，延迟降到毫秒级。但只允许追问 1 次（`QAQueryAttempts < 1`），防止无限追问。

> Q: 如果 Analyst 追问时挂了怎么办？
>
> A: 追问失败意味着证据不够，QA 会走正常的 Reject 流程。`tryQAQueryLoop` 返回 false，`handleQAReject` 会正式打回，状态转为 `Reworking`，走 Bus 异步重跑。这时 Analyst 执行的是完整的 `Run` 方法而非局部 `runQAQuery`。

---

## 3. 可追溯链路

### 3.1 是什么

可追溯是指**任何一个最终报告中的结论，都能追溯到：哪个 Agent 产出的、用了什么 LLM prompt、调了什么工具、读了哪些网页、中间推理了什么**。

CompeteAI 实现了三级追溯：

| 级别 | 追溯内容 | 存储 | 查看方式 |
|------|---------|------|---------|
| 任务级 | Task 状态流转 + 事件流 | event_logs 表 | SSE 实时推送 + REST 回放 |
| Agent 级 | 每次执行的输入输出 + 耗时 + Token | traces 表 | REST API |
| 步骤级 | Agent 内部 thinking/tool_call/note 子步骤 | trace_steps（嵌入 TraceNode）| REST API |

### 3.2 三级追溯的实现机制

#### 一级：任务事件流（event/HybridHub）

```go
// event/hybrid_hub.go:22
func (h *HybridHub) Publish(taskID, eventType string, data any) {
    h.TaskHub.Publish(taskID, eventType, data)   // 内存 SSE 实时推送
    h.pub.Publish(ctx, taskID, "", MapLegacyEvent(eventType), "", data)  // DB 持久化
}
```

Engine 中每次状态变更都会调 `hub.Publish`：

```go
// engine.go 中的各种事件发布
e.hub.Publish(taskID, "task_started", ...)     // 任务开始
e.hub.Publish(taskID, "agent_state", ...)      // Agent 状态变更
e.hub.Publish(taskID, "task_complete", ...)    // 任务完成
e.hub.Publish(taskID, "clarification", ...)   // 澄清请求
e.hub.Publish(taskID, "rejection", ...)        // QA 打回
e.hub.Publish(taskID, "tool_step", ...)        // 工具调用
e.hub.Publish(taskID, "agent_thinking", ...)   // Agent 推理过程
```

前端通过 SSE 实时订阅，断连时用 `Reader.ListSince(afterSeq)` 增量补齐。

#### 二级：Agent 执行节点（Trace）

每个 Agent 执行完毕后，`dispatch.go` 构建一个 `TraceNode` 并追加到 `Trace`：

```go
// dispatch.go:249
trace.Nodes = append(trace.Nodes, buildTraceNode(
    fmt.Sprintf("n%d", len(trace.Nodes)+1),
    name,                              // Agent 名
    ag.Card().DisplayName,             // Agent 显示名
    out.Status,                        // 执行状态
    out.Summary,                       // 输出摘要
    out.Metadata,                      // 含 duration_ms, token_count, trace_steps
))
e.traces.Save(ctx, trace)
```

`TraceNode` 记录的信息：

```go
type TraceNode struct {
    ID          string                 // "n1", "n2", ...
    Agent       AgentName              // "coordinator", "collector", ...
    Label       string                 // "Collector" (显示名)
    Status      AgentRunStatus         // completed / failed / rejected
    DurationMs  int                    // 执行耗时（毫秒）
    TokenCount  int                    // LLM Token 估算
    Output      string                 // 输出摘要（截断 300 字）
    Metadata    map[string]interface{} // 扩展元数据
    Steps       []TraceStep            // 细粒度步骤
    IsRetry     bool                   // 是否重试
    IsRejection bool                   // 是否 QA 打回重做
    ParentID    string                 // 父节点（打回时关联原始节点）
}
```

#### 三级：Agent 内部步骤（TraceStep）

这是最细粒度的追溯。每个 Agent 执行时，通过 `context` 注入两个 reporter：

```go
// qa_loop.go:20
agentCtx = agent.WithToolStepReporter(agentCtx, func(ev ToolStepEvent) {
    e.hub.Publish(taskID, "tool_step", map[string]any{
        "tool_name": ev.ToolName,     // "firecrawl_search"
        "tool_args": args,             // {"query": "Notion vs Obsidian"}
        "tool_result": result,         // 搜索结果摘要
        "status": ev.Status,           // "running" / "done"
    })
    traceSess.AppendStep("tool", args, ev.Status, ev.ToolName)
})

agentCtx = agent.WithAgentProgressReporter(agentCtx, func(ev AgentProgressEvent) {
    e.hub.Publish(taskID, "agent_thinking", map[string]any{
        "agent": string(name),
        "kind": ev.Kind,                // "thinking" / "note" / "output"
        "content": ev.Content,           // LLM 思考内容
    })
    traceSess.AppendStep(ev.Kind, ev.Content, ev.Status, "")
})
```

`TraceSession` 的数据最终写入 `TraceNode.Steps`：

```go
// qa_loop.go:45
if traceSess != nil {
    out.Metadata["trace_steps"] = traceSess.Steps     // 写入 metadata
    out.Metadata["duration_ms"] = traceSess.DurationMs()
    out.Metadata["token_count"] = tokenCount
}
```

### 3.3 一次完整执行的 Trace 结构示意

```json
{
  "taskId": "abc-123",
  "nodes": [
    {
      "id": "n1", "agent": "coordinator", "label": "Coordinator",
      "status": "completed", "durationMs": 1200, "tokenCount": 800,
      "output": "任务计划已生成，进入采集阶段",
      "steps": [
        {"kind": "thinking", "content": "分析用户请求：Notion vs Obsidian...", "status": "done"},
        {"kind": "note", "content": "用户需求明确，无需澄清", "status": "done"},
        {"kind": "output", "content": "生成计划：5 个交付物", "status": "done"}
      ]
    },
    {
      "id": "n2", "agent": "collector", "label": "Collector",
      "status": "completed", "durationMs": 8500, "tokenCount": 2000,
      "output": "采集完成，3 个素材源",
      "steps": [
        {"kind": "tool", "content": "{\"query\":\"Notion features\"}", "toolName": "firecrawl_search", "status": "done"},
        {"kind": "tool_result", "content": "Notion 是一个...", "toolName": "firecrawl_search", "status": "done"},
        {"kind": "tool", "content": "{\"query\":\"Obsidian features\"}", "toolName": "firecrawl_search", "status": "done"},
        {"kind": "thinking", "content": "整理素材：共 3 个源", "status": "done"}
      ]
    },
    // ... analyst、writer、qa 节点
  ]
}
```

### 3.4 为什么这样设计

> Q: 为什么要同时做 SSE 实时推送和 DB 持久化？一套不行吗？
>
> A: SSE 只保证"连接时的完整性"，但用户可能刷新页面、网络断连。如果只有 SSE，重新连接后之前的事件就丢了。DB 持久化的 event_logs 表用序列号递增，前端重连时调 `ListSince(lastSeq)` 取到所有缺失事件。但只存 DB 的延迟太高（每次写 DB 要 5-50ms），SSE 内存推送是亚毫秒级。双写 = 实时性 + 可靠性。

> Q: Trace 是实时写入还是执行完才写入？
>
> A: 每次执行完 `Save(ctx, trace)` 是覆盖写入（不是追加），因为 Trace 是一个完整的 JSON 文档，GORM 的 `Save` 会全量更新。这意味着前端轮询 `GET /api/traces/:taskId` 可以看到逐步增长的 Nodes 列表。

---

## 4. Outbox 模式详解

### 4.1 是什么

Outbox（发件箱）模式解决一个经典问题：**如何保证"写数据库"和"发消息"的原子性？**

在 CompeteAI 中，用户创建任务时需要：
1. 向 `tasks` 表写入任务记录
2. 向消息总线发送 `task_created` 消息触发 Worker 执行

如果先写 DB 再发消息，消息发送可能失败 → 任务存在但永远不会执行（幽灵任务）。
如果先发消息再写 DB，DB 写入可能失败 → 消息消费时找不到任务（幽灵消息）。

### 4.2 解决方案

把"发消息"拆成两步：
1. 在**同一事务**中：写 `tasks` 表 + 写 `outbox` 表（status='pending'）
2. 后台 goroutine 每 2 秒轮询 `outbox` 表，将 pending 消息发到 Bus，成功后更新 status='sent'

```
                 同一 DB 事务                          后台轮询（异步）
                 
BEGIN TX                              │
  INSERT INTO tasks ...               │  每 2 秒:
  INSERT INTO outbox (status=pending) │    SELECT * FROM outbox WHERE status='pending'
COMMIT                                │    │
                                      │    ├─→ Bus.Publish(topic, envelope)
                                      │    └─→ UPDATE outbox SET status='sent'
```

### 4.3 代码走读

#### 第一步：同事务写入

```go
// repository/outbox_tx.go:15
func CreateTaskWithOutbox(ctx context.Context, db *gorm.DB, task domain.Task, traceID string) error {
    entity, _ := taskToEntity(task)
    msg := domain.NewTaskCreatedMessage(task, traceID).WithStatus(domain.MessageStatusPending)
    raw, _ := json.Marshal(msg)
    
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(&entity).Error; err != nil {   // 写 tasks 表
            return err
        }
        return tx.Create(&dao.OutboxMessageEntity{          // 写 outbox 表
            TaskID: task.ID, MessageID: msg.MessageID,
            Topic: bus.TopicTaskCreate, PayloadJSON: string(raw),
            Status: "pending",
        }).Error
    })
}
```

**关键点**：两个 INSERT 在同一个 `Transaction` 中，要么都成功要么都回滚。

#### 第二步：后台轮询发布

```go
// outbox/publisher.go:44
func (p *Publisher) Run(ctx context.Context) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C: p.flush(ctx)
        }
    }
}

func (p *Publisher) flush(ctx context.Context) {
    rows, err := p.dao.ListPending(ctx, 50)  // 取 50 条 pending
    for _, row := range rows {
        var env domain.MessageEnvelope
        json.Unmarshal([]byte(row.PayloadJSON), &env)
        
        if err := p.bus.Publish(ctx, row.Topic, env); err != nil {
            log.Printf("[outbox] publish %s failed: %v", row.MessageID, err)
            continue  // 失败跳过，下次轮询重试
        }
        p.dao.MarkSent(ctx, row.ID)  // 标记已发送
    }
}
```

#### Outbox 表结构

```sql
CREATE TABLE outbox_messages (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id     VARCHAR(36) NOT NULL,
    message_id  VARCHAR(36) NOT NULL UNIQUE,   -- 幂等键
    topic       VARCHAR(128) NOT NULL,
    payload_json TEXT NOT NULL,
    status      VARCHAR(16) DEFAULT 'pending',  -- pending → sent
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 其他 Outbox 入口

除了任务创建，Clarify（澄清回答）和 Cancel（取消）也走 Outbox：

```go
// outbox/commands.go
func (p *Publisher) EnqueueClarify(ctx context.Context, taskID, traceID, answer string) error {
    msg := domain.NewEnvelope(taskID, traceID, "user", "coordinator",
        domain.MsgClarificationAnswered,
        domain.ClarificationPayload{Answer: answer})
    return p.dao.Enqueue(ctx, OutboxMessageEntity{
        TaskID: taskID, Topic: bus.TopicTaskClarify, ...
    })
}

func (p *Publisher) EnqueueCancel(ctx context.Context, taskID, traceID string) error { ... }
```

**为什么 Clarify 和 Cancel 不在 TaskService.Create 的事务中？**

因为它们是异步操作——用户提交澄清答案时，任务已经在运行中。这里 Outbox 的作用不是保证"DB 写+消息发"的原子性，而是保证"消息不丢"：即使 Bus 短暂不可用，Outbox 重试确保最终投递。

### 4.4 为什么不直接在事务中调用 Bus.Publish

| 方案 | 优点 | 缺点 |
|------|------|------|
| 直接 Publish | 简单 | DB 成功 Bus 失败 → 消息丢失 |
| 两阶段提交 | 强一致 | Bus 不支持 XA，性能差 |
| Outbox 模式 | 最终一致，无需分布式事务 | 有 2 秒延迟，需要轮询 |

### 4.5 幂等性保证

Outbox 的另一个隐含作用：**配合 orchestrator 的幂等检查，实现了端到端的 exactly-once 语义**。

```
Outbox 写入 → message_id 存入 outbox 表
    │
    ▼
Publisher 发到 Bus
    │
    ▼
Worker 消费 → orchestrator.Idempotent(Redis SetNX message_id, 7天)
    │
    ├─→ 首次消费：SetNX 成功 → 执行
    └─→ 重复消费：SetNX 失败 → 跳过
```

即使 Outbox flush 因为某种原因发送了两次（比如 MarkSent 失败后重新轮询），Worker 端幂等检查保证只执行一次。

### 4.6 面试问答

> Q: Outbox 2 秒延迟会不会影响用户体验？
>
> A: 对于任务创建，2 秒延迟可接受。用户看到的是"任务已创建，正在排队"。前端通过 SSE 实时获取后续状态变更。如果未来需要更低延迟，可以：1）缩小轮询间隔到 500ms；2）使用 MySQL binlog 监听（如 Debezium）替代轮询；3）在事务提交后直接 Publish（作为乐观推送，Outbox 作为兜底）。

> Q: 如果 outbox 表越来越大怎么办？
>
> A: 已发送的记录可以定期清理。`status='sent'` 的记录在确认下游消费完成（通过 message_logs 表确认 ACK）后可以删除。或者设置一个保留期（如 7 天），超过的 sent 记录自动清理。

> Q: outbox 模式和 Saga 模式有什么区别？
>
> A: Outbox 解决的是"同事务的多目标写入"——同一个 DB 事务中写业务表+消息表，保证不丢。Saga 解决的是"跨服务分布式事务"——编排多个服务补偿操作。CompeteAI 用的是 Outbox 保证本地消息不丢，而不是用 Saga。如果未来 Agent 需要跨微服务调用（如 LLM 调用独立服务化），可能需要 Saga。

---

## 5. Workflow 引擎详解

### 5.1 是什么

Workflow 引擎是 CompeteAI 的"大脑"——负责**任务的全生命周期管理**：状态转换、Agent 调度、重试控制、QA 打回编排、澄清循环、记忆摄取、Trace 记录。

它不是一个通用工作流框架（如 Temporal），而是**为"5 Agent 固定流水线 + QA 打回"这一特定场景设计的专用引擎**。

### 5.2 引擎架构

```go
// workflow/engine.go:22
type Engine struct {
    registry        *agent.Registry       // Agent 注册表
    tasks           repository.TaskRepository
    reports         repository.ReportRepository
    traces          repository.TraceRepository
    hub             *event.HybridHub       // 事件推送
    bus             bus.Bus                // 消息总线
    router          *bus.Router            // 消息路由
    redis           redis.Cmdable          // Redis 客户端
    useRedisBB      bool                   // 是否用 Redis Blackboard
    maxAgentRetries int                    // Agent 最大重试次数
    maxRounds       int                    // QA 打回最大轮次
    mem             memory.MemoryService   // 长期记忆
}
```

### 5.3 消息入口：HandleDelivery

Engine 的核心入口是 `HandleDelivery`，它根据消息的 Topic 路由到不同处理逻辑：

```go
// workflow/workers.go:21
func (e *Engine) HandleDelivery(ctx context.Context, topic string, msg domain.MessageEnvelope) error {
    return e.withIdempotent(ctx, msg, func() error {
        switch bus.NormalizeTopic(topic) {
        case bus.TopicTaskCreate:
            return e.handleTaskCreated(ctx, msg)
        case bus.TopicAgentCoordinator, bus.TopicAgentCollector,
             bus.TopicAgentAnalyst, bus.TopicAgentWriter, bus.TopicAgentQA:
            agentName, _ := agentForInputTopic(topic)
            return e.handleAgentInput(ctx, agentName, msg)
        case bus.TopicQAQuery:
            return e.handleQAQuery(ctx, msg)       // QA 追问（同步调 Analyst）
        case bus.TopicAnalystReply:
            return e.handleAnalystResponse(ctx, msg)  // Analyst 回复（同步调 QA）
        case bus.TopicTaskClarify:
            return e.ProcessClarification(ctx, msg)  // 用户澄清回答
        case bus.TopicTaskCancel:
            return e.ProcessCancel(ctx, msg.TaskID)  // 用户取消
        }
    })
}
```

### 5.4 核心方法：handleAgentInput（400 行宇宙）

`handleAgentInput` 是整个引擎最复杂的方法，处理一个 Agent 的完整生命周期：

```
handleAgentInput(name, msg)
    │
    ├─── 1. 取消检查
    │     └─→ if isCancelled: cancelTask + return
    │
    ├─── 2. 初始化
    │     ├─→ 获取 Blackboard + TaskStore
    │     ├─→ 加载或初始化 Trace
    │     └─→ 加载 WorkflowState (含 Round、MaxRounds)
    │
    ├─── 3. 判断是否 rework
    │     └─→ isRework = Round > 1 || msg.MessageType == MsgQAReject
    │         └─→ 如果是 rework 且当前 Agent 不需要重跑 → forwardToAgent(target)
    │
    ├─── 4. 获取 Agent 实例
    │     └─→ registry.Get(name)
    │
    ├─── 5. 设置 Agent 状态为 Running + SSE 推送
    │
    ├─── 6. 构建 RunInput（含记忆上下文注入）
    │     └─→ buildRunInput → buildMemoryContext → MemoryService.BuildAgentContext
    │
    ├─── 7. 执行 Agent（带重试）
    │     └─→ executeAgent → runAgentWithRetry(maxRetries=3)
    │         └─→ ag.Run(ctx, input, bb)
    │             ├─→ 调 LLM
    │             ├─→ 调 MCP 工具
    │             ├─→ 读/写 Blackboard
    │             └─→ 返回 RunOutput 或 AgentError
    │
    ├─── 8. 错误处理
    │     ├─→ AgentError{Kind: clarification_required} → 转为 Clarifying 状态 + 等待用户
    │     ├─→ AgentError{Kind: retryable} → 重试（已由 runAgentWithRetry 处理）
    │     └─→ 其他错误 → failTask
    │
    ├─── 9. 成功后处理
    │     ├─→ 构建 TraceNode + 保存 Trace
    │     ├─→ 构建下游消息 (NewAgentMessage)
    │     ├─→ 更新 Task.AgentStates
    │     ├─→ 更新 WorkflowState (CurrentAgent, NextAgent)
    │     ├─→ 记忆摄取 (ingestAfterAgent)
    │     └─→ SSE 推送 Agent 状态变更
    │
    └─── 10. 路由决策
          ├─→ QA Reject → tryQAQueryLoop(追问) 或 handleQAReject(打回)
          ├─→ QA Pass → completeTask (保存报告 + 通知完成)
          ├─→ 澄清请求 → 转 Clarifying 状态
          ├─→ 有 NextAgent → forwardToAgent (Bus.Publish 到下一 Agent 的 Topic)
          └─→ 无 NextAgent → router.Publish (按路由表自动路由)
```

### 5.5 状态机

Task 的状态转换由 `domain/task.go` 中的 `TaskTransitions` 定义：

```go
var TaskTransitions = map[TaskStatus][]TaskStatus{
    TaskStatusPending:           {TaskStatusQueued, TaskStatusRunning, TaskStatusCancelled},
    TaskStatusQueued:            {TaskStatusRunning, TaskStatusCancelled, TaskStatusFailed},
    TaskStatusRunning:           {TaskStatusClarifying, TaskStatusReworking, TaskStatusWaitingReply,
                                 TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled,
                                 TaskStatusAttentionRequired},
    TaskStatusClarifying:        {TaskStatusRunning, TaskStatusFailed, TaskStatusCancelled},
    TaskStatusReworking:         {TaskStatusRunning, TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled},
    TaskStatusWaitingReply:      {TaskStatusRunning, TaskStatusFailed, TaskStatusCancelled},
    TaskStatusAttentionRequired:{TaskStatusRunning, TaskStatusFailed, TaskStatusCancelled},
    TaskStatusCompleted:         {},  // 终态
    TaskStatusFailed:            {},  // 终态
    TaskStatusCancelled:         {},  // 终态
}
```

对于转换，调用 `ValidateTransition` 来执行验证：

```go
func (e *Engine) transitionTask(ctx context.Context, store, taskID string, from, to TaskStatus, progress int, errMsg string) error {
    if from != to {
        if err := ValidateTransition(from, to); err != nil { return err }
    }
    task, _ := e.tasks.Get(ctx, taskID)
    task.Status = to
    task.Progress = progress
    e.tasks.Update(ctx, task)
    store.SaveTaskStatus(ctx, TaskStatusSnapshot{Status: to, Progress: progress})
}
```

状态转换图：

```
pending → queued → running ─┬→ clarifying → running (用户回答)
                            ├→ reworking → running (重跑)
                            ├→ completed (QA 通过)
                            ├→ failed (错误/超限)
                            ├→ cancelled (用户取消)
                            └→ attention_required (DLQ 超限)
```

### 5.6 为什么不用 Temporal / Cadence

| 考虑 | 自研引擎 | Temporal |
|------|---------|----------|
| 学习成本 | 团队 3 人，Go 原生 | 需要 Temporal Server + 可视化面板 |
| 调试 | 单文件 400 行，断点直查 | 需要 Temporal UI |
| 可控性 | 自定义重试/超时/租约/记忆注入 | 需要适配 Temporal 模型 |
| 适用规模 | 5 Agent 固定流水线 | 动态 Agent 编排 |
| 运维 | 无额外组件 | 需要 Temporal Server（Go + DB） |

**如果未来 Agent 数量 > 10 或需要动态编排，迁移到 Temporal 是正确的选择。**

### 5.7 面试问答

> Q: Agent 重试和消息重试有什么区别？
>
> A: 两层重试机制：
> - **Agent 重试**（`runAgentWithRetry`）：同一个消息处理中，Agent `Run` 失败自动重试 3 次。如果 AgentError.Kind == "retryable" 就继续，否则直接返回。这是进程内同步重试，不经过 Bus。
> - **消息重试**（`orchestrator.wrapHandler`）：Agent 重试 3 次都失败后，消息返回 err 给 Bus。Bus 按指数退避重新投递（5s→15s→30s→60s→120s），最多 5 次。超过后进 DLQ。
>
> 也就是说：进程内最多重试 3 次 × 3 次 = 9 次 Agent 调用；然后 5 次消息级重试 = 45 次 Agent 调用。但实际不会这么多，因为绝大多数错误是"不可重试"的（如 `fatal` 或 `clarification_required`），第一层就停止了。

> Q: 如果 Worker 在执行 Agent 时崩溃了怎么办？
>
> A: 租约 + Recovery 机制：
> 1. 租约 30 秒 TTL：Worker 崩溃后 30 秒，Redis 中的 lease 自动过期
> 2. Recovery 进程每 30 秒扫描 `running`/`reworking` 状态的任务
> 3. 对每个 stale task（checkpoint 更新时间 > 2 分钟），重新发布 `task_created` 到 Bus
> 4. 新 Worker 消费消息，幂等检查发现未执行（因为租约未释放），获取新租约，从头执行
> 5. Blackboard 中的中间产物（如果用 Redis 模式）不会丢失

> Q: 为什么要把状态转换规则放在 domain 层而不是 workflow 层？
>
> A: 状态转换规则是业务规则，不是技术实现。`TaskStatusRunning → TaskStatusClarifying` 表达的是"任务执行中需要用户澄清"这个业务语义。如果放在 workflow 层，domain 包就变成了贫血模型——只有数据结构没有行为。按 DDD 原则，聚合根应该封装自己的不变量和状态转换规则。workflow 层只是调用 `ValidateTransition` 做校验，真正的规则定义在 domain。