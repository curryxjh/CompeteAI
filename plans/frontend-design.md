# 前端设计（Vue 3 + Vite）

## 1. 技术准备

| 类别 | 选型 | 说明 |
|---|---|---|
| 框架 | Vue 3 (Composition API) + TypeScript | 类型安全，`<script setup>` 简洁 |
| 构建 | Vite | 秒级 HMR，Vue 官方推荐 |
| 路由 | Vue Router 4 | SPA 页面切换 |
| 状态管理 | Pinia | Vue 3 官方推荐，轻量 |
| HTTP | Axios | 请求拦截、错误处理、SSE 封装 |
| UI 组件库 | **Element Plus** | 企业级风格，表格/表单/时间线组件丰富 |
| 图表 | **ECharts 5** | SWOT 雷达图、功能矩阵热力图、定价柱状图 |
| 实时通信 | SSE (Server-Sent Events) | Agent 状态推送，Go 端 Gin 原生支持 |
| 代码规范 | ESLint + Prettier | 团队规范 |

**项目初始化**：

```bash
npm create vite@latest web -- --template vue-ts
cd web
npm install vue-router@4 pinia axios element-plus echarts
npm install -D @types/node eslint prettier
```

## 2. 页面架构

```
┌──────────────────────────────────────────────────┐
│                   App.vue                         │
│  ┌────────────────────────────────────────────┐  │
│  │            Layout (侧边栏 + 顶栏)            │  │
│  │  ┌──────────┐ ┌──────────────────────────┐  │  │
│  │  │ Sidebar  │ │     <router-view />       │  │  │
│  │  │          │ │                          │  │  │
│  │  │ 任务管理  │ │  4 个页面：              │  │  │
│  │  │ 报告查看  │ │  Dashboard / Report /    │  │  │
│  │  │ Agent追踪│ │  Trace / Agents           │  │  │
│  │  │ Agent能力│ │                          │  │  │
│  │  └──────────┘ └──────────────────────────┘  │  │
│  └────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

## 3. 页面与组件清单

### 页面一：Dashboard（任务管理首页）

**路由**：`/`

**功能**：
- 新建竞品分析任务（输入竞品名称列表、选择分析维度）
- 任务列表（进行中 / 已完成 / 失败），支持筛选和搜索
- 实时任务进度卡片（SSE 推送 + 动画）
- 点击任务进入报告页或追踪页

```
┌─────────────────────────────────────────────┐
│  [+] 新建分析任务                            │
├─────────────────────────────────────────────┤
│  任务列表                                    │
│  ┌─────────────────────────────────────┐    │
│  │ t001  Cursor vs Copilot 对比分析    │    │
│  │ [████████████░░░░░░] 60%           │    │
│  │ Collector ✓ → Analyst ⏳ → Writer  │    │
│  │ 2026-05-25 14:30    [查看详情]     │    │
│  └─────────────────────────────────────┘    │
│  ┌─────────────────────────────────────┐    │
│  │ t002  AI编程工具市场竞品分析  ✓已完成│    │
│  │ 2026-05-24 10:00    [查看报告]     │    │
│  └─────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
```

| 组件 | 说明 |
|---|---|
| `CreateTaskDialog.vue` | 新建任务弹窗：竞品名输入、分析维度多选、提交按钮 |
| `TaskCard.vue` | 任务卡片：进度条、Agent 状态图标、耗时 |
| `TaskProgressBar.vue` | 基于 SSE 实时更新的进度条组件 |

### 页面二：ReportView（报告查看 — 核心页面）

**路由**：`/report/:taskId`

**功能**：
- 竞品分析报告完整展示
- 功能对比矩阵（可排序、筛选）
- SWOT 雷达图
- 定价对比表
- 用户画像对比
- 每条结论旁有**溯源标记**，点击跳转到原始来源
- **人工介入修正**：可对报告内容做标注/批注

```
┌─────────────────────────────────────────────┐
│  Cursor vs GitHub Copilot 竞品分析报告        │
│  生成时间: 2026-05-25 15:00  QA评分: 92%    │
├─────────────────────────────────────────────┤
│  [概要] [功能对比] [SWOT] [定价] [用户画像]   │  ← Tab 切换
├─────────────────────────────────────────────┤
│  SWOT 分析                                   │
│  ┌──────────────┐  ┌──────────────┐         │
│  │  Cursor      │  │  Copilot     │         │
│  │  S: 5项 🔗   │  │  S: 4项 🔗   │         │
│  │  W: 3项 🔗   │  │  W: 2项 🔗   │         │
│  │  O: 4项 🔗   │  │  O: 3项 🔗   │         │
│  │  T: 2项 🔗   │  │  T: 3项 🔗   │         │
│  └──────────────┘  └──────────────┘         │
│                                             │
│  功能对比矩阵                                │
│  ┌──────────┬────────┬──────────┐          │
│  │ 功能      │ Cursor │ Copilot  │          │
│  ├──────────┼────────┼──────────┤          │
│  │ 代码补全  │  ✅    │  ✅      │          │
│  │ Chat对话  │  ✅🔗  │  ✅🔗    │    ← 🔗 可点击溯源 │
│  │ 多文件编辑│  ✅    │  ❌      │          │
│  └──────────┴────────┴──────────┘          │
│                    [导出PDF] [导出Markdown]   │
└─────────────────────────────────────────────┘
```

| 组件 | 说明 |
|---|---|
| `ReportHeader.vue` | 报告标题、元信息、QA 评分 |
| `FeatureMatrix.vue` | 功能对比矩阵表格，支持排序、筛选、导出 |
| `SWOTRadar.vue` | ECharts 雷达图，每个维度可点击展开详情 |
| `PricingCompare.vue` | 定价对比卡片，横向排列 |
| `UserPersonaCard.vue` | 用户画像卡片 |
| `SourceRefPanel.vue` | 侧边滑出面板，显示结论对应的原始来源（URL + 摘录 + 采集时间） |
| `AnnotationTool.vue` | 人工标注工具：选中文本 → 添加批注 → 保存到后端 |

### 页面三：TraceView（Agent 追踪回放 — 评分亮点）

**路由**：`/trace/:taskId`

**功能**：
- Agent 执行时间线（类似 CI/CD Pipeline 可视化）
- 每个 Agent 的输入/输出/耗时/Token 消耗
- 点击节点展开该 Agent 的决策详情
- 打回/重做路径高亮显示
- 支持**逐步回放**模式

```
┌─────────────────────────────────────────────┐
│  t001 执行追踪                               │
├─────────────────────────────────────────────┤
│  ┌─ Coordinator ──────────────────────────┐ │
│  │ ⏱ 2.3s  📊 150 tokens  ✅ 完成        │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ Collector ────────────────────────────┐ │
│  │ ⏱ 8.5s  📊 3200 tokens  ✅ 完成       │ │
│  │ 搜索: 15 URLs → 抓取: 12 pages          │ │
│  │ [展开详情] [查看原始页面]               │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ Analyst ──────────────────────────────┐ │
│  │ ⏱ 5.1s  📊 1800 tokens  ✅ 完成       │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ Writer ───────────────────────────────┐ │
│  │ ⏱ 3.2s  📊 900 tokens  ✅ 完成        │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ QA ───────────────────────────────────┐ │
│  │ ⚠️ 发现 2 个问题 → 打回 Analyst        │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ Analyst (Retry #1) ───────────────────┐ │
│  │ ⏱ 3.5s  📊 1200 tokens  ✅ 完成       │ │
│  └────────────────────────────────────────┘ │
│         │                                    │
│  ┌─ QA (Retry #1) ────────────────────────┐ │
│  │ ✅ 全部通过  评分: 92%                  │ │
│  └────────────────────────────────────────┘ │
│                                             │
│  [▶ 逐步回放] [⏩ 加速] [📋 导出Trace]      │
└─────────────────────────────────────────────┘
```

| 组件 | 说明 |
|---|---|
| `TraceTimeline.vue` | 基于 Element Plus Timeline 的 Agent 执行时间线 |
| `AgentNodeDetail.vue` | 展开/折叠面板，显示单个 Agent 的完整输入输出 |
| `RejectionHighlight.vue` | 打回路径高亮（红色虚线 + 动画） |
| `TraceReplay.vue` | 逐步回放控件（播放/暂停/加速/步进） |
| `TokenUsageChart.vue` | ECharts 柱状图，各 Agent Token 消耗对比 |

### 页面四：AgentsView（Agent 能力展示）

**路由**：`/agents`

**功能**：
- 5 个 Agent 的能力卡片（读取 Agent Card）
- Agent 间依赖关系图（DAG 可视化）
- Agent 通信协议说明

| 组件 | 说明 |
|---|---|
| `AgentCardGrid.vue` | 5 个 Agent Card 的网格布局 |
| `AgentDAGGraph.vue` | 基于 ECharts Graph 的 Agent 依赖关系 DAG 图 |
| `AgentSkillList.vue` | 单个 Agent 的技能列表 + 对应 Tool 说明 |

## 4. 实时通信（SSE）

前端通过 SSE 接收后端推送的 Agent 状态变更：

```typescript
// composables/useTaskSSE.ts
export function useTaskSSE(taskId: string) {
  const status = ref<TaskStatus>()
  const agentStates = ref<AgentState[]>([])
  const error = ref<string>()

  let eventSource: EventSource

  function connect() {
    eventSource = new EventSource(`/api/tasks/${taskId}/stream`)

    eventSource.addEventListener('agent_state', (e) => {
      const data = JSON.parse(e.data)
      const idx = agentStates.value.findIndex(a => a.name === data.agent)
      if (idx >= 0) agentStates.value[idx] = data
      else agentStates.value.push(data)
    })

    eventSource.addEventListener('task_complete', (e) => {
      status.value = JSON.parse(e.data)
      eventSource.close()
    })

    eventSource.addEventListener('rejection', (e) => {
      const data = JSON.parse(e.data)
      showRejectionNotification(data)
    })

    eventSource.onerror = () => {
      error.value = '连接断开，正在重连...'
      setTimeout(connect, 3000)
    }
  }

  onMounted(connect)
  onUnmounted(() => eventSource?.close())

  return { status, agentStates, error }
}
```

**后端对应的 SSE 事件类型**：

| SSE Event | 触发时机 | 前端响应 |
|---|---|---|
| `task_started` | 任务开始 | Dashboard 进度条出现 |
| `agent_state` | 任一 Agent 状态变更 | TraceTimeline / TaskCard 更新 |
| `rejection` | QA 打回 | 红色高亮 + Toast 通知 |
| `clarification` | Coordinator 需要用户输入 | 弹出澄清对话框 |
| `task_complete` | 任务完成 | 跳转报告页 / 关闭 SSE |
| `task_failed` | 任务失败 | 错误提示 + 重试按钮 |

## 5. API 接口约定

```typescript
// api/task.ts
POST   /api/tasks                    // 创建任务
GET    /api/tasks                    // 任务列表
GET    /api/tasks/:id                // 任务详情
GET    /api/tasks/:id/stream         // SSE 实时状态流
POST   /api/tasks/:id/clarify        // 回复 Coordinator 的澄清问题
DELETE /api/tasks/:id                // 取消任务

// api/report.ts
GET    /api/reports/:taskId          // 查看报告
GET    /api/reports/:taskId/export   // 导出报告 ?format=pdf|md
POST   /api/reports/:taskId/annotate // 人工标注

// api/trace.ts
GET    /api/traces/:taskId           // 查看 Agent 执行链路
GET    /api/traces/:taskId/replay    // 回放数据

// api/agent.ts
GET    /api/agents                   // 获取所有 Agent Card
GET    /api/agents/:name             // 获取单个 Agent Card + 运行统计
```

## 6. 开发顺序（按优先级）

| 优先级 | 内容 | 原因 |
|---|---|---|
| P0 | 项目脚手架 + 路由 + Axios 封装 + SSE composable | 基础设施，后续所有页面依赖 |
| P0 | Dashboard + CreateTaskDialog | 能创建任务并看到进度，最早可联调 |
| P1 | ReportView + FeatureMatrix + SWOTRadar + SourceRefPanel | 核心展示页，评委最关注 |
| P1 | TraceTimeline + AgentNodeDetail | Agent 追踪可追溯，直接命中评分点 |
| P2 | TraceReplay（回放控件） | 锦上添花的交互 |
| P2 | AgentsView（DAG 可视化） | Agent 能力展示 |
| P2 | AnnotationTool（人工标注） | 人工介入修正 |
| P3 | 导出 PDF/Markdown | 演示加分项 |
| P3 | 暗色模式、Tab 切换动画 | 视觉打磨 |
