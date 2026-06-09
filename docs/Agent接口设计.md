# Agent 接口设计

## 1. 文档目标

本文档定义 CompeteAI 多 Agent 体系中的统一接口、角色边界、输入输出契约、错误语义与目录落位方式，用于指导后续 `internal/agent` 目录的实现。

目标不是直接约束某一版具体代码，而是先固定系统边界，避免后续在接入 Kafka、SSE、QA 打回、Blackboard 时反复返工。

---

## 2. 落位目录

建议新增目录：

```text
internal/
├── agent/
│   ├── agent.go
│   ├── agentcard.go
│   ├── coordinator.go
│   ├── collector.go
│   ├── analyst.go
│   ├── writer.go
│   └── qa.go
```

建议与现有目录协作关系如下：

- `internal/domain`
  - 放任务、报告、Trace、Agent 状态等对外稳定的数据结构
- `internal/agent`
  - 放 5 个 Agent 的接口与具体实现
- `internal/state`
  - 放 Blackboard 接口与实现
- `internal/protocol`
  - 放 Agent 间消息协议
- `internal/workflow`
  - 放路由器、状态机、流程执行器
- `internal/service`
  - 保留任务创建、查询、删除等业务入口

---

## 3. 设计原则

### 3.1 单一职责

每个 Agent 只关心自己的那一段工作，不直接承担上游或下游职责。

### 3.2 通过共享上下文协作

Agent 不直接依赖其他 Agent 的内部结构。所有阶段结果通过 Blackboard 读写。

### 3.3 输入输出结构化

Agent 间传递的数据必须是结构化 JSON 对象，不以自然语言长文本作为主协议。

### 3.4 可追踪

每个 Agent 一次执行都必须产生 Trace 节点，供前端 TraceView 和后续调试使用。

### 3.5 可重做

任意一个 Agent 都必须能在 QA 打回后重新运行，且只重做自己的职责范围。

---

## 4. Agent 枚举与角色清单

当前系统固定 5 个 Agent：

| Agent | 中文角色 | 核心职责 |
|---|---|---|
| `coordinator` | 协调者 | 任务拆解、路由控制、澄清判断、打回中转 |
| `collector` | 采集者 | 搜索、抓取、提取、清洗、来源整理 |
| `analyst` | 分析者 | SWOT、功能矩阵、定价、用户画像、结论结构化 |
| `writer` | 撰写者 | 组装报告、整理章节、对齐展示结构 |
| `qa` | 质检者 | 规则校验、来源抽查、通过/打回判定 |

建议继续复用现有 `internal/domain/analysis.go` 中的：

- `AgentName`
- `AgentRunStatus`
- `AgentState`

这部分已经是前后端共享语义，不应重复定义。

---

## 5. 统一 Agent 接口

## 5.1 顶层接口

建议 `internal/agent/agent.go` 中定义统一接口：

```go
type Agent interface {
    Name() domain.AgentName
    Card() AgentCard
    Run(ctx context.Context, input RunInput, bb state.Blackboard) (RunOutput, error)
}
```

说明：

- `Name()`：返回稳定的 Agent 名称，用于路由、Trace、前端展示
- `Card()`：返回 Agent 自描述信息，用于 `/api/agents`
- `Run(...)`：执行当前阶段逻辑

---

## 5.2 运行时输入 `RunInput`

建议统一运行时输入结构，而不是每个 Agent 自行发明调用参数：

```go
type RunInput struct {
    TaskID      string
    TraceID     string
    Attempt     int
    TriggerType string
    Reason      string
    Payload     map[string]any
}
```

字段含义：

- `TaskID`
  - 当前任务 ID
- `TraceID`
  - 当前流程或子流程的 Trace 根 ID
- `Attempt`
  - 第几次执行，QA 打回后自增
- `TriggerType`
  - 本次执行的触发原因，如 `task_created`、`qa_reject`、`clarification_answered`
- `Reason`
  - 触发说明，便于记录 Trace
