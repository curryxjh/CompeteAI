<script setup lang="ts">
import AgentIcon from '@/components/ui/AgentIcon.vue'
import StatusIcon from '@/components/ui/StatusIcon.vue'
import { AGENT_LABELS, AGENT_ORDER } from '@/utils/agent'
import type { AgentState } from '@/types'

defineProps<{
  progress: number
  agentStates: AgentState[]
}>()
</script>

<template>
  <div class="progress-wrap">
    <div class="progress-track">
      <div class="progress-fill" :style="{ width: `${progress}%` }" />
    </div>
    <div class="agent-pipeline">
      <span
        v-for="name in AGENT_ORDER"
        :key="name"
        class="agent-step"
        :class="agentStates.find((a) => a.name === name)?.status"
      >
        <AgentIcon :name="name" :size="26" class="step-avatar" />
        <span class="step-label">{{ AGENT_LABELS[name] }}</span>
        <StatusIcon
          :status="agentStates.find((a) => a.name === name)?.status ?? 'pending'"
          :size="13"
        />
      </span>
    </div>
  </div>
</template>

<style scoped>
.progress-wrap {
  margin-top: 14px;
  position: relative;
  z-index: 1;
}

.progress-track {
  height: 4px;
  background: var(--bg-base);
  border-radius: 999px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent), #818cf8, var(--accent));
  background-size: 200% 100%;
  animation: progress-flow 1.8s ease infinite;
  transition: width 0.6s cubic-bezier(0.22, 1, 0.36, 1);
}

@keyframes progress-flow {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}

.agent-pipeline {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  margin-top: 12px;
}

.agent-step {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
  transition: color 0.25s;
}

.agent-step.running {
  color: var(--accent);
}

.agent-step.completed {
  color: var(--success);
}

.agent-step.failed,
.agent-step.rejected {
  color: var(--danger);
}

.step-avatar :deep(.agent-icon) {
  width: 26px !important;
  height: 26px !important;
  border-radius: 8px;
}

.step-avatar :deep(svg) {
  width: 14px !important;
  height: 14px !important;
}

.step-label {
  font-size: 11px;
  font-family: var(--font-mono);
  opacity: 0.9;
}
</style>
