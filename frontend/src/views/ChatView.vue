<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { streamChat, type ChatMessage } from '@/api/chat'

interface UiMessage extends ChatMessage {
  id: string
  streaming?: boolean
}

const messages = ref<UiMessage[]>([
  {
    id: 'welcome',
    role: 'assistant',
    content: '你好，我是 CompeteAI 助手。有什么可以帮你的？',
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
    streaming: true,
  })
  loading.value = true
  scrollToBottom()

  const history: ChatMessage[] = messages.value
    .filter((m) => m.id !== 'welcome' && !(m.id === assistantId))
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
        onDelta(delta) {
          assistant.content += delta
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
    },
  ]
}

onMounted(scrollToBottom)
</script>

<template>
  <div class="chat-page">
    <header class="chat-header">
      <div class="chat-header-inner">
        <div>
          <h2>AI 对话</h2>
          <p class="model-tag">Doubao-Seed-2.0-lite</p>
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
          <p class="bubble-text">
            {{ msg.content }}
            <span v-if="msg.streaming" class="cursor">▍</span>
          </p>
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
      <p class="chat-hint">CompeteAI · powered by Volcengine Ark</p>
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
  max-width: 880px;
  margin: 0 auto;
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
  max-width: 780px;
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
  max-width: 85%;
}

.msg-row.user .bubble {
  display: flex;
  justify-content: flex-end;
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
  max-width: 780px;
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
  max-width: 780px;
  margin: 8px auto 0;
  font-size: 10px;
  color: var(--text-muted);
  text-align: center;
  font-family: var(--font-mono);
}
</style>
