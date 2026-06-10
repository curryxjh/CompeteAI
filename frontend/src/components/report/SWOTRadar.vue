<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { SWOTAnalysis } from '@/types'

const props = defineProps<{
  swot: Record<string, SWOTAnalysis>
}>()

const chartRef = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

function buildOption() {
  const names = Object.keys(props.swot)
  const dims = ['优势', '劣势', '机会', '威胁']
  const keys: (keyof SWOTAnalysis)[] = [
    'strengths',
    'weaknesses',
    'opportunities',
    'threats',
  ]
  const series = names.map((name) => ({
    name,
    type: 'radar' as const,
    data: [
      {
        value: keys.map((k) => props.swot[name][k].length),
        name,
      },
    ],
  }))
  return {
    tooltip: {},
    legend: { data: names, bottom: 0 },
    radar: {
      indicator: dims.map((d) => ({ name: d, max: 10 })),
    },
    series,
  }
}

function init() {
  if (!chartRef.value) return
  chart = echarts.init(chartRef.value)
  chart.setOption(buildOption())
}

onMounted(init)
watch(() => props.swot, () => chart?.setOption(buildOption()), { deep: true })
onUnmounted(() => chart?.dispose())

defineExpose({ chartRef })
</script>

<template>
  <div class="swot-section">
    <div ref="chartRef" class="chart" />
    <div class="swot-grid">
      <el-card
        v-for="(analysis, name) in swot"
        :key="name"
        class="swot-card"
        shadow="never"
      >
        <template #header>{{ name }}</template>
        <div v-for="key in ['strengths', 'weaknesses', 'opportunities', 'threats']" :key="key" class="swot-block">
          <strong>{{ { strengths: 'S', weaknesses: 'W', opportunities: 'O', threats: 'T' }[key] }}:</strong>
          <ul>
            <li v-for="(item, i) in analysis[key as keyof SWOTAnalysis]" :key="i">
              {{ item.text }}
              <el-button
                v-if="item.sourceIds?.length"
                link
                type="primary"
                size="small"
                @click="$emit('open-source', item.sourceIds![0])"
              >
                🔗
              </el-button>
            </li>
          </ul>
        </div>
      </el-card>
    </div>
  </div>
</template>

<style scoped>
.chart {
  height: 320px;
  margin-bottom: 16px;
}
.swot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}
.swot-block {
  margin-bottom: 8px;
  font-size: 13px;
}
.swot-block ul {
  margin: 4px 0 0;
  padding-left: 18px;
}
</style>
