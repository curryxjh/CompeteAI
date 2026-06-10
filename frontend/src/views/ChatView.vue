<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  streamChat,
  type ChatMessage,
  type ChatStreamEvent,
} from '@/api/chat'
import {
  fromMessageRecord,
  toMessageRecord,
  useChatConversations,
  welcomeMessage,
} from '@/composables/useChatConversations'
import { usePaneResize } from '@/composables/usePaneResize'
import { getReport } from '@/api/report'
import {
  createTask,
  subscribeTaskStream,
  type AgentThinkingEvent,
  type ClarificationEvent,
  type RejectionEvent,
  type TaskToolStepEvent,
} from '@/api/task'
import { renderMarkdown, stepTypeLabel, toolDisplayName } from '@/utils/markdown'
import { reportToMarkdown } from '@/utils/reportMarkdown'
import {
  ANALYSIS_DIMENSIONS,
  buildAnalysisPayload,
  DEFAULT_ANALYSIS_DIMENSIONS,
  parseCompetitorList,
} from '@/utils/analysis'
import type { CreateTaskPayload } from '@/types'
import { AGENT_LABELS, AGENT_ORDER, agentStatusIcon, formatDuration } from '@/utils/agent'
import type { AgentName, AgentRunStatus, AgentState } from '@/types'
import AgentIcon from '@/components/ui/AgentIcon.vue'
import StatusIcon from '@/components/ui/StatusIcon.vue'

type StepType = 'thinking' | 'tool' | 'agent' | 'event'
type StepFilter = 'all' | AgentName

interface ChatStep {
  id: string
  type: StepType
  content: string
  toolName?: string
  toolArgs?: string
  toolResult?: string
  status?: 'running' | 'done' | 'error'
  agentName?: AgentName
  agentStatus?: AgentRunStatus
  parentAgent?: AgentName
  thinkingKind?: 'path' | 'note' | 'thinking' | 'output' | 'analysis'
  eventKind?: 'clarification' | 'rejection'
  noteRunning?: boolean
  expanded?: boolean
  createdAt?: number
  finishedAt?: number
  question?: string
  targetAgent?: AgentName
  metricsLabel?: string
}

interface UiMessage extends ChatMessage {
  id: string
  streaming?: boolean
  steps?: ChatStep[]
  stepsVisible?: boolean
  taskMode?: boolean
  taskId?: string
  stepFilter?: StepFilter
  replayActive?: boolean
  replayPlaying?: boolean
  replayVisibleCount?: number
  replaySpeed?: number
}

const messages = ref<UiMessage[]>([])

const {
  conversations,
  activeId: activeConversationId,
  loadingList: conversationsLoading,
  switching: conversationSwitching,
  init: initConversations,
  createNew: createNewConversation,
  switchTo: switchConversationTo,
  persist: persistConversationMessages,
  remove: removeConversation,
} = useChatConversations()

const router = useRouter()
const auth = useAuthStore()
const useMock = import.meta.env.VITE_USE_MOCK !== 'false'

type ChatMode = 'chat' | 'analysis'

const chatMode = ref<ChatMode>('chat')
const analysisCompetitors = ref('')
const analysisDimensions = ref<string[]>([...DEFAULT_ANALYSIS_DIMENSIONS])

const parsedCompetitors = computed(() => parseCompetitorList(analysisCompetitors.value))

const canSendAnalysis = computed(
  () => parsedCompetitors.value.length >= 2 && analysisDimensions.value.length > 0,
)

const FOOTER_ANALYSIS_MIN = 228

const sidebarPane = usePaneResize({
  storageKey: 'competeai_chat_sidebar_w',
  defaultSize: 220,
  min: 180,
  max: 360,
  collapseKey: 'competeai_chat_sidebar_collapsed',
  collapsedSize: 64,
})

const footerPane = usePaneResize({
  storageKey: 'competeai_chat_footer_h',
  defaultSize: 148,
  min: 112,
  max: () => Math.min(480, Math.floor(window.innerHeight * 0.55)),
  collapseKey: 'competeai_chat_footer_collapsed',
  collapsedSize: 88,
})

const input = ref('')
const loading = ref(false)
const listRef = ref<HTMLElement | null>(null)
const abortRef = ref<AbortController | null>(null)
const taskStreamUnsub = ref<(() => void) | null>(null)
const replayTimers = new Map<string, number>()

