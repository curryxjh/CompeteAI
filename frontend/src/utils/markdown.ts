import MarkdownIt from 'markdown-it'
import multimdTable from 'markdown-it-multimd-table'
import { AGENT_LABELS } from '@/utils/agent'
import type { AgentName } from '@/types'

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

md.renderer.rules.table_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('class', 'md-chat-table')
  return `<div class="table-wrap md-table-card">\n${defaultTableOpen(tokens, idx, options, env, self)}`
}
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

const SWOT_QUADRANTS: Record<
  string,
  { letter: string; title: string; sub: string; cls: string }
> = {
  '优势 (S)': { letter: 'S', title: '优势', sub: 'Strengths', cls: 'md-swot-s' },
  '劣势 (W)': { letter: 'W', title: '劣势', sub: 'Weaknesses', cls: 'md-swot-w' },
  '机会 (O)': { letter: 'O', title: '机会', sub: 'Opportunities', cls: 'md-swot-o' },
  '威胁 (T)': { letter: 'T', title: '威胁', sub: 'Threats', cls: 'md-swot-t' },
}

interface H2Section {
  title?: string
  body: string
}

interface FeatureTreeNode {
  flag: 'yes' | 'no' | 'partial'
  label: string
  children: FeatureTreeNode[]
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function splitByH2(text: string): H2Section[] {
  const sections: H2Section[] = []
  let current: H2Section = { body: '' }

  for (const line of text.split('\n')) {
    const heading = line.match(/^## (.+)$/)
    if (heading) {
      if (current.title !== undefined || current.body.trim()) {
        sections.push(current)
      }
      current = { title: heading[1]!.trim(), body: '' }
      continue
    }
    current.body += `${line}\n`
  }

  if (current.title !== undefined || current.body.trim()) {
    sections.push(current)
  }

  return sections.length ? sections : [{ body: text }]
}

function renderSection(section: H2Section): string {
  const body = section.body.trim()
  if (!section.title) {
    return body ? md.render(body) : ''
  }

  const swotMatch = section.title.match(/^(.+?)\s*·\s*SWOT$/)
  if (swotMatch) return renderSwotHtml(swotMatch[1]!.trim(), body)
  if (section.title === '功能对比') return renderFeatureTableHtml(body)
  if (section.title === '功能树') return renderFeatureTreeHtml(body)
  if (section.title === '定价对比') return renderPricingHtml(body)
  if (section.title === '用户画像') return renderPersonasHtml(body)

  return md.render(`## ${section.title}\n${body}`)
}

function parseBrandSections(body: string): Array<{ name: string; lines: string[] }> {
  const chunks = body.split(/^### (.+)$/m)
  const brands: Array<{ name: string; lines: string[] }> = []

  if (chunks.length > 1) {
    for (let i = 1; i < chunks.length; i += 2) {
      brands.push({
        name: chunks[i]!.trim(),
        lines: (chunks[i + 1] ?? '').split('\n'),
      })
    }
  }

  return brands
}

function renderTagList(text: string): string {
  const items = text
    .split(/[、,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
  if (!items.length) return ''
  return items.map((t) => `<span class="md-tag">${escapeHtml(t)}</span>`).join('')
}

function parseSwotQuadrants(body: string) {
  const quadrants: Array<{
    letter: string
    title: string
    sub: string
    cls: string
    items: string[]
  }> = []

  let current: (typeof quadrants)[number] | null = null

  for (const line of body.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue

    const bold = trimmed.match(/^\*\*(.+?)\*\*$/)
    if (bold) {
      const meta = SWOT_QUADRANTS[bold[1]!]
      if (meta) {
        current = { ...meta, items: [] }
        quadrants.push(current)
        continue
      }
    }

    const bullet = trimmed.match(/^[-*]\s+(.+)$/)
    if (bullet && current) {
      current.items.push(bullet[1]!)
    }
  }

  return quadrants
}

function renderSwotHtml(brand: string, body: string): string {
  const quadrants = parseSwotQuadrants(body)
  if (!quadrants.length) {
    return md.render(`## ${brand} · SWOT\n${body}`)
  }

  const cards = quadrants
    .map((q) => {
      const items = q.items.length
        ? q.items
            .map((item) => `<li>${escapeHtml(item)}</li>`)
            .join('')
        : '<li class="md-swot-empty">暂无</li>'

      return `<div class="md-swot-card ${q.cls}">
  <div class="md-swot-card-head">
    <span class="md-swot-letter">${q.letter}</span>
    <div>
      <div class="md-swot-card-title">${q.title}</div>
      <div class="md-swot-card-sub">${q.sub}</div>
    </div>
  </div>
  <ul class="md-swot-list">${items}</ul>
</div>`
    })
    .join('')

  return `<div class="md-swot">
  <div class="md-swot-header">
    <span class="md-swot-brand">${escapeHtml(brand)}</span>
    <span class="md-swot-badge">SWOT</span>
  </div>
  <div class="md-swot-grid">${cards}</div>
</div>`
}

function parseTableRows(body: string): string[][] | null {
  const lines = body
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.startsWith('|') && l.endsWith('|'))
  if (lines.length < 2) return null

  const parseRow = (line: string) =>
    line
      .split('|')
      .slice(1, -1)
      .map((c) => c.trim())

  return lines.filter((_, i) => i !== 1).map(parseRow)
}

function renderCompareCell(value: string): string {
  const v = value.trim()
  if (v === '✓' || v === '✔' || v.toLowerCase() === 'yes' || v === 'true') {
    return '<span class="md-cell-yes" title="支持">✓</span>'
  }
  if (v === '—' || v === '-' || v === '✗' || v === '×' || v.toLowerCase() === 'no' || v === 'false') {
    return '<span class="md-cell-no" title="不支持">—</span>'
  }
  return escapeHtml(v)
}

function renderFeatureTableHtml(body: string): string {
  const rows = parseTableRows(body)
  if (!rows?.length) {
    return md.render(`## 功能对比\n${body}`)
  }

  const [header, ...data] = rows
  const headHtml = header!
    .map((h, i) => {
      const cls = i === 0 ? 'md-compare-feature' : 'md-compare-brand'
      return `<th class="${cls}">${escapeHtml(h)}</th>`
    })
    .join('')

  const bodyHtml = data
    .map((row) => {
      const cells = row
        .map((cell, i) => {
          if (i === 0) {
            return `<td class="md-compare-feature">${escapeHtml(cell)}</td>`
          }
          return `<td class="md-compare-value">${renderCompareCell(cell)}</td>`
        })
        .join('')
      return `<tr>${cells}</tr>`
    })
    .join('')

  return `<div class="md-report-section md-feature-compare">
  <div class="md-section-head">
    <span class="md-section-title">功能对比</span>
    <span class="md-section-badge">Matrix</span>
  </div>
  <div class="table-wrap md-compare-wrap">
    <table class="md-compare-table">
      <thead><tr>${headHtml}</tr></thead>
      <tbody>${bodyHtml}</tbody>
    </table>
  </div>
</div>`
}

function parseTreeFlag(raw: string): FeatureTreeNode['flag'] {
  if (raw === '✓' || raw === '✔') return 'yes'
  if (raw === '✗' || raw === '×') return 'no'
  return 'partial'
}

function parseFeatureTreeLine(line: string): { depth: number; node: FeatureTreeNode } | null {
  const match = line.match(/^(\s*)- \[([✓✔✗×·])\] (.+)$/)
  if (!match) return null
  const depth = Math.floor(match[1]!.length / 2)
  return {
    depth,
    node: {
      flag: parseTreeFlag(match[2]!),
      label: match[3]!.trim(),
      children: [],
    },
  }
}

function buildFeatureTree(lines: string[]): FeatureTreeNode[] {
  const roots: FeatureTreeNode[] = []
  const stack: Array<{ depth: number; node: FeatureTreeNode }> = []

  for (const line of lines) {
    const parsed = parseFeatureTreeLine(line)
    if (!parsed) continue

    while (stack.length && stack[stack.length - 1]!.depth >= parsed.depth) {
      stack.pop()
    }

    if (!stack.length) {
      roots.push(parsed.node)
    } else {
      stack[stack.length - 1]!.node.children.push(parsed.node)
    }
    stack.push(parsed)
  }

  return roots
}

function renderTreeNode(node: FeatureTreeNode, depth: number): string {
  const flagLabel = node.flag === 'yes' ? '支持' : node.flag === 'no' ? '不支持' : '部分'
  const kids = node.children.map((c) => renderTreeNode(c, depth + 1)).join('')
  const childrenHtml = kids ? `<div class="md-tree-children">${kids}</div>` : ''

  return `<div class="md-tree-node md-tree-depth-${Math.min(depth, 3)}">
  <div class="md-tree-row">
    <span class="md-tree-flag md-tree-flag-${node.flag}" title="${flagLabel}">${
      node.flag === 'yes' ? '✓' : node.flag === 'no' ? '×' : '·'
    }</span>
    <span class="md-tree-label">${escapeHtml(node.label)}</span>
  </div>
  ${childrenHtml}
</div>`
}

function renderFeatureTreeHtml(body: string): string {
  const chunks = body.split(/^### (.+)$/m)
  const brands: Array<{ name: string; nodes: FeatureTreeNode[] }> = []

  if (chunks.length > 1) {
    for (let i = 1; i < chunks.length; i += 2) {
      const name = chunks[i]!.trim()
      const content = chunks[i + 1] ?? ''
      const nodes = buildFeatureTree(content.split('\n'))
      if (nodes.length) brands.push({ name, nodes })
    }
  } else {
    const nodes = buildFeatureTree(body.split('\n'))
    if (nodes.length) brands.push({ name: '功能结构', nodes })
  }

  if (!brands.length) {
    return md.render(`## 功能树\n${body}`)
  }

  const brandHtml = brands
    .map(
      (b) => `<div class="md-tree-brand">
  <div class="md-tree-brand-name">${escapeHtml(b.name)}</div>
  <div class="md-tree-root">${b.nodes.map((n) => renderTreeNode(n, 0)).join('')}</div>
</div>`,
    )
    .join('')

  return `<div class="md-report-section md-feature-tree">
  <div class="md-section-head">
    <span class="md-section-title">功能树</span>
    <span class="md-section-badge">Tree</span>
  </div>
  <div class="md-tree-grid">${brandHtml}</div>
</div>`
}

function parsePricingTiers(lines: string[]) {
  const tiers: Array<{ name: string; price: string; desc: string }> = []

  for (const line of lines) {
    const trimmed = line.trim()
    const match = trimmed.match(/^[-*]\s+\*\*(.+?)\*\*\s*·\s*(.+)$/)
    if (!match) continue

    const tail = match[2]!
    const dash = tail.indexOf(' — ')
    if (dash >= 0) {
      tiers.push({
        name: match[1]!.trim(),
        price: tail.slice(0, dash).trim(),
        desc: tail.slice(dash + 3).trim(),
      })
    } else {
      tiers.push({ name: match[1]!.trim(), price: tail.trim(), desc: '' })
    }
  }

  return tiers
}

function renderPricingHtml(body: string): string {
  const brands = parseBrandSections(body)
    .map((b) => ({ name: b.name, tiers: parsePricingTiers(b.lines) }))
    .filter((b) => b.tiers.length)

  if (!brands.length) {
    return md.render(`## 定价对比\n${body}`)
  }

  const brandHtml = brands
    .map((b) => {
      const tiers = b.tiers
        .map(
          (t) => `<div class="md-pricing-tier">
  <div class="md-pricing-tier-top">
    <span class="md-pricing-tier-name">${escapeHtml(t.name)}</span>
    <span class="md-pricing-tier-price">${escapeHtml(t.price)}</span>
  </div>
  ${t.desc ? `<div class="md-pricing-tier-desc">${escapeHtml(t.desc)}</div>` : ''}
</div>`,
        )
        .join('')

      return `<div class="md-pricing-brand">
  <div class="md-pricing-brand-name">${escapeHtml(b.name)}</div>
  <div class="md-pricing-tiers">${tiers}</div>
</div>`
    })
    .join('')

  return `<div class="md-report-section md-pricing">
  <div class="md-section-head">
    <span class="md-section-title">定价对比</span>
    <span class="md-section-badge">Pricing</span>
  </div>
  <div class="md-pricing-grid">${brandHtml}</div>
</div>`
}

const PERSONA_LABELS: Record<string, string> = {
  人群: 'Audience',
  痛点: 'Pain Points',
  场景: 'Scenarios',
}

function parsePersonaFields(lines: string[]) {
  const fields: Array<{ key: string; sub: string; value: string }> = []

  for (const line of lines) {
    const trimmed = line.trim()
    const match = trimmed.match(/^[-*]\s+\*\*(.+?)\*\*[：:]\s*(.+)$/)
    if (!match) continue
    fields.push({
      key: match[1]!.trim(),
      sub: PERSONA_LABELS[match[1]!.trim()] ?? '',
      value: match[2]!.trim(),
    })
  }

  return fields
}

function renderPersonasHtml(body: string): string {
  const brands = parseBrandSections(body)
    .map((b) => ({ name: b.name, fields: parsePersonaFields(b.lines) }))
    .filter((b) => b.fields.length)

  if (!brands.length) {
    return md.render(`## 用户画像\n${body}`)
  }

  const brandHtml = brands
    .map((b) => {
      const rows = b.fields
        .map(
          (f) => `<div class="md-persona-row">
  <div class="md-persona-label">
    <span class="md-persona-key">${escapeHtml(f.key)}</span>
    ${f.sub ? `<span class="md-persona-sub">${f.sub}</span>` : ''}
  </div>
  <div class="md-persona-tags">${renderTagList(f.value)}</div>
</div>`,
        )
        .join('')

      return `<div class="md-persona-card">
  <div class="md-persona-brand">${escapeHtml(b.name)}</div>
  <div class="md-persona-body">${rows}</div>
</div>`
    })
    .join('')

  return `<div class="md-report-section md-personas">
  <div class="md-section-head">
    <span class="md-section-title">用户画像</span>
    <span class="md-section-badge">Persona</span>
  </div>
  <div class="md-persona-grid">${brandHtml}</div>
</div>`
}

export function renderMarkdown(text: string): string {
  if (!text.trim()) return ''
  return splitByH2(text.trim()).map(renderSection).join('\n')
}

export function toolDisplayName(name?: string): string {
  if (!name) return '工具调用'
  const key = name.replace(/^firecrawl_/, '')
  return TOOL_LABELS[key] ?? key.replace(/_/g, ' ')
}

export function stepTypeLabel(step: {
  type: string
  toolName?: string
  agentName?: string
  thinkingKind?: string
  parentAgent?: string
}): string {
  if (step.type === 'thinking') {
    if (step.thinkingKind === 'path') return '执行路径'
    if (step.thinkingKind === 'note') return '说明'
    if (step.thinkingKind === 'output') return '生成内容'
    if (step.thinkingKind === 'analysis') return '分析结果'
    return '推理'
  }
  if (step.type === 'agent' && step.agentName) {
    return AGENT_LABELS[step.agentName as AgentName] ?? step.agentName
  }
  return toolDisplayName(step.toolName)
}
