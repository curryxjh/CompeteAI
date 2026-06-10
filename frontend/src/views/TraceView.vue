<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getTrace } from '@/api/trace'
import { getTask } from '@/api/task'
import TraceTimeline from '@/components/trace/TraceTimeline.vue'
import TraceReplay from '@/components/trace/TraceReplay.vue'
import TokenUsageChart from '@/components/trace/TokenUsageChart.vue'
import type { Task, Trace } from '@/types'

const route = useRoute()
const taskId = computed(() => route.params.taskId as string)
const trace = ref<Trace | null>(null)
const task = ref<Task | null>(null)
const loading = ref(false)
const loadError = ref<string>('')
const visibleCount = ref(99)
const replayPlaying = ref(false)
const replaySpeed = ref(1)
let replayTimer: number | null = null

const statusLabel: Record<string, string> = {
  pending: '等待中',
  queued: '排队中',
  running: '执行中',
  clarifying: '等待澄清',
  reworking: '重做中',
  waiting_reply: '等待回复',
  attention_required: '执行失败',
  completed: '已完成',
  failed: '已失败',
  cancelled: '已取消',
}

onMounted(async () => {
  loading.value = true
  try {
    const [traceResult, taskResult] = await Promise.allSettled([
      getTrace(taskId.value),
      getTask(taskId.value),
    ])
    if (traceResult.status === 'fulfilled') {
      trace.value = traceResult.value
    } else {
      loadError.value = (traceResult.reason as Error)?.message ?? '加载失败'
    }
    if (taskResult.status === 'fulfilled') {
      task.value = taskResult.value
    }
  } finally {
    loading.value = false
  }
})

const visibleNodes = computed(() =>
  trace.value?.nodes.slice(0, visibleCount.value) ?? [],
)

function onStep(index: number) {
  if (!trace.value) return
  visibleCount.value = Math.max(0, Math.min(index, trace.value.nodes.length))
  if (visibleCount.value >= trace.value.nodes.length) {
    stopReplay()
  }
}

