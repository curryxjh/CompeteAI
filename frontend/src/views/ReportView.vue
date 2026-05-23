<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useReport } from '@/composables/useReport'
import { exportReport } from '@/api/report'
import ReportHeader from '@/components/report/ReportHeader.vue'
import FeatureMatrix from '@/components/report/FeatureMatrix.vue'
import SWOTRadar from '@/components/report/SWOTRadar.vue'
import PricingCompare from '@/components/report/PricingCompare.vue'
import UserPersonaCard from '@/components/report/UserPersonaCard.vue'
import SourceRefPanel from '@/components/report/SourceRefPanel.vue'
import AnnotationTool from '@/components/report/AnnotationTool.vue'
import type { SourceRef } from '@/types'

const route = useRoute()
const taskId = computed(() => route.params.taskId as string)
const { report, loading, error, load } = useReport(taskId)

const activeTab = ref('summary')
const sourcePanelVisible = ref(false)
const activeSource = ref<SourceRef | null>(null)

onMounted(load)

function openSource(id: string) {
  const src = report.value?.sources[id]
  if (src) {
    activeSource.value = src
    sourcePanelVisible.value = true
  }
}

async function doExport(format: 'pdf' | 'md') {
  try {
    const blob = await exportReport(taskId.value, format)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `report-${taskId.value}.${format === 'pdf' ? 'pdf' : 'md'}`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出已开始')
  } catch {
    ElMessage.error('导出失败')
  }
}

const competitors = computed(() => {
  if (!report.value) return []
  return Object.keys(report.value.swot)
})
</script>

<template>
  <div v-loading="loading" class="report-view">
    <button type="button" class="back" @click="$router.push('/')">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="19" y1="12" x2="5" y2="12"/>
        <polyline points="12 19 5 12 12 5"/>
      </svg>
      返回任务列表
    </button>

    <el-alert v-if="error" type="error" :title="error" show-icon class="err" />

    <template v-if="report">
      <el-card>
        <ReportHeader :report="report" />
        <div class="export-bar">
          <el-button @click="doExport('md')">导出 Markdown</el-button>
          <el-button @click="doExport('pdf')">导出 PDF</el-button>
        </div>
      </el-card>

      <el-card class="content-card">
        <el-tabs v-model="activeTab">
          <el-tab-pane label="概要" name="summary">
            <p class="tab-text">{{ report.summary }}</p>
          </el-tab-pane>
          <el-tab-pane label="功能对比" name="features">
            <FeatureMatrix
              :features="report.features"
              :competitors="competitors"
              :sources="report.sources"
              @open-source="openSource"
            />
          </el-tab-pane>
          <el-tab-pane label="SWOT" name="swot">
            <SWOTRadar :swot="report.swot" @open-source="openSource" />
          </el-tab-pane>
          <el-tab-pane label="定价" name="pricing">
            <PricingCompare :pricing="report.pricing" />
          </el-tab-pane>
          <el-tab-pane label="用户画像" name="persona">
            <UserPersonaCard :personas="report.personas" />
          </el-tab-pane>
        </el-tabs>
      </el-card>

      <AnnotationTool :task-id="taskId" />
    </template>

    <SourceRefPanel
      :visible="sourcePanelVisible"
      :source="activeSource"
      @close="sourcePanelVisible = false"
    />
  </div>
</template>

<style scoped>
.report-view {
  max-width: 1100px;
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

.err {
  margin: 12px 0;
}

.content-card {
  margin-top: 20px;
}

.tab-text {
  font-size: 14px;
  line-height: 1.8;
  color: var(--text-secondary);
}

.export-bar {
  margin-top: 16px;
  display: flex;
  gap: 8px;
}
</style>
