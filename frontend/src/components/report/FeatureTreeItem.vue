<script setup lang="ts">
import type { FeatureTreeNode } from '@/types'

defineProps<{ node: FeatureTreeNode }>()
const emit = defineEmits<{ 'open-source': [ids: string[]] }>()
</script>

<template>
  <li class="tree-item">
    <span>{{ node.supported === true ? '✅' : node.supported === false ? '❌' : '◦' }}</span>
    <strong>{{ node.name }}</strong>
    <span v-if="node.description" class="desc"> — {{ node.description }}</span>
    <button
      v-if="node.sourceIds?.length"
      type="button"
      class="link-btn"
      @click="emit('open-source', node.sourceIds!)"
    >
      🔗
    </button>
    <ul v-if="node.children?.length">
      <FeatureTreeItem
        v-for="(c, i) in node.children"
        :key="i"
        :node="c"
        @open-source="emit('open-source', $event)"
      />
    </ul>
  </li>
</template>

<style scoped>
.tree-item {
  font-size: 13px;
  line-height: 1.9;
  color: var(--text-secondary);
  list-style: none;
}
.tree-item ul {
  padding-left: 16px;
  margin: 0;
}
.desc {
  color: var(--text-muted);
}
.link-btn {
  margin-left: 6px;
  border: none;
  background: none;
  cursor: pointer;
}
</style>
