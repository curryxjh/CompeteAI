import request from './request'
import { mockTasks } from '@/mock/data'
import type { CreateTaskPayload, Task } from '@/types'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

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
