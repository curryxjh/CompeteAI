<script setup lang="ts">
import { computed, ref } from 'vue'
import type { FeatureRow, SourceRef } from '@/types'

const props = defineProps<{
  features: FeatureRow[]
  competitors: string[]
  sources: Record<string, SourceRef>
}>()

const emit = defineEmits<{ 'open-source': [id: string] }>()

const filter = ref('')

const filtered = computed(() => {
  let rows = props.features
  if (filter.value) {
    const q = filter.value.toLowerCase()
    rows = rows.filter((r) => r.feature.toLowerCase().includes(q))
  }
  return rows
})

function cellValue(v: boolean | string): string {
  if (v === true) return '✅'
  if (v === false) return '❌'
  return String(v)
}

function openSource(ids?: string[]) {
  if (ids?.[0]) emit('open-source', ids[0])
}
</script>

<template>
  <div>
    <el-input
      v-model="filter"
      placeholder="筛选功能..."
      clearable
      style="max-width: 240px; margin-bottom: 12px"
    />
    <el-table :data="filtered" border stripe>
      <el-table-column prop="feature" label="功能" min-width="140" sortable />
      <el-table-column
        v-for="c in competitors"
        :key="c"
        :label="c"
      >
        <template #default="{ row }">
          <span>{{ cellValue(row.values[c]) }}</span>
          <el-button
            v-if="row.sourceIds?.[c]?.length"
            link
            type="primary"
            size="small"
            @click="openSource(row.sourceIds[c])"
          >
            🔗
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