function scrollToBottom() {
  nextTick(() => {
    const el = listRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function uid() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

async function persistTurn(userMsg: UiMessage, assistantMsg: UiMessage) {
  try {
    await persistConversationMessages([
      toMessageRecord(userMsg),
      toMessageRecord(assistantMsg),
    ])
  } catch {
    ElMessage.warning('对话已产生，但保存到服务器失败')
  }
}

async function loadConversation(id: string) {
  if (loading.value) stop()
  closeTaskStream()
  clearAllReplays()
  const loaded = await switchConversationTo(id)
  messages.value = loaded as UiMessage[]
  scrollToBottom()
}

async function startNewConversation() {
  if (loading.value) stop()
  closeTaskStream()
  clearAllReplays()
  await createNewConversation()
  messages.value = [fromMessageRecord(welcomeMessage()) as UiMessage]
  input.value = ''
  scrollToBottom()
}

async function deleteConversationItem(id: string) {
  try {
    await ElMessageBox.confirm('确定删除该对话？删除后无法恢复。', '删除对话', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  const wasActive = activeConversationId.value === id
  await removeConversation(id)
  if (wasActive) {
    if (conversations.value.length) {
      await loadConversation(conversations.value[0]!.id)
    } else {
      await startNewConversation()
    }
  }
  ElMessage.success('对话已删除')
}

function go(path: string) {
  router.push(path)
}

watch(chatMode, (mode) => {
  if (footerPane.collapsed.value) return
  if (mode === 'analysis' && footerPane.size.value < FOOTER_ANALYSIS_MIN) {
    footerPane.size.value = FOOTER_ANALYSIS_MIN
    footerPane.persistSize()
  }
})

function formatConvTime(iso: string) {
  const d = new Date(iso)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

function initAgentSteps(): ChatStep[] {
  return AGENT_ORDER.map((name) => ({
    id: uid(),
    type: 'agent' as const,
    content: '',
    agentName: name,
    agentStatus: name === 'coordinator' ? 'running' : 'pending',
    createdAt: name === 'coordinator' ? Date.now() : undefined,
    metricsLabel: '0 tool',
  }))
}

function taskAgentStatus(msg: UiMessage, agentName: AgentName): AgentRunStatus {
  const step = msg.steps?.find((item) => item.type === 'agent' && item.agentName === agentName)
  return step?.agentStatus ?? 'pending'
}

function agentStep(msg: UiMessage, agentName: AgentName) {
  return msg.steps?.find((item) => item.type === 'agent' && item.agentName === agentName)
}

function agentToolCount(msg: UiMessage, agentName: AgentName): number {
  return (
    msg.steps?.filter((step) => step.parentAgent === agentName && step.type === 'tool').length ?? 0
  )
}

function agentDurationMs(msg: UiMessage, agentName: AgentName): number | null {
  const step = agentStep(msg, agentName)
  if (!step?.createdAt) return null
  const effectiveEnd = step.finishedAt ?? Date.now()
  return Math.max(0, effectiveEnd - step.createdAt)
}

function agentMetricsSummary(msg: UiMessage, agentName: AgentName): string {
  const duration = agentDurationMs(msg, agentName)
  const tools = agentToolCount(msg, agentName)
  const parts: string[] = []
  if (duration !== null) parts.push(formatDuration(duration))
  parts.push(`${tools} tool`)
  return parts.join(' · ')
}

function refreshAgentMetrics(msg: UiMessage) {
  for (const name of AGENT_ORDER) {
    const step = agentStep(msg, name)
    if (!step) continue
    step.metricsLabel = agentMetricsSummary(msg, name)
  }
}

function filteredSteps(msg: UiMessage): ChatStep[] {
  const steps = msg.steps ?? []
  const filter = msg.stepFilter ?? 'all'
  if (filter === 'all') return steps
  return steps.filter((step) => {
    if (step.type === 'agent') return step.agentName === filter
    return step.parentAgent === filter
  })
}

function visibleSteps(msg: UiMessage): ChatStep[] {
  const scoped = filteredSteps(msg)
  if (!msg.replayActive) return scoped
  const visibleCount = msg.replayVisibleCount ?? scoped.length
  return scoped.slice(0, visibleCount)
}

function displayedStepCount(msg: UiMessage): number {
  return visibleSteps(msg).length
}

function hasStepFilter(msg: UiMessage): boolean {
  return (msg.stepFilter ?? 'all') !== 'all'
}

function setStepFilter(msg: UiMessage, filter: StepFilter) {
  msg.stepFilter = filter
  if (msg.replayActive) {
    const limit = filteredSteps(msg).length
    msg.replayVisibleCount = Math.min(msg.replayVisibleCount ?? limit, limit)
  }
}

function canReplayProcess(msg: UiMessage): boolean {
  return !!msg.taskMode && !msg.streaming && (filteredSteps(msg).length > 1)
}

function stopProcessReplay(msg: UiMessage) {
  const timer = replayTimers.get(msg.id)
  if (timer !== undefined) {
    window.clearInterval(timer)
    replayTimers.delete(msg.id)
  }
  msg.replayPlaying = false
}

function scheduleProcessReplay(msg: UiMessage) {
  stopProcessReplay(msg)
  const total = filteredSteps(msg).length
  if (!total) return
  msg.replayPlaying = true
  msg.replayActive = true
  const delay = Math.max(240, Math.floor(1200 / (msg.replaySpeed ?? 1)))
  const timer = window.setInterval(() => {
    const currentTotal = filteredSteps(msg).length
    const next = (msg.replayVisibleCount ?? 0) + 1
    if (!currentTotal || next > currentTotal) {
      stopProcessReplay(msg)
      return
    }
    msg.replayVisibleCount = next
    scrollToBottom()
    if (next >= currentTotal) {
      stopProcessReplay(msg)
    }
  }, delay)
  replayTimers.set(msg.id, timer)
}

function toggleProcessReplay(msg: UiMessage) {
  if (!canReplayProcess(msg)) return
  if (msg.replayPlaying) {
    stopProcessReplay(msg)
    return
  }
  const total = filteredSteps(msg).length
  if ((msg.replayVisibleCount ?? total) >= total) {
    msg.replayVisibleCount = 0
  }
  scheduleProcessReplay(msg)
}

function stepProcessReplay(msg: UiMessage) {
  if (!canReplayProcess(msg)) return
  stopProcessReplay(msg)
  msg.replayActive = true
  const total = filteredSteps(msg).length
  msg.replayVisibleCount = Math.min((msg.replayVisibleCount ?? 0) + 1, total)
}

function resetProcessReplay(msg: UiMessage) {
  stopProcessReplay(msg)
  msg.replayActive = true
  msg.replayVisibleCount = 0
}

function showFullProcess(msg: UiMessage) {
  stopProcessReplay(msg)
  msg.replayActive = false
  msg.replayVisibleCount = filteredSteps(msg).length
}

function updateReplaySpeed(msg: UiMessage, value: number) {
  msg.replaySpeed = value
  if (msg.replayPlaying) {
    scheduleProcessReplay(msg)
  }
}

function insertAfterAgent(assistant: UiMessage, agentName: AgentName, step: ChatStep) {
  if (!assistant.steps) assistant.steps = []
  const agentIdx = assistant.steps.findIndex(
    (s) => s.type === 'agent' && s.agentName === agentName,
  )
  if (agentIdx < 0) {
    assistant.steps.push(step)
    return
  }
  let insertAt = agentIdx + 1
  while (insertAt < assistant.steps.length) {
    const cur = assistant.steps[insertAt]
    if (cur.type === 'agent') break
    insertAt++
  }
  assistant.steps.splice(insertAt, 0, step)
}

function updateAgentStep(assistant: UiMessage, data: AgentState & { message?: string }) {
  if (!assistant.steps) return
  const name = data.name
  if (!name) return
  const step = assistant.steps.find((s) => s.type === 'agent' && s.agentName === name)
  if (!step) return
  if (data.status === 'running' && step.agentStatus !== 'running') {
    step.createdAt = Date.now()
    step.finishedAt = undefined
  }
  step.agentStatus = data.status
  if (data.message) step.content = data.message
  if (data.status === 'running') {
    step.expanded = true
  }
  if (data.status === 'completed' || data.status === 'failed' || data.status === 'rejected') {
    step.finishedAt = step.finishedAt ?? Date.now()
    step.expanded = false
  }
  refreshAgentMetrics(assistant)
}

function handleAgentThinking(assistant: UiMessage, ev: AgentThinkingEvent) {
  if (!assistant.steps) assistant.steps = []
  const agent = ev.agent as AgentName | undefined
  if (!agent) return
  const content = ev.content ?? ''
  const kind = ev.kind ?? 'note'

  if (kind === 'output') {
    const last = [...assistant.steps].reverse().find(
      (s) =>
        s.type === 'thinking' &&
        s.thinkingKind === 'output' &&
        s.parentAgent === agent,
    )
    if (last && ev.status === 'running') {
      last.content += content
      last.expanded = true
      return
    }
    insertAfterAgent(assistant, agent, {
      id: uid(),
      type: 'thinking',
      thinkingKind: 'output',
      content,
      parentAgent: agent,
      expanded: true,
    })
    return
  }

  if (kind === 'analysis') {
    const existing = assistant.steps.find(
      (s) => s.thinkingKind === 'analysis' && s.parentAgent === agent,
    )
    if (existing) {
      existing.content = content
      existing.expanded = true
    } else {
      insertAfterAgent(assistant, agent, {
        id: uid(),
        type: 'thinking',
        thinkingKind: 'analysis',
        content,
        parentAgent: agent,
        expanded: true,
      })
    }
    assistant.content = [
      '## 分析报告（Analyst 预览）',
      '',
      content,
      '',
      '---',
      '',
      '_Writer / QA 仍在处理，完成后将在此展示完整终稿…_',
    ].join('\n')
    return
  }

  if (kind === 'thinking') {
    if (!content) return
    const last = [...assistant.steps].reverse().find(
      (s) =>
        s.type === 'thinking' &&
        s.thinkingKind === 'thinking' &&
        s.parentAgent === agent,
    )
    if (last && ev.status === 'running') {
      last.content += content
      last.expanded = true
      return
    }
    insertAfterAgent(assistant, agent, {
      id: uid(),
      type: 'thinking',
      thinkingKind: 'thinking',
      content,
      parentAgent: agent,
      expanded: true,
    })
    return
  }

  if (kind === 'note' && ev.status === 'running') {
    const runningNote = [...assistant.steps].reverse().find(
      (s) =>
        s.thinkingKind === 'note' &&
        s.parentAgent === agent &&
        s.noteRunning,
    )
    if (runningNote) {
      runningNote.content = content
      return
    }
  }

  if (!content) return

  insertAfterAgent(assistant, agent, {
    id: uid(),
    type: 'thinking',
    thinkingKind: kind === 'path' ? 'path' : 'note',
    content,
    parentAgent: agent,
    expanded: kind === 'path',
    noteRunning: kind === 'note' && ev.status === 'running',
  })
}

function handleTaskToolStep(assistant: UiMessage, ev: TaskToolStepEvent) {
  if (!assistant.steps) assistant.steps = []
  const toolName = ev.tool_name
  if (!toolName) return
  const parentAgent = (ev.agent ?? 'collector') as AgentName

  if (ev.status === 'running') {
    insertAfterAgent(assistant, parentAgent, {
      id: uid(),
      type: 'tool',
      content: '',
      toolName,
      toolArgs: ev.tool_args,
      status: 'running',
      parentAgent,
      expanded: false,
    })
    return
  }

  const step = [...assistant.steps].reverse().find(
    (s) =>
      s.type === 'tool' &&
      s.toolName === toolName &&
      s.status === 'running' &&
      s.parentAgent === parentAgent,
  )
  if (step) {
    step.toolResult = ev.tool_result
    step.toolArgs = ev.tool_args ?? step.toolArgs
    step.status = ev.status === 'error' ? 'error' : 'done'
  } else {
    insertAfterAgent(assistant, parentAgent, {
      id: uid(),
      type: 'tool',
      content: '',
      toolName,
      toolArgs: ev.tool_args,
      toolResult: ev.tool_result,
      status: ev.status === 'error' ? 'error' : 'done',
      parentAgent,
      expanded: false,
    })
  }
  refreshAgentMetrics(assistant)
}

function insertProcessEvent(assistant: UiMessage, step: ChatStep) {
  if (!assistant.steps) assistant.steps = []
  assistant.steps.push(step)
}

function handleClarificationEvent(assistant: UiMessage, ev: ClarificationEvent) {
  if (!assistant.steps) assistant.steps = []
  const question = ev.question ?? '需要进一步澄清任务信息'
  const owner = (ev.agent as AgentName | undefined) ?? 'coordinator'
  insertProcessEvent(assistant, {
    id: uid(),
    type: 'event',
    eventKind: 'clarification',
    content: question,
    question,
    parentAgent: owner,
    createdAt: Date.now(),
    expanded: true,
  })
  refreshAgentMetrics(assistant)
}

function handleRejectionEvent(assistant: UiMessage, ev: RejectionEvent) {
  if (!assistant.steps) assistant.steps = []
  const fromAgent = ev.fromAgent as AgentName | undefined
  const target = ev.toAgent as AgentName | undefined
  const reason = ev.reason ?? 'QA 打回，需重做'
  insertProcessEvent(assistant, {
    id: uid(),
    type: 'event',
    eventKind: 'rejection',
    content: reason,
    parentAgent: fromAgent ?? 'qa',
    targetAgent: target,
    createdAt: Date.now(),
    expanded: true,
  })
  refreshAgentMetrics(assistant)
}

function chatStepLabel(step: ChatStep): string {
  if (step.type === 'event') {
    return step.eventKind === 'clarification' ? '待澄清' : 'QA 打回'
  }
  return stepTypeLabel(step)
}

function stepSummary(step: ChatStep, max = 56): string {
  if (step.type === 'event') {
    if (step.eventKind === 'clarification') {
      return truncate(step.question ?? step.content, max)
    }
    const target = step.targetAgent ? AGENT_LABELS[step.targetAgent] : '目标 Agent'
    return truncate(`${target} · ${step.content}`, max)
  }
  if (step.type === 'thinking') {
    if (step.thinkingKind === 'path') return truncate(step.content, max)
    if (step.thinkingKind === 'note') return truncate(step.content, max)
    if (step.thinkingKind === 'analysis') return '结构化分析结果'
    if (step.thinkingKind === 'output') {
      return step.content ? truncate(step.content.replace(/\s+/g, ' '), max) : '生成中…'
    }
    return step.content ? truncate(step.content, max) : '思考中…'
  }
  if (step.type === 'agent') {
    if (step.content) return truncate(step.content, max)
    if (step.metricsLabel) return step.metricsLabel
    if (step.agentStatus === 'running') return '执行中…'
    if (step.agentStatus === 'completed') return '已完成'
    if (step.agentStatus === 'failed') return '失败'
    if (step.agentStatus === 'rejected') return '被打回'
    return '等待中'
  }
  const arg = step.toolArgs ?? ''
  const query = arg.match(/^query:\s*(.+)/m)?.[1]
  if (query) return truncate(query, max)
  const url = arg.match(/^url:\s*(.+)/m)?.[1]
  if (url) {
    try {
      const u = new URL(url)
      const path = u.pathname.length > 1 ? u.pathname : ''
      return truncate(`${u.hostname.replace(/^www\./, '')}${path}`, max)
    } catch {
      return truncate(url, max)
    }
  }
  if (arg) return truncate(arg.split('\n')[0], max)
  if (step.toolResult) {
    const first = step.toolResult.split('\n').find((l) => l.trim()) ?? ''
    return truncate(first.replace(/^#+\s*/, '').replace(/\*\*/g, ''), max)
  }
  return step.status === 'running' ? '执行中…' : '已完成'
}

function stepCount(msg: UiMessage): number {
  return displayedStepCount(msg)
}

function processStats(msg: UiMessage): string {
  const steps = visibleSteps(msg)
  if (!steps.length) return ''

  const agents = steps.filter((s) => s.type === 'agent')
  const tools = steps.filter((s) => s.type === 'tool')
  const thinking = steps.filter((s) => s.type === 'thinking').length

  if (msg.taskMode && agents.length) {
    const running = agents.find((s) => s.agentStatus === 'running')
    const thinkingCount = steps.filter(
      (s) => s.type === 'thinking' && s.thinkingKind === 'thinking',
    ).length
    if (running?.agentName) {
      const extra = thinkingCount ? ` · 推理 ${thinkingCount} 段` : ''
      return `${AGENT_LABELS[running.agentName]} 执行中…${extra}`
    }
    const toolRunning = tools.find((s) => s.status === 'running')
    if (toolRunning) {
      return `正在 ${toolDisplayName(toolRunning.toolName)}…`
    }
    const doneAgents = agents.filter((s) => s.agentStatus === 'completed').length
    const parts: string[] = [`Agent ${doneAgents}/${agents.length}`]
    if (hasStepFilter(msg)) {
      parts.push(`聚焦 ${AGENT_LABELS[(msg.stepFilter ?? 'all') as AgentName]}`)
    }
    if (msg.replayActive) {
      parts.push(`回放 ${displayedStepCount(msg)}/${filteredSteps(msg).length}`)
    }
    if (tools.length) {
      const byTool = new Map<string, number>()
      for (const s of tools.filter((t) => t.status === 'done')) {
        const key = stepTypeLabel(s)
        byTool.set(key, (byTool.get(key) ?? 0) + 1)
      }
      for (const [name, n] of byTool) {
        parts.push(`${name} ${n} 次`)
      }
    }
    return parts.join('，')
  }

  const running = tools.filter((s) => s.status === 'running').length
  if (running > 0) {
    const cur = tools.find((s) => s.status === 'running')
    const name = cur ? toolDisplayName(cur.toolName) : '工具'
    return `正在 ${name}…`
  }
  const parts: string[] = []
  if (thinking) parts.push(`推理 ${thinking} 次`)
  if (msg.replayActive) parts.push(`回放 ${displayedStepCount(msg)}/${filteredSteps(msg).length}`)
  const byTool = new Map<string, number>()
  for (const s of tools) {
    const key = stepTypeLabel(s)
    byTool.set(key, (byTool.get(key) ?? 0) + 1)
  }
  for (const [name, n] of byTool) {
    parts.push(`${name} ${n} 次`)
  }
  return parts.join('，')
}

function toggleSteps(msg: UiMessage) {
  msg.stepsVisible = !msg.stepsVisible
}

function truncate(text: string, max: number) {
  const t = text.replace(/\s+/g, ' ').trim()
  return t.length > max ? `${t.slice(0, max)}…` : t
}

function handleStreamEvent(assistant: UiMessage, ev: ChatStreamEvent) {
  if (!assistant.steps) assistant.steps = []

  switch (ev.type) {
    case 'thinking': {
      const last = assistant.steps.at(-1)
      if (last?.type === 'thinking' && assistant.streaming) {
        last.content += ev.content ?? ''
      } else {
        assistant.steps.push({
          id: uid(),
          type: 'thinking',
          content: ev.content ?? '',
          expanded: false,
        })
      }
      break
    }
    case 'tool_call': {
      assistant.steps.push({
        id: uid(),
        type: 'tool',
        content: '',
        toolName: ev.tool_name,
        toolArgs: ev.tool_args,
        status: 'running',
        expanded: false,
      })
      break
    }
    case 'tool_result': {
      const step = [...assistant.steps].reverse().find(
        (s) => s.type === 'tool' && s.toolName === ev.tool_name && s.status === 'running',
      )
      if (step) {
        step.toolResult = ev.tool_result
        step.toolArgs = ev.tool_args ?? step.toolArgs
        step.status = ev.status === 'error' ? 'error' : 'done'
        step.expanded = false
      } else {
        assistant.steps.push({
          id: uid(),
          type: 'tool',
          content: '',
          toolName: ev.tool_name,
          toolArgs: ev.tool_args,
          toolResult: ev.tool_result,
          status: ev.status === 'error' ? 'error' : 'done',
          expanded: false,
        })
      }
      break
    }
    case 'content':
      assistant.content += ev.content ?? ''
      break
    default:
      if (ev.content) {
        assistant.content += ev.content
      }
  }
}

function closeTaskStream() {
  taskStreamUnsub.value?.()
  taskStreamUnsub.value = null
}

function clearAllReplays() {
  for (const msg of messages.value) {
    stopProcessReplay(msg)
  }
}

async function sendAnalysisTask(payload: CreateTaskPayload, userMsg: UiMessage) {
  if (!payload) return

  const assistantId = uid()
  messages.value.push({
    id: assistantId,
    role: 'assistant',
    content: '正在启动多 Agent 竞品分析流水线…',
    steps: initAgentSteps(),
    stepsVisible: true,
    streaming: true,
    taskMode: true,
    stepFilter: 'all',
    replaySpeed: 1,
  })
  refreshAgentMetrics(messages.value[messages.value.length - 1]!)
  loading.value = true
  scrollToBottom()

  const assistant = messages.value.find((m) => m.id === assistantId)
  if (!assistant) return

  closeTaskStream()

  try {
    const task = await createTask(payload)
    assistant.taskId = task.id
    assistant.content = `已创建任务 **${task.title}**，Agent 流水线执行中…`

    taskStreamUnsub.value = subscribeTaskStream(task.id, {
      onStarted(data) {
        if (data.agentStates?.length) {
          for (const st of data.agentStates) {
            updateAgentStep(assistant, st)
          }
        }
        refreshAgentMetrics(assistant)
        scrollToBottom()
      },
      onAgentState(data) {
        const name = (data.name ?? data.agent) as AgentName | undefined
        if (!name) return
        updateAgentStep(assistant, { ...data, name })
        scrollToBottom()
      },
      onToolStep(ev) {
        handleTaskToolStep(assistant, ev)
        scrollToBottom()
      },
      onAgentThinking(ev) {
        handleAgentThinking(assistant, ev)
        scrollToBottom()
      },
      onTaskStatus(data) {
        if (data.status === 'clarifying') {
          assistant.content += '\n\n⚠️ Coordinator 需要澄清，请前往任务页回复。'
        }
        if (data.status === 'reworking') {
          assistant.content += '\n\n↩ QA 打回，正在局部重跑…'
        }
      },
      onClarification(data) {
        handleClarificationEvent(assistant, data)
        scrollToBottom()
      },
      onRejection(data) {
        handleRejectionEvent(assistant, data)
        scrollToBottom()
      },
      async onComplete() {
        assistant.streaming = false
        assistant.replayActive = false
        assistant.replayVisibleCount = assistant.steps?.length ?? 0
        try {
          const report = await getReport(task.id)
          assistant.content = reportToMarkdown(report)
        } catch {
          assistant.content += '\n\n任务已完成，但加载报告失败，请前往任务页查看。'
        }
        loading.value = false
        await persistTurn(userMsg, assistant)
        scrollToBottom()
      },
      onFailed(data) {
        assistant.streaming = false
        assistant.replayActive = false
        assistant.replayVisibleCount = assistant.steps?.length ?? 0
        assistant.content = `任务失败：${data.message ?? '未知错误'}`
        loading.value = false
        ElMessage.error(data.message ?? '任务失败')
        void persistTurn(userMsg, assistant)
        scrollToBottom()
      },
    })
  } catch (e) {
    assistant.streaming = false
    assistant.replayActive = false
    assistant.replayVisibleCount = assistant.steps?.length ?? 0
    const msg = e instanceof Error ? e.message : '创建任务失败'
    assistant.content = `抱歉，无法启动竞品分析任务：${msg}`
    ElMessage.error(msg)
    loading.value = false
    void persistTurn(userMsg, assistant)
  }
}

async function sendChatMessage(text: string, assistantId: string, userMsg: UiMessage) {
  const history: ChatMessage[] = messages.value
    .filter((m) => m.id !== 'welcome' && m.id !== assistantId && !m.taskMode)
    .map(({ role, content }) => ({ role, content }))
  history.push({ role: 'user', content: text })

  const assistant = messages.value.find((m) => m.id === assistantId)
  if (!assistant) return

  abortRef.value?.abort()
  abortRef.value = new AbortController()

  try {
    await streamChat(
      history.filter((m) => m.content),
      {
        onEvent(ev) {
          handleStreamEvent(assistant, ev)
          scrollToBottom()
        },
        onError(msg) {
          ElMessage.error(msg)
          if (!assistant.content) {
            assistant.content = '抱歉，请求失败了。'
          }
        },
      },
      abortRef.value.signal,
    )
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') return
    const msg = e instanceof Error ? e.message : '发送失败'
    ElMessage.error(msg)
    if (!assistant.content) {
      assistant.content = '抱歉，暂时无法连接模型服务。'
    }
  } finally {
    assistant.streaming = false
    assistant.stepsVisible = false
    assistant.replayActive = false
    assistant.replayVisibleCount = assistant.steps?.length ?? 0
    loading.value = false
    if (assistant.content || assistant.steps?.length) {
      await persistTurn(userMsg, assistant)
    }
    scrollToBottom()
  }
}

function setChatMode(mode: ChatMode) {
  if (loading.value || chatMode.value === mode) return
  chatMode.value = mode
}

function toggleAnalysisDimension(dim: string) {
  const idx = analysisDimensions.value.indexOf(dim)
  if (idx >= 0) {
    if (analysisDimensions.value.length <= 1) {
      ElMessage.warning('请至少保留一个分析维度')
      return
    }
    analysisDimensions.value = analysisDimensions.value.filter((d) => d !== dim)
    return
  }
  analysisDimensions.value = [...analysisDimensions.value, dim]
}

function formatAnalysisUserMessage(payload: CreateTaskPayload, note: string) {
  const lines = [
    `【竞品分析】${payload.competitors.join('、')}`,
    `维度：${payload.dimensions.join('、')}`,
  ]
  if (note) lines.push(note)
  return lines.join('\n')
}

async function send() {
  if (loading.value) return

  if (chatMode.value === 'analysis') {
    const note = input.value.trim()
    const payload = buildAnalysisPayload(
      analysisCompetitors.value,
      analysisDimensions.value,
      note || undefined,
    )
    if (!payload) {
      ElMessage.warning('请至少输入 2 个竞品名称（逗号、顿号或换行分隔）')
      return
    }

    input.value = ''
    const userMsg: UiMessage = {
      id: uid(),
      role: 'user',
      content: formatAnalysisUserMessage(payload, note),
    }
    messages.value.push(userMsg)
    await sendAnalysisTask(payload, userMsg)
    return
  }

  const text = input.value.trim()
  if (!text) return

  input.value = ''
  const userMsg: UiMessage = { id: uid(), role: 'user', content: text }
  messages.value.push(userMsg)

  const assistantId = uid()
  messages.value.push({
    id: assistantId,
    role: 'assistant',
    content: '',
    steps: [],
    stepsVisible: true,
    streaming: true,
    stepFilter: 'all',
    replaySpeed: 1,
  })
  loading.value = true
  scrollToBottom()
  await sendChatMessage(text, assistantId, userMsg)
}

function stop() {
  abortRef.value?.abort()
  closeTaskStream()
  clearAllReplays()
  loading.value = false
  const last = messages.value.at(-1)
  if (last?.streaming) last.streaming = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function toggleStep(step: ChatStep) {
  step.expanded = !step.expanded
}

onMounted(async () => {
  try {
    const loaded = await initConversations()
    messages.value = loaded as UiMessage[]
  } catch {
    messages.value = [fromMessageRecord(welcomeMessage()) as UiMessage]
    ElMessage.warning('无法加载历史对话，请确认已登录')
  }
  scrollToBottom()
})
onUnmounted(() => {
  closeTaskStream()
  clearAllReplays()
})
</script>

<template>
  <div class="chat-page">
    <aside
      class="unified-sidebar"
      :class="{ 'unified-sidebar--collapsed': sidebarPane.collapsed.value }"
      :style="{ width: `${sidebarPane.effectiveSize.value}px` }"
    >
      <div class="sidebar-head">
        <div class="sidebar-brand" @click="go('/')">
          <div class="sidebar-brand-mark">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/>
              <path d="M2 17l10 5 10-5"/>
              <path d="M2 12l10 5 10-5"/>
            </svg>
          </div>
          <div v-show="!sidebarPane.collapsed.value" class="sidebar-brand-text">
            <div class="sidebar-brand-name">CompeteAI</div>
            <div class="sidebar-brand-sub">agent workspace</div>
          </div>
          <button
            v-show="!sidebarPane.collapsed.value"
            type="button"
            class="pane-collapse-btn sidebar-collapse"
            title="收起侧边栏"
            @click.stop="sidebarPane.toggleCollapse()"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M15 18l-6-6 6-6"/>
            </svg>
          </button>
        </div>
        <button
          v-if="sidebarPane.collapsed.value"
          type="button"
          class="rail-btn rail-btn--ghost"
          title="展开侧边栏"
          @click="sidebarPane.toggleCollapse()"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <path d="M9 18l6-6-6-6"/>
          </svg>
        </button>
      </div>

      <nav class="sidebar-nav">
        <button type="button" class="sidebar-nav-item" title="任务管理" @click="go('/')">
          <svg class="sidebar-nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/>
            <rect x="9" y="3" width="6" height="4" rx="1"/>
            <path d="M9 12h6M9 16h4"/>
          </svg>
          <span v-show="!sidebarPane.collapsed.value" class="sidebar-nav-label">任务管理</span>
        </button>
        <button type="button" class="sidebar-nav-item" title="Agent 能力" @click="go('/agents')">
          <svg class="sidebar-nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2a4 4 0 0 1 4 4c0 1.5-.8 2.8-2 3.4V12h1a7 7 0 0 1 7 7v1H3v-1a7 7 0 0 1 7-7h1V9.4A4 4 0 0 1 12 2z"/>
            <path d="M9 20v1a3 3 0 0 0 6 0v-1"/>
          </svg>
          <span v-show="!sidebarPane.collapsed.value" class="sidebar-nav-label">Agent 能力</span>
        </button>
        <button type="button" class="sidebar-nav-item active" title="AI 对话">
          <svg class="sidebar-nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 3c-4 0-7 2.5-7 6 0 2.2 1.2 4.1 3 5.2V19l4-2 4 2v-4.8c1.8-1.1 3-3 3-5.2 0-3.5-3-6-7-6z"/>
            <path d="M9.5 10.5h.01M14.5 10.5h.01"/>
          </svg>
          <span v-show="!sidebarPane.collapsed.value" class="sidebar-nav-label">AI 对话</span>
        </button>
      </nav>

      <template v-if="!sidebarPane.collapsed.value">
      <div class="sidebar-divider" />

      <div class="conv-sidebar-head">
        <span class="conv-sidebar-label">历史对话</span>
      </div>
      <div class="conv-sidebar-actions">
        <button type="button" class="btn-new" @click="startNewConversation">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          新建对话
        </button>
      </div>
      </template>
      <button
        v-else
        type="button"
        class="rail-btn rail-btn--primary"
        title="新建对话"
        @click="startNewConversation"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <line x1="12" y1="5" x2="12" y2="19"/>
          <line x1="5" y1="12" x2="19" y2="12"/>
        </svg>
      </button>
      <div v-if="sidebarPane.collapsed.value" class="sidebar-rail-spacer" />
      <div
        v-show="!sidebarPane.collapsed.value"
        v-loading="conversationsLoading || conversationSwitching"
        class="conv-list"
      >
        <button
          v-for="conv in conversations"
          :key="conv.id"
          type="button"
          class="conv-item"
          :class="{ active: conv.id === activeConversationId }"
          @click="loadConversation(conv.id)"
        >
          <span class="conv-title">{{ conv.title || '新对话' }}</span>
          <span class="conv-preview">{{ conv.preview || '暂无消息' }}</span>
          <span class="conv-meta">
            <span class="conv-time">{{ formatConvTime(conv.updatedAt) }}</span>
            <span
              class="conv-delete"
              title="删除"
              @click.stop="deleteConversationItem(conv.id)"
            >×</span>
          </span>
        </button>
        <p v-if="!conversations.length && !conversationsLoading" class="conv-empty">
          暂无历史对话
        </p>
      </div>

      <div class="sidebar-foot">
        <span class="status-dot" :class="{ on: auth.isLoggedIn }" />
        <span v-show="!sidebarPane.collapsed.value" class="status-text">{{ auth.isLoggedIn ? 'session active' : 'offline' }}</span>
        <span v-if="useMock && !sidebarPane.collapsed.value" class="mock-chip">mock</span>
      </div>
    </aside>

    <div
      v-if="!sidebarPane.collapsed.value"
      class="pane-resize-handle pane-resize-handle--col"
      title="拖动调节侧边栏宽度，双击恢复默认"
      @mousedown="(e) => sidebarPane.startResize(e, 'col')"
      @dblclick="sidebarPane.reset()"
    />

    <div class="chat-main">
    <div ref="listRef" class="chat-list">
      <div
        v-for="msg in messages"
        :key="msg.id"
        class="msg-row"
        :class="msg.role"
      >
        <div class="avatar">
          <template v-if="msg.role === 'user'">U</template>
          <template v-else>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/>
              <path d="M2 17l10 5 10-5"/>
              <path d="M2 12l10 5 10-5"/>
            </svg>
          </template>
        </div>
        <div class="bubble">
          <div
            v-if="msg.role === 'assistant' && (msg.content || msg.streaming)"
            class="bubble-text answer-card"
          >
            <div
              v-if="msg.content"
              class="md-body"
              v-html="renderMarkdown(msg.content)"
            />
            <span v-if="msg.streaming && !msg.content" class="cursor">▍</span>
            <span v-else-if="msg.streaming" class="cursor">▍</span>
          </div>
          <p v-else-if="msg.role === 'assistant' && msg.streaming && !msg.steps?.length" class="bubble-text muted">
            正在思考<span class="cursor">▍</span>
          </p>
          <p v-else-if="msg.role === 'user' && msg.content" class="bubble-text">
            {{ msg.content }}
          </p>

          <div
            v-if="msg.role === 'assistant' && msg.steps?.length"
            class="process-panel"
            :class="{ open: msg.stepsVisible, streaming: msg.streaming }"
          >
            <button type="button" class="process-head" @click="toggleSteps(msg)">
              <span class="process-icon">{{ msg.streaming ? '⏳' : '⚙️' }}</span>
              <span class="process-title">
                {{ msg.streaming ? '执行中' : '执行过程' }}
                <span class="process-count">
                  {{ stepCount(msg) }} / {{ filteredSteps(msg).length }} 步
                </span>
              </span>
              <span class="process-stats">{{ processStats(msg) }}</span>
              <span class="process-toggle">{{ msg.stepsVisible ? '收起详情' : '展开详情' }}</span>
            </button>
            <div v-if="msg.taskMode" class="process-toolbar">
              <div class="agent-rail">
                <button
                  type="button"
                  class="agent-chip"
                  :class="{ active: !hasStepFilter(msg) }"
                  @click.stop="setStepFilter(msg, 'all')"
                >
                  <span class="agent-chip-label">全部</span>
                  <span class="agent-chip-count">{{ msg.steps?.length ?? 0 }}</span>
                </button>
                <button
                  v-for="name in AGENT_ORDER"
                  :key="name"
                  type="button"
                  class="agent-chip"
                  :class="[taskAgentStatus(msg, name), { active: msg.stepFilter === name }]"
                  @click.stop="setStepFilter(msg, name)"
                >
                  <AgentIcon :name="name" :size="18" />
                  <span class="agent-chip-label">{{ AGENT_LABELS[name] }}</span>
                  <span class="agent-chip-metrics">{{ agentMetricsSummary(msg, name) }}</span>
                  <StatusIcon :status="taskAgentStatus(msg, name)" :size="12" />
                  <span class="agent-chip-count">{{ agentToolCount(msg, name) }}</span>
                </button>
              </div>
              <div v-if="canReplayProcess(msg)" class="replay-toolbar">
                <button type="button" class="btn-ghost btn-xs" @click.stop="toggleProcessReplay(msg)">
                  {{ msg.replayPlaying ? 'pause' : 'replay' }}
                </button>
                <button type="button" class="btn-ghost btn-xs" @click.stop="stepProcessReplay(msg)">step</button>
                <button type="button" class="btn-ghost btn-xs" @click.stop="resetProcessReplay(msg)">reset</button>
                <button type="button" class="btn-ghost btn-xs" @click.stop="showFullProcess(msg)">all</button>
                <el-radio-group
                  class="replay-speed"
                  :model-value="msg.replaySpeed ?? 1"
                  size="small"
                  @update:model-value="(value: string | number) => updateReplaySpeed(msg, Number(value))"
                >
                  <el-radio-button :value="1">1x</el-radio-button>
                  <el-radio-button :value="2">2x</el-radio-button>
                  <el-radio-button :value="4">4x</el-radio-button>
                </el-radio-group>
              </div>
            </div>
            <ul v-if="!msg.stepsVisible" class="process-preview">
              <li
                v-for="(step, index) in visibleSteps(msg)"
                :key="step.id"
                class="process-preview-item"
              >
                <span class="preview-index">{{ index + 1 }}</span>
                <span class="preview-label">{{ chatStepLabel(step) }}</span>
                <span class="preview-text">{{ stepSummary(step, 80) }}</span>
              </li>
            </ul>
            <div v-show="msg.stepsVisible" class="process-body">
              <div
                v-for="step in visibleSteps(msg)"
                :key="step.id"
                class="step-row"
                :class="[step.type, step.status, step.thinkingKind, { open: step.expanded || (step.type === 'agent' && step.agentStatus === 'running') || (step.type === 'thinking' && step.thinkingKind === 'thinking' && step.expanded) }]"
              >
                <button type="button" class="step-row-head" @click="toggleStep(step)">
                  <span class="step-dot" />
                  <span class="step-row-label">
                    {{ chatStepLabel(step) }}
                  </span>
                  <span class="step-row-summary">{{ stepSummary(step) }}</span>
                  <span v-if="step.type === 'tool'" class="step-row-badge" :class="step.status">
                    {{ step.status === 'running' ? '…' : step.status === 'error' ? '!' : '✓' }}
                  </span>
                  <span v-else-if="step.type === 'agent'" class="step-row-badge" :class="step.agentStatus">
                    {{ agentStatusIcon(step.agentStatus ?? 'pending') }}
                  </span>
                  <span v-else-if="step.type === 'event'" class="step-row-badge" :class="step.eventKind">
                    {{ step.eventKind === 'clarification' ? '?' : '↩' }}
                  </span>
                </button>
                <div v-show="step.expanded || (step.type === 'agent' && step.agentStatus === 'running') || (step.type === 'thinking' && (step.thinkingKind === 'thinking' || step.thinkingKind === 'path' || step.thinkingKind === 'output' || step.thinkingKind === 'analysis'))" class="step-row-detail">
                  <template v-if="step.type === 'thinking'">
                    <div
                      v-if="step.thinkingKind === 'analysis'"
                      class="step-md md-body"
                      v-html="renderMarkdown(step.content)"
                    />
                    <pre v-else-if="step.thinkingKind === 'output'" class="step-text output">{{ step.content || '生成中…' }}</pre>
                    <pre v-else class="step-text" :class="step.thinkingKind">{{ step.content || '思考中…' }}</pre>
                  </template>
                  <template v-else-if="step.type === 'agent'">
                    <pre v-if="step.content" class="step-text">{{ step.content }}</pre>
                    <p v-else-if="step.agentStatus === 'running'" class="step-pending">等待子步骤输出…</p>
                    <p v-if="step.metricsLabel" class="agent-metrics">{{ step.metricsLabel }}</p>
                  </template>
                  <template v-else-if="step.type === 'event'">
                    <div class="event-card" :class="step.eventKind">
                      <p class="event-title">
                        {{ step.eventKind === 'clarification' ? '等待澄清' : 'QA 打回重做' }}
                      </p>
                      <p v-if="step.eventKind === 'clarification' && step.question" class="event-text">
                        {{ step.question }}
                      </p>
                      <p v-else class="event-text">
                        {{ step.content }}
                      </p>
                      <p v-if="step.targetAgent" class="event-meta">
                        目标 Agent：{{ AGENT_LABELS[step.targetAgent] }}
                      </p>
                      <p v-if="step.parentAgent" class="event-meta">
                        来源 Agent：{{ AGENT_LABELS[step.parentAgent] }}
                      </p>
                    </div>
                  </template>
                  <template v-else>
                    <div v-if="step.toolArgs" class="step-block">
                      <p class="step-label">参数</p>
                      <pre class="step-text args">{{ step.toolArgs }}</pre>
                    </div>
                    <div v-if="step.toolResult" class="step-block">
                      <p class="step-label">结果</p>
                      <div
                        class="step-md md-body"
                        v-html="renderMarkdown(step.toolResult)"
                      />
                    </div>
                    <p v-else-if="step.status === 'running'" class="step-pending">正在调用…</p>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!messages.length" class="empty-chat">
        <p>开始一段新的对话</p>
      </div>
    </div>

    <div
      v-if="!footerPane.collapsed.value"
      class="pane-resize-handle pane-resize-handle--row"
      title="拖动调节输入区高度，双击恢复默认"
      @mousedown="(e) => footerPane.startResize(e, 'row')"
      @dblclick="footerPane.reset()"
    />

    <footer
      class="chat-footer"
      :class="{ 'chat-footer--collapsed': footerPane.collapsed.value }"
      :style="{ height: `${footerPane.effectiveSize.value}px` }"
    >
      <div class="chat-footer-toolbar">
        <button
          type="button"
          class="pane-collapse-btn"
          :title="footerPane.collapsed.value ? '展开输入区' : '收起输入区'"
          @click="footerPane.toggleCollapse()"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path v-if="footerPane.collapsed.value" d="M6 9l6 6 6-6"/>
            <path v-else d="M6 15l6-6 6 6"/>
          </svg>
        </button>
        <span v-if="!footerPane.collapsed.value" class="footer-resize-hint">拖动上方横条可调节高度</span>
      </div>
      <div class="chat-footer-inner">
      <div v-show="!footerPane.collapsed.value" class="chat-mode-bar">
        <div class="mode-switch">
          <button
            type="button"
            class="mode-btn"
            :class="{ active: chatMode === 'chat' }"
            @click="setChatMode('chat')"
          >
            普通对话
          </button>
          <button
            type="button"
            class="mode-btn"
            :class="{ active: chatMode === 'analysis' }"
            @click="setChatMode('analysis')"
          >
            竞品分析
          </button>
        </div>
        <p v-if="chatMode === 'analysis'" class="mode-tip">
          需填写至少 2 个竞品，支持同时分析多个产品
        </p>
      </div>

      <div v-if="chatMode === 'analysis' && !footerPane.collapsed.value" class="analysis-panel">
        <label class="analysis-label" for="analysis-competitors">竞品名称</label>
        <input
          id="analysis-competitors"
          v-model="analysisCompetitors"
          class="analysis-competitors-input"
          type="text"
          placeholder="例如：王老吉, 加多宝, 和其正（逗号 / 顿号 / 换行分隔）"
          :disabled="loading"
        />
        <div v-if="parsedCompetitors.length" class="competitor-tags">
          <span
            v-for="name in parsedCompetitors"
            :key="name"
            class="competitor-tag"
          >{{ name }}</span>
          <span class="competitor-count" :class="{ ok: canSendAnalysis }">
            {{ parsedCompetitors.length }} 个竞品
          </span>
        </div>
        <div class="dimension-row">
          <span class="analysis-label">分析维度</span>
          <div class="dimension-chips">
            <button
              v-for="dim in ANALYSIS_DIMENSIONS"
              :key="dim"
              type="button"
              class="dim-chip"
              :class="{ active: analysisDimensions.includes(dim) }"
              :disabled="loading"
              @click="toggleAnalysisDimension(dim)"
            >
              {{ dim }}
            </button>
          </div>
        </div>
      </div>

      <div class="chat-input-box" :class="{ 'chat-input-box--analysis': chatMode === 'analysis' }">
        <textarea
          v-model="input"
          class="chat-input"
          rows="1"
          :placeholder="chatMode === 'analysis'
            ? '补充说明（可选），Enter 开始分析，Shift+Enter 换行'
            : '输入消息，Enter 发送，Shift+Enter 换行'"
          :disabled="loading"
          @keydown="onKeydown"
        />
        <div class="chat-actions">
          <button
            v-if="loading"
            type="button"
            class="btn-stop"
            @click="stop"
          >
            停止
          </button>
          <button
            v-else
            type="button"
            class="btn-send"
            :class="{ 'btn-send--analysis': chatMode === 'analysis' }"
            :disabled="chatMode === 'analysis' ? !canSendAnalysis : !input.trim()"
            @click="send"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="19" x2="12" y2="5"/>
              <polyline points="5 12 12 5 19 12"/>
            </svg>
          </button>
        </div>
      </div>
      <p v-if="!footerPane.collapsed.value" class="chat-hint">
        <template v-if="chatMode === 'analysis'">
          竞品分析将启动多 Agent 流水线（Coordinator → Collector → Analyst → Writer → QA）
        </template>
        <template v-else>
          普通对话由大模型直接回答，不会自动进入竞品分析流水线
        </template>
      </p>
      </div>
    </footer>
    </div>
  </div>
</template>

<style scoped>
.chat-page {
  display: flex;
  flex-direction: row;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--bg-base);
}

.unified-sidebar {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border-subtle);
  background: var(--bg-panel);
  min-height: 0;
  transition: width 0.2s ease;
  overflow: hidden;
}

.unified-sidebar--collapsed {
  background: linear-gradient(180deg, #fafafb 0%, #f4f4f6 100%);
}

.unified-sidebar--collapsed .sidebar-head {
  align-items: center;
  padding-bottom: 10px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--border-subtle);
}

.unified-sidebar--collapsed .sidebar-brand {
  justify-content: center;
  padding: 12px 0 0;
  width: 100%;
}

.unified-sidebar--collapsed .sidebar-nav {
  align-items: center;
  padding: 10px 0;
  gap: 6px;
}

.unified-sidebar--collapsed .sidebar-nav-item {
  width: 40px;
  height: 40px;
  justify-content: center;
  padding: 0;
  border-radius: 11px;
}

.unified-sidebar--collapsed .sidebar-nav-item:hover {
  background: rgba(99, 102, 241, 0.08);
}

.unified-sidebar--collapsed .sidebar-nav-item.active {
  background: var(--accent-dim);
  box-shadow: inset 0 0 0 1px rgba(99, 102, 241, 0.22);
}

.unified-sidebar--collapsed .sidebar-nav-icon {
  opacity: 0.5;
}

.unified-sidebar--collapsed .sidebar-nav-item:hover .sidebar-nav-icon,
.unified-sidebar--collapsed .sidebar-nav-item.active .sidebar-nav-icon {
  opacity: 1;
  color: var(--accent);
}

.unified-sidebar--collapsed .sidebar-foot {
  justify-content: center;
  padding: 14px 0 16px;
  border-top: 1px solid var(--border-subtle);
  margin-top: auto;
}

.unified-sidebar--collapsed .status-dot.on {
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15);
}

.sidebar-head {
  flex-shrink: 0;
}

.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 14px 12px;
  cursor: pointer;
  transition: opacity 0.15s;
}

.sidebar-brand-text {
  flex: 1;
  min-width: 0;
}

.sidebar-collapse {
  margin-left: auto;
  flex-shrink: 0;
}

.sidebar-rail-spacer {
  flex: 1;
  min-height: 8px;
}

.rail-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 36px;
  margin: 0 auto;
  border: none;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
  color: var(--text-muted);
}

.rail-btn--ghost {
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
}

.rail-btn--ghost:hover {
  color: var(--accent);
  border-color: rgba(99, 102, 241, 0.35);
  background: var(--accent-dim);
}

.rail-btn--primary {
  margin-top: 4px;
  background: linear-gradient(135deg, var(--accent), #7c3aed);
  color: #fff;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.28);
}

.rail-btn--primary:hover {
  filter: brightness(1.06);
  transform: translateY(-1px);
}

.sidebar-brand:hover {
  opacity: 0.85;
}

.sidebar-brand-mark {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: linear-gradient(135deg, var(--accent), #7c3aed);
  color: #fff;
  flex-shrink: 0;
}

.sidebar-brand-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
}

.sidebar-brand-sub {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 0 12px 8px;
}

.sidebar-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-family: var(--font-sans);
  transition: all 0.15s;
}

.sidebar-nav-icon {
  flex-shrink: 0;
  opacity: 0.45;
  transition: opacity 0.15s, color 0.15s;
}

.sidebar-nav-item:hover .sidebar-nav-icon {
  opacity: 0.75;
}

.sidebar-nav-item.active .sidebar-nav-icon {
  opacity: 1;
  color: var(--accent);
}

.sidebar-nav-label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sidebar-nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.sidebar-nav-item.active {
  background: var(--accent-dim);
  color: var(--accent);
  font-weight: 600;
}

.sidebar-divider {
  height: 1px;
  margin: 4px 16px 8px;
  background: var(--border-subtle);
}

.sidebar-foot {
  margin-top: auto;
  padding: 14px 16px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  flex-shrink: 0;
}

.status-dot.on {
  background: var(--success);
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.4);
}

.status-text {
  flex: 1;
}

.mock-chip {
  font-size: 9px;
  padding: 1px 6px;
  border-radius: 999px;
  border: 1px solid var(--border-default);
}

.conv-sidebar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 14px 14px 10px;
}

.conv-sidebar-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.conv-sidebar-actions {
  padding: 0 12px 12px;
}

.btn-new {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 9px 12px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  background: var(--bg-base);
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.btn-new:hover {
  background: var(--bg-hover);
  border-color: var(--accent);
  color: var(--accent);
}


.btn-icon {
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  padding: 4px 8px;
}

.conv-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
}

.conv-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  width: 100%;
  padding: 10px 12px;
  margin-bottom: 4px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s;
}

.conv-item:hover {
  background: var(--bg-hover);
}

.conv-item.active {
  background: var(--bg-hover);
  border-color: var(--el-color-primary-light-5);
}

.conv-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.conv-preview {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.conv-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin-top: 6px;
}

.conv-time {
  font-size: 11px;
  color: var(--text-muted);
}

.conv-delete {
  font-size: 16px;
  color: var(--text-muted);
  opacity: 0;
  padding: 0 4px;
  line-height: 1;
}

.conv-item:hover .conv-delete {
  opacity: 1;
}

.conv-delete:hover {
  color: var(--el-color-danger);
}

.conv-empty {
  font-size: 12px;
  color: var(--text-muted);
  text-align: center;
  padding: 24px 8px;
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.chat-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 24px 24px 12px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.msg-row {
  display: flex;
  gap: 12px;
  max-width: min(1000px, calc(100% - 16px));
  width: 100%;
  margin: 0 auto;
}

.msg-row.user {
  flex-direction: row-reverse;
}

.avatar {
  flex-shrink: 0;
  width: 34px;
  height: 34px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  font-family: var(--font-mono);
  border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  color: var(--text-muted);
}

.msg-row.user .avatar {
  background: linear-gradient(135deg, var(--accent), #7c3aed);
  color: #fff;
  border: none;
}

.msg-row.assistant .avatar {
  background: var(--bg-panel);
  border-color: var(--border-default);
  color: var(--accent-light);
}

.bubble {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.msg-row.assistant .bubble {
  max-width: 100%;
}

.msg-row.user .bubble {
  max-width: 78%;
}

.msg-row.user .bubble {
  align-items: flex-end;
}

.process-panel:not(.open) {
  opacity: 1;
}

.process-panel:not(.open) .process-head {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-subtle);
}

.process-preview {
  list-style: none;
  margin: 0;
  padding: 8px 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-top: 1px solid var(--border-subtle);
}

.process-preview-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
}

.preview-index {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 10px;
  font-weight: 600;
  font-family: var(--font-mono);
  background: var(--bg-elevated);
  color: var(--text-muted);
}

.preview-label {
  flex-shrink: 0;
  min-width: 56px;
  font-weight: 600;
  color: var(--text-primary);
}

.preview-text {
  flex: 1;
  min-width: 0;
  color: var(--text-muted);
  word-break: break-word;
}

.process-panel {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: var(--bg-panel);
  overflow: hidden;
}

.process-panel.streaming {
  border-color: rgba(245, 158, 11, 0.3);
}

.process-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  color: var(--text-secondary);
  font-size: 12px;
}

.process-icon {
  font-size: 13px;
  line-height: 1;
  flex-shrink: 0;
}

.process-title {
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
}

.process-count {
  margin-left: 4px;
  font-weight: 400;
  color: var(--text-muted);
}

.process-stats {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 11px;
}

.process-toggle {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--accent-light);
}

