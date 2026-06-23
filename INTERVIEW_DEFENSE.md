# CompeteAI 面试答辩文档

> 本文档用于面试中介绍 CompeteAI 项目的架构设计、技术决策和延伸思考。建议结合 PROJECT_GUIDE.md 一起阅读。

---

## 一、项目一句话概述

**CompeteAI 是一个多 Agent 协作的竞品分析平台：用户提交任务后，5 个专职 Agent 按流水线执行采集→分析→写作→质检，输出包含 SWOT、功能矩阵、定价对比、用户画像的结构化报告，全程可追溯、可回放。**

项目规模：~16,000 行 Go 代码，144 个源文件，14 个测试文件。

---

## 二、为什么做这个项目？解决了什么问题？

### 业务痛点

1. **竞品分析高度依赖人工**：分析师需要手动搜索、阅读、对比、写报告，一个完整报告需要 2-3 天
2. **报告质量不稳定**：不同分析师产出的报告格式、深度差异大
3. **无法追溯**：传统流程中，报告结论的出处难以验证
4. **无法迭代**：质检打回后从零开始，无法局部修正

### 系统目标

| 目标 | 实现方式 |
|------|----------|
| 全自动流水线 | 5 Agent 固定顺序执行，无需人工干预 |
| 结构化输出 | SWOT/功能矩阵/定价/画像四种分析维度，Schema 约束 |
| 质量保障 | QA Agent 自动打分（0-100），低于 70 分自动打回重做 |
| 可追溯 | Trace 全程记录每步输入输出，Blackboard 保存中间产物 |
| 可修正 | QA 打回后从出错 Agent 开始局部重跑，而非全量重做 |

---

## 三、核心架构：逐层讲清楚

### 3.1 五层架构总览

```
┌─────────────────────────────────────────────┐
│              HTTP / SSE 层 (web/)             │ ← Gin + JWT + SSE
├─────────────────────────────────────────────┤
│            应用服务层 (service/)              │ ← TaskService、编排逻辑
├─────────────────────────────────────────────┤
│           工作流引擎层 (workflow/)             │ ← 状态机、路由、调度
├─────────────────────────────────────────────┤
│           Agent 适配器层 (agent/)             │ ← 5 Agent 实现 + LLM 适配
├─────────────────────────────────────────────┤
│     领域层 (domain/)     │  基础设施层       │ ← 聚合根 + 消息总线 + 持久化
└─────────────────────────────────────────────┘
```

**面试话术**：这不是一个简单的 CRUD 应用。5 个 Agent 通过消息总线异步协作，共享 Blackboard 状态，支持 QA 打回局部重跑，需要处理幂等消费、租约并发、死信队列等分布式系统问题。

### 3.2 领域层设计（DDD 核心重点）

```
domain/
├── task.go        # Task 聚合根 + 状态机 + Agent 枚举
├── report.go      # Report 聚合根 + SWOT/Feature/Pricing 值对象
├── trace.go        # Trace 聚合根 + TraceNode/Step
├── chat.go        # Chat 会话聚合根
├── user.go        # User 模型
├── protocol.go     # 消息协议：13 种 MessageType + Envelope + 路由
└── agent.go        # Agent 接口 + RunInput/RunOutput
```

**为什么这样设计**：

1. **聚合根封装行为**：`Task` 不只是数据结构，它有 `CanTransitionTo()` 和 `TransitionTo()` 方法。状态转换规则不是散落在 workflow 里，而是 Task 自己说了算。这符合 DDD 的"聚合根是事务一致性边界"原则。

2. **领域层零依赖**：`domain/` 包只导入标准库和 `github.com/google/uuid`，不引用任何 `internal/` 包。这意味着你可以只看 domain 包就理解整个业务的核心概念，不需要追踪依赖链。`Agent.Run()` 的 `bb` 参数用 `any` 类型而非 `state.Blackboard`，就是为了避免 domain → state 的循环依赖。

3. **消息协议是领域语言**：`MsgPlanReady`、`MsgQAReject` 这些不是技术概念，是业务语言。把 13 种消息类型放在 domain 包，意味着任何人读到 `domain.MsgQAReject` 就知道它的业务含义，而非去 `protocol/types.go` 里找魔法字符串。