function onExport() {
  if (!trace.value) return
  const blob = new Blob([JSON.stringify(trace.value, null, 2)], {
    type: 'application/json',
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `trace-${taskId.value}.json`
  a.click()
  URL.revokeObjectURL(url)
}

function startReplay() {
  if (!trace.value || !trace.value.nodes.length) return
  if (visibleCount.value >= trace.value.nodes.length) {
    visibleCount.value = 0
  }
  replayPlaying.value = true
  scheduleReplay()
}

function stopReplay() {
  replayPlaying.value = false
  if (replayTimer !== null) {
    window.clearInterval(replayTimer)
    replayTimer = null
  }
}

function resetReplay() {
  stopReplay()
  visibleCount.value = 0
}

function onReplaySpeed(value: number) {
  replaySpeed.value = value
}

function scheduleReplay() {
  stopReplay()
  if (!trace.value || !trace.value.nodes.length) return
  replayPlaying.value = true
  const delay = Math.max(220, Math.floor(1200 / replaySpeed.value))
  replayTimer = window.setInterval(() => {
    if (!trace.value) {
      stopReplay()
      return
    }
    if (visibleCount.value >= trace.value.nodes.length) {
      stopReplay()
      return
    }
    visibleCount.value += 1
    if (visibleCount.value >= trace.value.nodes.length) {
      stopReplay()
    }
  }, delay)
}

watch(replaySpeed, () => {
  if (replayPlaying.value) {
    scheduleReplay()
  }
})

watch(
  () => taskId.value,
  () => {
    stopReplay()
    visibleCount.value = 99
  },
)

onUnmounted(stopReplay)
</script>

<template>
  <div v-loading="loading" class="trace-view">
    <button type="button" class="back" @click="$router.push('/')">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="19" y1="12" x2="5" y2="12"/>
        <polyline points="12 19 5 12 12 5"/>
      </svg>
      返回任务列表
    </button>

    <header class="trace-head">
      <p class="eyebrow">Trace · {{ taskId }}</p>
      <h2>Agent 执行链路</h2>
      <div v-if="task" class="task-meta">
        <span class="task-title">{{ task.title }}</span>
        <span
          class="status-badge"
          :class="`status-${task.status}`"
        >{{ statusLabel[task.status] ?? task.status }}</span>
      </div>
    </header>

    <!-- Trace 有数据时正常展示 -->
    <template v-if="trace && trace.nodes.length > 0">
      <TraceReplay
        :playing="replayPlaying"
        :speed="replaySpeed"
        :step-index="visibleCount"
        :max-steps="trace.nodes.length"
        @play="startReplay"
        @pause="stopReplay"
        @step="onStep"
        @reset="resetReplay"
        @speed="onReplaySpeed"
        @export="onExport"
      />
      <TokenUsageChart :nodes="trace.nodes" />
      <section class="panel trace-panel">
        <TraceTimeline :nodes="visibleNodes" />
      </section>
    </template>

    <!-- 空态：trace 不存在或节点为空 -->
    <div v-else-if="!loading" class="empty-state">
      <div class="empty-icon">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>

      <template v-if="task?.status === 'attention_required' || task?.status === 'failed'">
        <h3>任务执行失败</h3>
        <p class="empty-desc">{{ task?.errorMessage || '任务在执行过程中发生错误，请检查模型配置和服务状态。' }}</p>
        <p class="empty-hint">常见原因：LLM 接入点 ID 不正确、API Key 无效、Worker 进程未启动</p>
      </template>

      <template v-else-if="task?.status === 'running' || task?.status === 'queued' || task?.status === 'pending'">
        <h3>执行中，暂无追踪数据</h3>
        <p class="empty-desc">任务正在执行，Trace 将在完成后生成。</p>
      </template>

      <template v-else-if="task?.status === 'completed'">
        <h3>Trace 数据缺失</h3>
        <p class="empty-desc">任务已完成，但未找到对应的 Trace 记录。</p>
      </template>

      <template v-else>
        <h3>暂无追踪数据</h3>
        <p class="empty-desc">{{ loadError || '该任务尚未产生执行记录，请确认 Worker 进程已启动。' }}</p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.trace-view {
  max-width: 900px;
  margin: 0 auto;
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  padding: 0;
  margin-bottom: 18px;
  transition: color 0.15s;
  font-family: var(--font-sans);
}

.back:hover {
  color: var(--accent-light);
}

.eyebrow {
  margin: 0 0 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-light);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.trace-head h2 {
  margin: 0 0 22px;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.trace-panel {
  padding: 18px;
  margin-top: 18px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.task-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.task-title {
  font-size: 13px;
  color: var(--text-muted);
}

.status-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 99px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
}

.status-badge.status-running,
.status-badge.status-queued,
.status-badge.status-pending {
  color: #60a5fa;
  border-color: #60a5fa44;
  background: #60a5fa11;
}

.status-badge.status-completed {
  color: #34d399;
  border-color: #34d39944;
  background: #34d39911;
}

.status-badge.status-failed,
.status-badge.status-attention_required {
  color: #f87171;
  border-color: #f8717144;
  background: #f8717111;
}

/* ── 空态 ── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 64px 24px;
  gap: 12px;
}

.empty-icon {
  color: var(--text-muted);
  opacity: 0.4;
  margin-bottom: 8px;
}

.empty-state h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #e2e8f0);
}

.empty-desc {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
  max-width: 480px;
  line-height: 1.6;
}

.empty-hint {
  margin: 0;
  font-size: 12px;
  color: #f87171;
  max-width: 480px;
  padding: 8px 14px;
  background: #f8717111;
  border: 1px solid #f8717133;
  border-radius: var(--radius-sm, 6px);
  line-height: 1.6;
}
</style>
