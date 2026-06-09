<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getTrace } from '@/api/trace'
import TraceTimeline from '@/components/trace/TraceTimeline.vue'
import TraceReplay from '@/components/trace/TraceReplay.vue'
import TokenUsageChart from '@/components/trace/TokenUsageChart.vue'
import type { Trace } from '@/types'

const route = useRoute()
const taskId = computed(() => route.params.taskId as string)
const trace = ref<Trace | null>(null)
const loading = ref(false)
const visibleCount = ref(99)
const replayPlaying = ref(false)
const replaySpeed = ref(1)
let replayTimer: number | null = null

onMounted(async () => {
  loading.value = true
  try {
    trace.value = await getTrace(taskId.value)
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

    <header v-if="trace" class="trace-head">
      <p class="eyebrow">Trace · {{ taskId }}</p>
      <h2>Agent 执行链路</h2>
    </header>

    <template v-if="trace">
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
</style>