4. **Agent 接口在领域层定义**：接口在 `domain/agent.go`，实现在 `agent/coordinator.go` 等适配器层。这是 DDD 的经典反腐败层模式——领域层定义"需要什么样的能力"，适配器层提供"具体怎么做到"。

**面试追问准备**：

> Q: 为什么不用贫血模型？
> 
> A: 如果 Task 只是 `{ ID, Status, ... }` 的数据结构，状态转换规则就会散落在 workflow 包的 `CanTransition()` 函数里。任何人修改状态机都要翻遍 workflow 代码找所有引用点。把 `CanTransitionTo` 放在 Task 上，修改只影响一处，并且编译器帮你检查所有调用方。

### 3.3 Agent 流水线与 QA 打回

```
用户 → Coordinator → Collector → Analyst → Writer → QA → 报告完成
                ↑                                         |
                └─────────── QA 打回 (qa_reject) ─────────┘
```

**为什么是固定流水线而非动态编排？**

1. **可预测性**：5 Agent 固定顺序，调试时知道每一步的输入来自谁
2. **QA 打回确定性**：打回后从目标 Agent 重跑，Coordinator 始终执行（重新规划）
3. **简单有效**：竞品分析的场景就是线性流程，动态 Agent 选择增加了复杂性但不增加价值

**QA 打回的完整机制**：

```go
// QA 打回后，ShouldRunAgent 决定哪些 Agent 需要重跑
func ShouldRunAgent(name, target AgentName, isRework bool) bool {
    if !isRework { return true }       // 正常流程全跑
    if name == AgentCoordinator { return true } // Coordinator 始终跑（重新规划）
    return AgentIndex(name) >= AgentIndex(target) // 从 target 开始往后跑
}
```

> Q: 为什么不打回给 Writer 而是给 Coordinator？
> 
> A: 打回信息（Issue 列表）需要被重新规划。Coordinator 收到 `qa_reject` 后会解析 Issue，决定是让 Analyst 重做分析、还是 Writer 重写报告、还是直接补充证据。如果直接打回 Writer，它无法自主决策下一步。

### 3.4 消息协议设计

```go
type MessageEnvelope struct {
    MessageID     string          // UUID，幂等去重
    CorrelationID string          // 关联 ID，串联一次任务的所有消息
    CausationID   string          // 因果 ID，可追溯消息触发链
    TaskID        string          // 任务 ID
    FromAgent     string          // 发送方
    ToAgent       string          // 接收方
    MessageType   MessageType     // 消息类型（13 种之一）
    Kind          string          // command | event | query | reply
    Payload       json.RawMessage // 业务载荷（延迟反序列化）
    Artifacts     []ArtifactRef   // 大数据走 Blackboard 引用
    Status        MessageStatus   // pending | running | completed | failed
    Attempt       int             // 重试次数
}
```

**为什么用信封模式而非直接传递结构体？**

1. **解耦**：Payload 是 `json.RawMessage`，消费者按 MessageType 选择反序列化为哪种结构体。发送方不需要知道消费方的类型定义
2. **可追溯**：CorrelationID + CausationID 组成因果链，可以在日志/Trace 中追踪一条消息的前世今生
3. **统一传输**：无论 Kafka、Redis Stream 还是内存队列，信封结构不变。Bus 接口只传 `MessageEnvelope`
4. **超大数据不穿队列**：Artifacts 引用 Blackboard 里的数据，队列只传引用（`task:123:collector:output`），大数据在 Redis 里用 key 取

### 3.5 Blackboard 共享状态模式

```
task:ID:scope:field → JSON value with version

task:abc123:task:meta        → TaskMeta{Competitors, Dimensions, ...}
task:abc123:workflow:state   → WorkflowState{CurrentAgent, NextAgent, ...}
task:abc123:collector:output → CollectorOutput{Query, Sources, Materials}
task:abc123:analysis:output  → AnalysisOutput{SWOT, Features, Pricing, ...}
task:abc123:report:draft     → ReportDraft{Sections, References}
task:abc123:qa:result         → QARecord{Score, Issues, ...}
```

**为什么用 Blackboard 而不是数据库？**

