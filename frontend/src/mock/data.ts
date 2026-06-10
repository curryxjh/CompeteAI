import type { AgentCard, Report, Task, Trace } from '@/types'

export const mockTasks: Task[] = [
  {
    id: 't001',
    title: 'Cursor vs GitHub Copilot 对比分析',
    competitors: ['Cursor', 'GitHub Copilot'],
    dimensions: ['功能对比', 'SWOT', '定价', '用户画像'],
    status: 'running',
    progress: 60,
    agentStates: [
      { name: 'coordinator', status: 'completed' },
      { name: 'collector', status: 'completed' },
      { name: 'analyst', status: 'running', message: '正在生成功能矩阵...' },
      { name: 'writer', status: 'pending' },
      { name: 'qa', status: 'pending' },
    ],
    createdAt: '2026-05-25T14:30:00+08:00',
  },
  {
    id: 't002',
    title: 'AI 编程工具市场竞品分析',
    competitors: ['Cursor', 'Copilot', 'Windsurf', 'Claude Code'],
    dimensions: ['功能对比', 'SWOT', '定价'],
    status: 'completed',
    progress: 100,
    agentStates: [
      { name: 'coordinator', status: 'completed' },
      { name: 'collector', status: 'completed' },
      { name: 'analyst', status: 'completed' },
      { name: 'writer', status: 'completed' },
      { name: 'qa', status: 'completed' },
    ],
    createdAt: '2026-05-24T10:00:00+08:00',
    updatedAt: '2026-05-24T11:20:00+08:00',
  },
]

export const mockReport: Report = {
  taskId: 't002',
  title: 'Cursor vs GitHub Copilot 竞品分析报告',
  generatedAt: '2026-05-25T15:00:00+08:00',
  qaScore: 92,
  summary:
    'Cursor 在多文件编辑与 Agent 工作流上领先；Copilot 在 IDE 集成广度与 GitHub 生态上占优。两者在代码补全基线能力上已趋同。',
  swot: {
    Cursor: {
      strengths: [
        { text: '多文件 Agent 编辑能力强', sourceIds: ['s1'] },
        { text: 'Composer 工作流体验领先', sourceIds: ['s2'] },
      ],
      weaknesses: [{ text: '企业合规案例相对少' }],
      opportunities: [{ text: '中文开发者市场渗透' }],
      threats: [{ text: '大厂 IDE 内置 AI 挤压' }],
    },
    'GitHub Copilot': {
      strengths: [
        { text: 'GitHub 生态深度集成', sourceIds: ['s3'] },
        { text: '多 IDE 覆盖广', sourceIds: ['s4'] },
      ],
      weaknesses: [{ text: '多文件重构能力弱于 Cursor' }],
      opportunities: [{ text: 'Enterprise 套餐扩展' }],
      threats: [{ text: '独立 AI IDE 分流用户' }],
    },
  },
  features: [
    {
      feature: '代码补全',
      values: { Cursor: true, 'GitHub Copilot': true },
    },
    {
      feature: 'Chat 对话',
      values: { Cursor: true, 'GitHub Copilot': true },
      sourceIds: { Cursor: ['s1'], 'GitHub Copilot': ['s3'] },
    },
    {
      feature: '多文件编辑',
      values: { Cursor: true, 'GitHub Copilot': false },
    },
    {
      feature: 'Agent 工作流',
      values: { Cursor: true, 'GitHub Copilot': '部分' },
    },
  ],
  pricing: [
    {
      competitor: 'Cursor',
      tiers: [
        { name: 'Hobby', price: '免费', features: ['有限 Agent 请求'] },
        { name: 'Pro', price: '$20/月', features: ['无限补全', '更多 Agent'] },
      ],
    },
    {
      competitor: 'GitHub Copilot',
      tiers: [
        { name: 'Individual', price: '$10/月', features: ['代码补全', 'Chat'] },
        { name: 'Business', price: '$19/用户/月', features: ['策略管理', '审计'] },
      ],
    },
  ],
  personas: [
    {
      competitor: 'Cursor',
      segments: ['独立开发者', '全栈工程师'],
      painPoints: ['上下文切换多', '需要跨文件重构'],
      useCases: ['功能开发', 'Bug 修复', '代码审查'],
    },
    {
      competitor: 'GitHub Copilot',
      segments: ['企业开发团队', '开源贡献者'],
      painPoints: ['需要与现有 GitHub 工作流一致'],
      useCases: ['日常补全', 'PR 辅助', '文档生成'],
    },
  ],
  sources: {
    s1: {
      url: 'https://cursor.com/docs',
      excerpt: 'Cursor supports multi-file editing via Composer...',
      collectedAt: '2026-05-24T09:00:00+08:00',
      title: 'Cursor Docs',
    },
    s2: {
      url: 'https://cursor.com/blog',
      excerpt: 'Agent workflow updates in latest release...',
      collectedAt: '2026-05-24T09:15:00+08:00',
      title: 'Cursor Blog',
    },
    s3: {
      url: 'https://docs.github.com/copilot',
      excerpt: 'GitHub Copilot integrates with VS Code, JetBrains...',
      collectedAt: '2026-05-24T09:30:00+08:00',
      title: 'GitHub Copilot Docs',
    },
    s4: {
      url: 'https://github.com/features/copilot/plans',
      excerpt: 'Pricing tiers for Individual and Business...',
      collectedAt: '2026-05-24T09:45:00+08:00',
      title: 'Copilot Plans',
    },
  },
}

