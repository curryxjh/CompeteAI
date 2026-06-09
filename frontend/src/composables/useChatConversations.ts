import { ref } from 'vue'
import {
  appendMessages,
  clearActiveConversationId,
  createConversation,
  deleteConversation,
  getActiveConversationId,
  getConversation,
  listConversations,
  setActiveConversationId,
  type ChatConversation,
  type ChatMessageRecord,
} from '@/api/conversation'

const WELCOME_TEXT =
  '你好，我是 CompeteAI 助手。你可以直接描述竞品分析任务（例如「分析 Cursor 与 GitHub Copilot 的功能、定价与 SWOT」），我会启动多 Agent 流水线并展示完整执行过程；也可以进行普通对话。'

export function welcomeMessage(): ChatMessageRecord {
  return {
    id: 'welcome',
    role: 'assistant',
    content: WELCOME_TEXT,
  }
}

/** 将 UI 消息序列化为持久化记录（含 steps / taskMode 等扩展字段）。 */
export function toMessageRecord(msg: {
  id: string
  role: string
  content: string
  steps?: unknown[]
  taskMode?: boolean
  taskId?: string
  stepsVisible?: boolean
  stepFilter?: string
  replaySpeed?: number
}): ChatMessageRecord {
  const payload: Record<string, unknown> = {}
  if (msg.steps?.length) payload.steps = msg.steps
  if (msg.taskMode) payload.taskMode = msg.taskMode
  if (msg.taskId) payload.taskId = msg.taskId
  if (msg.stepsVisible !== undefined) payload.stepsVisible = msg.stepsVisible
  if (msg.stepFilter) payload.stepFilter = msg.stepFilter
  if (msg.replaySpeed) payload.replaySpeed = msg.replaySpeed
  return {
    id: msg.id,
    role: msg.role as ChatMessageRecord['role'],
    content: msg.content,
    payload: Object.keys(payload).length ? payload : undefined,
  }
}

/** 从持久化记录还原 UI 消息。 */
export function fromMessageRecord(r: ChatMessageRecord) {
  const p = r.payload ?? {}
  return {
    id: r.id,
    role: r.role as 'user' | 'assistant',
    content: r.content,
    steps: (p.steps as unknown[]) ?? [],
    taskMode: Boolean(p.taskMode),
    taskId: p.taskId as string | undefined,
    stepsVisible: p.stepsVisible as boolean | undefined,
    stepFilter: (p.stepFilter as string | undefined) ?? 'all',
    replaySpeed: (p.replaySpeed as number | undefined) ?? 1,
    streaming: false,
    replayActive: false,
  }
}

export function useChatConversations() {
  const conversations = ref<ChatConversation[]>([])
  const activeId = ref<string | null>(null)
  const loadingList = ref(false)
  const switching = ref(false)

  async function refreshList() {
    loadingList.value = true
    try {
      conversations.value = await listConversations()
    } finally {
      loadingList.value = false
    }
  }

  async function createNew(title?: string): Promise<string> {
    const conv = await createConversation(title)
    activeId.value = conv.id
    setActiveConversationId(conv.id)
    await refreshList()
    return conv.id
  }

  async function loadMessages(conversationId: string) {
    switching.value = true
    try {
      const detail = await getConversation(conversationId)
      activeId.value = detail.id
      setActiveConversationId(detail.id)
      if (!detail.messages.length) {
        return [fromMessageRecord(welcomeMessage())]
      }
      return detail.messages.map(fromMessageRecord)
    } finally {
      switching.value = false
    }
  }

  async function switchTo(conversationId: string) {
    return loadMessages(conversationId)
  }

  async function ensureActive(): Promise<string> {
    if (activeId.value) return activeId.value
    const saved = getActiveConversationId()
    await refreshList()
    if (saved && conversations.value.some((c) => c.id === saved)) {
      activeId.value = saved
      return saved
    }
    if (conversations.value.length) {
      activeId.value = conversations.value[0]!.id
      setActiveConversationId(activeId.value)
      return activeId.value
    }
    return createNew()
  }

  async function init(): Promise<ReturnType<typeof fromMessageRecord>[]> {
    await refreshList()
    const saved = getActiveConversationId()
    if (saved && conversations.value.some((c) => c.id === saved)) {
      return loadMessages(saved)
    }
    if (conversations.value.length) {
      return loadMessages(conversations.value[0]!.id)
    }
    await createNew()
    return [fromMessageRecord(welcomeMessage())]
  }

  async function persist(records: ChatMessageRecord[]) {
    if (!activeId.value || !records.length) return
    await appendMessages(activeId.value, records)
    await refreshList()
  }

  async function remove(conversationId: string) {
    await deleteConversation(conversationId)
    if (activeId.value === conversationId) {
      activeId.value = null
      clearActiveConversationId()
    }
    await refreshList()
  }

  return {
    conversations,
    activeId,
    loadingList,
    switching,
    refreshList,
    createNew,
    switchTo,
    ensureActive,
    init,
    persist,
    remove,
    welcomeMessage,
  }
}
