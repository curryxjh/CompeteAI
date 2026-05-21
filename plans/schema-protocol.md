# Schema、数据库与通信协议设计

## 1. 竞品知识 Schema 设计

核心 Schema 定义（Go struct + GORM Model），所有 Agent 输出必须符合：

```go
// --- 竞品基础信息 ---
type CompetitorProfile struct {
    gorm.Model
    Name         string   `json:"name" gorm:"index"`
    Company      string   `json:"company"`
    Category     string   `json:"category"`
    Description  string   `json:"description"`
    TargetUsers  []string `json:"target_users" gorm:"serializer:json"`
    Website      string   `json:"website"`
    TaskID       string   `json:"task_id" gorm:"index"`
}

// --- 功能树 ---
type FeatureTreeNode struct {
    gorm.Model
    Name        string             `json:"name"`
    Description string             `json:"description"`
    ParentID    *uint              `json:"parent_id,omitempty"`
    Children    []*FeatureTreeNode `json:"children,omitempty" gorm:"-"`
    Supported   *bool              `json:"supported,omitempty"`
    ProductID   uint               `json:"product_id" gorm:"index"`
    SourceRef   string             `json:"source_ref"`
}

// --- 定价模型 ---
type PricingModel struct {
    gorm.Model
    ProductID    uint          `json:"product_id" gorm:"index"`
    FreeTier     string        `json:"free_tier"`
    PaidTiers    []PricingTier `json:"paid_tiers" gorm:"serializer:json"`
    PricingModel string        `json:"pricing_model"`
    Currency     string        `json:"currency"`
    SourceRef    string        `json:"source_ref"`
}

type PricingTier struct {
    Name     string   `json:"name"`
    Price    float64  `json:"price"`
    Billing  string   `json:"billing"`
    Features []string `json:"features"`
}

// --- 用户画像 ---
type UserPersona struct {
    gorm.Model
    ProductID    uint     `json:"product_id" gorm:"index"`
    Role         string   `json:"role"`
    PainPoints   []string `json:"pain_points" gorm:"serializer:json"`
    UseCases     []string `json:"use_cases" gorm:"serializer:json"`
    Satisfaction string   `json:"satisfaction"`
    SourceRef    string   `json:"source_ref"`
}

// --- SWOT 分析 ---
type SWOTAnalysis struct {
    gorm.Model
    ProductID       uint     `json:"product_id" gorm:"index"`
    TaskID          string   `json:"task_id" gorm:"index"`
    Strengths       []string `json:"strengths" gorm:"serializer:json"`
    Weaknesses      []string `json:"weaknesses" gorm:"serializer:json"`
    Opportunities   []string `json:"opportunities" gorm:"serializer:json"`
    Threats         []string `json:"threats" gorm:"serializer:json"`
    SourceRefs      []string `json:"source_refs" gorm:"serializer:json"`
}

// --- 功能对比矩阵 ---
type FeatureMatrix struct {
    gorm.Model
    TaskID   string                       `json:"task_id" gorm:"index"`
    Features []string                     `json:"features" gorm:"serializer:json"`
    Matrix   map[string]map[string]string `json:"matrix" gorm:"serializer:json"`
}

// --- 最终报告 ---
type CompetitiveReport struct {
    gorm.Model
    TaskID         string              `json:"task_id" gorm:"uniqueIndex"`
    Title          string              `json:"title"`
    ExecutiveSummary string            `json:"executive_summary"`
    Products       []CompetitorProfile `json:"products" gorm:"-"`
    FeatureMatrix  FeatureMatrix       `json:"feature_matrix" gorm:"-"`
    SWOTs          []SWOTAnalysis      `json:"swots" gorm:"-"`
    Personas       []UserPersona       `json:"personas" gorm:"-"`
    KeyInsights    []string            `json:"key_insights" gorm:"serializer:json"`
    QAReport       QAReport            `json:"qa_report" gorm:"-"`
    TraceSummary   TraceSummary        `json:"trace_summary" gorm:"-"`
}
```

---

## 2. Agent 通信协议（A2A-inspired + Kafka 传输）

### 2.1 协议选型

采用 **A2A 协议的概念模型**（Agent Card、Task 生命周期、Message/Part/Artifact），跑在 **Kafka 异步传输** 上。不采用 A2A 原生的 HTTP/JSON-RPC 传输，原因：

- 竞品分析长链路（5 个 Agent 串行）如果走同步 HTTP，超时风险极高
- Kafka 天然支持消息持久化、回溯、重试，适合 Agent Trace
- A2A 的 Agent Card 和 Task 状态机可以直接复用，不需要自定义协议