- `Payload`
  - 附加上下文，例如路由器传入的修正要求

---

## 5.3 运行时输出 `RunOutput`

建议所有 Agent 统一输出结构：

```go
type RunOutput struct {
    Status       domain.AgentRunStatus
    NextAgent    *domain.AgentName
    MessageType  string
    Summary      string
    Artifacts    []protocol.ArtifactRef
    NeedsRetry   bool
    Retryable    bool
    Metadata     map[string]any
}
```

字段说明：

- `Status`
  - 本次 Agent 执行状态
- `NextAgent`
  - 建议的下一跳；QA 打回时可为空，由 Router 决定
- `MessageType`
  - 输出消息类型，如 `plan_ready`、`materials_ready`
- `Summary`
  - 用于 Task 时间线和 Trace 的摘要
- `Artifacts`
  - 本次产出的工件引用
- `NeedsRetry`
  - 是否建议工作流引擎立即重试
- `Retryable`
  - 错误是否可重试
- `Metadata`
  - 额外上下文，如打分、统计信息、校验详情

---

## 6. AgentCard 设计

建议 `internal/agent/agentcard.go` 定义：

```go
type AgentCard struct {
    Name            domain.AgentName `json:"name"`
    DisplayName     string           `json:"displayName"`
    Description     string           `json:"description"`
    Skills          []string         `json:"skills"`
    Tools           []string         `json:"tools"`
    DependsOn       []string         `json:"dependsOn"`
    InputArtifacts  []string         `json:"inputArtifacts"`
    OutputArtifacts []string         `json:"outputArtifacts"`
}
```

在现有 `/api/agents` 基础上，后续应从静态卡片逐步过渡到真实实现返回的 `Card()`。

---

## 7. 五个 Agent 的职责边界

## 7.1 Coordinator

### 职责

- 接收任务创建事件
- 判断任务信息是否足够
- 生成分析计划
- 指定下一跳 Agent
- 处理 QA 打回后的重跑路由
- 处理用户澄清回复

### 不负责

- 不直接做网页抓取
- 不直接产出 SWOT 或报告正文
- 不直接做最终 QA 评分

### 输入

- 任务基础信息
- 用户补充澄清信息
- QA 打回结果

### 输出

- 任务计划
- 路由决策
- 澄清请求

---

## 7.2 Collector

### 职责

- 根据计划执行搜索
- 抓取目标页面
- 提取文本
- 组织来源和原始材料
- 输出供 Analyst 使用的结构化素材

### 推荐的五阶段内部流程

1. `Search`
2. `Fetch`
3. `Extract`
4. `Enrich`
5. `Validate`

### 输入

- 任务信息
- 计划信息
- 重采要求

### 输出

- 来源列表
- 原始摘录
- 清洗后的分析素材

---

## 7.3 Analyst

### 职责

- 基于采集素材生成结构化分析
- 产出 SWOT、功能矩阵、定价、用户画像
- 输出供 Writer 组装报告的数据对象

### 不负责

- 不直接决定页面展示文案
- 不负责最终报告结构编排

### 输入

- Collector 产出的素材与来源
- QA 针对分析结论的修正要求

### 输出

- 分析对象
- 关键结论摘要

---

## 7.4 Writer

### 职责

- 将分析对象组装为报告草稿
- 处理标题、摘要、章节组织
- 对齐前端 `Report` 结构

### 输入

- Analyst 结构化分析结果
- 任务元信息

### 输出

- 报告草稿
- 最终可落库的 `domain.Report`

---

## 7.5 QA

### 职责

- 检查报告结构是否完整
- 检查核心字段是否齐全
- 核查是否存在明显无来源结论
- 产出 `pass` 或 `reject`

### 输入

- 报告草稿
- 来源列表
- 分析结果

### 输出

- 质检结果
- 问题列表
- 打回目标 Agent

---

## 8. 各 Agent 输入输出契约

## 8.1 CoordinatorInput / CoordinatorOutput

