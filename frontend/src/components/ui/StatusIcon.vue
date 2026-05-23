<script setup lang="ts">
import type { AgentRunStatus } from '@/types'

withDefaults(
  defineProps<{
    status: AgentRunStatus
    size?: number
  }>(),
  { size: 14 },
)
</script>

<template>
  <span class="status-icon" :class="status">
    <!-- pending -->
    <svg
      v-if="status === 'pending'"
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
    >
      <circle cx="12" cy="12" r="9" stroke-dasharray="4 3" />
    </svg>

    <!-- running -->
    <svg
      v-else-if="status === 'running'"
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      class="spin"
    >
      <path d="M12 2a10 10 0 0 1 10 10" />
    </svg>

    <!-- completed -->
    <svg
      v-else-if="status === 'completed'"
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M20 6L9 17l-5-5" />
    </svg>

    <!-- rejected -->
    <svg
      v-else-if="status === 'rejected'"
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M9 14l-4-4 4-4" />
      <path d="M5 10h11a4 4 0 0 1 0 8h-1" />
    </svg>

    <!-- failed -->
    <svg
      v-else
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2.5"
      stroke-linecap="round"
    >
      <circle cx="12" cy="12" r="9" />
      <path d="M15 9l-6 6M9 9l6 6" />
    </svg>
  </span>
</template>

<style scoped>
.status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.status-icon.pending {
  color: var(--text-muted);
}

.status-icon.running {
  color: var(--accent);
}

.status-icon.completed {
  color: var(--success);
}

.status-icon.failed,
.status-icon.rejected {
  color: var(--danger);
}

.spin {
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
