# AI 驱动的竞品分析 Agent 协作系统 — 技术方案

## 文档索引

本文档为项目总览。各专题详见：

| 文档 | 内容 |
|---|---|
| [architecture.md](architecture.md) | 系统架构、Agent 通信两层模型、Eino 工作流、打回策略 |
| [collector-design.md](collector-design.md) | Collector Agent 五阶段流水线（Search→Fetch→Extract→Enrich→Validate） |
| [schema-protocol.md](schema-protocol.md) | 竞品知识 Schema、A2A 通信协议、数据库设计 |
| [frontend-design.md](frontend-design.md) | Vue 3 前端设计：4 页面、组件树、SSE 实时通信、API 约定 |

---

## 一、项目概述

构建一个 Go 语言实现的多 Agent 协作竞品分析系统，模拟"数字调研小组"，由 5 个专职 Agent 自动完成从公开信息采集到结构化竞品报告的全链路产出，并通过交叉审查与反馈闭环实现自我校验。

**核心指标**：端到端可运行、DAG 可追溯、反馈闭环真实可触发、信息可溯源。

**5 个 Agent**：Coordinator（协调者）→ Collector（采集者）→ Analyst（分析者）→ Writer（撰写者）→ QA（质检者）

---

## 二、技术选型

| 层面 | 选型 | 理由 |
|---|---|---|
| 语言 | Go 1.22+ | 要求技术栈；并发模型天然适合 Agent 编排 |
| HTTP 框架 | **Gin** | 轻量、生态成熟、文档丰富 |
| ORM | **GORM** | Go 最成熟的 ORM，支持 MySQL、自动迁移、关联查询 |
| Agent 编排 | **Eino**（字节跳动开源） | 字节自研 Go Agent 框架，原生支持 Graph/Chain 编排、组件化、流式输出、Callback 机制 |
| 消息队列 | **Kafka** | Agent 间异步通信的消息总线，天然支持消息持久化、回溯、多消费者 |
| 数据库 | **MySQL** | 关系型数据存储（任务、报告、Schema 数据），GORM 完美适配 |
| 缓存 | **Redis**（按需） | LLM 响应缓存、Session 管理、限流计数；可降级为内存缓存 |
| 向量数据库 | **Milvus**（按需） | 竞品文档/网页的向量化存储，支撑 RAG 检索 |
| RAG 框架 | **go-rag** | Go 原生 RAG 框架，文档加载→分块→Embedding→检索→生成 |
| 前端 | **Vue 3** + Vite | 前后端分离；报告查看、溯源跳转、Agent 决策回放等交互适合 SPA |
| 配置 | **Viper** | Go 标准配置库，支持 YAML/环境变量/远程配置，热加载 |
| 日志 | **Zap** | 高性能结构化日志，适合 Agent Trace 等高吞吐场景 |
| LLM 调用 | 直接 HTTP 调用豆包 API（OpenAI 兼容接口） | 无需中间层，Go 的 `net/http` + JSON 即可 |
| Agent 通信协议 | **A2A-inspired over Kafka** | 采纳 Google A2A 的 Agent Card、Task 状态机、Message/Part/Artifact 概念模型，跑在 Kafka 异步传输上 |

### 选型补充说明

- **Eino vs 自研 DAG**：Eino 是字节跳动开源的 Go Agent 框架，原生支持 Graph 编排、Chain 组合、Tool 调用、Callback 切面，且和挑战赛主办方技术栈一致。用 Eino 编排 Agent 流程，Kafka 负责 Agent 间的异步消息传递，两者互补。
- **Kafka 的必要性**：Agent 间如果同步调用（HTTP/gRPC），会导致长链路超时风险高；Kafka 将 Agent 解耦为异步消费者，每个 Agent 独立消费 Topic 消息，处理后投递到下一个 Topic。Kafka 消息持久化天然形成 Agent Trace 日志。
- **Milvus + go-rag 按需启用**：如果 Web 采集的信息量大，启用 RAG 链路做语义检索；如果竞品信息通过 LLM 直接搜索 + 总结可满足，则跳过。
- **Redis 按需启用**：优先用 Go 内存缓存（sync.Map / go-cache），如果多实例部署或需要持久化缓存，再接入 Redis。

---

## 三、目录结构

