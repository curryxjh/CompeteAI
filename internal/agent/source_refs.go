package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"fmt"
	"sort"
	"strings"
)

// formatSourceCatalog 将来源目录格式化为 Analyst Prompt 附录。
func formatSourceCatalog(sources map[string]domain.SourceRef) string {
	if len(sources) == 0 {
		return "\n\n（暂无采集来源，sourceIds 可留空数组）"
	}
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var sb strings.Builder
	sb.WriteString("\n\n## 可用来源目录（sourceIds 必须从此列表选取）\n")
	for _, id := range ids {
		src := sources[id]
		title := src.Title
		if title == "" {
			title = src.URL
		}
		sb.WriteString(fmt.Sprintf("- %s: %s — %s\n", id, title, truncate(src.Excerpt, 120)))
	}
	return sb.String()
}

const analystSystemPrompt = `你是竞品分析专家。输出严格 JSON，不要 markdown 代码块。
每条 SWOT 结论、功能对比行必须标注 sourceIds（来自来源目录中的 id）。
功能树 featureTree 按竞品组织，节点可嵌套 children 表示层级功能。

{
  "summary": "执行摘要",
  "swot": {
    "竞品名": {
      "strengths": [{"text": "...", "sourceIds": ["s1"]}],
      "weaknesses": [],
      "opportunities": [],
      "threats": []
    }
  },
  "features": [{
    "feature": "功能名",
    "values": {"竞品A": true, "竞品B": false},
    "sourceIds": {"竞品A": ["s1"], "竞品B": ["s2"]}
  }],
  "featureTree": {
    "竞品名": [{
      "name": "核心功能",
      "description": "说明",
      "supported": true,
      "sourceIds": ["s1"],
      "children": [{"name": "子功能", "supported": true, "sourceIds": ["s1"]}]
    }]
  },
  "pricing": [{
    "competitor": "名",
    "tiers": [{"name": "套餐", "price": "$10/月", "features": []}],
    "sourceIds": ["s1"]
  }],
  "personas": [{
    "competitor": "名",
    "segments": [],
    "painPoints": [],
    "useCases": [],
    "sourceIds": ["s1"]
  }]
}`

// enrichAnalysisSources 校验 sourceIds 合法性，并对缺失项尝试自动关联。
func enrichAnalysisSources(out *state.AnalysisOutput, sources map[string]domain.SourceRef) {
	if out == nil {
		return
	}
	validIDs := sourceIDSet(sources)
	defaultID := firstSourceID(sources)

	for name, swot := range out.SWOT {
		swot.Strengths = enrichSWOTItems(swot.Strengths, validIDs, defaultID, sources)
		swot.Weaknesses = enrichSWOTItems(swot.Weaknesses, validIDs, defaultID, sources)
		swot.Opportunities = enrichSWOTItems(swot.Opportunities, validIDs, defaultID, sources)
		swot.Threats = enrichSWOTItems(swot.Threats, validIDs, defaultID, sources)
		out.SWOT[name] = swot
	}

	for i := range out.Features {
		if out.Features[i].SourceIDs == nil {
			out.Features[i].SourceIDs = map[string][]string{}
		}
		for comp, val := range out.Features[i].Values {
			ids := sanitizeSourceIDs(out.Features[i].SourceIDs[comp], validIDs)
			if len(ids) == 0 {
				ids = inferSourceIDs(fmt.Sprintf("%s %v", out.Features[i].Feature, val), sources, defaultID)
			}
			out.Features[i].SourceIDs[comp] = ids
		}
	}

	for comp, nodes := range out.FeatureTree {
		out.FeatureTree[comp] = enrichFeatureTreeNodes(nodes, validIDs, defaultID, sources)
	}

	for i := range out.Pricing {
		out.Pricing[i].SourceIDs = sanitizeSourceIDs(out.Pricing[i].SourceIDs, validIDs)
		if len(out.Pricing[i].SourceIDs) == 0 {
			text := out.Pricing[i].Competitor
			for _, t := range out.Pricing[i].Tiers {
				text += " " + t.Name + " " + t.Price
			}
			out.Pricing[i].SourceIDs = inferSourceIDs(text, sources, defaultID)
		}
	}

	for i := range out.Personas {
		out.Personas[i].SourceIDs = sanitizeSourceIDs(out.Personas[i].SourceIDs, validIDs)
		if len(out.Personas[i].SourceIDs) == 0 {
			text := strings.Join(append(out.Personas[i].Segments, out.Personas[i].PainPoints...), " ")
			out.Personas[i].SourceIDs = inferSourceIDs(text, sources, defaultID)
		}
	}
}

