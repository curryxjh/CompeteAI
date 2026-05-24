import MarkdownIt from 'markdown-it'
import multimdTable from 'markdown-it-multimd-table'

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
}).use(multimdTable, { multiline: true, rowspan: true })

const defaultTableOpen =
  md.renderer.rules.table_open ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))
const defaultTableClose =
  md.renderer.rules.table_close ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.table_open = (...args) =>
  `<div class="table-wrap">\n${defaultTableOpen(...args)}`
md.renderer.rules.table_close = (...args) =>
  `${defaultTableClose(...args)}\n</div>`

const TOOL_LABELS: Record<string, string> = {
  search: '搜索网页',
  scrape: '抓取页面',
  map: '站点地图',
  crawl: '爬取站点',
  extract: '提取数据',
  batch_scrape: '批量抓取',
  check_crawl_status: '检查爬取状态',
  deep_research: '深度研究',
}

export function renderMarkdown(text: string): string {
  if (!text.trim()) return ''
  return md.render(text)
}

export function toolDisplayName(name?: string): string {
  if (!name) return '工具调用'
  const key = name.replace(/^firecrawl_/, '')
  return TOOL_LABELS[key] ?? key.replace(/_/g, ' ')
}

export function stepTypeLabel(step: {
  type: string
  toolName?: string
}): string {
  if (step.type === 'thinking') return '推理'
  return toolDisplayName(step.toolName)
}
