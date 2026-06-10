package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"fmt"
	"strings"
)

// FormatAnalysisMarkdown 将 Analyst 结构化结果转为可读 Markdown（供 Chat 展示）。
func FormatAnalysisMarkdown(competitors []string, out state.AnalysisOutput) string {
	var sb strings.Builder
	sb.WriteString("### 执行摘要\n\n")
	if strings.TrimSpace(out.Summary) != "" {
		sb.WriteString(out.Summary)
	} else {
		sb.WriteString("（暂无摘要）")
	}
	sb.WriteString("\n")

	for _, name := range competitors {
		swot, ok := out.SWOT[name]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n### %s · SWOT\n\n", name))
		writeSWOTSection(&sb, "优势 (S)", swot.Strengths)
		writeSWOTSection(&sb, "劣势 (W)", swot.Weaknesses)
		writeSWOTSection(&sb, "机会 (O)", swot.Opportunities)
		writeSWOTSection(&sb, "威胁 (T)", swot.Threats)
	}

	if len(out.Features) > 0 {
		sb.WriteString("\n### 功能对比\n\n")
		cols := competitors
		if len(cols) == 0 {
			cols = featureCompetitors(out.Features)
		}
		sb.WriteString("| 功能 |")
		for _, c := range cols {
			sb.WriteString(" " + c + " |")
		}
		sb.WriteString("\n| --- |")
		for range cols {
			sb.WriteString(" --- |")
		}
		sb.WriteString("\n")
		for _, row := range out.Features {
			sb.WriteString("| " + row.Feature + " |")
			for _, c := range cols {
				sb.WriteString(" " + formatFeatureValue(row.Values[c]) + " |")
			}
			sb.WriteString("\n")
		}
	}

	if len(out.Pricing) > 0 {
		sb.WriteString("\n### 定价对比\n\n")
		for _, p := range out.Pricing {
			sb.WriteString(fmt.Sprintf("**%s**\n\n", p.Competitor))
			for _, tier := range p.Tiers {
				sb.WriteString(fmt.Sprintf("- **%s** · %s", tier.Name, tier.Price))
				if len(tier.Features) > 0 {
					sb.WriteString(" — " + strings.Join(tier.Features, "、"))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	if len(out.Personas) > 0 {
		sb.WriteString("### 用户画像\n\n")
		for _, p := range out.Personas {
			sb.WriteString(fmt.Sprintf("**%s**\n", p.Competitor))
			if len(p.Segments) > 0 {
				sb.WriteString("- 人群：" + strings.Join(p.Segments, "、") + "\n")
			}
			if len(p.PainPoints) > 0 {
				sb.WriteString("- 痛点：" + strings.Join(p.PainPoints, "、") + "\n")
			}
			if len(p.UseCases) > 0 {
				sb.WriteString("- 场景：" + strings.Join(p.UseCases, "、") + "\n")
			}
			sb.WriteString("\n")
		}
	}

	if len(out.FeatureTree) > 0 {
		sb.WriteString("### 功能树\n\n")
		for comp, nodes := range out.FeatureTree {
			sb.WriteString(fmt.Sprintf("**%s**\n", comp))
			writeFeatureTree(&sb, nodes, 0)
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func writeFeatureTree(sb *strings.Builder, nodes []domain.FeatureTreeNode, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, n := range nodes {
		flag := "?"
		if n.Supported != nil {
			if *n.Supported {
				flag = "✓"
			} else {
				flag = "✗"
			}
		}
		sb.WriteString(fmt.Sprintf("%s- [%s] %s", indent, flag, n.Name))
		if n.Description != "" {
			sb.WriteString(" — " + n.Description)
		}
		sb.WriteString("\n")
		if len(n.Children) > 0 {
			writeFeatureTree(sb, n.Children, depth+1)
		}
	}
}

func writeSWOTSection(sb *strings.Builder, title string, items []domain.SWOTItem) {
	sb.WriteString("**" + title + "**\n")
	if len(items) == 0 {
		sb.WriteString("- （无）\n")
		return
	}
	for _, it := range items {
		sb.WriteString("- " + it.Text + "\n")
	}
	sb.WriteString("\n")
}

func formatFeatureValue(v interface{}) string {
	if v == nil {
		return "—"
	}
	switch t := v.(type) {
	case bool:
		if t {
			return "✓"
		}
		return "—"
	case string:
		if t == "" {
			return "—"
		}
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func featureCompetitors(rows []domain.FeatureRow) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, row := range rows {
		for k := range row.Values {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
}
