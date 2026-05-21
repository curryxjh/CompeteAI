# 系统架构与工作流设计

## 1. 系统架构

```
┌──────────────────────────────────────────────────────────┐
│                    前端 (Vue 3 + Vite)                     │
│        报告查看 / 溯源跳转 / Agent 决策回放 / 任务管理        │
└──────────────────────┬───────────────────────────────────┘
                       │ HTTP REST + SSE (实时状态推送)
┌──────────────────────▼───────────────────────────────────┐
│                    API 层 (Gin)                            │
│  /api/tasks  创建/查询任务                                  │
│  /api/tasks/:id/status  任务状态 (SSE)                     │
│  /api/reports/:id  查看报告                                │
│  /api/traces/:task_id  查看 Agent 执行链路                  │
│  /api/agents/:id/decisions  查看 Agent 决策过程              │
└──────────────────────┬───────────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────────┐
│         业务编排层 (Coordinator + Eino Graph + Kafka)      │
│                                                          │
│  ┌─────────────────────────────────────────────────┐     │
│  │           Coordinator Agent (任务协调者)          │     │
│  │  任务拆解 / 竞品列表确认 / 进度管理 / 最终交付     │     │
│  └──────────────────────┬──────────────────────────┘     │
│                         │                                │
│  ┌──────────────────────▼──────────────────────────┐     │
│  │              Eino Graph (工作流编排)              │     │
│  │                                                  │     │
│  │  Collector ──→ Analyst ──→ Writer ──→ QA ──→    │     │
│  │       ▲            ▲            │       │  │     │     │
│  │       └─── 打回 ───┴─── 打回 ───┘  通过  │  │     │     │
│  │       │            │            │  不通过│  │     │     │
│  │       └──────── Kafka ──────────┴────────┘     │     │
│  └─────────────────────────────────────────────────┘     │
│                                                          │
│  每个 Agent 节点: 消费 Kafka Topic → 执行 → 投递下个 Topic    │
│  Coordinator 通过 shared_state (Redis/MySQL) 监控全局进度   │
└──────────────────────┬───────────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────────┐
│               Agent 层 (5 个 Agent)                        │
│                                                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ ┌───┐│
│  │Coordinator│ │Collector │ │ Analyst  │ │ Writer │ │QA ││
│  │  协调者   │ │  采集者   │ │  分析者   │ │ 撰写者  │ │质检││
│  │          │ │          │ │          │ │        │ │   ││
│  │任务拆解  │ │信息搜索  │ │功能对比  │ │报告生成│ │事实││
│  │需求澄清  │ │数据爬取  │ │SWOT分析  │ │Schema化│ │校验││
│  │进度追踪  │ │RAG检索   │ │用户评价  │ │格式化  │ │完整性│
│  │结果汇总  │ │          │ │          │ │        │ │   ││
│  └──────────┘ └──────────┘ └──────────┘ └────────┘ └───┘│
│                                                          │
│          每个 Agent: Prompt模板 + LLM调用 + Schema校验      │
│          所有 Agent 通过 Shared State 读写全局上下文         │
└──────────────────────┬───────────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────────┐
│              Shared State (共享上下文 / Blackboard)        │
│                                                          │
│  ┌──────────────────────────────────────────────────┐    │
│  │  task_context: { task_id, competitors[], status } │    │
│  │  collector_state: { raw_data, sources[] }         │    │
│  │  analyst_state: { swot, feature_matrix }          │    │
│  │  writer_state: { draft_report }                   │    │
│  │  qa_state: { issues[], verified }                 │    │
│  │  artifacts: { 每个Agent的中间产出, 溯源引用 }        │    │
│  └──────────────────────────────────────────────────┘    │
│              ↓ 存储于 Redis (实时) + MySQL (持久)           │
└──────────────────────┬───────────────────────────────────┘
                       │
┌──────────────────────▼───────────────────────────────────┐
│                    基础设施层                              │
│                                                          │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌───────┐  │
│  │  LLM   │ │ Kafka  │ │ MySQL  │ │ Redis  │ │Milvus │  │
│  │ Client │ │ 消息总线│ │+ GORM  │ │共享状态 │ │(可选) │  │
│  │(豆包API)│ │控制流   │ │持久存储│ │+ 缓存  │ │       │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └───────┘  │
│  ┌────────┐ ┌────────┐                                  │
│  │ Viper  │ │  Zap   │                                  │
│  │ 配置管理│ │ 日志   │                                  │
│  └────────┘ └────────┘                                  │
└─────────────────────────────────────────────────────────┘
```

