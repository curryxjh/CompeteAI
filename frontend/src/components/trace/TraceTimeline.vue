<script setup lang="ts">
import AgentNodeDetail from './AgentNodeDetail.vue'
import type { TraceNode } from '@/types'

defineProps<{ nodes: TraceNode[] }>()
</script>

<template>
  <div class="chain-list">
    <p class="chain-label font-mono">// chain-of-thought</p>
    <AgentNodeDetail
      v-for="(node, index) in nodes"
      :key="node.id"
      :node="node"
      :index="index"
    />
  </div>
</template>

<style scoped>
.chain-list {
  display: flex;
  flex-direction: column;
}
.chain-label {
  margin: 0 0 12px;
  font-size: 11px;
  color: var(--text-muted);
}
</style>
