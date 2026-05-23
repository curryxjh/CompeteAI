import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as taskApi from '@/api/task'
import type { CreateTaskPayload, Task } from '@/types'

export const useTaskStore = defineStore('task', () => {
  const tasks = ref<Task[]>([])
  const loading = ref(false)
  const filterStatus = ref<string>('')
  const searchQuery = ref('')

  async function fetchTasks() {
    loading.value = true
    try {
      tasks.value = await taskApi.listTasks()
    } finally {
      loading.value = false
    }
  }

  async function create(payload: CreateTaskPayload) {
    const task = await taskApi.createTask(payload)
    tasks.value.unshift(task)
    return task
  }

  async function remove(id: string) {
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
  }
})