### 关键设计决策：Kafka（控制流）+ Shared State（数据流）

Agent 间通信分为两层：

- **控制流（Kafka）**：传递"谁该做什么"的事件通知，消息体轻量（任务 ID + 元信息）。Agent 收到事件后从 Shared State 读取完整上下文。
- **数据流（Shared State / Blackboard）**：存储每个 Agent 的输入输出、中间产物、溯源引用。所有 Agent 可读写自己负责的部分，Coordinator 和 QA 可跨 Agent 读取做全局判断。

### Kafka Topic 设计（5 Agent）

```
┌──────────────────────────────────────────────────────────┐
│                  Kafka Topic 流转                          │
│                                                          │
│  task.create ──→ coordinator.input                        │
│                       │                                  │
│           ┌───────────┴────────────┐                     │
│           ▼                        ▼                     │
│    需要用户澄清              任务已就绪                    │
│    → user.clarify           → collector.input            │
│           │                        │                     │
│           ▼                        ▼                     │
│    用户回复后               collector.output              │
│    → coordinator.input            │                     │
│                        ┌──────────┘                     │
│                        ▼                                │
│                  analyst.input ──→ analyst.output       │
│                                          │              │
│                        ┌─────────────────┘              │
│                        ▼                                │
│                  writer.input ──→ writer.output         │
│                                       │                 │
│                        ┌──────────────┘                 │
│                        ▼                                │
│                  qa.input ──→ qa.output                 │
│                        │           │                    │
│              ┌─────────┘           └─────────┐          │
│              ▼ (打回)                       ▼ (通过)     │
│     coordinator.input              report.final        │
│     (含 reject_target                 │                │
│      和 reject_reason)                ▼                │
│      │                         API 通知前端             │
│      ▼                                                │
│  Coordinator 解析 reject_target                        │
│  → collector.input / analyst.input / writer.input      │
└─────────────────────────────────────────────────────────┘
```

---

## 2. Agent 通信设计（两层模型）

Agent 间通信分为两层：**控制流**（Kafka 事件通知）和**数据流**（Shared State / Blackboard 共享上下文）。

### 2.1 为什么需要两层通信？

纯 Kafka 流转存在两个问题：
1. **消息体膨胀**：每个 Agent 的输出（如完整的竞品数据 JSON）可能几十 KB，放 Kafka 消息体太重
2. **无法跨 Agent 读中间产物**：Writer 需要看 Collector 的原始采集数据，QA 需要对比 Analyst 的输入输出判断是否有幻觉，纯 Topic 流转做不到

因此采用 **Kafka（轻量通知）+ Shared State（完整数据）** 的分层设计：

```
┌──────────────────────────────────────────────────────┐
│                  Agent 通信两层模型                     │
│                                                      │
│  Agent A ──→ Kafka (事件: "我完成了, 数据在 key=xxx")  │
│                                           │          │
│                   ┌───────────────────────┘          │
│                   ▼                                  │
│            Agent B 收到事件                           │
│                   │                                  │
│                   ▼                                  │
│            从 Shared State 读取完整上下文               │
│            (Redis: task:{task_id}:collector_output)   │
│                   │                                  │
│                   ▼                                  │
│            Agent B 执行，完成后：                      │
│            1. 写入结果到 Shared State                 │
│            2. 发送事件到下一个 Kafka Topic              │
└──────────────────────────────────────────────────────┘
```

### 2.2 Shared State 结构（Blackboard 模式）

```
Redis Key                          │ 内容
───────────────────────────────────┼──────────────────────
task:{id}:meta                     │ 任务元信息、状态、竞品列表
task:{id}:coordinator_input        │ 用户原始输入 + 澄清结果
task:{id}:collector_output         │ 采集到的原始数据 + 来源URL
task:{id}:analyst_output           │ SWOT、功能矩阵、用户画像
task:{id}:writer_output            │ 报告草稿
task:{id}:qa_output                │ 质检结果、问题列表
task:{id}:artifacts                │ 所有中间产物索引
task:{id}:trace                    │ 执行时间线
```

每个 Agent 的执行流程统一为：