func enrichSWOTItems(items []domain.SWOTItem, valid map[string]struct{}, defaultID string, sources map[string]domain.SourceRef) []domain.SWOTItem {
	out := make([]domain.SWOTItem, len(items))
	for i, it := range items {
		ids := sanitizeSourceIDs(it.SourceIDs, valid)
		if len(ids) == 0 {
			ids = inferSourceIDs(it.Text, sources, defaultID)
		}
		out[i] = domain.SWOTItem{Text: it.Text, SourceIDs: ids}
	}
	return out
}

func enrichFeatureTreeNodes(nodes []domain.FeatureTreeNode, valid map[string]struct{}, defaultID string, sources map[string]domain.SourceRef) []domain.FeatureTreeNode {
	out := make([]domain.FeatureTreeNode, len(nodes))
	for i, n := range nodes {
		ids := sanitizeSourceIDs(n.SourceIDs, valid)
		if len(ids) == 0 {
			ids = inferSourceIDs(n.Name+" "+n.Description, sources, defaultID)
		}
		out[i] = domain.FeatureTreeNode{
			Name: n.Name, Description: n.Description,
			Supported: n.Supported, SourceIDs: ids,
			Children: enrichFeatureTreeNodes(n.Children, valid, defaultID, sources),
		}
	}
	return out
}

func sourceIDSet(sources map[string]domain.SourceRef) map[string]struct{} {
	set := make(map[string]struct{}, len(sources))
	for id := range sources {
		set[id] = struct{}{}
	}
	return set
}

func firstSourceID(sources map[string]domain.SourceRef) string {
	for id := range sources {
		return id
	}
	return ""
}

func sanitizeSourceIDs(ids []string, valid map[string]struct{}) []string {
	if len(valid) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := valid[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func inferSourceIDs(text string, sources map[string]domain.SourceRef, fallback string) []string {
	if len(sources) == 0 {
		return nil
	}
	words := significantWords(text)
	if len(words) == 0 {
		if fallback != "" {
			return []string{fallback}
		}
		return nil
	}
	type scored struct {
		id    string
		score int
	}
	var hits []scored
	for id, src := range sources {
		corpus := strings.ToLower(src.Excerpt + " " + src.Title + " " + src.URL)
		score := 0
		for _, w := range words {
			if strings.Contains(corpus, w) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{id: id, score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) == 0 {
		if fallback != "" {
			return []string{fallback}
		}
		return nil
	}
	return []string{hits[0].id}
}

func significantWords(text string) []string {
	repl := strings.NewReplacer("，", " ", "。", " ", "、", " ", "；", " ", "：", " ", "(", " ", ")", " ")
	raw := strings.Fields(repl.Replace(strings.ToLower(text)))
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if len([]rune(w)) >= 2 {
			out = append(out, w)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// countSourcedConclusions 统计已标注来源的结论数量与总数。
func countSourcedConclusions(report domain.Report) (sourced, total int) {
	for _, swot := range report.SWOT {
		for _, items := range [][]domain.SWOTItem{swot.Strengths, swot.Weaknesses, swot.Opportunities, swot.Threats} {
			for _, it := range items {
				total++
				if len(it.SourceIDs) > 0 {
					sourced++
				}
			}
		}
	}
	for _, row := range report.Features {
		for _, ids := range row.SourceIDs {
			total++
			if len(ids) > 0 {
				sourced++
			}
		}
	}
	return sourced, total
}