1. **低延迟**：Redis 单次读写 <1ms，MySQL 单次查询 5-50ms。Agent 每步都要读写 Blackboard
2. **分区一致性**：每个 Agent 只写自己的分区（如 Collector 只写 `collector:*`），读别人的分区。无写冲突
3. **版本控制**：每个值带 `Version` 字段，支持乐观并发。QA 打回后重跑时，读取 `v2` 覆盖 `v1`
4. **自动过期**：Redis TTL 24h，任务完成后自然释放内存

**权衡**：Blackboard 数据不会永久保留。最终产出物（报告、Trace）通过 Repository 写入 MySQL 做持久化。

### 3.6 消息总线与可靠投递

```
                    ┌─────────────────────┐
                    │     Bus 接口         │
                    │  Publish/Register   │
                    │       /Run           │
                    └──────┬──────────────┘
                           │
              ┌────────────┼────────────────┐
              │            │                │
        KafkaBus      RedisStreamBus    MemoryBus
        (生产)          (中等规模)       (开发)
```

**幂等消费机制**：

```go
// orchestrator/runtime.go 核心逻辑
func (r *Runtime) wrapHandler(handler Handler) Handler {
    return func(ctx context.Context, msg domain.MessageEnvelope) error {
        // 1. 幂等检查：7天内同一消息ID只处理一次
        if !r.store.Idempotent(ctx, msg.MessageID) {
            return nil // 已处理，跳过
        }
        // 2. 租约获取：同一 task+agent 同时只有一个 Worker
        lease, ok := r.store.AcquireLease(ctx, taskID, agentName)
        if !ok {
            return ErrLeaseConflict // 其他 Worker 正在处理
        }
        defer r.store.ReleaseLease(ctx, lease)
        
        // 3. 执行业务逻辑
        err := handler(ctx, msg)
        
        // 4. 成功则 XACK + 保存 Checkpoint
        // 5. 失败则重试 5 次，超过则进入 DLQ
    }
}
```

**为什么需要这三种机制？**

| 机制 | 解决的问题 | 不加会怎样 |
|------|-----------|-----------|
| 幂等去重 | 消息重复投递（Kafka/Redis 至少一次语义） | 同一条任务被执行两次 |
| 租约控制 | 多 Worker 竞争同一任务 | 两个 Worker 同时跑同一个 Agent |
| 死信队列 | 永久失败的消息不能无限重试 | 系统卡在重试死循环 |

### 3.7 事件系统双写架构

```
          Agent 完成 → HybridHub.Publish(taskID, eventType, data)
                          │
                  ┌───────┴───────┐
                  │               │
             TaskHub          Publisher
            (内存)          (DB 持久化)
              │               │
         SSE 推送         event_logs 表
       (实时 32 条缓冲)    (序列号递增)
              │               │
          前端浏览器      API 重连时
                        ListSince 增量补齐
```

**为什么双写？**

- SSE 实时性好但不可靠（断连丢事件）
- DB 持久化可靠但延迟高
- 双写 + `ListSince(afterSeq)` 实现断连后增量补齐，前端不会丢失任何事件

### 3.8 长期记忆系统

四层记忆架构的设计逻辑：

| 层 | 存储 | 生命周期 | 作用 |
|----|------|---------|------|
| L0 执行记忆 | Blackboard (Redis) | 任务级 | 当前任务的运行时状态 |
| L1 显式记忆 | MySQL preferences | 项目级 | 用户偏好、COMPETE.md 规则 |
| L2 事实记忆 | MySQL + Milvus | 跨任务 | 可复用的结构化事实 |
| L3 经验记忆 | MySQL + Milvus | 跨任务 | 任务经验摘要/教训 |

**Hybrid Retriever 如何工作**：

1. Agent 请求记忆上下文，指定 Token 预算（如 Analyst 1800 tokens）
2. `HybridRetriever` 同时查询 Milvus（向量相似度）和 MySQL（关键词匹配）
3. `Assembler` 按 Agent 类型分配预算、去重、截断
4. 注入 `RunInput.Memory` 字段，作为 LLM prompt 的后缀

**降级策略**：
- Milvus 不可用 → 纯 MySQL 关键词检索
- Embedding API 失败 → FNV-1a 哈希作为本地 fallback
- Firecrawl MCP 不可用 → 纯 LLM 模式运行（无外部数据源）