.process-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px 10px;
  border-top: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.65);
}

.agent-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.agent-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}

.agent-chip:hover {
  border-color: var(--border-default);
  color: var(--text-primary);
}

.agent-chip.active {
  border-color: rgba(99, 102, 241, 0.28);
  background: rgba(99, 102, 241, 0.08);
  color: var(--accent-light);
}

.agent-chip.running {
  border-color: rgba(245, 158, 11, 0.25);
}

.agent-chip.completed {
  border-color: rgba(34, 197, 94, 0.22);
}

.agent-chip.failed,
.agent-chip.rejected {
  border-color: rgba(239, 68, 68, 0.22);
}

.agent-chip-label {
  font-family: var(--font-mono);
}

.agent-chip-metrics {
  color: var(--text-muted);
  font-size: 10px;
  font-family: var(--font-mono);
}

.agent-chip-count {
  min-width: 16px;
  padding: 0 4px;
  border-radius: 999px;
  background: var(--bg-base);
  color: var(--text-muted);
  text-align: center;
  font-size: 10px;
}

.replay-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.btn-xs {
  padding: 4px 8px;
  font-size: 11px;
}

.replay-speed {
  --el-border-radius-base: 8px;
}

.process-body {
  padding: 4px 12px 10px;
  border-top: 1px solid var(--border-subtle);
}

