import type { CreateTaskPayload } from '@/types'

export const DEFAULT_ANALYSIS_DIMENSIONS = ['功能对比', 'SWOT', '定价']

export const ANALYSIS_DIMENSIONS = [
  '功能对比',
  'SWOT',
  '定价',
  '用户画像',
  '市场定位',
] as const

/** 单次任务最多支持的竞品数量。 */
export const MAX_COMPETITORS = 10

/** 从显式输入（逗号 / 顿号 / 换行分隔）解析竞品列表。 */
export function parseCompetitorList(text: string): string[] {
  const seen = new Set<string>()
  const result: string[] = []

  for (const raw of text.split(/[,，、\n]/)) {
    const name = raw.trim()
    if (!name) continue
    const key = name.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    result.push(name)
    if (result.length >= MAX_COMPETITORS) break
  }

  return result
}

/** 根据显式竞品与维度构建任务 payload；不足 2 个竞品时返回 null。 */
export function buildAnalysisPayload(
  competitorsText: string,
  dimensions: string[],
  note?: string,
): CreateTaskPayload | null {
  const competitors = parseCompetitorList(competitorsText)
  if (competitors.length < 2 || !dimensions.length) return null

  const trimmedNote = note?.trim()
  const title =
    trimmedNote ||
    `分析 ${competitors.join('、')} 的${dimensions.slice(0, 3).join('、')}`

  return {
    competitors,
    dimensions: [...dimensions],
    title: title.slice(0, 80),
  }
}

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
  if (byJoiner.length >= 2) return byJoiner.slice(0, MAX_COMPETITORS)

  const byComma = trimmed
    .split(/[,，、\n]/)
    .map((s) => s.trim())
    .filter(Boolean)
  if (byComma.length >= 2) return byComma.slice(0, MAX_COMPETITORS)

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
