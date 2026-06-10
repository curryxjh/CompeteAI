import request from './request'
import { mockTrace } from '@/mock/data'
import type { Trace } from '@/types'
import type { AxiosError } from 'axios'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

/** 获取 Trace；若尚未生成（404）则返回 null，不弹错提示。 */
export async function getTrace(taskId: string): Promise<Trace | null> {
  if (USE_MOCK) {
    await delay(200)
    return { ...mockTrace, taskId }
  }
  try {
    return await request.get<unknown, Trace>(`/traces/${taskId}`, {
      _skipError: true,
    } as object)
  } catch (e) {
    const err = e as AxiosError
    if (err.response?.status === 404) return null
    // 非 404 错误仍然抛出，由调用方决定是否展示
    throw e
  }
}

export async function getTraceReplay(taskId: string): Promise<Trace | null> {
  if (USE_MOCK) {
    return getTrace(taskId)
  }
  try {
    return await request.get<unknown, Trace>(`/traces/${taskId}/replay`, {
      _skipError: true,
    } as object)
  } catch (e) {
    const err = e as AxiosError
    if (err.response?.status === 404) return null
    throw e
  }
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms))
}
