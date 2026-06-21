package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"
)

type Coordinator struct{}

func NewCoordinator() *Coordinator { return &Coordinator{} }

func (a *Coordinator) Name() domain.AgentName { return domain.AgentCoordinator }

func (a *Coordinator) Card() AgentCard {
	return AgentCard{
		Name:            domain.AgentCoordinator,
		DisplayName:     "Coordinator",
		Description:     "任务协调者：拆解竞品分析任务、路由 Agent、处理澄清循环",
		Skills:          []string{"任务拆解", "DAG 编排", "澄清对话"},
		Tools:           []string{"TaskPlanner", "StateRouter"},
		DependsOn:       []string{},
		InputArtifacts:  []string{"task:meta", "qa:result", "clarification:answer"},
		OutputArtifacts: []string{"workflow:plan", "workflow:next_agent"},
	}
}

func (a *Coordinator) Run(ctx context.Context, input domain.RunInput, bb any) (domain.RunOutput, error) {
	blackboard := bb.(state.Blackboard)
	store := state.ForTask(blackboard, input.TaskID)

	meta, err := store.LoadTaskMeta(ctx)
	if err != nil {
		return domain.RunOutput{}, &domain.AgentError{Kind: domain.ErrorKindFatal, Message: "task meta not found"}
	}

	wf, _ := store.LoadWorkflowState(ctx)
	planNotes := ""

	if len(meta.Competitors) < 1 {
		clarify := domain.ClarificationPayload{Question: "请提供至少一个竞品名称"}
		return domain.RunOutput{
			Status:      domain.AgentRunFailed,
			MessageType: string(domain.MsgClarificationRequired),
			Payload:     clarify,
			Summary:     clarify.Question,
		}, &domain.AgentError{
			Kind:    domain.ErrorKindClarificationRequired,
			Message: clarify.Question,
		}
	}

	if input.TriggerType == domain.TriggerClarificationAnswered {
		var ans state.ClarificationAnswer
		_ = blackboard.Get(ctx, state.ClarificationAnswerKey(input.TaskID), &ans)
		var question state.ClarificationQuestion
		_ = blackboard.Get(ctx, state.ClarificationQuestionKey(input.TaskID), &question)
		meta = state.MergeClarificationIntoMeta(meta, ans, question)
		_ = store.SaveTaskMeta(ctx, meta)
		wf.ClarificationResolved = true
		wf.NeedsClarification = false
		planNotes = fmt.Sprintf("用户澄清：%s", strings.TrimSpace(ans.Answer))
	}

	if len(meta.Competitors) < 1 {
		clarify := domain.ClarificationPayload{Question: "澄清后仍缺少竞品信息，请补充具体竞品名称"}
		return domain.RunOutput{
			Status:      domain.AgentRunFailed,
			MessageType: string(domain.MsgClarificationRequired),
			Payload:     clarify,
			Summary:     clarify.Question,
		}, &domain.AgentError{
			Kind:    domain.ErrorKindClarificationRequired,
			Message: clarify.Question,
		}
	}

	planSummary := fmt.Sprintf("对 %s 进行 %s 维度竞品分析",
		strings.Join(meta.Competitors, " vs "),
		strings.Join(meta.Dimensions, "、"))
	if planNotes != "" {
		planSummary = planSummary + "（" + planNotes + "）"
	}

	deliverables := []string{"SWOT 分析", "功能矩阵", "定价对比", "用户画像", "综合报告"}
	planPayload := domain.PlanPayload{
		Summary:      planSummary,
		Competitors:  meta.Competitors,
		Dimensions:   meta.Dimensions,
		Deliverables: deliverables,
	}
	_ = store.SaveWorkflowPlan(ctx, state.WorkflowPlan{
		Summary:      planSummary,
		Deliverables: deliverables,
		Dimensions:   meta.Dimensions,
		Competitors:  meta.Competitors,
		Notes:        planNotes,
	})

	next := domain.AgentCollector
	summary := "任务计划已生成，进入采集阶段"

	EmitProgress(ctx, "path", "Coordinator → Collector → Analyst → Writer → QA", "done")
	EmitProgress(ctx, "note", planSummary, "done")
	EmitProgress(ctx, "note", "交付物："+strings.Join(deliverables, "、"), "done")

	if input.TriggerType == domain.TriggerQAReject {
		rework, rerr := store.LoadWorkflowRework(ctx)
		if rerr == nil && rework.TargetAgent != "" {
			next = domain.AgentName(rework.TargetAgent)
			summary = fmt.Sprintf("QA 打回，重跑 %s", rework.TargetAgent)
		} else {
			next = domain.AgentAnalyst
			summary = "QA 打回，重跑分析"
		}
	}

	wf.CurrentAgent = string(domain.AgentCoordinator)
	wf.NextAgent = string(next)
	wf.Plan = planSummary
	wf.TraceID = input.TraceID
	wf.Attempt = input.Attempt
	if wf.Round == 0 {
		wf.Round = 1
	}
	_ = store.SaveWorkflowState(ctx, wf)

	plan, _ := store.LoadWorkflowPlan(ctx)
	planArtifact := domain.NewArtifactRef(domain.ArtifactPlan, state.WorkflowPlanKey(input.TaskID), plan.Version)

	return domain.RunOutput{
		Status:      domain.AgentRunCompleted,
		NextAgent:   ptrAgent(next),
		MessageType: string(domain.MsgPlanReady),
		Payload:     planPayload,
		Summary:     summary,
		Artifacts:   []domain.ArtifactRef{planArtifact},
	}, nil
}
