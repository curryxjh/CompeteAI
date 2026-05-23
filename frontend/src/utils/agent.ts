import type { AgentName, AgentRunStatus } from '@/types'

export const AGENT_LABELS: Record<AgentName, string> = {
  coordinator: 'Coordinator',
  collector: 'Collector',
  analyst: 'Analyst',
  writer: 'Writer',
  qa: 'QA',
}

export const AGENT_ORDER: AgentName[] = [
  'coordinator',
  'collector',
  'analyst',
  'writer',
  'qa',
]

export function agentStatusIcon(status: AgentRunStatus): string {
  switch (status) {
    case 'completed':
      return '✓'
    case 'running':
      return '⏳'
    case 'failed':
      return '✗'
    case 'rejected':
      return '↩'
    default:
      return '○'
  }
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export const STATUS_TAG: Record<string, 'success' | 'warning' | 'danger' | 'info'> = {
  completed: 'success',
  running: 'warning',
  failed: 'danger',
  pending: 'info',
  cancelled: 'info',
}
