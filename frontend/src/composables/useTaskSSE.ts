import { onMounted, onUnmounted, ref } from 'vue'
import { ElNotification } from 'element-plus'
import type { AgentState, RejectionEvent, TaskStatus } from '@/types'

export function useTaskSSE(taskId: string) {
  const status = ref<TaskStatus>()
  const agentStates = ref<AgentState[]>([])
  const error = ref<string>()
  const connected = ref(false)

  let eventSource: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout>

  function connect() {
    if (eventSource) {
      eventSource.close()
    }
    eventSource = new EventSource(`/api/tasks/${taskId}/stream`)

    eventSource.addEventListener('task_started', (e) => {
      const data = JSON.parse(e.data)
      status.value = data.status ?? 'running'
      connected.value = true
    })

    eventSource.addEventListener('agent_state', (e) => {
      const data = JSON.parse(e.data) as AgentState & { agent?: string }
      const name = data.name ?? (data as { agent: string }).agent
      const idx = agentStates.value.findIndex((a) => a.name === name)
      const entry = { ...data, name: name as AgentState['name'] }
      if (idx >= 0) agentStates.value[idx] = entry
      else agentStates.value.push(entry)
    })

    eventSource.addEventListener('task_complete', (e) => {
      status.value = JSON.parse(e.data).status ?? 'completed'
      eventSource?.close()
      connected.value = false
    })

    eventSource.addEventListener('task_failed', (e) => {
      status.value = 'failed'
      error.value = JSON.parse(e.data).message ?? '任务失败'
      eventSource?.close()
    })

    eventSource.addEventListener('rejection', (e) => {
      const data = JSON.parse(e.data) as RejectionEvent
      ElNotification({
        title: 'QA 打回',
        message: `${data.fromAgent} → ${data.toAgent}: ${data.reason}`,
        type: 'warning',
        duration: 8000,
      })
    })

    eventSource.addEventListener('clarification', (e) => {
      const data = JSON.parse(e.data)
      ElNotification({
        title: '需要澄清',
        message: data.question ?? 'Coordinator 需要您的输入',
        type: 'info',
        duration: 0,
      })
    })

    eventSource.onerror = () => {
      connected.value = false
      error.value = '连接断开，正在重连...'
      eventSource?.close()
      reconnectTimer = setTimeout(connect, 3000)
    }
  }

  onMounted(connect)
  onUnmounted(() => {
    clearTimeout(reconnectTimer)
    eventSource?.close()
  })

  return { status, agentStates, error, connected }
}
