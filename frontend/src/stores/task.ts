import { defineStore } from 'pinia'

import { ref } from 'vue'

import * as taskApi from '@/api/task'

import type { AgentState, CreateTaskPayload, Task, TaskStatus } from '@/types'



export const useTaskStore = defineStore('task', () => {

  const tasks = ref<Task[]>([])

  const loading = ref(false)

  const filterStatus = ref<string>('')

  const searchQuery = ref('')



  const sseMap = new Map<string, EventSource>()



  async function fetchTasks() {

    loading.value = true

    try {

      tasks.value = await taskApi.listTasks()

      for (const t of tasks.value) {

        if (isActiveStatus(t.status)) {

          subscribeSSE(t.id)

        }

      }

    } finally {

      loading.value = false

    }

  }



  async function create(payload: CreateTaskPayload) {

    const task = await taskApi.createTask(payload)

    tasks.value.unshift(task)

    if (isActiveStatus(task.status)) {

      subscribeSSE(task.id)

    }

    return task

  }



  function isActiveStatus(status: TaskStatus) {
    return (
      status === 'queued' ||
      status === 'running' ||
      status === 'pending' ||
      status === 'clarifying' ||
      status === 'reworking' ||
      status === 'waiting_reply'
    )
  }



  function subscribeSSE(taskId: string) {

    if (sseMap.has(taskId)) return



    const es = new EventSource(`/api/tasks/${taskId}/stream`)



    es.addEventListener('task_started', (e) => {

      patchTask(taskId, JSON.parse(e.data))

    })



    es.addEventListener('agent_state', (e) => {

      const data = JSON.parse(e.data) as AgentState & { agent?: string; progress?: number }

      const name = data.name ?? data.agent

      if (!name) return

      updateTaskAgent(taskId, { ...data, name: name as AgentState['name'] })

      if (typeof data.progress === 'number') {

        patchTask(taskId, { progress: data.progress })

      }

    })

    es.addEventListener('tool_step', (e) => {
      // 预留：Dashboard 可扩展展示工具步骤
      void JSON.parse(e.data)
    })



    es.addEventListener('task_complete', (e) => {

      const data = JSON.parse(e.data)

      patchTask(taskId, { status: data.status ?? 'completed', progress: data.progress ?? 100 })

      closeSSE(taskId)

    })



    es.addEventListener('task_failed', (e) => {

      const data = JSON.parse(e.data)

      patchTask(taskId, { status: 'failed', errorMessage: data.message })

      closeSSE(taskId)

    })



    es.addEventListener('task_status', (e) => {
      const data = JSON.parse(e.data) as { status?: TaskStatus; progress?: number }
      patchTask(taskId, {
        ...(data.status ? { status: data.status } : {}),
        ...(typeof data.progress === 'number' ? { progress: data.progress } : {}),
      })
    })

    es.addEventListener('clarification', () => {

      patchTask(taskId, { status: 'clarifying' })

    })



    es.addEventListener('rejection', () => {

      patchTask(taskId, { status: 'reworking' })

    })



    es.onerror = () => {

      closeSSE(taskId)

    }



    sseMap.set(taskId, es)

  }



  function patchTask(taskId: string, patch: Partial<Task>) {

    const idx = tasks.value.findIndex((t) => t.id === taskId)

    if (idx >= 0) {

      tasks.value[idx] = { ...tasks.value[idx], ...patch }

    }

  }



  function updateTaskAgent(taskId: string, agent: AgentState) {

    const idx = tasks.value.findIndex((t) => t.id === taskId)

    if (idx < 0) return

    const task = tasks.value[idx]

    const states = [...task.agentStates]

    const ai = states.findIndex((a) => a.name === agent.name)

    if (ai >= 0) states[ai] = { ...states[ai], ...agent }

    else states.push(agent)

    tasks.value[idx] = { ...task, agentStates: states }

  }



  function closeSSE(taskId: string) {

    const es = sseMap.get(taskId)

    if (es) {

      es.close()

      sseMap.delete(taskId)

    }

  }



  function closeAllSSE() {

    for (const id of sseMap.keys()) {

      closeSSE(id)

    }

  }



  async function remove(id: string) {

    closeSSE(id)

    await taskApi.deleteTask(id)

    tasks.value = tasks.value.filter((t) => t.id !== id)

  }



  function filteredTasks() {

    return tasks.value.filter((t) => {

      if (filterStatus.value && t.status !== filterStatus.value) return false

      if (searchQuery.value) {

        const q = searchQuery.value.toLowerCase()

        return (

          t.title.toLowerCase().includes(q) ||

          t.competitors.some((c) => c.toLowerCase().includes(q))

        )

      }

      return true

    })

  }



  return {

    tasks,

    loading,

    filterStatus,

    searchQuery,

    fetchTasks,

    create,

    remove,

    filteredTasks,

    closeAllSSE,

  }

})


