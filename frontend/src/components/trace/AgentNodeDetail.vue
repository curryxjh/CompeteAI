<script setup lang="ts">
import ChainStep from '@/components/ui/ChainStep.vue'
import { formatDuration } from '@/utils/agent'
import type { TraceNode } from '@/types'

const props = defineProps<{ node: TraceNode; index?: number }>()

function mapStatus(s: string): 'pending' | 'running' | 'completed' | 'failed' | 'rejected' {
  if (props.node.isRejection) return 'rejected'
  if (s === 'running') return 'running'
  if (s === 'failed') return 'failed'
  if (s === 'completed') return 'completed'
  return 'pending'
}
</script>

<template>
  <ChainStep
    :title="node.label"
    :status="mapStatus(node.status)"
    :subtitle="`⏱ ${formatDuration(node.durationMs)} · ${node.tokenCount} tokens`"
    :input="node.input"
    :output="node.output"
    :tool-name="node.agent"
    :index="index ?? 0"
    :default-open="node.isRejection || node.isRetry"
  >
    <div v-if="node.metadata" class="meta font-mono">
      <span v-for="(v, k) in node.metadata" :key="k">{{ k }}={{ v }}</span>
    </div>
  </ChainStep>
</template>

<style scoped>
.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 11px;
  color: var(--text-muted);
  padding: 4px 0;
}
</style>
