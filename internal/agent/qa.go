package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type QA struct{}

func NewQA() *QA { return &QA{} }

func (a *QA) Name() domain.AgentName { return domain.AgentQA }

func (a *QA) Card() AgentCard {
	return AgentCard{
		Name:            domain.AgentQA,
		DisplayName:     "QA",
		Description:     "质检者：事实校验、Schema 合规、打回重做闭环",
		Skills:          []string{"事实核验", "Schema 校验", "打回路由"},
		Tools:           []string{"FactChecker", "SchemaValidator"},
		DependsOn:       []string{"writer"},
		InputArtifacts:  []string{"report:final", "collector:sources", "analysis:result"},
		OutputArtifacts: []string{"qa:result"},
	}
}

func (a *QA) Run(ctx context.Context, input domain.RunInput, bb any) (domain.RunOutput, error) {
	blackboard := bb.(state.Blackboard)
	store := state.ForTask(blackboard, input.TaskID)

	report, err := store.LoadReportFinal(ctx)
	if err != nil {
		return domain.RunOutput{}, &domain.AgentError{Kind: domain.ErrorKindFatal, Message: "report not found"}
	}

	meta, _ := store.LoadTaskMeta(ctx)

	EmitProgress(ctx, "path", "结构校验 → SWOT 覆盖 → 功能矩阵 → 来源追溯 → 评分", "running")

	score := 100
	var issues []domain.Issue

	if report.Summary == "" {
		score -= 20
		EmitProgress(ctx, "note", "✗ 缺少执行摘要", "done")
		issues = append(issues, domain.Issue{
			ID: uuid.NewString(), Category: domain.IssueReportStructure,
			Location: "report.summary", Problem: "缺少执行摘要",
			Suggestion: "Writer 需补充 summary 字段", Severity: "high",
		})
	}
	if len(report.SWOT) < len(meta.Competitors) {
		score -= 15
		issues = append(issues, domain.Issue{
			ID: uuid.NewString(), Category: domain.IssueAnalysisIncomplete,
			Location: "report.swot", Problem: "SWOT 未覆盖全部竞品",
			Suggestion: "Analyst 需补全各竞品 SWOT", Severity: "medium",
		})
	}
	if len(report.Features) < 2 {
		score -= 15
		issues = append(issues, domain.Issue{
			ID: uuid.NewString(), Category: domain.IssueAnalysisIncomplete,
			Location: "report.features", Problem: "功能矩阵条目不足",
			Suggestion: "Analyst 需增加功能对比行", Severity: "medium",
		})
	}
	if len(report.Sources) == 0 {
		score -= 10
		issues = append(issues, domain.Issue{
			ID: uuid.NewString(), Category: domain.IssueSourceMissing,
			Location: "report.sources", Problem: "缺少来源引用",
			Suggestion: "Collector 需补充可追溯来源", Severity: "low",
		})
	}

	deductions, evidenceIssues := validateReportEvidence(report, meta.Competitors)
	score -= deductions
	issues = append(issues, evidenceIssues...)
	for _, iss := range evidenceIssues {
		if iss.Category == domain.IssueUnsupportedClaim {
			EmitProgress(ctx, "note", "证据校验："+iss.Problem, "done")
		}
	}

	result := domain.QAResultPass
	target := domain.AgentName("")
	reason := ""
	msgType := string(domain.MsgQAPass)
	status := domain.AgentRunCompleted

	wf, _ := store.LoadWorkflowState(ctx)
	maxRounds := wf.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 3
	}

	if score < 70 {
		result = domain.QAResultReject
		target = qaTargetAgentForIssues(issues)
		if len(issues) > 0 {
			reason = fmt.Sprintf("质检未通过(%d分): %s", score, issues[0].Problem)
		} else {
			reason = fmt.Sprintf("质检未通过(%d分)", score)
		}
		msgType = string(domain.MsgQAReject)
		status = domain.AgentRunRejected
	}

	qaOut := domain.QAResult{
		Score: score, Result: result, Issues: issues,
		TargetAgent: target, Reason: reason,
	}
	_ = store.SaveQAResult(ctx, state.QARecordFromProtocol(qaOut))

	report.QAScore = score
	_ = store.SaveReportFinal(ctx, report)

	qaPayload := qaOut.ToPayload()
	out := domain.RunOutput{
		Status: status, MessageType: msgType, Payload: qaPayload,
		Summary: fmt.Sprintf("QA 得分 %d，结果 %s", score, result),
		Artifacts: []domain.ArtifactRef{
			domain.NewArtifactRef(domain.ArtifactQAReview, state.QAResultKey(input.TaskID), 1),
		},
		Metadata: map[string]any{
			"qa.score": score, "qa.result": result, "qa.round": wf.Round, "qa.max_rounds": maxRounds,
		},
	}

	if result == domain.QAResultReject {
		out.NextAgent = ptrAgent(domain.AgentCoordinator)
		EmitProgress(ctx, "note", fmt.Sprintf("质检未通过(%d分)，打回 %s", score, target), "done")
	} else {
		EmitProgress(ctx, "note", fmt.Sprintf("质检通过，得分 %d", score), "done")
	}
	return out, nil
}

// qaTargetAgentForIssues §9 打回规则表（与 workflow.TargetAgentForIssues 保持一致）。
func qaTargetAgentForIssues(issues []domain.Issue) domain.AgentName {
	priority := []struct {
		category string
		agent    domain.AgentName
	}{
		{domain.IssueSourceMissing, domain.AgentCollector},
		{domain.IssueMissingSourceRef, domain.AgentAnalyst},
		{domain.IssueAnalysisIncomplete, domain.AgentAnalyst},
		{domain.IssuePricingMissing, domain.AgentAnalyst},
		{domain.IssueUnsupportedClaim, domain.AgentAnalyst},
		{domain.IssueReportStructure, domain.AgentWriter},
	}
	for _, p := range priority {
		for _, iss := range issues {
			if iss.Category == p.category {
				return p.agent
			}
		}
	}
	return domain.AgentAnalyst
}

func claimSupportedBySources(summary string, sources map[string]domain.SourceRef) bool {
	words := strings.Fields(strings.NewReplacer("，", " ", "。", " ", "、", " ").Replace(summary))
	hits := 0
	checked := 0
	for _, w := range words {
		if len([]rune(w)) < 4 {
			continue
		}
		checked++
		for _, src := range sources {
			if strings.Contains(strings.ToLower(src.Excerpt), strings.ToLower(w)) ||
				strings.Contains(strings.ToLower(src.Title), strings.ToLower(w)) {
				hits++
				break
			}
		}
		if checked >= 5 {
			break
		}
	}
	if checked == 0 {
		return true
	}
	return hits >= 1
}
