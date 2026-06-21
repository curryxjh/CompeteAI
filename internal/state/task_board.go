package state

import (
	"CompeteAI/internal/domain"
	
	"context"
	"fmt"
	"strings"
	"time"
)

const defaultMaxRounds = 2

// TaskStore 语义化 Blackboard 访问（§8 TaskBlackboard）。
type TaskStore interface {
	// task.*
	SaveTaskMeta(ctx context.Context, meta TaskMeta) error
	LoadTaskMeta(ctx context.Context) (TaskMeta, error)
	SaveTaskStatus(ctx context.Context, snap TaskStatusSnapshot) error
	LoadTaskStatus(ctx context.Context) (TaskStatusSnapshot, error)

	// workflow.*
	SaveWorkflowState(ctx context.Context, wf WorkflowState) error
	LoadWorkflowState(ctx context.Context) (WorkflowState, error)
	SaveWorkflowPlan(ctx context.Context, plan WorkflowPlan) error
	LoadWorkflowPlan(ctx context.Context) (WorkflowPlan, error)
	SaveWorkflowRework(ctx context.Context, rework WorkflowRework) error
	LoadWorkflowRework(ctx context.Context) (WorkflowRework, error)

	// collector.*
	SaveCollectorOutput(ctx context.Context, out CollectorOutput) error
	LoadCollectorOutput(ctx context.Context) (CollectorOutput, error)

	// analysis.*
	SaveAnalysisOutput(ctx context.Context, out AnalysisOutput) error
	LoadAnalysisOutput(ctx context.Context) (AnalysisOutput, error)
	SaveAnalysisReviewNotes(ctx context.Context, notes AnalysisReviewNotes) error

	// report.*
	SaveReportDraft(ctx context.Context, draft ReportDraft) error
	LoadReportDraft(ctx context.Context) (ReportDraft, error)
	SaveReportFinal(ctx context.Context, report domain.Report) error
	LoadReportFinal(ctx context.Context) (domain.Report, error)
	SaveReportMeta(ctx context.Context, meta ReportMeta) error
	LoadReportMeta(ctx context.Context) (ReportMeta, error)

	// qa.*
	SaveQAResult(ctx context.Context, result QARecord) error
	LoadQAResult(ctx context.Context) (QARecord, error)
	LoadQAHistory(ctx context.Context) (QAHistory, error)

	// misc
	SaveClarificationAnswer(ctx context.Context, answer ClarificationAnswer) error
	ListKeys(ctx context.Context) ([]string, error)
}

type taskStore struct {
	taskID string
	bb     Blackboard
}

// ForTask 绑定任务 ID 的语义化 Store。
func ForTask(bb any, taskID string) TaskStore {
	blackboard, ok := bb.(Blackboard)
	if !ok {
		return &taskStore{bb: NewMemoryBlackboard(), taskID: taskID}
	}
	return &taskStore{bb: blackboard, taskID: taskID}
}

func (s *taskStore) SaveTaskMeta(ctx context.Context, meta TaskMeta) error {
	return s.bb.Put(ctx, TaskMetaKey(s.taskID), meta)
}

func (s *taskStore) LoadTaskMeta(ctx context.Context) (TaskMeta, error) {
	var meta TaskMeta
	return meta, s.bb.Get(ctx, TaskMetaKey(s.taskID), &meta)
}

func (s *taskStore) SaveTaskStatus(ctx context.Context, snap TaskStatusSnapshot) error {
	return s.bb.Put(ctx, TaskStatusKey(s.taskID), snap)
}

func (s *taskStore) LoadTaskStatus(ctx context.Context) (TaskStatusSnapshot, error) {
	var snap TaskStatusSnapshot
	return snap, s.bb.Get(ctx, TaskStatusKey(s.taskID), &snap)
}

