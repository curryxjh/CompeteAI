<script setup lang="ts">
import { useRouter } from 'vue-router'
import TaskProgressBar from './TaskProgressBar.vue'
import { AGENT_LABELS, formatDate } from '@/utils/agent'
import type { Task } from '@/types'

const props = withDefaults(
  defineProps<{
    task: Task
    index?: number
  }>(),
  { index: 0 },
)

const router = useRouter()

const statusLabel: Record<string, string> = {
  queued: '排队中',
  running: '运行中',
  clarifying: '待澄清',
  reworking: '重做中',
  waiting_reply: '等待回复',
  attention_required: '需人工处理',
  completed: '已完成',
  failed: '失败',
  pending: '等待中',
  cancelled: '已取消',
}

function viewReport() {
  router.push({ name: 'report', params: { taskId: props.task.id } })
}

function viewTrace() {
  router.push({ name: 'trace', params: { taskId: props.task.id } })
}
</script>

<template>
  <article
    class="task-card card-interactive"
    :class="[task.status, { 'is-running': ['queued','running','reworking','clarifying','waiting_reply'].includes(task.status) }]"
    :style="{ '--stagger': index }"
  >
    <div class="card-head">
      <span class="task-icon" :class="task.status" aria-hidden="true">
        <svg v-if="task.status === 'running'" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>
        </svg>
        <svg v-else-if="task.status === 'completed'" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
          <path d="M22 4L12 14.01l-3-3"/>
        </svg>
        <svg v-else-if="task.status === 'failed' || task.status === 'attention_required'" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <circle cx="12" cy="12" r="10"/>
          <path d="M15 9l-6 6M9 9l6 6"/>
        </svg>
        <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <circle cx="12" cy="12" r="10"/>
          <path d="M12 6v6l4 2"/>
        </svg>
      </span>
      <div class="card-body">
        <div class="card-main">
          <div class="card-top-row">
            <span class="task-id">{{ task.id }}</span>
            <span class="status-chip" :class="task.status">
              <span v-if="task.status === 'running'" class="live-dot" />
              {{ statusLabel[task.status] ?? task.status }}
            </span>
          </div>
          <h3>{{ task.title }}</h3>
        </div>
        <div class="actions">
          <button
            v-if="task.status === 'completed'"
            type="button"
            class="btn-accent action-btn"
            @click="viewReport"
          >
            查看报告
          </button>
          <button type="button" class="btn-ghost action-btn" @click="viewTrace">
            追踪
          </button>
        </div>
      </div>
    </div>

    <TaskProgressBar
      v-if="task.status === 'running' || task.status === 'reworking' || task.status === 'clarifying'"
      :progress="task.progress"
      :agent-states="task.agentStates"
    />

    <div
      v-if="task.latestToolActivity && ['running', 'reworking', 'clarifying', 'waiting_reply'].includes(task.status)"
      class="tool-activity"
      :class="task.latestToolActivity.status"
    >
      <span class="tool-activity-label">
        {{ task.latestToolActivity.agent ? AGENT_LABELS[task.latestToolActivity.agent] : 'Agent' }}
      </span>
      <span class="tool-activity-text">
        {{ task.latestToolActivity.summary ?? task.latestToolActivity.toolName }}
      </span>
    </div>

    <footer class="meta">
      <span>{{ formatDate(task.createdAt) }}</span>
      <span v-if="task.competitors.length" class="meta-sep">·</span>
      <span v-if="task.competitors.length">{{ task.competitors.join(' · ') }}</span>
    </footer>
  </article>
</template>

<style scoped>
.task-card {
  padding: 18px 20px;
  margin-bottom: 10px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.task-card.failed {
  border-color: rgba(239, 68, 68, 0.2);
}

.task-card.failed:hover {
  border-color: rgba(239, 68, 68, 0.35);
}

.card-head {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  position: relative;
  z-index: 1;
}

.card-body {
  flex: 1;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  min-width: 0;
}

.task-icon {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: var(--bg-panel);
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
}

.task-icon.running {
  color: var(--accent);
  background: var(--accent-dim);
  border-color: rgba(99, 102, 241, 0.25);
  animation: icon-spin 2s linear infinite;
}

.task-icon.completed {
  color: var(--success);
  background: var(--success-dim);
  border-color: rgba(16, 185, 129, 0.25);
}

.task-icon.failed {
  color: var(--danger);
  background: var(--danger-dim);
  border-color: rgba(239, 68, 68, 0.2);
}

@keyframes icon-spin {
  to {
    transform: rotate(360deg);
  }
}

.card-main {
  flex: 1;
  min-width: 0;
}

.card-top-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.task-id {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  transition: color 0.2s;
  letter-spacing: -0.01em;
  line-height: 1.4;
}

.task-card:hover h3 {
  color: var(--accent-light);
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px solid var(--border-subtle);
  color: var(--text-muted);
}

.status-chip.running {
  color: var(--accent-light);
  border-color: rgba(99, 102, 241, 0.3);
  background: var(--accent-dim);
}

.live-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 6px var(--accent-glow);
  animation: status-pulse 1.2s ease-in-out infinite;
}

.status-chip.completed {
  color: var(--success);
  border-color: rgba(16, 185, 129, 0.25);
  background: var(--success-dim);
}

.status-chip.failed {
  color: var(--danger);
  border-color: rgba(239, 68, 68, 0.2);
  background: var(--danger-dim);
}

.actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.action-btn {
  font-size: 12px;
  transition: transform 0.2s ease;
}

.action-btn:hover {
  transform: scale(1.03);
}

.action-btn:active {
  transform: scale(0.98);
}

.meta {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--text-muted);
  display: flex;
  gap: 4px;
  position: relative;
  z-index: 1;
  font-family: var(--font-mono);
}

.tool-activity {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  background: var(--bg-panel);
  font-size: 11px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.tool-activity.running {
  border-color: rgba(245, 158, 11, 0.22);
  color: #a16207;
}

.tool-activity.error {
  border-color: rgba(239, 68, 68, 0.22);
  color: var(--danger);
}

.tool-activity-label {
  flex-shrink: 0;
  text-transform: uppercase;
  color: var(--text-muted);
}

.tool-activity-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-sep {
  color: var(--border-default);
}
</style>
