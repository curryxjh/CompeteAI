<script setup lang="ts">
import type { AgentName } from '@/types'

withDefaults(
  defineProps<{
    name: AgentName
    size?: number
  }>(),
  { size: 40 },
)

const colors: Record<AgentName, { bg: string; fg: string }> = {
  coordinator: { bg: 'rgba(99, 102, 241, 0.12)', fg: '#6366f1' },
  collector: { bg: 'rgba(59, 130, 246, 0.12)', fg: '#3b82f6' },
  analyst: { bg: 'rgba(139, 92, 246, 0.12)', fg: '#8b5cf6' },
  writer: { bg: 'rgba(20, 184, 166, 0.12)', fg: '#14b8a6' },
  qa: { bg: 'rgba(16, 185, 129, 0.12)', fg: '#10b981' },
}
</script>

<template>
  <span
    class="agent-icon"
    :style="{
      width: `${size ?? 40}px`,
      height: `${size ?? 40}px`,
      background: colors[name].bg,
      color: colors[name].fg,
    }"
  >
    <!-- Coordinator: orchestration / flow -->
    <svg
      v-if="name === 'coordinator'"
      :width="size ? size * 0.5 : 20"
      :height="size ? size * 0.5 : 20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="12" cy="5" r="2" />
      <circle cx="5" cy="19" r="2" />
      <circle cx="19" cy="19" r="2" />
      <path d="M12 7v4M8.5 15.5 10 13M15.5 15.5 14 13" />
    </svg>

    <!-- Collector: search + fetch -->
    <svg
      v-else-if="name === 'collector'"
      :width="size ? size * 0.5 : 20"
      :height="size ? size * 0.5 : 20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="11" cy="11" r="7" />
      <path d="M21 21l-4.3-4.3" />
      <path d="M11 8v6M8 11h6" />
    </svg>

    <!-- Analyst: chart -->
    <svg
      v-else-if="name === 'analyst'"
      :width="size ? size * 0.5 : 20"
      :height="size ? size * 0.5 : 20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M4 19V5" />
      <path d="M4 19h16" />
      <path d="M8 17V11" />
      <path d="M12 17V7" />
      <path d="M16 17v-4" />
    </svg>

    <!-- Writer: document -->
    <svg
      v-else-if="name === 'writer'"
      :width="size ? size * 0.5 : 20"
      :height="size ? size * 0.5 : 20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <path d="M8 13h8M8 17h5" />
    </svg>

    <!-- QA: shield check -->
    <svg
      v-else
      :width="size ? size * 0.5 : 20"
      :height="size ? size * 0.5 : 20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="M9 12l2 2 4-4" />
    </svg>
  </span>
</template>

<style scoped>
.agent-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  flex-shrink: 0;
  transition: transform 0.3s cubic-bezier(0.22, 1, 0.36, 1), box-shadow 0.3s ease;
}
</style>
