import { ref, watch, type Ref } from 'vue'
import { getReport } from '@/api/report'
import type { Report } from '@/types'

export function useReport(taskId: Ref<string> | string) {
  const report = ref<Report | null>(null)
  const loading = ref(false)
  const error = ref<string>()

  async function load() {
    const id = typeof taskId === 'string' ? taskId : taskId.value
    if (!id) return
    loading.value = true
    error.value = undefined
    try {
      report.value = await getReport(id)
    } catch (e) {
      error.value =
        e instanceof Error && e.message.includes('404')
          ? '报告尚未生成，分析可能仍在进行中，请稍后刷新'
          : e instanceof Error
            ? e.message
            : '加载报告失败'
    } finally {
      loading.value = false
    }
  }

  if (typeof taskId !== 'string') {
    watch(taskId, load)
  }

  return { report, loading, error, load }
}