func (s *taskStore) SaveWorkflowState(ctx context.Context, wf WorkflowState) error {
	if wf.MaxRounds == 0 {
		wf.MaxRounds = defaultMaxRounds
	}
	return s.bb.Put(ctx, WorkflowStateKey(s.taskID), wf)
}

func (s *taskStore) LoadWorkflowState(ctx context.Context) (WorkflowState, error) {
	var wf WorkflowState
	return wf, s.bb.Get(ctx, WorkflowStateKey(s.taskID), &wf)
}

func (s *taskStore) SaveWorkflowPlan(ctx context.Context, plan WorkflowPlan) error {
	var prev WorkflowPlan
	if err := s.bb.Get(ctx, WorkflowPlanKey(s.taskID), &prev); err == nil {
		plan.Version = prev.Version + 1
	} else {
		plan.Version = 1
	}
	wf, _ := s.LoadWorkflowState(ctx)
	plan.Round = wf.Round
	return s.bb.Put(ctx, WorkflowPlanKey(s.taskID), plan)
}

func (s *taskStore) LoadWorkflowPlan(ctx context.Context) (WorkflowPlan, error) {
	var plan WorkflowPlan
	return plan, s.bb.Get(ctx, WorkflowPlanKey(s.taskID), &plan)
}

func (s *taskStore) SaveWorkflowRework(ctx context.Context, rework WorkflowRework) error {
	wf, _ := s.LoadWorkflowState(ctx)
	rework.Round = wf.Round
	var prev WorkflowRework
	if err := s.bb.Get(ctx, WorkflowReworkKey(s.taskID), &prev); err == nil {
		rework.Version = prev.Version + 1
	} else {
		rework.Version = 1
	}
	return s.bb.Put(ctx, WorkflowReworkKey(s.taskID), rework)
}

func (s *taskStore) LoadWorkflowRework(ctx context.Context) (WorkflowRework, error) {
	var rework WorkflowRework
	return rework, s.bb.Get(ctx, WorkflowReworkKey(s.taskID), &rework)
}

func (s *taskStore) SaveCollectorOutput(ctx context.Context, out CollectorOutput) error {
	wf, _ := s.LoadWorkflowState(ctx)
	round := wf.Round
	if round == 0 {
		round = 1
	}

	var prev CollectorSummary
	if err := s.bb.Get(ctx, CollectorSummaryKey(s.taskID), &prev); err == nil {
		out.Version = prev.Version + 1
	} else {
		out.Version = 1
	}
	out.Round = round

	usable := 0
	for _, src := range out.Sources {
		if src.URL != "" {
			usable++
		}
	}

	_ = s.bb.Put(ctx, CollectorQueryKey(s.taskID), CollectorQuery{
		Query: out.Query, Lang: "zh", Limit: 5, Versioned: Versioned{Round: round, Version: out.Version},
	})
	_ = s.bb.Put(ctx, CollectorURLsKey(s.taskID), CollectorURLs{
		URLs: out.URLs, Versioned: Versioned{Round: round, Version: out.Version},
	})
	_ = s.bb.Put(ctx, CollectorSourcesKey(s.taskID), CollectorSources{
		Items: out.Sources, Versioned: Versioned{Round: round, Version: out.Version},
	})

	items := make([]MaterialItem, 0)
	if out.Materials != "" {
		items = append(items, MaterialItem{
			SourceID: "aggregate",
			Content:  out.Materials,
			Summary:  out.Summary,
		})
	}
	for id, src := range out.Sources {
		items = append(items, MaterialItem{
			SourceID: id,
			Content:  src.Excerpt,
			Summary:  src.Title,
			Tags:     []string{"source"},
		})
	}
	_ = s.bb.Put(ctx, CollectorMaterialsKey(s.taskID), CollectorMaterials{
		Items: items, Versioned: Versioned{Round: round, Version: out.Version},
	})
	return s.bb.Put(ctx, CollectorSummaryKey(s.taskID), CollectorSummary{
		SourceCount:       len(out.Sources),
		UsableSourceCount: usable,
		Notes:             out.Summary,
		Versioned:         Versioned{Round: round, Version: out.Version},
	})
}

