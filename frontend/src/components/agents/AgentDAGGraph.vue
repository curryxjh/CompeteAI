<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { AGENT_LABELS } from '@/utils/agent'
import type { AgentCard } from '@/types'

const props = defineProps<{ agents: AgentCard[] }>()
const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildGraph() {
  const nodes = props.agents.map((a) => ({
    id: a.name,
    name: a.displayName,
    symbolSize: 56,
    itemStyle: { color: '#409eff' },
  }))
  const links: { source: string; target: string }[] = []
  for (const a of props.agents) {
    for (const dep of a.dependsOn) {
      links.push({ source: dep, target: a.name })
    }
  }
  return { nodes, links }
}

function render() {
  if (!chartRef.value) return
  if (!chart) chart = echarts.init(chartRef.value)
  const { nodes, links } = buildGraph()
  chart.setOption({
    tooltip: {
      formatter: (p: { data?: { name: string; id: string } }) => {
        const d = p.data
        return d ? `${d.name}<br/>${AGENT_LABELS[d.id as keyof typeof AGENT_LABELS]}` : ''
      },
    },
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        label: { show: true, formatter: '{b}' },
        force: { repulsion: 200, edgeLength: 120 },
        data: nodes,
        links,
        lineStyle: { color: 'rgba(0,0,0,0.15)', curveness: 0.2 },
        edgeSymbol: ['none', 'arrow'],
      },
    ],
  })
}

onMounted(render)
watch(() => props.agents, render, { deep: true })
onUnmounted(() => chart?.dispose())
</script>

<template>
  <div ref="chartRef" class="dag-chart" />
</template>

<style scoped>
.dag-chart {
  height: 400px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}
</style>