### 输入

- `task.id`
- `task.title`
- `task.competitors`
- `task.dimensions`
- `workflow.round`
- `qa.result`（可选）
- `clarification.answer`（可选）

### 输出

- `workflow.plan`
- `workflow.current_agent`
- `workflow.next_agent`
- `workflow.needs_clarification`
- `workflow.rework_target`

---

## 8.2 CollectorInput / CollectorOutput

### 输入

- `task.*`
- `workflow.plan`
- `workflow.rework_reason`（可选）

### 输出

- `collector.query`
- `collector.urls`
- `collector.sources`
- `collector.materials`
- `collector.summary`

---

## 8.3 AnalystInput / AnalystOutput

### 输入

- `collector.materials`
- `collector.sources`
- `task.competitors`
- `task.dimensions`
- `workflow.rework_reason`（可选）

### 输出

- `analysis.summary`
- `analysis.swot`
- `analysis.features`
- `analysis.pricing`
- `analysis.personas`

---

## 8.4 WriterInput / WriterOutput

### 输入

- `task.*`
- `analysis.*`
- `collector.sources`

### 输出

- `report.title`
- `report.draft`
- `report.final`
- `report.generated_at`

---

## 8.5 QAInput / QAOutput

### 输入

- `report.draft`
- `report.final`
- `collector.sources`
- `analysis.*`

### 输出

- `qa.score`
- `qa.result`
- `qa.issues`
- `qa.target_agent`
- `qa.reason`

---

## 9. 错误语义与失败分类

建议在 Agent 层统一划分错误类型，而不是把所有错误都当成普通 `error`：

| 类型 | 含义 | 处理方式 |
|---|---|---|
| `retryable` | 临时性失败，如网络抖动、LLM 超时 | 原地重试 |
| `fatal` | 无法恢复，如关键配置缺失、数据结构损坏 | 任务失败 |
| `clarification_required` | 用户输入不足 | 进入澄清态 |
| `rejected_by_qa` | 质检不通过 | 跳转到重做流程 |

推荐的设计方式：

- 保留标准 `error`
- 额外定义带类别的错误结构

例如：

```go
type AgentError struct {
    Kind      string
    Message   string
    Retryable bool
    Metadata  map[string]any
}
```

---

## 10. Trace 要求

每个 Agent 每次执行都必须生成一个 Trace 节点，至少包含：

- `id`
- `agent`
- `label`
- `status`
- `durationMs`
- `tokenCount`
- `input`
- `output`
- `metadata`
- `isRetry`
- `isRejection`
- `parentId`

推荐直接复用现有 `domain.TraceNode`，不要另造一套。

---

## 11. 与现有系统的对接策略

当前仓库已有：

- `internal/domain/analysis.go`
- `internal/service/analysis_service.go`
- `internal/web/agent.go`
- `Task / Report / Trace` 三张表

建议迁移顺序：

1. 先新增 `internal/agent`
2. 将现有 `AnalysisService` 中的逻辑拆分到 5 个 Agent
3. 新增 `workflow engine` 负责串接这 5 个 Agent
4. 保留 `Task / Report / Trace` 作为最终落库模型

也就是说，先把当前的单体编排重构成：

`TaskService -> WorkflowEngine -> Agent[Coordinator/Collector/Analyst/Writer/QA]`

---

## 12. 里程碑建议

### 第一阶段

- 先定义接口和空实现
- 先让 5 个 Agent 能串行走通

### 第二阶段

- 接 Blackboard
- 接统一消息协议
- 接 QA 打回

### 第三阶段

- 接 Kafka
- 接 SSE
- 接重试与恢复

---

## 13. 本文档产出的直接结论

实现时应保证：

1. `Agent` 接口统一
2. 5 个 Agent 边界固定
3. 输入输出均走结构化对象
4. 所有中间结果写 Blackboard
5. 所有执行过程可写 Trace
6. QA 打回只触发局部重做，不推翻整个流程
