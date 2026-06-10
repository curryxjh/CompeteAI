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
    <div v-if="node.steps?.length" class="steps">
      <details v-for="(step, si) in node.steps" :key="si" class="step-detail">
        <summary>
          <span class="step-kind">{{ step.kind }}</span>
          <span v-if="step.toolName" class="step-tool">{{ step.toolName }}</span>
          <span class="step-status">{{ step.status }}</span>
        </summary>
        <pre class="step-content">{{ step.content }}</pre>
      </details>
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
.steps {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.step-detail {
  font-size: 11px;
  color: var(--text-secondary);
}
.step-detail summary {
  cursor: pointer;
  list-style: none;
  display: flex;
  gap: 8px;
  align-items: center;
}
.step-kind {
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
}
.step-tool {
  color: var(--el-color-primary);
}
.step-content {
  margin: 6px 0 0;
  padding: 8px;
  background: var(--bg-hover);
  border-radius: var(--radius-sm);
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 200px;
  overflow: auto;
  font-size: 11px;
}
</style>
