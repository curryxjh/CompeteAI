<script setup lang="ts">
import { computed, ref } from 'vue'
import TerminalBlock from './TerminalBlock.vue'

export type StepStatus = 'pending' | 'running' | 'completed' | 'failed' | 'rejected'

const props = withDefaults(
  defineProps<{
    title: string
    status: StepStatus
    subtitle?: string
    input?: string
    output?: string
    toolName?: string
    defaultOpen?: boolean
    index?: number
  }>(),
  { index: 0 },
)

const open = ref(props.defaultOpen ?? props.status === 'running')

const statusIcon = computed(() => {
  switch (props.status) {
    case 'running':
      return 'spinner'
    case 'completed':
      return 'check'
    case 'failed':
    case 'rejected':
      return 'fail'
    default:
      return 'dot'
  }
})
</script>

<template>
  <div
    class="chain-step"
    :class="[status, { open, running: status === 'running' }]"
    :style="{ '--stagger': index }"
  >
    <button type="button" class="chain-head" @click="open = !open">
      <span class="status-icon" :class="statusIcon">
        <span v-if="statusIcon === 'spinner'" class="spin ring" />
        <span v-else-if="statusIcon === 'check'" class="check">✓</span>
        <span v-else-if="statusIcon === 'fail'" class="fail">×</span>
        <span v-else class="dot" />
      </span>
      <span class="chain-text">
        <span class="chain-title">{{ title }}</span>
        <span v-if="subtitle" class="chain-sub font-mono">{{ subtitle }}</span>
      </span>
      <span class="chevron font-mono">{{ open ? '−' : '+' }}</span>
    </button>

    <Transition name="expand">
      <div v-if="open" class="chain-body">
        <TerminalBlock
          v-if="input"
          :title="toolName ? `>_ ${toolName} · input` : '>_ input'"
          :content="input"
        />
        <TerminalBlock
          v-if="output"
          :title="toolName ? `>_ ${toolName} · output` : '>_ output'"
          :content="output"
          class="out-block"
        />
        <slot />
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.chain-step {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-elevated);
  margin-bottom: 8px;
  animation: card-rise 0.45s cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: calc(var(--stagger, 0) * 55ms);
  transition:
    transform 0.22s ease,
    border-color 0.2s,
    box-shadow 0.22s;
}
.chain-step:hover {
  transform: translateX(3px);
  border-color: rgba(99, 102, 241, 0.2);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.05);
}
.chain-step.running {
  border-color: var(--border-active);
  animation:
    card-rise 0.45s cubic-bezier(0.22, 1, 0.36, 1) both,
    border-flow 2.2s ease-in-out infinite;
  animation-delay: calc(var(--stagger, 0) * 55ms), 0s;
}
.chain-step.rejected,
.chain-step.failed {
  border-color: rgba(248, 113, 113, 0.35);
}
.chain-head {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  color: inherit;
}
.status-icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 2px;
}
.ring {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-subtle);
  border-top-color: var(--accent);
  border-radius: 50%;
}
.check {
  color: var(--success);
  font-size: 14px;
  font-weight: 700;
}
.fail {
  color: var(--danger);
  font-size: 16px;
}
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
}
.chain-text {
  flex: 1;
  min-width: 0;
}
.chain-title {
  display: block;
  font-size: 14px;
  color: var(--text-primary);
  line-height: 1.4;
}
.chain-sub {
  display: block;
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-muted);
}
.chevron {
  color: var(--text-muted);
  font-size: 14px;
}
.chain-body {
  padding: 0 14px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.out-block {
  margin-top: 0;
}
.expand-enter-active {
  transition:
    opacity 0.28s ease,
    transform 0.28s cubic-bezier(0.22, 1, 0.36, 1),
    max-height 0.35s ease;
  overflow: hidden;
}
.expand-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
  overflow: hidden;
}
.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