对评分维度的直接贡献：
- **"Agent 间通信协议设计"** → A2A 标准语义 + Agent Card 自描述
- **"结构化消息传递，非纯自然语言对话"** → A2A Message/Part 结构化模型
- **"DAG 任务流转可视化"** → A2A Task 生命周期天然可追踪

### 2.2 Agent Card（Agent 能力自描述）

每个 Agent 启动时注册自己的 Agent Card，描述能力、输入输出 Schema、依赖的上游 Agent：

```go
type AgentCard struct {
    Name         string           `json:"name"`
    DisplayName  string           `json:"display_name"`
    Description  string           `json:"description"`
    Skills       []AgentSkill     `json:"skills"`
    InputSchema  json.RawMessage  `json:"input_schema"`
    OutputSchema json.RawMessage  `json:"output_schema"`
    Upstream     []string         `json:"upstream"`
    Downstream   []string         `json:"downstream"`
    KafkaTopic   AgentTopics      `json:"kafka_topic"`
    MaxRetry     int              `json:"max_retry"`
}

type AgentSkill struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    ToolName    string `json:"tool_name"`
}
```

**示例 — Collector Agent 的 Card**：

```json
{
  "name": "collector",
  "display_name": "信息采集 Agent",
  "description": "负责搜索和采集竞品的公开信息，包括官网、新闻报道、用户评价、定价页面等，输出结构化竞品数据",
  "skills": [
    { "name": "web_search", "description": "搜索引擎检索竞品信息", "tool_name": "SearchWeb" },
    { "name": "url_fetch", "description": "抓取指定 URL 内容", "tool_name": "FetchURL" },
    { "name": "rag_retrieve", "description": "从已采集文档中语义检索", "tool_name": "RAGRetrieve" }
  ],
  "input_schema": { "type": "object", "required": ["competitors"], "properties": { "competitors": { "type": "array", "items": { "type": "string" } } } },
  "output_schema": { "type": "object", "required": ["products"], "properties": { "products": { "type": "array", "items": { "$ref": "#/CompetitorProfile" } } } },
  "upstream": ["coordinator"],
  "downstream": ["analyst"],
  "kafka_topic": { "input": "collector.input", "output": "collector.output" },
  "max_retry": 3
}
```

> Agent Card 注册到 Redis 后，前端可直接调用 `/api/agents` 展示所有 Agent 的能力图，天然满足评分要求的"角色划分清晰，职责边界明确"。

### 2.3 A2A 消息格式（控制流 — Kafka 传输）

Kafka 消息体直接采用 A2A 的 Message/Part 结构：

```go
// A2A Message — Kafka 消息体
type Message struct {
    ID          string                   `json:"id"`
    TaskID      string                   `json:"task_id"`
    Role        string                   `json:"role"`
    Parts       []Part                   `json:"parts"`
    Metadata    map[string]interface{}   `json:"metadata"`
}

// A2A Part — 消息内容单元
type Part struct {
    Type        PartType        `json:"type"`        // text / data / file
    Text        string          `json:"text,omitempty"`
    Data        json.RawMessage `json:"data,omitempty"`
    MimeType    string          `json:"mime_type,omitempty"`
}

type PartType string
const (
    PartTypeText PartType = "text"  // 自然语言（仅用于 QA query/response）
    PartTypeData PartType = "data"  // 结构化数据（Agent 间主要方式）
    PartTypeFile PartType = "file"  // 文件引用
)
```

**关键约束**：Agent 之间主要使用 `PartType = "data"`（结构化 JSON，非自然语言），只在 QA 质疑 Analyst（请求-响应模式）时使用 `PartType = "text"` 做简短的自然语言解释。这直接满足评分要求的 **"非纯自然语言对话"**。

### 2.4 A2A Task 状态机

每个竞品分析任务的生命周期遵循 A2A Task 状态机：

```
                        ┌──────────┐
                        │ pending  │  Coordinator 收到任务
                        └────┬─────┘
                             │
                    ┌────────▼────────┐
                    │   working       │  各 Agent 流水线执行
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │completed │  │  failed  │  │cancelled │
        │QA通过    │  │重试耗尽  │  │用户取消  │
        └──────────┘  └──────────┘  └──────────┘
              │
              ▼
        report.final
```

每个 Agent 内部子任务也遵循同样的状态机，前端可以通过 Kafka 消息追踪每个子任务的状态变化。

### 2.5 A2A Artifact → Shared State 映射

| A2A Artifact | Shared State Key | 内容 |
|---|---|---|
| `task:{id}:collector_artifact` | `task:{id}:collector_output` | 采集的竞品原始数据 + 来源URL |
| `task:{id}:analyst_artifact` | `task:{id}:analyst_output` | SWOT、功能矩阵、用户画像 |
| `task:{id}:writer_artifact` | `task:{id}:writer_output` | 报告草稿 |
| `task:{id}:qa_artifact` | `task:{id}:qa_output` | 质检报告 |
| `task:{id}:final_artifact` | `task:{id}:final_report` | 最终报告 |

