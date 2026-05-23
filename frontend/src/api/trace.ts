import request from './request'
import { mockTrace } from '@/mock/data'
import type { Trace } from '@/types'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

export async function getTrace(taskId: string): Promise<Trace> {
  if (USE_MOCK) {
    await delay(200)
    return { ...mockTrace, taskId }
  }
  return request.get<unknown, Trace>(`/traces/${taskId}`)
}

export async function getTraceReplay(taskId: string): Promise<Trace> {
  if (USE_MOCK) {
    return getTrace(taskId)
  }
  return request.get<unknown, Trace>(`/traces/${taskId}/replay`)
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms))
}