```
1. 消费 Kafka Topic 中的事件通知（轻量 JSON，只含 task_id + 元信息）
2. 从 Shared State (Redis) 读取上游 Agent 的完整输出 + 任务上下文
3. 执行（调用 LLM / RAG / 工具）
4. 将结果写入 Shared State
5. 投递事件通知到下一个 Kafka Topic
```

### 2.3 Agent 间交互模式

除了标准的流水线流转，还有两种交互模式：

**模式一：请求-响应（Agent 之间的同步问答）**

当 QA Agent 质疑 Analyst 某条结论时，不是直接打回全量重做，而是先发一个"质疑请求"：

```
QA Agent                                    Analyst Agent
   │                                              │
   │── Kafka: qa.query ─────────────────────────→│
   │   { type: "query",                          │
   │     question: "SWOT中'性能领先'的结论        │
   │               依据是什么？请提供数据来源"       │
   │   }                                         │
   │                                              │── 查询 Shared State
   │                                              │   找到原始采集数据
   │                                              │── 生成解释
   │←─ Kafka: analyst.response ──────────────────│
   │   { type: "response",                        │
   │     answer: "...",                           │
   │     evidence: [{source_url, excerpt}]        │
   │   }                                         │
   │                                              │
   │── QA 判断解释是否合理                        │
   │   ├─ 合理 → 通过                             │
   │   └─ 不合理 → 正式打回（含具体修正要求）       │
```

**模式二：Coordinator 干预（用户澄清循环）**

Coordinator 收到任务后，如果发现竞品列表模糊或需求不明确，主动向用户提问：

```
Coordinator Agent                           用户（前端）
   │                                              │
   │── Kafka: user.clarify ─────────────────────→│
   │   { type: "clarification",                  │
   │     questions: [                            │
   │       "请确认要对比的竞品：Cursor vs          │
   │        GitHub Copilot vs Codeium，             │
   │        还是全部？",                           │
   │       "你更关注功能对比还是定价策略？"           │
   │     ]                                       │
   │   }                                         │
   │                                              │── 前端展示问题
   │                                              │── 用户回复
   │←─ API: POST /api/tasks/:id/clarify ─────────│
   │   { answers: [...] }                        │
   │                                              │
   │── 更新 Shared State，继续流程                  │
```

---

## 3. Eino 工作流设计

### 3.1 Agent 数量分析：为什么是 5 个？

| Agent | 职责 | 为什么不能合并 |
|---|---|---|
| **Coordinator**（协调者） | 任务拆解、需求澄清、进度管理、结果汇总 | Collector 不应该管进度，QA 不应该管用户交互。需要一个"项目经理"角色，这也是评委会看重的编排能力体现 |
| **Collector**（采集者） | Web 搜索、网页抓取、RAG 检索、数据预处理 | 如果 Collector 还管需求澄清和进度管理，职责过重且 Prompt 难以写好 |
| **Analyst**（分析者） | 功能对比、SWOT 分析、用户评价聚合、定价对比 | 分析和采集是不同的思维链，合并会导致单次 LLM 调用上下文过长 |
| **Writer**（撰写者） | 报告生成、Schema 格式化、可视化数据准备 | 分析和撰写分开的优势：Writer 只关注"怎么呈现"，不需要理解"怎么分析" |
| **QA**（质检者） | 事实校验、完整性检查、Schema 合规检查 | QA 必须独立，否则"自己审自己"形成伪闭环（扣分项） |

> 结论：4 个是最小可用集（Collector + Analyst + Writer + QA），5 个是推荐方案（加 Coordinator）。Coordinator 是评委会重点关注的"编排能力"的直接体现。

### 3.2 Eino 核心概念与映射

| Eino 概念 | 本项目映射 | 说明 |
|---|---|---|
| **Component** | 每个 Agent | 可复用的功能单元，内部是 Chain（Prompt → LLM → Parse → Validate） |
| **Graph** | 竞品分析工作流 | 5 个 Component 的有向无环图 + 条件边 |
| **Chain** | Agent 内部的 LLM 调用链 | Prompt 模板 → LLM 调用 → JSON 解析 → Schema 校验 → Shared State 写入 |
| **Tool** | Web 搜索、RAG 检索、URL 抓取 | Agent 可调用的外部能力，Collector 和 QA 使用最多 |
| **Callback** | Trace 日志、Token 统计、耗时记录 | 切面注入，每个 Agent 执行前后自动触发 |

