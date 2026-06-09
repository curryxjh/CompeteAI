import request from './request'
import { mockTasks } from '@/mock/data'
import type { AgentState, CreateTaskPayload, Task, TaskStatus } from '@/types'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

export interface TaskToolStepEvent {
  tool_name?: string
  tool_args?: string
  tool_result?: string
  status?: 'running' | 'done' | 'error'
  agent?: string
}

export interface AgentThinkingEvent {
  agent?: string
  kind?: 'path' | 'note' | 'thinking' | 'output' | 'analysis'
  content?: string
  status?: 'running' | 'done'
}

export interface ClarificationEvent {
  question?: string
  agent?: string
}

export interface RejectionEvent {
  fromAgent?: string
  toAgent?: string
  reason?: string
}

export interface TaskStreamHandlers {
  onStarted?: (data: { status?: TaskStatus; progress?: number; agentStates?: AgentState[] }) => void
  onAgentState?: (data: AgentState & { progress?: number; agent?: string }) => void
  onToolStep?: (data: TaskToolStepEvent) => void
  onAgentThinking?: (data: AgentThinkingEvent) => void
  onTaskStatus?: (data: { status?: TaskStatus; progress?: number }) => void
  onComplete?: (data: { status?: TaskStatus; progress?: number }) => void
  onFailed?: (data: { message?: string }) => void
  onClarification?: (data: ClarificationEvent) => void
  onRejection?: (data: RejectionEvent) => void
}

export function subscribeTaskStream(
  taskId: string,
  handlers: TaskStreamHandlers,
): () => void {
  const es = new EventSource(`/api/tasks/${taskId}/stream`)

  es.addEventListener('task_started', (e) => {
    handlers.onStarted?.(JSON.parse(e.data))
  })
  es.addEventListener('agent_state', (e) => {
    handlers.onAgentState?.(JSON.parse(e.data))
  })
  es.addEventListener('tool_step', (e) => {
    handlers.onToolStep?.(JSON.parse(e.data))
  })
  es.addEventListener('agent_thinking', (e) => {
    handlers.onAgentThinking?.(JSON.parse(e.data))
  })
  es.addEventListener('task_status', (e) => {
    handlers.onTaskStatus?.(JSON.parse(e.data))
  })
  es.addEventListener('task_complete', (e) => {
    handlers.onComplete?.(JSON.parse(e.data))
    es.close()
  })
  es.addEventListener('task_failed', (e) => {
    handlers.onFailed?.(JSON.parse(e.data))
    es.close()
  })
  es.addEventListener('clarification', (event) => {
    handlers.onClarification?.(JSON.parse(event.data))
  })
  es.addEventListener('rejection', (event) => {
    handlers.onRejection?.(JSON.parse(event.data))
  })
  es.onerror = () => {
    es.close()
  }

  return () => es.close()
}

export async function listTasks(): Promise<Task[]> {
  if (USE_MOCK) {
    await delay(200)
    return [...mockTasks]
  }
  return request.get<unknown, Task[]>('/tasks')
}

export async function getTask(id: string): Promise<Task> {
  if (USE_MOCK) {
    await delay(150)
    const t = mockTasks.find((x) => x.id === id)
    if (!t) throw new Error('任务不存在')
    return { ...t }
  }
  return request.get<unknown, Task>(`/tasks/${id}`)
}

export async function createTask(payload: CreateTaskPayload): Promise<Task> {
  if (USE_MOCK) {
    await delay(400)
    const id = `t${Date.now().toString(36)}`
    const title =
      payload.title ??
      `${payload.competitors.join(' vs ')} 竞品分析`
    const task: Task = {
      id,
      title,
      competitors: payload.competitors,
      dimensions: payload.dimensions,
      status: 'running',
      progress: 5,
      agentStates: [{ name: 'coordinator', status: 'running' }],
      createdAt: new Date().toISOString(),
    }
    mockTasks.unshift(task)
    return task
  }
  return request.post<unknown, Task>('/tasks', payload)
}

export async function deleteTask(id: string): Promise<void> {
  if (USE_MOCK) {
    const idx = mockTasks.findIndex((t) => t.id === id)
    if (idx >= 0) mockTasks.splice(idx, 1)
    return
  }
  return request.delete(`/tasks/${id}`)
}

export async function clarifyTask(
  id: string,
  answer: string,
): Promise<void> {
  if (USE_MOCK) {
    console.info('clarify', id, answer)
    return
  }
  return request.post(`/tasks/${id}/clarify`, { answer })
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms))
}
