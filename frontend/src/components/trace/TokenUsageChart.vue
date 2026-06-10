<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { AGENT_LABELS } from '@/utils/agent'
import type { TraceNode } from '@/types'

const props = defineProps<{ nodes: TraceNode[] }>()
const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function render() {
  if (!chartRef.value) return
  if (!chart) chart = echarts.init(chartRef.value)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    xAxis: {
      type: 'category',
      data: props.nodes.map((n) => n.label),
      axisLabel: { rotate: 30, fontSize: 11 },
    },
    yAxis: { type: 'value', name: 'Tokens' },
    series: [
      {
        type: 'bar',
        data: props.nodes.map((n) => ({
          value: n.tokenCount,
          itemStyle: {
            color: n.isRejection ? '#f56c6c' : n.isRetry ? '#e6a23c' : '#409eff',
          },
        })),
        label: {
          show: true,
          position: 'top',
          formatter: (p: unknown) => {
            const param = p as { dataIndex?: number }
            const idx = param.dataIndex ?? 0
            const agent = props.nodes[idx]?.agent ?? 'coordinator'
            return `${AGENT_LABELS[agent]}`
          },
        },
      },
    ],
  })
}

onMounted(render)
watch(() => props.nodes, render, { deep: true })
onUnmounted(() => chart?.dispose())
</script>

<template>
  <div ref="chartRef" class="token-chart" />
</template>

<style scoped>
.token-chart {
  height: 280px;
  margin-bottom: 20px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 8px;
}
</style>