### 3.3 主流程（含 Coordinator）

```
                    ┌──────────────┐
                    │  用户输入      │
                    │ (竞品分析需求)  │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ Coordinator  │  任务协调 Agent
                    │              │  解析需求 → 拆解竞品列表
                    │              │  不明确则发 user.clarify
                    └──────┬───────┘
                           │ 竞品列表已确认
                    ┌──────▼───────┐
                    │ Collector    │  信息采集 Agent
                    │              │  搜索/抓取/去重/结构化
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ Analyst      │  分析 Agent
                    │              │  SWOT / 功能矩阵 / 画像
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ Writer       │  报告撰写 Agent
                    │              │  报告草稿 / Schema 格式化
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ QA           │  质检 Agent
                    │              │  事实校验 / 完整性 / 合规
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │ 通过        │ 不通过      │
              ▼            ▼            │
        ┌──────────┐  ┌──────────┐     │
        │report.    │  │ QA 输出到 │     │
        │final     │  │coordinator│     │
        │          │  │.input     │─────┘
        │ 通知前端  │  │(含 reject_target
        └──────────┘  │ 和问题详情)  │
                      └────────────┘
                           │
                    ┌──────▼───────┐
                    │ Coordinator  │  解析打回信息
                    │              │  路由到对应 Agent 的 input
                    └──────────────┘
```

### 3.4 打回策略（反馈闭环 — 35% 权重核心得分项）

打回不直接投递给目标 Agent，而是统一经过 Coordinator 中转，由 Coordinator 决定路由和优先级：

```
QA 发现问题 → qa.output (type: rejection)
                    │
            ┌───────▼────────┐
            │  Coordinator   │
            │  解析 reject   │
            │  判断严重程度   │
            └───────┬────────┘
                    │
      ┌─────────────┼─────────────┐
      ▼             ▼             ▼
collector.input  analyst.input  writer.input
(数据缺失)      (分析矛盾)     (格式问题)
```

| QA 发现的问题 | 打回目标 | 重做要求 | 严重程度 |
|---|---|---|---|
| 信息源不足 / 数据缺失 | Collector | 补充指定竞品的数据采集 | 中 |
| 分析逻辑矛盾 / 结论无依据 | Analyst | 修正分析，标注推理链 | 高 |
| 报告格式不符 Schema / 字段缺失 | Writer | 按 Schema 修正格式 | 低 |
| 事实性错误（幻觉） | Collector + Analyst | 重新采集并修正 | 严重 |

**打回消息格式**：

```json
{
  "type": "rejection",
  "from": "qa_agent",
  "task_id": "xxx",
  "severity": "high",
  "reject_targets": ["analyst"],
  "issues": [
    {
      "id": "issue-001",
      "location": "SWOT.strengths[2]",
      "claim": "Cursor 的性能在同类产品中最优",
      "problem": "结论缺乏数据支撑，未标注来源",
      "suggestion": "补充性能对比数据或标注'基于用户评价推断'",
      "require_retrace": true
    }
  ],
  "original_message_id": "msg-001",
  "attempt": 1
}
```

### 3.5 Eino Graph 定义示例

```go
graph := eino.NewGraph[*AgentMessage, *AgentMessage]()

// 5 个 Agent 节点
graph.AddNode("coordinator", coordinatorNode)
graph.AddNode("collector", collectorNode)
graph.AddNode("analyst", analystNode)
graph.AddNode("writer", writerNode)
graph.AddNode("qa", qaNode)

// 主流程边
graph.AddEdge("coordinator", "collector")
graph.AddEdge("collector", "analyst")
graph.AddEdge("analyst", "writer")
graph.AddEdge("writer", "qa")

// QA 条件边：通过 → 结束 / 不通过 → Coordinator（由它路由打回）
graph.AddConditionalEdge("qa", qaDecision, map[string]string{
    "pass":   eino.END,
    "reject": "coordinator",
})

// Coordinator 条件边：如果用户输入已明确 → Collector / 需要澄清 → 等待用户
graph.AddConditionalEdge("coordinator", coordinatorDecision, map[string]string{
    "ready":    "collector",
    "clarify":  "wait_user_input",  // 挂起，等待用户通过 API 回复
})
```