.step-row {
  position: relative;
}

.step-row + .step-row {
  margin-top: 2px;
}

.step-row-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 4px 4px 0;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
  color: var(--text-secondary);
  font-size: 11px;
}

.step-row-head:hover {
  color: var(--text-primary);
}

.step-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--border-default);
}

.step-row.thinking .step-dot {
  background: rgba(99, 102, 241, 0.5);
}

.step-row.tool.running .step-dot {
  background: #d97706;
  animation: pulse 1.2s ease-in-out infinite;
}

.step-row.tool.done .step-dot {
  background: #16a34a;
}

.step-row.tool.error .step-dot {
  background: var(--danger);
}

.step-row-label {
  flex-shrink: 0;
  font-weight: 600;
  font-family: var(--font-mono);
  min-width: 44px;
}

.step-row-summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
}

.step-row-badge {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 10px;
  font-weight: 700;
}

.step-row-badge.running {
  color: #d97706;
  background: rgba(245, 158, 11, 0.12);
}

.step-row-badge.done {
  color: #16a34a;
  background: rgba(34, 197, 94, 0.12);
}

.step-row-badge.error {
  color: var(--danger);
  background: var(--danger-dim);
}

.step-row-badge.completed {
  color: #16a34a;
  background: rgba(34, 197, 94, 0.12);
}