func (s *taskStore) LoadCollectorOutput(ctx context.Context) (CollectorOutput, error) {
	var summary CollectorSummary
	if err := s.bb.Get(ctx, CollectorSummaryKey(s.taskID), &summary); err != nil {
		return CollectorOutput{}, err
	}
	var query CollectorQuery
	_ = s.bb.Get(ctx, CollectorQueryKey(s.taskID), &query)
	var urls CollectorURLs
	_ = s.bb.Get(ctx, CollectorURLsKey(s.taskID), &urls)
	var sources CollectorSources
	_ = s.bb.Get(ctx, CollectorSourcesKey(s.taskID), &sources)
	var materials CollectorMaterials
	_ = s.bb.Get(ctx, CollectorMaterialsKey(s.taskID), &materials)

	var b strings.Builder
	for _, item := range materials.Items {
		if item.SourceID == "aggregate" {
			b.WriteString(item.Content)
		}
	}
	materialsText := b.String()
	if materialsText == "" {
		for _, item := range materials.Items {
			b.WriteString(item.Content)
			b.WriteString("\n")
		}
		materialsText = b.String()
	}

	return CollectorOutput{
		Query:     query.Query,
		URLs:      urls.URLs,
		Sources:   sources.Items,
		Materials: materialsText,
		Summary:   summary.Notes,
		Versioned: summary.Versioned,
	}, nil
}

func (s *taskStore) SaveAnalysisOutput(ctx context.Context, out AnalysisOutput) error {
	wf, _ := s.LoadWorkflowState(ctx)
	out.Round = wf.Round
	if out.Round == 0 {
		out.Round = 1
	}

	var prev AnalysisOutput
	if err := s.bb.Get(ctx, AnalysisResultKey(s.taskID), &prev); err == nil {
		var history AnalysisHistory
		_ = s.bb.Get(ctx, AnalysisHistoryKey(s.taskID), &history)
		history.Entries = append(history.Entries, prev)
		_ = s.bb.Put(ctx, AnalysisHistoryKey(s.taskID), history)
		out.Version = prev.Version + 1
	} else {
		out.Version = 1
	}
	return s.bb.Put(ctx, AnalysisResultKey(s.taskID), out)
}

func (s *taskStore) LoadAnalysisOutput(ctx context.Context) (AnalysisOutput, error) {
	var out AnalysisOutput
	return out, s.bb.Get(ctx, AnalysisResultKey(s.taskID), &out)
}

func (s *taskStore) SaveAnalysisReviewNotes(ctx context.Context, notes AnalysisReviewNotes) error {
	wf, _ := s.LoadWorkflowState(ctx)
	notes.Round = wf.Round
	return s.bb.Put(ctx, AnalysisReviewNotesKey(s.taskID), notes)
}

func (s *taskStore) SaveReportDraft(ctx context.Context, draft ReportDraft) error {
	var prev ReportDraft
	if err := s.bb.Get(ctx, ReportDraftKey(s.taskID), &prev); err == nil {
		draft.Version = prev.Version + 1
	} else {
		draft.Version = 1
	}
	return s.bb.Put(ctx, ReportDraftKey(s.taskID), draft)
}

func (s *taskStore) LoadReportDraft(ctx context.Context) (ReportDraft, error) {
	var draft ReportDraft
	return draft, s.bb.Get(ctx, ReportDraftKey(s.taskID), &draft)
}

func (s *taskStore) SaveReportFinal(ctx context.Context, report domain.Report) error {
	var prev domain.Report
	version := 1
	if err := s.bb.Get(ctx, ReportFinalKey(s.taskID), &prev); err == nil {
		version = 2
	}
	_ = s.SaveReportMeta(ctx, ReportMeta{
		GeneratedAt: report.GeneratedAt,
		WrittenBy:   string(domain.AgentWriter),
		Versioned:   Versioned{Version: version},
	})
	return s.bb.Put(ctx, ReportFinalKey(s.taskID), report)
}

