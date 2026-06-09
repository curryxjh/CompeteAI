<script setup lang="ts">
import type { FeatureTreeNode, SourceRef } from '@/types'
import FeatureTreeItem from './FeatureTreeItem.vue'

defineProps<{
  featureTree: Record<string, FeatureTreeNode[]>
  sources: Record<string, SourceRef>
}>()

const emit = defineEmits<{ 'open-source': [id: string] }>()

function openSource(ids: string[]) {
  if (ids[0]) emit('open-source', ids[0])
}
</script>

<template>
  <div v-for="(nodes, comp) in featureTree" :key="comp" class="tree-block">
    <h4>{{ comp }}</h4>
    <ul class="tree-root">
      <FeatureTreeItem
        v-for="(node, i) in nodes"
        :key="comp + '-' + i"
        :node="node"
        @open-source="openSource"
      />
    </ul>
  </div>
  <el-empty v-if="!Object.keys(featureTree).length" description="暂无功能树数据" />
</template>

<style scoped>
.tree-block {
  margin-bottom: 20px;
}
h4 {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
}
.tree-root {
  list-style: none;
  margin: 0;
  padding-left: 0;
}
</style>
