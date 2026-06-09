import type { CreateTaskPayload } from '@/types'

export const DEFAULT_ANALYSIS_DIMENSIONS = ['功能对比', 'SWOT', '定价']

/** 从自然语言描述中解析竞品名称（至少 2 个）。 */
export function parseCompetitors(text: string): string[] {
  const trimmed = text.trim()
  if (!trimmed) return []

  const afterAnalyze = trimmed.match(/^分析\s+(.+)/i)?.[1] ?? trimmed
  const core = afterAnalyze
    .replace(/(?:的)?(?:功能|定价|SWOT|对比|分析|竞品).*$/, '')
    .trim()

  const byJoiner = core
    .split(/\s*(?:与|和|vs\.?|VS)\s*/i)
    .map((s) => s.trim())
    .filter(Boolean)
  if (byJoiner.length >= 2) return byJoiner.slice(0, 4)

  const byComma = trimmed
    .split(/[,，、\n]/)
    .map((s) => s.trim())
    .filter(Boolean)
  if (byComma.length >= 2) return byComma.slice(0, 4)

  return []
}

/** 判断是否为竞品分析请求，并生成创建任务 payload。 */
export function parseAnalysisRequest(text: string): CreateTaskPayload | null {
  const trimmed = text.trim()
  if (!trimmed) return null

  const competitors = parseCompetitors(trimmed)
  if (competitors.length < 2) return null

  const hasIntent =
    /分析|对比|竞品|swot|定价|功能/i.test(trimmed) ||
    /\bvs\.?\b/i.test(trimmed) ||
    /与|和/.test(trimmed)
  if (!hasIntent) return null

  return {
    competitors,
    dimensions: DEFAULT_ANALYSIS_DIMENSIONS,
    title: trimmed.slice(0, 80) || undefined,
  }
}