---

## 四、关键设计决策与权衡

### 4.1 为什么用 Agent 流水线而非 ReAct 循环？

| 方案 | 优点 | 缺点 |
|------|------|------|
| ReAct 循环 | 灵活，Agent 自主决策 | 不可预测、Token 消耗高、Debug 困难 |
| 固定流水线 + QA 打回 | 可预测、可追溯、可断点续跑 | 不够灵活 |

**选择理由**：竞品分析有明确的步骤（采集→分析→写报告→质检），每步的输入输出都有定义。ReAct 适合探索性任务，但这里的任务是确定性的。打个比方：做菜用 ReAct 可以不断尝味道调整，但照着菜谱做流水线效率更高。

### 4.2 为什么 Outbox 模式而不是两阶段提交？

```
方案 A：直接发消息
  DB 写入 → Bus 发送
  问题：DB 成功但 Bus 失败 → 消息丢了

方案 B：两阶段提交
  问题：性能差，Bus 不支持 XA

方案 C（我们选的）：Outbox 模式
  DB 写入 + Outbox 写入（同一事务）→ 后台轮询 → Bus 发送
  优点：保证至少一次投递，无需分布式事务
```

### 4.3 为什么 Blackboard 用 Redis 而不是进程内缓存？

- **多进程部署**：API 进程写作、Worker 进程消费，必须共享状态
- **自动过期**：Redis TTL 解决了任务完成后的内存释放
- **原子操作**：Redis `SetNX` 支持租约和幂等，无需额外中间件

### 4.4 为什么 Agent 接口的 `bb` 参数用 `any` 而不是 `state.Blackboard`？

DDD 中领域层不应该依赖基础设施层。`state.Blackboard` 是 Redis 实现的接口，放在 domain 会导致 `domain → state → (Redis)` 的依赖链。用 `any` 解除依赖后：
- domain 包只定义"需要能存取状态"这个契约
- agent 实现层负责类型断言
- 未来如果 Blackboard 实现换成 etcd 或内存，domain 不需要改

### 4.5 为什么消息载荷用 `json.RawMessage` 而不是泛型？

1. **消费者按需反序列化**：Coordinator 只关心 `TaskCreatedPayload`，不需要知道 `QAResultPayload` 的结构
2. **向后兼容**：新增载荷类型不影响已有消息的处理
3. **日志友好**：直接 `json.Marshal` 就能看到原始 payload，不需要类型断言

---

## 五、可以改进的地方

### 5.1 架构层面

| 改进点 | 现状 | 建议 | 优先级 |
|--------|------|------|--------|
| **CQRS 分离读写** | Web 层直接读 MySQL | Report/Trace 查询走独立 read model（如 ClickHouse），避免复杂报表查询影响写入 | 中 |
| **事件溯源** | 当前只做 event log（双写） | 完整 EventSourcing：聚合根状态由事件重建，支持任意时间点回放 | 低 |
| **Saga 替代 QA 打回** | QA 打回经 Coordinator 重新规划 | 引入 Saga 编排器，支持并行 Agent 执行（如 Analyst 和 Writer 并行） | 中 |
| **gRPC 内部通信** | Agent 间走 Bus 异步消息 | 高频同步查询走 gRPC（如查询 Blackboard），异步事件走 Bus | 低 |
| **可观测性** | 只有 Trace + 日志 | 引入 OpenTelemetry 链路追踪 + Prometheus 告警规则 | 高 |

### 5.2 代码层面

| 改进点 | 现状 | 建议 |
|--------|------|------|
| **wire.go 未生效** | `app/web_server.go` 手工编排依赖 | 清理 wire.go 或确认手工编排 |
| **测试覆盖率** | 14 个测试文件，核心逻辑缺测试 | Engine、Dispatch、QA Loop 需要单元测试 |
| **错误码体系** | 用 `AgentError.Kind` 字符串 | 定义枚举常量，前端根据 code 做 i18n |
| **配置热更新** | Viper 热加载但 Agent 未消费 | 监听 config 变更 channel，运行时切换 LLM 模型 |
| **memory 包职责过重** | memory/ 有 12 个文件 | 抽取 `Embedder`/`Retriever`/`Assembler` 为独立包 |