func (s *taskStore) LoadReportFinal(ctx context.Context) (domain.Report, error) {
	var report domain.Report
	return report, s.bb.Get(ctx, ReportFinalKey(s.taskID), &report)
}

func (s *taskStore) SaveReportMeta(ctx context.Context, meta ReportMeta) error {
	return s.bb.Put(ctx, ReportMetaKey(s.taskID), meta)
}

func (s *taskStore) LoadReportMeta(ctx context.Context) (ReportMeta, error) {
	var meta ReportMeta
	return meta, s.bb.Get(ctx, ReportMetaKey(s.taskID), &meta)
}

func (s *taskStore) SaveQAResult(ctx context.Context, result QARecord) error {
	wf, _ := s.LoadWorkflowState(ctx)
	result.Round = wf.Round
	if result.Round == 0 {
		result.Round = 1
	}

	var prev QARecord
	if err := s.bb.Get(ctx, QAResultKey(s.taskID), &prev); err == nil {
		result.Version = prev.Version + 1
	} else {
		result.Version = 1
	}

	var history QAHistory
	_ = s.bb.Get(ctx, QAHistoryKey(s.taskID), &history)
	history.Entries = append(history.Entries, QAHistoryEntry{
		Round:  result.Round,
		Score:  result.Score,
		Result: result.Result,
		Issues: result.Issues,
	})
	_ = s.bb.Put(ctx, QAHistoryKey(s.taskID), history)
	return s.bb.Put(ctx, QAResultKey(s.taskID), result)
}

func (s *taskStore) LoadQAResult(ctx context.Context) (QARecord, error) {
	var result QARecord
	return result, s.bb.Get(ctx, QAResultKey(s.taskID), &result)
}

func (s *taskStore) LoadQAHistory(ctx context.Context) (QAHistory, error) {
	var history QAHistory
	err := s.bb.Get(ctx, QAHistoryKey(s.taskID), &history)
	if err != nil {
		return QAHistory{Entries: []QAHistoryEntry{}}, nil
	}
	return history, nil
}

func (s *taskStore) SaveClarificationAnswer(ctx context.Context, answer ClarificationAnswer) error {
	return s.bb.Put(ctx, ClarificationAnswerKey(s.taskID), answer)
}

func (s *taskStore) ListKeys(ctx context.Context) ([]string, error) {
	return s.bb.ListByPrefix(ctx, TaskPrefix(s.taskID))
}

// IssuesToStrings 将 protocol Issue 转为摘要字符串列表。
func IssuesToStrings(issues []domain.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, iss := range issues {
		out = append(out, fmt.Sprintf("[%s] %s: %s", iss.Category, iss.Location, iss.Problem))
	}
	return out
}

// IssuesFromProtocol 转换 domain.Issue 到 state.Issue。
func IssuesFromProtocol(issues []domain.Issue) []Issue {
	out := make([]Issue, len(issues))
	for i, iss := range issues {
		out[i] = Issue{
			ID: iss.ID, Category: iss.Category, Location: iss.Location,
			Problem: iss.Problem, Suggestion: iss.Suggestion, Severity: iss.Severity,
		}
	}
	return out
}

// QARecordFromProtocol 从 domain.QAResult 构建 Blackboard QARecord。
func QARecordFromProtocol(qa domain.QAResult) QARecord {
	return QARecord{
		Score:       qa.Score,
		Result:      qa.Result,
		Issues:      IssuesToStrings(qa.Issues),
		IssuesFull:  IssuesFromProtocol(qa.Issues),
		TargetAgent: string(qa.TargetAgent),
		Reason:      qa.Reason,
	}
}

// NowRFC3339 当前时间字符串。
func NowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
