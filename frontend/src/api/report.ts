import request from './request'
import { mockReport } from '@/mock/data'
import type { AnnotationPayload, Report } from '@/types'

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false'

export async function getReport(taskId: string): Promise<Report> {
  if (USE_MOCK) {
    await delay(200)
    return { ...mockReport, taskId }
  }
  return request.get<unknown, Report>(`/reports/${taskId}`)
}

export async function exportReport(
  taskId: string,
  format: 'pdf' | 'md',
): Promise<Blob> {
  if (USE_MOCK) {
    const text = `# Report ${taskId}\n\nMock export (${format})`
    return new Blob([text], {
      type: format === 'pdf' ? 'application/pdf' : 'text/markdown',
    })
  }
  return request.get(`/reports/${taskId}/export`, {
    params: { format },
    responseType: 'blob',
  }) as Promise<Blob>
}

export async function annotateReport(
  payload: AnnotationPayload,
): Promise<void> {
  if (USE_MOCK) {
    console.info('annotate', payload)
    return
  }
  return request.post(`/reports/${payload.taskId}/annotate`, payload)
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms))
}