### 5.3 业务层面

| 改进点 | 说明 |
|--------|------|
| **并行采集** | 当前 Collector 是单次搜索，可以并行搜索多个竞品 |
| **增量报告** | 当前整个报告重生成，可以只重写 QA 指出问题的章节 |
| **用户偏好学习** | L1 显式记忆只读 preferences，可以自动从历史反馈中学习 |
| **多语言支持** | 报告目前是中文，可以加 language 参数控制输出语言 |
| **成本控制** | 每次任务 LLM token 消耗无上限，可以按任务设置 token budget |

---

## 六、面试高频问题与参考回答

### Q1: 这个项目最难的技术问题是什么？

**分布式消息的精确一次投递**。在"至少一次"投递的 Kafka/Redis Stream 上，要保证"恰好一次"语义，需要三层机制：

1. **Outbox 模式**：DB 写入和消息发送在同一事务，保证不丢
2. **幂等消费**：Redis SetNX 7 天去重，保证不重
3. **租约+心跳**：SetNX 30s 防并发，保证不多

三层加起来，理论上做到了 at-least-once + idempotent ≈ exactly-once。

### Q2: 为什么不用现成的 Workflow 引擎（如 Temporal）？

1. **学习成本**：团队 3 人，引入 Temporal 需要运维 Server + 可视化面板
2. **轻量够用**：当前 5 个 Agent + 固定流水线，状态转换表只有 9 条规则
3. **消息自治**：Agent 间通过 Bus 异步通信，每个 Agent 独立消费、独立租约，天然支持水平扩容
4. **调试透明**：所有状态转换代码都在一个文件里（`workflow/state.go`），比 Temporal 的可视化面板更容易排查

如果未来 Agent 数量 > 10 或需要动态编排，可以迁移到 Temporal。

### Q3: QA 打回的边界条件怎么处理？

1. **最大轮次**：`defaultMaxRounds = 3`，超过就强制完成（`completed`），不再打回
2. **目标选择**：`ReworkTarget()` 从 `QAResultPayload.TargetAgent` 解析重做目标，默认 Analyst
3. **状态隔离**：打回后 Workflow.Round++，新一轮的 Blackboard 值 Version 递增，不会和旧数据冲突
4. **租约保护**：同一 task+agent 同时只有一个 Worker 在处理

### Q4: 如果 Redis 挂了怎么办？

| 组件 | Redis 挂了的影响 | 降级策略 |
|------|-----------------|---------|
| Blackboard | Agent 无法读写中间状态 | ⚠️ 不可用，必须恢复 |
| Bus | 消息无法投递 | 降级为 MemoryBus（开发模式） |
| 幂等检查 | 重复消息可能被处理两次 | 外部幂等保护（DB 唯一约束） |
| 租约 | 多 Worker 可能同时执行 | 加大 `MaxAttempts` 容错 |
| JWT | 用户无法登录 | 降级为内存 Token |

建议：生产环境 Redis 用 Sentinel/Cluster，至少 2 个从节点。

### Q5: 消息顺序性怎么保证？

1. **Kafka**：同一 TaskID 的消息发到同一 Partition（用 TaskID 做 Partition Key），保证同一任务的消息有序
2. **Redis Stream**：XReadGroup 保证同一 Consumer Group 内不重复消费
3. **幂等保护**：即使乱序到达，幂等检查 + 租约保证只有最新状态被处理

### Q6: 你提到了 DDD，这个项目的聚合根是什么？为什么？

**Task 是唯一的聚合根**。原因：

1. **事务一致性边界**：一个 Task 的所有状态变更（创建→运行→完成/失败）必须原子性
2. **生命周期完整**：Task 从创建到完成是一个完整的业务流程，Report/Trace/Chat 是附属值对象
3. **入口点唯一**：所有操作都通过 TaskID 关联，Task 是状态机的中心

Report 和 Trace 是 Task 的值对象而非独立聚合根，因为它们的生命周期完全依附于 Task。

### Q7: 系统的水平扩展能力怎样？

