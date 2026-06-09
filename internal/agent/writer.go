package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"
)

type Writer struct{}

func NewWriter() *Writer { return &Writer{} }

func (a *Writer) Name() domain.AgentName { return domain.AgentWriter }

func (a *Writer) Card() AgentCard {
	return AgentCard{
		Name:            domain.AgentWriter,
		DisplayName:     "Writer",
		Description:     "报告撰写者：将分析结果合成为可读竞品报告",
		Skills:          []string{"报告撰写", "章节组织", "结论提炼"},
		Tools:           []string{"ReportComposer"},
		DependsOn:       []string{"analyst"},
		InputArtifacts:  []string{"analysis:result", "collector:sources", "task:meta"},
		OutputArtifacts: []string{"report:draft", "report:final"},
	}
}

func (a *Writer) Run(ctx context.Context, input RunInput, bb state.Blackboard) (RunOutput, error) {
	store := state.ForTask(bb, input.TaskID)

	meta, err := store.LoadTaskMeta(ctx)
	if err != nil {
		return RunOutput{}, &protocol.AgentError{Kind: protocol.ErrorKindFatal, Message: "task meta not found"}
	}
	analysis, err := store.LoadAnalysisOutput(ctx)
	if err != nil {
		return RunOutput{}, &protocol.AgentError{Kind: protocol.ErrorKindFatal, Message: "analysis result not found"}
	}
	coll, _ := store.LoadCollectorOutput(ctx)

	rework := ""
	if input.Payload != nil {
		if r, ok := input.Payload["rework_reason"].(string); ok {
			rework = r
		}
	}

	sourceIDs := make([]string, 0, len(coll.Sources))
	for id := range coll.Sources {
		sourceIDs = append(sourceIDs, id)
	}

	title := meta.Title
	sections := []string{"summary", "swot", "features", "pricing", "personas", "sources"}
	EmitProgress(ctx, "path", "摘要 → SWOT → 功能矩阵 → 定价 → 用户画像 → 来源", "done")
	if rework != "" && isSummaryOnlyRework(rework) {
		EmitProgress(ctx, "note", "局部重写：仅更新执行摘要", "running")
		report, err := store.LoadReportFinal(ctx)
		if err != nil {
			report = domain.Report{TaskID: meta.ID, Title: title}
		}
		report.Summary = buildReportSummary(analysis, coll.Sources)
		report.GeneratedAt = state.NowRFC3339()
		if err := store.SaveReportFinal(ctx, report); err != nil {
			return RunOutput{}, err
		}
		EmitProgress(ctx, "note", "执行摘要已局部更新", "done")
		return RunOutput{
			Status: domain.AgentRunCompleted, NextAgent: ptrAgent(domain.AgentQA),
			MessageType: string(protocol.MsgReportReady),
			Payload:     protocol.ReportPayload{Title: title, Summary: report.Summary, SourceCount: len(report.Sources)},
			Summary:     "报告摘要已局部重写",
		}, nil
	}
	EmitProgress(ctx, "note", "正在合成报告章节…", "running")

	if err := store.SaveReportDraft(ctx, state.ReportDraft{
		Title:     title,
		Summary:   analysis.Summary,
		Sections:  sections,
		SourceIDs: sourceIDs,
	}); err != nil {
		return RunOutput{}, err
	}
	savedDraft, _ := store.LoadReportDraft(ctx)

	report := domain.Report{
		TaskID:      meta.ID,
		Title:       title,
		GeneratedAt: state.NowRFC3339(),
		QAScore:     0,
		Summary:     buildReportSummary(analysis, coll.Sources),
		SWOT:        analysis.SWOT,
		Features:    analysis.Features,
		Pricing:     analysis.Pricing,
		Personas:    analysis.Personas,
		Sources:     coll.Sources,
	}
	if report.Sources == nil {
		report.Sources = map[string]domain.SourceRef{}
	}

	if err := store.SaveReportFinal(ctx, report); err != nil {
		return RunOutput{}, err
	}
	reportMeta, _ := store.LoadReportMeta(ctx)

	reportPayload := protocol.ReportPayload{
		Title:       title,
		Summary:     analysis.Summary,
		SourceCount: len(report.Sources),
	}

	EmitProgress(ctx, "note", fmt.Sprintf("报告已生成，共 %d 个章节、%d 条来源", len(sections), len(report.Sources)), "done")

	return RunOutput{
		Status:      domain.AgentRunCompleted,
		NextAgent:   ptrAgent(domain.AgentQA),
		MessageType: string(protocol.MsgReportReady),
		Payload:     reportPayload,
		Summary:     "报告草稿已生成",
		Artifacts: []protocol.ArtifactRef{
			protocol.NewArtifactRef(protocol.ArtifactReportDraft, state.ReportDraftKey(input.TaskID), savedDraft.Version),
			protocol.NewArtifactRef(protocol.ArtifactReportFinal, state.ReportFinalKey(input.TaskID), reportMeta.Version),
		},
	}, nil
}

func buildReportSummary(analysis state.AnalysisOutput, sources map[string]domain.SourceRef) string {
	var sb strings.Builder
	if strings.TrimSpace(analysis.Summary) != "" {
		sb.WriteString(analysis.Summary)
	} else {
		sb.WriteString("竞品分析报告")
	}
	if len(sources) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n本报告引用 %d 条来源。", len(sources)))
	}
	if len(analysis.Features) > 0 {
		sb.WriteString(fmt.Sprintf(" 功能对比 %d 项。", len(analysis.Features)))
	}
	if len(analysis.SWOT) > 0 {
		sb.WriteString(fmt.Sprintf(" 覆盖 %d 个竞品 SWOT。", len(analysis.SWOT)))
	}
	return sb.String()
}

func isSummaryOnlyRework(reason string) bool {
	lower := strings.ToLower(reason)
	if strings.Contains(lower, "summary") || strings.Contains(reason, "摘要") {
		return true
	}
	for _, kw := range []string{"report.summary", "执行摘要", "结构"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
