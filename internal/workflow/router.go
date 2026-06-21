package workflow

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
)

// RouteInput Router 输入（§11）。
type RouteInput struct {
	MessageType domain.MessageType
	FromAgent   domain.AgentName
	QA          *domain.QAResultPayload
	Workflow    state.WorkflowState
}

// RouteOutput Router 输出（§11）。
type RouteOutput struct {
	NextAgent  domain.AgentName
	TaskStatus domain.TaskStatus
	Done       bool
	Failed     bool
	Reason     string
}

// Route 根据消息与工作流状态决定下一跳（§12）。
func Route(in RouteInput) RouteOutput {
	switch in.MessageType {
	case domain.MsgQAPass:
		return RouteOutput{
			TaskStatus: domain.TaskStatusCompleted,
			Done:       true,
		}
	case domain.MsgQAReject:
		return RouteQAReject(in.QA, in.Workflow)
	case domain.MsgClarificationRequired:
		return RouteOutput{TaskStatus: domain.TaskStatusClarifying}
	case domain.MsgClarificationAnswered:
		return RouteOutput{
			NextAgent:  domain.AgentCoordinator,
			TaskStatus: domain.TaskStatusRunning,
		}
	case domain.MsgTaskFailed:
		return RouteOutput{
			TaskStatus: domain.TaskStatusFailed,
			Failed:     true,
			Reason:     "agent fatal error",
		}
	}

	if next, ok := NextAgentInPipeline(in.FromAgent); ok {
		return RouteOutput{NextAgent: next, TaskStatus: domain.TaskStatusRunning}
	}
	if in.FromAgent == domain.AgentQA {
		return RouteOutput{TaskStatus: domain.TaskStatusCompleted, Done: true}
	}
	return RouteOutput{TaskStatus: domain.TaskStatusRunning}
}

// RouteQAReject QA 打回路由（§12.2）。
func RouteQAReject(qa *domain.QAResultPayload, wf state.WorkflowState) RouteOutput {
	if qa == nil {
		return RouteOutput{
			NextAgent:  domain.AgentCoordinator,
			TaskStatus: domain.TaskStatusReworking,
		}
	}

	maxRounds := wf.MaxRounds
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}
	if wf.Round >= maxRounds {
		return RouteOutput{
			TaskStatus: domain.TaskStatusFailed,
			Failed:     true,
			Reason:     "超过最大打回轮次",
		}
	}

	target := domain.AgentName(qa.TargetAgent)
	if target == "" {
		target = TargetAgentForIssues(qa.Issues)
	}

	return RouteOutput{
		NextAgent:  target,
		TaskStatus: domain.TaskStatusReworking,
	}
}

// TargetAgentForIssues §9 QA 打回规则表：按问题类别路由到单一目标 Agent。
func TargetAgentForIssues(issues []domain.Issue) domain.AgentName {
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

// ReworkTargetFromQA 解析 QA 打回目标 Agent。
func ReworkTargetFromQA(qa domain.QAResultPayload) domain.AgentName {
	if qa.TargetAgent != "" {
		return domain.AgentName(qa.TargetAgent)
	}
	return TargetAgentForIssues(qa.Issues)
}
