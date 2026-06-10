<script setup lang="ts">
import type { Report } from '@/types'

defineProps<{ report: Report }>()
</script>

<template>
  <div class="report-header">
    <h2>{{ report.title }}</h2>
    <div class="meta">
      <span>生成时间：{{ new Date(report.generatedAt).toLocaleString('zh-CN') }}</span>
      <el-tag type="success">QA 评分 {{ report.qaScore }}%</el-tag>
    </div>
    <p class="summary">{{ report.summary }}</p>
  </div>
</template>

<style scoped>
.report-header h2 {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.meta {
  display: flex;
  gap: 16px;
  align-items: center;
  font-size: 13px;
  color: var(--text-muted);
}
.summary {
  margin-top: 12px;
  color: var(--text-secondary);
  line-height: 1.7;
}
</style>