```
competeai/
├── cmd/
│   └── server/
│       └── main.go                 # 入口，启动 Gin + Kafka Consumers
├── internal/
│   ├── agent/                      # Agent 层 (5 个 Agent)
│   │   ├── agent.go                # Agent 接口定义
│   │   ├── agentcard.go            # A2A Agent Card 定义 + 注册
│   │   ├── coordinator.go          # 任务协调 Agent
│   │   ├── collector.go            # 信息采集 Agent
│   │   ├── analyst.go              # 分析 Agent
│   │   ├── writer.go               # 报告撰写 Agent
│   │   └── qa.go                   # 质检 Agent
│   ├── eino/                       # Eino 编排层
│   │   ├── graph.go                # Graph 工作流定义
│   │   ├── nodes.go                # 自定义 Node 实现
│   │   ├── callbacks.go            # Callback（Trace/Token 统计）
│   │   └── tools.go                # Tool 定义（Web搜索/RAG检索等）
│   ├── kafka/                      # Kafka 层
│   │   ├── producer.go             # Kafka Producer 封装
│   │   ├── consumer.go             # Kafka Consumer 封装
│   │   ├── topics.go               # Topic 常量定义
│   │   └── router.go               # 消息路由（根据 QA 结果决定下个 Topic）
│   ├── state/                      # Shared State 层（Blackboard）
│   │   ├── redis.go                # Redis 读写封装
│   │   ├── keys.go                 # Key 常量定义
│   │   └── blackboard.go           # Blackboard 接口（跨 Agent 读写）
│   ├── llm/                        # LLM 调用层
│   │   ├── client.go               # 豆包 API 客户端
│   │   └── prompt.go               # Prompt 模板管理
│   ├── rag/                        # RAG 层（按需）
│   │   ├── loader.go               # 文档加载
│   │   ├── splitter.go             # 文档分块
│   │   ├── embedder.go             # Embedding 生成
│   │   └── retriever.go            # Milvus 检索
│   ├── schema/                     # Schema 定义层
│   │   ├── competitor.go           # 竞品 Schema
│   │   ├── analysis.go             # 分析结果 Schema
│   │   ├── report.go               # 报告 Schema
│   │   └── validator.go            # Schema 校验器
│   ├── api/                        # HTTP API 层
│   │   ├── router.go               # Gin 路由注册
│   │   ├── handler_task.go         # 任务接口
│   │   ├── handler_report.go       # 报告接口
│   │   ├── handler_trace.go        # 追踪接口
│   │   └── middleware.go           # 中间件（CORS/日志/恢复）
│   ├── service/                    # 业务逻辑层
│   │   ├── task_service.go         # 任务管理
│   │   ├── report_service.go       # 报告查询
│   │   └── trace_service.go        # Trace 查询
│   ├── store/                      # 存储层
│   │   ├── mysql.go                # MySQL/GORM 初始化
│   │   ├── models.go               # 所有 GORM 数据模型
│   │   ├── task_repo.go            # 任务 Repository
│   │   ├── report_repo.go          # 报告 Repository
│   │   └── trace_repo.go           # Trace Repository
│   └── config/                     # 配置
│       ├── config.go               # Viper 配置结构体 + 加载
│       └── zap.go                  # Zap Logger 初始化
├── web/                            # 前端 (Vue 3 + Vite + TypeScript)
│   ├── src/
│   │   ├── views/
│   │   │   ├── Dashboard.vue       # 任务管理首页
│   │   │   ├── ReportView.vue      # 报告查看页（核心）
│   │   │   ├── TraceView.vue       # Agent 追踪回放页
│   │   │   └── AgentsView.vue      # Agent 能力展示页
│   │   ├── components/
│   │   │   ├── task/               # 任务相关组件
│   │   │   ├── report/             # 报告相关组件
│   │   │   ├── trace/              # 追踪相关组件
│   │   │   └── agents/             # Agent 能力组件
│   │   ├── composables/            # Vue Composables
│   │   │   ├── useTaskSSE.ts       # SSE 实时状态 Hook
│   │   │   └── useReport.ts        # 报告数据 Hook
│   │   ├── api/                    # Axios API 封装
│   │   ├── stores/                 # Pinia 状态管理
│   │   ├── router/
│   │   └── App.vue
│   ├── package.json
│   └── vite.config.ts
├── prompts/                        # Prompt 模板文件
│   ├── collector.txt
│   ├── analyst.txt
│   ├── writer.txt
│   └── qa.txt
├── config/
│   └── config.yaml
├── sql/
│   └── schema.sql
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── docs/
    ├── plan.md                     # 本文档（总索引）
    ├── architecture.md             # 架构与工作流设计
    ├── collector-design.md         # Collector Agent 详细设计
    ├── schema-protocol.md          # Schema、数据库与通信协议
    └── frontend-design.md          # Vue 前端设计
```

---

## 四、三周里程碑计划

### 第 1 周（5/20 - 5/26）：基础设施 + Agent 单体开发