| 组件 | 扩展方式 | 瓶颈 |
|------|---------|------|
| API 进程 | 无状态，直接加实例 | 数据库连接数 |
| Worker 进程 | 加实例 + `WORKER_ROLE` 角色化 | Redis Stream 消费者组自动均衡 |
| LLM 调用 | 加 API Key + 加并发 | ARK API Rate Limit |
| Blackboard | Redis Cluster 分片 | Redis 内存上限 |
| Bus | Kafka Partition 数量 | Partition 数量限制并发消费组 |

---

## 七、需要掌握的知识点清单

### 7.1 分布式系统

| 知识点 | 项目中的体现 | 深度要求 |
|--------|------------|---------|
| 消息投递语义 | at-least-once + 幂等 ≈ exactly-once | 能讲出三层机制 |
| 分布式锁 | Redis SetNX 租约 + 心跳续约 | 能画出租约生命周期图 |
| 事务发件箱 | Outbox 表 + 后台轮询 | 能对比 2PC 和 Outbox |
| 死信队列 | 5 次重试 → DLQ 表 → 人工介入 | 能讲出指数退避 |
| CQRS | 读写分离思路（MySQL 写 + 潜在读分离） | 理解概念即可 |

### 7.2 Go 语言

| 知识点 | 项目中的体现 | 深度要求 |
|--------|------------|---------|
| interface 满足 | `domain.Agent` 接口，5 个实现隐式满足 | 能解释隐式接口的优点 |
| 闭包 | `bus.MemoryBus` 的 handler 注册 | 能写带状态的闭包 |
| channel | `event.TaskHub` 的 Publish/Subscribe | 能讲 channel buffer 满时的丢弃策略 |
| sync 原语 | `BlackboardRegistry` 的 RWMutex | 能区分 RWMutex 和 Mutex 场景 |
| context | 所有 Agent Run 方法第一个参数 | 能讲超时传播和取消 |

### 7.3 DDD

| 知识点 | 项目中的体现 | 深度要求 |
|--------|------------|---------|
| 聚合根 | Task 是唯一聚合根 | 能解释为什么 Report 不是 |
| 值对象 | SWOT、FeatureRow、PricingTable | 能区分 Entity 和 Value Object |
| 领域服务 vs 应用服务 | `domain.RouteNext` vs `workflow.Engine` | 能举 3 个例子 |
| 反腐败层 | `Agent` 接口在 domain，实现在 agent | 能画洋葱架构图 |
| 通用语言 | `MsgQAReject`、`AgentCoordinator` | 能讲命名即文档 |

### 7.4 数据结构

| 知识点 | 项目中的体现 | 深度要求 |
|--------|------------|---------|
| 状态机 | `TaskTransitions` 转换表 | 能画出完整状态转换图 |
| 信封模式 | `MessageEnvelope` + `json.RawMessage` | 能讲延迟反序列化的好处 |
| 发布-订阅 | `TaskHub.Subscribe/Publish` + SSE | 能画出背压处理 |
| 对象池 | Redis 连接池、DB 连接池 | 理解即可 |

### 7.5 Redis

| 知识点 | 项目中的体现 | 深度要求 |
|--------|------------|---------|
| Stream | XADD/XReadGroup/XACK | 能讲出消费者组 vs 普通队列 |
| Hash | Blackboard 分区存储 | 能解释为什么用 Hash 而不是 String |
| SetNX | 幂等去重 + 租约 | 能讲出 SetNX vs SET + EX 的区别 |
| TTL | 24h 自动过期 | 能解释为什么不用 DEL |
| Pipeline | 无直接使用，但理解批量写入 | 概念理解 |

---

## 八、项目亮点总结（30 秒版）

> CompeteAI 是一个**多 Agent 协作的竞品分析平台**，核心架构亮点是：
> 1. **DDD 驱动**：领域层零依赖，Task 聚合根封装状态机，协议层定义业务语言
> 2. **三层可靠性**：Outbox 保证不丢 + 幂等保证不重 + 租约保证不多
> 3. **QA 打回机制**：质检不合格自动局部重跑，不是从头开始
> 4. **双写事件系统**：内存实时推送 + DB 持久化回放，断线不丢数据
> 5. **四层记忆架构**：从执行记忆到经验记忆，Agent 越做越好