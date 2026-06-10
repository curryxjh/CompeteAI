import { getAccessToken } from '@/utils/token'

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
}

export type ChatStreamEventType =
  | 'thinking'
  | 'tool_call'
  | 'tool_result'
  | 'content'

export interface ChatStreamEvent {
  type?: ChatStreamEventType
  content?: string
  tool_name?: string
  tool_args?: string
  tool_result?: string
  status?: 'running' | 'done' | 'error'
  error?: string
  done?: boolean
}

interface StreamHandlers {
  onEvent: (ev: ChatStreamEvent) => void
  onDone?: () => void
  onError?: (msg: string) => void
}

export async function streamChat(
  messages: ChatMessage[],
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const token = getAccessToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  const res = await fetch('/api/chat/stream', {
    method: 'POST',
    headers,
    body: JSON.stringify({ messages }),
    signal,
  })

  if (!res.ok) {
    const err = await res.text()
    throw new Error(err || `HTTP ${res.status}`)
  }

  const reader = res.body?.getReader()
  if (!reader) {
    throw new Error('无法读取响应流')
  }

  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      if (!line.startsWith('data:')) continue
      const payload = line.slice(5).trim()
      if (!payload) continue

      try {
        const ev = JSON.parse(payload) as ChatStreamEvent
        if (ev.error) {
          handlers.onError?.(ev.error)
          return
        }
        if (ev.done) {
          handlers.onDone?.()
          continue
        }
        handlers.onEvent(ev)
      } catch {
        // ignore malformed chunks
      }
    }
  }

  handlers.onDone?.()
}