| 天数 | 任务 | 产出 |
|---|---|---|
| Day 1-2 | Go module 初始化、目录结构、Viper 配置加载、Zap 日志初始化、GORM + MySQL 建表 | 可编译运行的空架子 |
| Day 2-3 | LLM Client（豆包 API 封装）、Prompt 模板引擎 | 能调通 LLM 并获取结构化 JSON 响应 |
| Day 3-4 | Schema 定义（竞品/功能树/定价/SWOT/报告）、GORM Model + 校验器 | Go struct + 数据库表 + 校验逻辑 |
| Day 4-5 | Coordinator Agent + Collector Agent（Search → Fetch → Extract → Enrich → Validate 五阶段 Pipeline） | 协调者拆解任务；采集者 5 阶段流水线输出结构化竞品数据 + 溯源引用 |
| Day 5-6 | Analyst Agent + Writer Agent 单体开发 | 各自独立可运行 |
| Day 7 | QA Agent + Shared State（Blackboard）封装 | 5 个 Agent 全部可用，跨 Agent 读写上下文 |

### 第 2 周（5/27 - 6/2）：Kafka + Eino 编排 + 前端

| 天数 | 任务 | 产出 |
|---|---|---|
| Day 8-9 | Kafka Producer/Consumer 封装、Topic 定义、Router（含 Coordinator 打回路由） | Agent 可通过 Kafka + Shared State 通信 |
| Day 9-10 | Eino Graph 工作流编排（主流程 5 Agent + 条件边打回 + Coordinator 澄清循环） | 完整流程跑通 |
| Day 10-11 | 反馈闭环联调（QA 打回 → Coordinator 路由 → Agent 重做 → 输出改善） | 闭环可触发，重做后改善 |
| Day 11-12 | Trace 系统（每条消息 Kafka 留存 + MySQL Trace 表）、信息溯源链路 | 每条结论可定位到来源 |
| Day 12-13 | Gin API 层 + Vue 前端（Dashboard创建任务 + ReportView报告查看 + TraceView追踪回放） | 可演示的 Web UI，SSE 实时状态 |
| Day 14 | 缓冲 + 联调修复 | 端到端可用 |

### 第 3 周（6/3 - 6/10）：打磨 + 文档 + 答辩准备

| 天数 | 任务 | 产出 |
|---|---|---|
| Day 15-16 | 系统稳定性（Kafka 重试、LLM 超时重试、降级策略） | 演示不卡顿不崩溃 |
| Day 16-17 | Vue 前端交互优化（SWOT 雷达图、功能矩阵热力图、溯源面板动画、Trace 回放控件、人工标注） | 产品体验达标 |
| Day 17-18 | 文档：README、架构图（Mermaid）、Agent 协议、部署说明 | 文档齐全 |
| Day 18-19 | 录屏 Demo 制作、答辩 PPT | 可演示的完整材料 |
| Day 19-20 | 答辩演练 + 最终检查 | 准备就绪 |

---

## 五、关键技术风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **LLM 输出格式不稳定**（不满足 JSON Schema） | 高 — Agent 间通信断裂 | 1) 豆包 API 开启 JSON Mode / function calling 约束输出 2) Go 侧 Schema 校验 + 自动重试（最多 3 次）3) 解析失败投递到死信 Topic 人工介入 |
| **幻觉导致报告不可信** | 高 — 评分核心项（35%） | 1) 每条结论强制标注来源 URL 2) QA Agent 抽样核验关键事实 3) RAG 检索优先使用实际采集的网页内容 4) 不可溯源的信息标注"待验证" |
| **Kafka 运维复杂度** | 中 — 开发环境依赖多 | 1) 提供 docker-compose 一键启动 Kafka + MySQL + Redis 2) 开发阶段可用 channel 模拟 Kafka 做单元测试 3) 降级方案：无 Kafka 时 Agent 间直接调用 |
| **Eino 学习曲线** | 中 — 框架较新，文档可能不全 | 1) 优先阅读 Eino 源码和 examples 2) 封装一层抽象，降低对 Eino API 的直接依赖 3) 预留切换自研 DAG 的 Fallback |
| **豆包 API 限流/不稳定** | 中 — 演示风险 | 1) LLM Client 内置指数退避重试 2) Redis/内存缓存 LLM 响应 3) Demo 准备预采集数据的回放模式 |
| **前端开发进度** | 中 — Go 团队可能不熟悉 Vue | 1) Vue 3 组件按优先级开发（报告页 > 任务页 > 追踪页）2) 最坏情况可用 Gin 模板渲染降级 |

---

## 六、验证策略

1. **单元测试**：每个 Agent 用 mock LLM + mock Kafka 独立测试
2. **集成测试**：docker-compose 启动 Kafka + MySQL，构造已知竞品（Cursor vs Copilot），验证端到端输出
3. **反馈闭环测试**：QA Agent 故意注入错误 → 验证打回消息正确路由 → 重做后输出改善
4. **Schema 合规测试**：每个 Agent 输出 JSON → 反序列化为 Go Struct → 校验必填字段
5. **E2E 测试**：Vue UI 手动操作完整流程，录屏备用
6. **稳定性测试**：连续运行 10 次，观察 Kafka 消息积压、LLM 失败率、Token 消耗