.step-row-badge.pending {
  color: var(--text-muted);
  background: var(--bg-base);
}

.step-row-badge.failed,
.step-row-badge.rejected {
  color: var(--danger);
  background: var(--danger-dim);
}

.step-row.agent.running .step-dot {
  background: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-dim);
}

.step-row.thinking.path .step-dot {
  background: #6366f1;
}

.step-text.path {
  color: var(--accent-light);
  font-family: var(--font-mono);
  font-size: 12px;
}

.step-text.output {
  max-height: 280px;
  overflow: auto;
  font-size: 11px;
  color: var(--text-muted);
  white-space: pre-wrap;
  word-break: break-all;
}

.step-row.thinking.analysis .step-dot {
  background: var(--success);
}

.step-row.thinking .step-row-detail,
.step-row.agent.running .step-row-detail {
  display: block;
}

.step-row-detail {
  margin: 2px 0 6px 12px;
  padding-left: 10px;
  border-left: 2px solid var(--border-subtle);
}

@keyframes pulse {
  50% { opacity: 0.4; }
}

.step-block + .step-block {
  margin-top: 8px;
}

.step-label {
  margin: 0 0 4px;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.step-text {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  max-height: 240px;
  overflow: auto;
}

.step-text.result {
  max-height: 320px;
}

.step-pending {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.agent-metrics {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.event-card {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
}

.event-card.clarification {
  border-color: rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.06);
}

.event-card.rejection {
  border-color: rgba(239, 68, 68, 0.28);
  background: rgba(239, 68, 68, 0.06);
}

.event-title {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.event-text {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.event-meta {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--text-muted);
}

.step-row-badge.clarification {
  color: #d97706;
  background: rgba(245, 158, 11, 0.12);
}

.step-row-badge.rejection {
  color: var(--danger);
  background: var(--danger-dim);
}

.bubble-text {
  margin: 0;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  font-size: 14px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
}

.bubble-text.muted {
  color: var(--text-muted);
}

.answer-card {
  white-space: normal;
}

.answer-card .md-body:empty {
  display: none;
}

.step-md {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  max-height: 320px;
  overflow: auto;
}

.step-text.args {
  max-height: 120px;
}

.md-body {
  font-size: 14px;
  line-height: 1.65;
  color: var(--text-primary);
}

.md-body :first-child {
  margin-top: 0;
}

.md-body :last-child {
  margin-bottom: 0;
}

.md-body p,
.md-body ul,
.md-body ol,
.md-body pre,
.md-body blockquote {
  margin: 0.5em 0;
}

.md-body h1,
.md-body h2,
.md-body h3,
.md-body h4 {
  margin: 0.8em 0 0.4em;
  line-height: 1.35;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.md-body h1 {
  font-size: 1.35em;
  padding-bottom: 0.35em;
  border-bottom: 1px solid var(--border-subtle);
}

.md-body h2 {
  font-size: 1.08em;
  color: var(--text-primary);
  margin-top: 1.1em;
  padding-top: 0.25em;
}

.md-body h3 {
  font-size: 1em;
  color: var(--text-primary);
}

.md-body h2 + p,
.md-body h3 + p {
  margin-top: 0.45em;
}

.md-body > blockquote {
  margin: 0.75em 0 1em;
  padding: 10px 14px;
  border-left: 3px solid var(--accent);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  background: var(--accent-dim);
  color: var(--text-secondary);
  font-size: 13px;
}

.md-body ul,
.md-body ol {
  padding-left: 1.4em;
}

.md-body li + li {
  margin-top: 0.25em;
}

.md-body a {
  color: var(--accent-light);
  text-decoration: none;
}

.md-body a:hover {
  text-decoration: underline;
}

.md-body code {
  padding: 0.15em 0.35em;
  border-radius: 4px;
  font-size: 0.9em;
  font-family: var(--font-mono);
  background: rgba(99, 102, 241, 0.12);
}

.md-body pre {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
  overflow: auto;
}

.md-body pre code {
  padding: 0;
  background: none;
}

.md-body blockquote {
  padding-left: 12px;
  border-left: 3px solid var(--border-subtle);
  color: var(--text-secondary);
}

.msg-row.user .bubble-text {
  background: rgba(99, 102, 241, 0.1);
  border-color: rgba(99, 102, 241, 0.2);
}

.cursor {
  display: inline-block;
  animation: blink 1s step-end infinite;
  color: var(--accent-light);
}

@keyframes blink {
  50% { opacity: 0; }
}

.empty-chat {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 14px;
}

.chat-footer {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 88px;
  border-top: 1px solid var(--border-subtle);
  background: rgba(247, 247, 248, 0.96);
  backdrop-filter: blur(12px);
  box-shadow: 0 -8px 24px rgba(0, 0, 0, 0.04);
}

.chat-footer--collapsed .chat-footer-inner {
  padding-top: 0;
}

.chat-footer-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 16px 0;
  flex-shrink: 0;
}

.footer-resize-hint {
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.chat-footer-inner {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 24px 12px;
}

.chat-mode-bar {
  max-width: 1000px;
  margin: 0 auto 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mode-switch {
  display: inline-flex;
  padding: 3px;
  border-radius: var(--radius-sm);
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
}

.mode-btn {
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  padding: 7px 14px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
  font-family: var(--font-sans);
}

.mode-btn:hover {
  color: var(--text-primary);
}

.mode-btn.active {
  background: var(--bg-base);
  color: var(--text-primary);
  box-shadow: var(--shadow-card);
}

.mode-btn.active:last-child {
  color: var(--accent);
}

.mode-tip {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.analysis-panel {
  max-width: 1000px;
  margin: 0 auto 8px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.analysis-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.analysis-competitors-input {
  width: 100%;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  font: inherit;
  font-size: 14px;
  color: var(--text-primary);
  background: var(--bg-base);
  outline: none;
  transition: border-color 0.15s;
}

.analysis-competitors-input:focus {
  border-color: var(--border-active);
}

.analysis-competitors-input:disabled {
  opacity: 0.6;
}

.competitor-tags {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.competitor-tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--accent);
  background: var(--accent-dim);
  border: 1px solid rgba(99, 102, 241, 0.15);
}

.competitor-count {
  font-size: 11px;
  color: var(--danger);
  font-family: var(--font-mono);
}

.competitor-count.ok {
  color: var(--success);
}

.dimension-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dimension-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.dim-chip {
  border: 1px solid var(--border-subtle);
  background: var(--bg-base);
  color: var(--text-secondary);
  font-size: 12px;
  padding: 5px 10px;
  border-radius: 999px;
  cursor: pointer;
  transition: all 0.15s;
  font-family: var(--font-sans);
}

.dim-chip:hover {
  border-color: var(--border-hover);
  color: var(--text-primary);
}

.dim-chip.active {
  color: var(--accent);
  border-color: rgba(99, 102, 241, 0.35);
  background: var(--accent-dim);
}

.dim-chip:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chat-input-box--analysis:focus-within {
  border-color: rgba(99, 102, 241, 0.45);
}

.btn-send--analysis:not(:disabled) {
  background: linear-gradient(135deg, #6366f1, #7c3aed);
}

.chat-input-box {
  max-width: 1000px;
  margin: 0 auto;
  display: flex;
  gap: 10px;
  align-items: flex-end;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  transition: border-color 0.15s;
}

.chat-input-box:focus-within {
  border-color: var(--border-active);
}

.chat-input {
  flex: 1;
  border: none;
  outline: none;
  resize: none;
  font: inherit;
  font-size: 14px;
  line-height: 1.5;
  min-height: 24px;
  max-height: 160px;
  background: transparent;
  color: var(--text-primary);
}

.chat-input::placeholder {
  color: var(--text-muted);
}

.chat-input:disabled {
  opacity: 0.5;
}

.chat-actions {
  flex-shrink: 0;
}

.btn-send {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-sm);
  background: var(--accent);
  color: #fff;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-send:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.btn-send:not(:disabled):hover {
  filter: brightness(1.1);
}

.btn-stop {
  padding: 7px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid rgba(239, 68, 68, 0.3);
  background: var(--danger-dim);
  color: var(--danger);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-stop:hover {
  background: rgba(239, 68, 68, 0.2);
}

.chat-hint {
  max-width: 1000px;
  margin: 6px auto 0;
  font-size: 10px;
  color: var(--text-muted);
  text-align: center;
  font-family: var(--font-mono);
}
</style>
