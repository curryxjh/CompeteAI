<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  streamChat,
  type ChatMessage,
  type ChatStreamEvent,
} from '@/api/chat'
import { renderMarkdown, stepTypeLabel, toolDisplayName } from '@/utils/markdown'

type StepType = 'thinking' | 'tool'

interface ChatStep {
  id: string
  type: StepType
  content: string
  toolName?: string
  toolArgs?: string
  toolResult?: string
  status?: 'running' | 'done' | 'error'
  expanded?: boolean
}

interface UiMessage extends ChatMessage {
  id: string
  streaming?: boolean
  steps?: ChatStep[]
  stepsVisible?: boolean
}

const messages = ref<UiMessage[]>([
  {
    id: 'welcome',
    role: 'assistant',
    content: '你好，我是 CompeteAI 助手。有什么可以帮你的？',
    steps: [],
  },
])

const input = ref('')
const loading = ref(false)
const listRef = ref<HTMLElement | null>(null)
const abortRef = ref<AbortController | null>(null)

function scrollToBottom() {
  nextTick(() => {
    const el = listRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function uid() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function stepSummary(step: ChatStep, max = 56): string {
  if (step.type === 'thinking') {
    return step.content ? truncate(step.content, max) : '思考中…'
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
  return msg.steps?.length ?? 0
}

function processStats(msg: UiMessage): string {
  const steps = msg.steps ?? []
  if (!steps.length) return ''
  const tools = steps.filter((s) => s.type === 'tool')
  const thinking = steps.filter((s) => s.type === 'thinking').length
  const running = tools.filter((s) => s.status === 'running').length
  if (running > 0) {
    const cur = tools.find((s) => s.status === 'running')
    const name = cur ? toolDisplayName(cur.toolName) : '工具'
    return `正在 ${name}…`
  }
  const parts: string[] = []
  if (thinking) parts.push(`推理 ${thinking} 次`)
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

async function send() {
  const text = input.value.trim()
  if (!text || loading.value) return

  input.value = ''
  messages.value.push({ id: uid(), role: 'user', content: text })

  const assistantId = uid()
  messages.value.push({
    id: assistantId,
    role: 'assistant',
    content: '',
    steps: [],
    stepsVisible: true,
    streaming: true,
  })
  loading.value = true
  scrollToBottom()

  const history: ChatMessage[] = messages.value
    .filter((m) => m.id !== 'welcome' && m.id !== assistantId)
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
    loading.value = false
    scrollToBottom()
  }
}

function stop() {
  abortRef.value?.abort()
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

function clearChat() {
  if (loading.value) stop()
  messages.value = [
    {
      id: 'welcome',
      role: 'assistant',
      content: '对话已清空。继续提问吧。',
      steps: [],
    },
  ]
}

function toggleStep(step: ChatStep) {
  step.expanded = !step.expanded
}

onMounted(scrollToBottom)
</script>

<template>
  <div class="chat-page">
    <header class="chat-header">
      <div class="chat-header-inner">
        <div>
          <h2>AI 对话</h2>
          <p class="model-tag">Doubao-Seed-2.0-lite · Firecrawl MCP</p>
        </div>
        <button type="button" class="btn-ghost btn-sm" @click="clearChat">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
          清空
        </button>
      </div>
    </header>

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
                <span class="process-count">{{ stepCount(msg) }} 步</span>
              </span>
              <span class="process-stats">{{ processStats(msg) }}</span>
              <span class="process-toggle">{{ msg.stepsVisible ? '收起详情' : '展开详情' }}</span>
            </button>
            <ul v-if="!msg.stepsVisible" class="process-preview">
              <li
                v-for="(step, index) in msg.steps"
                :key="step.id"
                class="process-preview-item"
              >
                <span class="preview-index">{{ index + 1 }}</span>
                <span class="preview-label">{{ stepTypeLabel(step) }}</span>
                <span class="preview-text">{{ stepSummary(step, 80) }}</span>
              </li>
            </ul>
            <div v-show="msg.stepsVisible" class="process-body">
              <div
                v-for="step in msg.steps"
                :key="step.id"
                class="step-row"
                :class="[step.type, step.status, { open: step.expanded }]"
              >
                <button type="button" class="step-row-head" @click="toggleStep(step)">
                  <span class="step-dot" />
                  <span class="step-row-label">
                    {{ stepTypeLabel(step) }}
                  </span>
                  <span class="step-row-summary">{{ stepSummary(step) }}</span>
                  <span v-if="step.type === 'tool'" class="step-row-badge" :class="step.status">
                    {{ step.status === 'running' ? '…' : step.status === 'error' ? '!' : '✓' }}
                  </span>
                </button>
                <div v-show="step.expanded" class="step-row-detail">
                  <template v-if="step.type === 'thinking'">
                    <pre class="step-text">{{ step.content }}</pre>
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

    <footer class="chat-footer">
      <div class="chat-input-box">
        <textarea
          v-model="input"
          class="chat-input"
          rows="1"
          placeholder="输入消息，Enter 发送，Shift+Enter 换行"
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
            :disabled="!input.trim()"
            @click="send"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="19" x2="12" y2="5"/>
              <polyline points="5 12 12 5 19 12"/>
            </svg>
          </button>
        </div>
      </div>
      <p class="chat-hint">CompeteAI · 支持展示思考过程与 MCP 工具调用</p>
    </footer>
  </div>
</template>

<style scoped>
.chat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--bg-base);
}

.chat-header {
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(16px);
}

.chat-header-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 24px;
  max-width: 1000px;
  margin: 0 auto;
  width: 100%;
}

.chat-header h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.model-tag {
  margin: 3px 0 0;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.btn-sm {
  font-size: 12px;
  padding: 5px 12px;
}

.chat-list {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
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
  word-break: break-word;
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
}

.md-body h1 { font-size: 1.35em; }
.md-body h2 { font-size: 1.2em; }
.md-body h3 { font-size: 1.05em; }

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

.md-body table {
  width: max-content;
  min-width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.md-body .table-wrap {
  overflow-x: auto;
  margin: 0.75em 0;
  max-width: 100%;
  -webkit-overflow-scrolling: touch;
}

.md-body th,
.md-body td {
  padding: 8px 12px;
  border: 1px solid var(--border-subtle);
  vertical-align: top;
  text-align: left;
  white-space: normal;
  min-width: 88px;
  max-width: 360px;
}

.md-body th {
  background: var(--bg-base);
  font-weight: 600;
  white-space: nowrap;
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
  padding: 12px 24px 16px;
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-panel);
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
  margin: 8px auto 0;
  font-size: 10px;
  color: var(--text-muted);
  text-align: center;
  font-family: var(--font-mono);
}
</style>
