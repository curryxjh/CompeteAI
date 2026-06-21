package agent

import (
	"CompeteAI/internal/domain"

	"github.com/google/uuid"
)

const qaPassThreshold = 70

// validateReportEvidence 对报告做证据驱动质检，返回扣分与问题列表。
func validateReportEvidence(report domain.Report, competitors []string) (deductions int, issues []domain.Issue) {
	if len(report.Sources) == 0 {
		return 0, nil
	}

	sourced, total := countSourcedConclusions(report)
	if total > 0 {
		ratio := float64(sourced) / float64(total)
		if ratio < 0.5 {
			deductions += 15
			issues = append(issues, domain.Issue{
				ID: uuid.NewString(), Category: domain.IssueMissingSourceRef,
				Location: "report.conclusions", Problem: "超过半数分析结论缺少 sourceIds 标注",
				Suggestion: "Analyst 需为每条 SWOT/功能结论标注来源", Severity: "high",
			})
		} else if ratio < 0.8 {
			deductions += 8
			issues = append(issues, domain.Issue{
				ID: uuid.NewString(), Category: domain.IssueMissingSourceRef,
				Location: "report.conclusions", Problem: "部分分析结论缺少 sourceIds 标注",
				Suggestion: "Analyst 需补全来源引用", Severity: "medium",
			})
		}
	}

	for _, comp := range competitors {
		swot, ok := report.SWOT[comp]
		if !ok {
			continue
		}
		issues, deductions = checkSWOTSources(comp, swot, report.Sources, issues, deductions)
	}

	for i, row := range report.Features {
		for comp, ids := range row.SourceIDs {
			if len(ids) == 0 {
				continue
			}
			if !sourceIDsValid(ids, report.Sources) {
				deductions += 3
				issues = append(issues, domain.Issue{
					ID: uuid.NewString(), Category: domain.IssueMissingSourceRef,
					Location: "report.features[" + row.Feature + "]." + comp,
					Problem: "功能对比来源 ID 无效",
					Suggestion: "Analyst 需使用有效 sourceIds", Severity: "low",
				})
				continue
			}
			val := row.Values[comp]
			if !claimSupportedBySourceIDs(formatFeatureValue(val)+" "+row.Feature, ids, report.Sources) {
				deductions += 5
				issues = append(issues, domain.Issue{
					ID: uuid.NewString(), Category: domain.IssueUnsupportedClaim,
					Location: "report.features[" + row.Feature + "]." + comp,
					Problem: "功能结论与引用来源 excerpt 不匹配",
					Suggestion: "Analyst 需修正结论或更换来源", Severity: "medium",
				})
			}
			_ = i
		}
	}

	if report.Summary != "" && len(report.Sources) > 0 {
		if !claimSupportedBySources(report.Summary, report.Sources) {
			deductions += 10
			issues = append(issues, domain.Issue{
				ID: uuid.NewString(), Category: domain.IssueUnsupportedClaim,
				Location: "report.summary", Problem: "执行摘要缺少来源支撑",
				Suggestion: "Writer 需基于有来源依据的内容重写摘要", Severity: "medium",
			})
		}
	}

	return deductions, issues
}

func checkSWOTSources(comp string, swot domain.SWOTAnalysis, sources map[string]domain.SourceRef, issues []domain.Issue, deductions int) ([]domain.Issue, int) {
	quadrants := []struct {
		name  string
		items []domain.SWOTItem
	}{
		{"strengths", swot.Strengths},
		{"weaknesses", swot.Weaknesses},
		{"opportunities", swot.Opportunities},
		{"threats", swot.Threats},
	}
	for _, q := range quadrants {
		for _, it := range q.items {
			if len(it.SourceIDs) == 0 {
				deductions += 2
				issues = append(issues, domain.Issue{
					ID: uuid.NewString(), Category: domain.IssueMissingSourceRef,
					Location: "report.swot." + comp + "." + q.name,
					Problem: "SWOT 条目缺少 sourceIds: " + truncate(it.Text, 40),
					Suggestion: "Analyst 需标注来源", Severity: "medium",
				})
				continue
			}
			if !claimSupportedBySourceIDs(it.Text, it.SourceIDs, sources) {
				deductions += 5
				issues = append(issues, domain.Issue{
					ID: uuid.NewString(), Category: domain.IssueUnsupportedClaim,
					Location: "report.swot." + comp + "." + q.name,
					Problem: "SWOT 结论与引用来源 excerpt 不匹配: " + truncate(it.Text, 40),
					Suggestion: "Analyst 需修正表述或更换来源", Severity: "medium",
				})
			}
		}
	}
	return issues, deductions
}

func sourceIDsValid(ids []string, sources map[string]domain.SourceRef) bool {
	for _, id := range ids {
		if _, ok := sources[id]; !ok {
			return false
		}
	}
	return len(ids) > 0
}

func claimSupportedBySourceIDs(text string, ids []string, sources map[string]domain.SourceRef) bool {
	subset := make(map[string]domain.SourceRef, len(ids))
	for _, id := range ids {
		if src, ok := sources[id]; ok {
			subset[id] = src
		}
	}
	if len(subset) == 0 {
		return false
	}
	return claimSupportedBySources(text, subset)
}