export const mockTrace: Trace = {
  taskId: 't001',
  nodes: [
    {
      id: 'n1',
      agent: 'coordinator',
      label: 'Coordinator',
      status: 'completed',
      durationMs: 2300,
      tokenCount: 150,
      output: '拆解任务：Cursor vs Copilot，维度 4 项',
    },
    {
      id: 'n2',
      agent: 'collector',
      label: 'Collector',
      status: 'completed',
      durationMs: 8500,
      tokenCount: 3200,
      output: '搜索 15 URLs → 抓取 12 pages',
      metadata: { urls: 15, pages: 12 },
    },
    {
      id: 'n3',
      agent: 'analyst',
      label: 'Analyst',
      status: 'completed',
      durationMs: 5100,
      tokenCount: 1800,
    },
    {
      id: 'n4',
      agent: 'writer',
      label: 'Writer',
      status: 'completed',
      durationMs: 3200,
      tokenCount: 900,
    },
    {
      id: 'n5',
      agent: 'qa',
      label: 'QA',
      status: 'rejected',
      durationMs: 2100,
      tokenCount: 600,
      isRejection: true,
      output: '发现 2 个问题 → 打回 Analyst',
    },
    {
      id: 'n6',
      agent: 'analyst',
      label: 'Analyst (Retry #1)',
      status: 'completed',
      durationMs: 3500,
      tokenCount: 1200,
      isRetry: true,
      parentId: 'n5',
    },
    {
      id: 'n7',
      agent: 'qa',
      label: 'QA (Retry #1)',
      status: 'completed',
      durationMs: 1800,
      tokenCount: 500,
      isRetry: true,
      output: '全部通过，评分 92%',
    },
  ],
}

export const mockAgentCards: AgentCard[] = [
  {
    name: 'coordinator',
    displayName: 'Coordinator',
    description: '任务协调者：拆解竞品分析任务、路由 Agent、处理澄清循环',
    skills: ['任务拆解', 'DAG 编排', '澄清对话'],
    tools: ['TaskPlanner', 'StateRouter'],
    dependsOn: [],
  },
  {
    name: 'collector',
    displayName: 'Collector',
    description: '信息采集者：Search → Fetch → Extract → Enrich → Validate 五阶段流水线',
    skills: ['网页搜索', '内容抓取', '结构化抽取', '溯源标注'],
    tools: ['WebSearch', 'PageFetcher', 'SchemaValidator'],
    dependsOn: ['coordinator'],
  },
  {
    name: 'analyst',
    displayName: 'Analyst',
    description: '分析者：功能矩阵、SWOT、定价与用户画像结构化分析',
    skills: ['竞品对比', 'SWOT 生成', '定价建模'],
    tools: ['FeatureMatrix', 'SWOTGenerator'],
    dependsOn: ['collector'],
  },
  {
    name: 'writer',
    displayName: 'Writer',
    description: '报告撰写者：将分析结果合成为可读竞品报告',
    skills: ['报告撰写', '章节组织', '结论提炼'],
    tools: ['ReportComposer'],
    dependsOn: ['analyst'],
  },
  {
    name: 'qa',
    displayName: 'QA',
    description: '质检者：事实校验、Schema 合规、打回重做闭环',
    skills: ['事实核验', 'Schema 校验', '打回路由'],
    tools: ['FactChecker', 'SchemaValidator'],
    dependsOn: ['writer'],
  },
]

export const ANALYSIS_DIMENSIONS = [
  '功能对比',
  'SWOT',
  '定价',
  '用户画像',
  '市场定位',
] as const
