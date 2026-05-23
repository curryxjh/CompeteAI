import { getAccessToken } from '@/utils/token'

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
}

interface StreamHandlers {
  onDelta: (text: string) => void
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
        const ev = JSON.parse(payload) as {
          content?: string
          error?: string
          done?: boolean
        }
        if (ev.error) {
          handlers.onError?.(ev.error)
          return
        }
        if (ev.content) {
          handlers.onDelta(ev.content)
        }
        if (ev.done) {
          handlers.onDone?.()
        }
      } catch {
        // ignore malformed chunks
      }
    }
  }

  handlers.onDone?.()
}
