import type { Report } from '@/types'

function formatBool(v: boolean | string | undefined): string {
  if (v === true) return '✓'
  if (v === false) return '—'
  if (v === undefined || v === null || v === '') return '—'
  return String(v)
}

function swotSection(title: string, items: { text: string }[]): string[] {
  const lines = [`**${title}**`]
  if (!items.length) {
    lines.push('- （无）')
  } else {
    for (const it of items) {
      lines.push(`- ${it.text}`)
    }
  }
  return lines
}

/** 将 Report 转为 Chat 可直接渲染的 Markdown 全文。 */
export function reportToMarkdown(report: Report): string {
  const competitors = Object.keys(report.swot)
  const lines: string[] = [
    `# ${report.title}`,
    '',
    `> 生成时间：${new Date(report.generatedAt).toLocaleString('zh-CN')} · QA 评分：**${report.qaScore}**`,
    '',
    '## 执行摘要',
    '',
    report.summary,
    '',
  ]

  for (const name of competitors) {
    const swot = report.swot[name]
    if (!swot) continue
    lines.push(`## ${name} · SWOT`, '')
    lines.push(...swotSection('优势 (S)', swot.strengths), '')
    lines.push(...swotSection('劣势 (W)', swot.weaknesses), '')
    lines.push(...swotSection('机会 (O)', swot.opportunities), '')
    lines.push(...swotSection('威胁 (T)', swot.threats), '')
  }

  if (report.features.length) {
    lines.push('## 功能对比', '')
    const cols =
      competitors.length > 0
        ? competitors
        : [...new Set(report.features.flatMap((r) => Object.keys(r.values)))]
    lines.push(`| 功能 | ${cols.join(' | ')} |`)
    lines.push(`| --- | ${cols.map(() => '---').join(' | ')} |`)
    for (const row of report.features) {
      lines.push(
        `| ${row.feature} | ${cols.map((c) => formatBool(row.values[c] as boolean | string | undefined)).join(' | ')} |`,
      )
    }
    lines.push('')
  }

  if (report.pricing.length) {
    lines.push('## 定价对比', '')
    for (const p of report.pricing) {
      lines.push(`### ${p.competitor}`, '')
      for (const tier of p.tiers) {
        const feats = tier.features.length ? ` — ${tier.features.join('、')}` : ''
        lines.push(`- **${tier.name}** · ${tier.price}${feats}`)
      }
      lines.push('')
    }
  }

  if (report.personas.length) {
    lines.push('## 用户画像', '')
    for (const p of report.personas) {
      lines.push(`### ${p.competitor}`, '')
      if (p.segments.length) lines.push(`- **人群**：${p.segments.join('、')}`)
      if (p.painPoints.length) lines.push(`- **痛点**：${p.painPoints.join('、')}`)
      if (p.useCases.length) lines.push(`- **场景**：${p.useCases.join('、')}`)
      lines.push('')
    }
  }

  const sources = Object.entries(report.sources ?? {})
  if (sources.length) {
    lines.push('## 参考来源', '')
    for (const [id, src] of sources) {
      const title = src.title || src.url
      lines.push(`- [${title}](${src.url}) \`${id}\``)
      if (src.excerpt) {
        lines.push(`  > ${src.excerpt.replace(/\s+/g, ' ').slice(0, 160)}`)
      }
    }
  }

  return lines.join('\n').trim()
}