### 2.6 Shared State 接口

```go
// Shared State 写入接口
type StateWriter interface {
    PutArtifact(taskID string, agentName string, artifact *Artifact) error
    PutTraceRef(taskID string, ref TraceRef) error
}

// Shared State 读取接口
type StateReader interface {
    GetArtifact(taskID string, agentName string) (*Artifact, error)
    GetAllUpstream(taskID string, agentName string) ([]*Artifact, error)
    GetTraceRefs(taskID string) ([]TraceRef, error)
}

// A2A Artifact
type Artifact struct {
    ID          string              `json:"id"`
    TaskID      string              `json:"task_id"`
    AgentName   string              `json:"agent_name"`
    Parts       []Part              `json:"parts"`
    TraceRefs   []TraceRef          `json:"trace_refs"`
    CreatedAt   time.Time           `json:"created_at"`
    Attempt     int                 `json:"attempt"`
}
```

### 2.7 完整消息流转示例

以"Cursor vs Copilot 竞品分析"为例：

```
Step 1: API → Kafka task.create → Coordinator
  Message: { role: "user", parts: [{ type: "data", data: { competitors: [...] } }] }
  State:  task:t001:meta → A2A Task { status: "pending" }

Step 2: Coordinator 确认需求 → 创建子任务 → 通知 Collector
  State:  task:t001:collector_input → A2A Task { status: "working" }
  Kafka:  { role: "agent", parts: [{ type: "data", data: { state_key: "task:t001:collector_input" } }] }

Step 3: Collector 完成 → 写入 Artifact → 通知 Analyst
  State:  task:t001:collector_artifact → Artifact { ... }
  Kafka:  { role: "agent", parts: [{ type: "data", data: { state_key: "task:t001:collector_artifact" } }] }

Step 4-5: Analyst → Writer 同理

Step 6a: QA 通过
  Kafka:  { role: "agent", parts: [{ type: "data", data: { verdict: "pass" } }] }
  State:  task:t001:qa_artifact → { passed: true }

Step 6b: QA 不通过 → 打回到 Coordinator
  Kafka:  { role: "agent", parts: [
            { type: "text", text: "SWOT 分析中'性能领先'结论无数据支撑" },
            { type: "data", data: { reject_target: "analyst", issues: [...] } }
          ]}
  Coordinator 路由 → analyst.input（带修正要求）
  → Analyst 重做 → 回到 Step 4
```

### 2.8 Kafka Topic 规范

| Topic | 消息方向 | 说明 |
|---|---|---|
| `task.create` | API → Coordinator | 用户创建新任务 |
| `coordinator.input` | API / QA(打回) → Coordinator | 任务事件 / 打回路由 |
| `collector.input` | Coordinator → Collector | 采集任务 |
| `collector.output` | Collector → Router | 采集完成通知 |
| `analyst.input` | Router / Coordinator(打回) → Analyst | 待分析 / 重分析 |
| `analyst.output` | Analyst → Router | 分析完成通知 |
| `writer.input` | Router / Coordinator(打回) → Writer | 待撰写 / 重写 |
| `writer.output` | Writer → Router | 报告就绪通知 |
| `qa.input` | Router → QA | 待质检 |
| `qa.output` | QA → Router | 质检结果（通过/打回） |
| `qa.query` | QA → Analyst/Collector | QA 质疑某个结论 |
| `analyst.response` | Analyst/Collector → QA | 对质疑的回复 |
| `user.clarify` | Coordinator → API(SSE) | 需要用户澄清问题 |
| `report.final` | Router → API(SSE) | 最终报告就绪 |

所有 Topic 消息体控制在 500B 以内，完整数据通过 `state_key` 在 Shared State 中读取。

---

## 3. 数据库设计（MySQL + GORM）

### 核心表关系

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│     tasks        │     │  competitor_      │     │  feature_tree_   │
│                  │     │  profiles         │     │  nodes           │
│ id (PK)         │────→│ task_id (FK)      │     │ product_id (FK)  │
│ title           │     │ name              │     │ name             │
│ status          │     │ company           │     │ parent_id        │
│ competitors_json│     │ website           │     │ supported        │
│ created_at      │     │ ...               │     │ source_ref       │
└─────────────────┘     └──────────────────┘     └─────────────────┘
         │
         ├──→ pricing_models (product_id, task_id)
         ├──→ user_personas (product_id, task_id)
         ├──→ swot_analyses (product_id, task_id)
         ├──→ feature_matrices (task_id)
         ├──→ competitive_reports (task_id, unique)
         ├──→ trace_records (task_id, agent_name)
         └──→ agent_messages (task_id, from_agent, to_agent)
```
