<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
  visibleCount.value = index + 1
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
      <TraceReplay @step="onStep" @export="onExport" />
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
