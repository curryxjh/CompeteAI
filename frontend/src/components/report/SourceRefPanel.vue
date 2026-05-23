<script setup lang="ts">
import type { SourceRef } from '@/types'

defineProps<{
  visible: boolean
  source: SourceRef | null
}>()

const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <el-drawer
    :model-value="visible"
    title="信息溯源"
    size="400px"
    @close="emit('close')"
  >
    <template v-if="source">
      <h4>{{ source.title ?? '原始来源' }}</h4>
      <el-link :href="source.url" target="_blank" type="primary">
        {{ source.url }}
      </el-link>
      <p class="excerpt">{{ source.excerpt }}</p>
      <p class="time">采集时间：{{ new Date(source.collectedAt).toLocaleString('zh-CN') }}</p>
    </template>
    <el-empty v-else description="暂无溯源数据" />
  </el-drawer>
</template>

<style scoped>
h4 {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
}

.excerpt {
  margin-top: 16px;
  padding: 14px;
  background: var(--bg-hover);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  font-size: 13px;
  line-height: 1.7;
  color: var(--text-secondary);
}

.time {
  margin-top: 14px;
  font-size: 12px;
  color: var(--text-muted);
}
</style>
