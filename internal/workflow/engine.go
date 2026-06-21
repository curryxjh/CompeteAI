package workflow

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/event"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/pkg/metrics"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/state"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultMaxRetry = 3

// Engine 工作流执行器：TaskService → WorkflowEngine → Agent[5]。
type Engine struct {
	registry        *agent.Registry
	tasks           repository.TaskRepository
	reports         repository.ReportRepository
	traces          repository.TraceRepository
	hub             *event.HybridHub
	bus             bus.Bus
	router          *bus.Router
	redis           redis.Cmdable
	useRedisBB      bool
	maxAgentRetries int
	maxRounds       int
	mem             memory.MemoryService
}

func NewEngine(
	registry *agent.Registry,
	tasks repository.TaskRepository,
	reports repository.ReportRepository,
	traces repository.TraceRepository,
	hub *event.HybridHub,
	bus bus.Bus,
	router *bus.Router,
	redisClient redis.Cmdable,
	useRedisBB bool,
	maxAgentRetries int,
	maxRounds int,
	mem memory.MemoryService,
) *Engine {
	if maxAgentRetries <= 0 {
		maxAgentRetries = defaultMaxRetry
	}
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}
	if mem == nil {
		mem = memory.NoopService{}
	}
	return &Engine{
		registry:        registry,
		tasks:           tasks,
		reports:         reports,
		traces:          traces,
		hub:             hub,
		bus:             bus,
		router:          router,
		redis:           redisClient,
		useRedisBB:      useRedisBB,
		maxAgentRetries: maxAgentRetries,
		maxRounds:       maxRounds,
		mem:             mem,
	}
}

// Run 兼容入口：初始化并发布 task.created，由 Worker 消息链驱动后续 Agent。
func (e *Engine) Run(ctx context.Context, taskID string) error {
	task, err := e.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	return e.EnqueueTask(ctx, task)
}

func (e *Engine) handleQAReject(ctx context.Context, store state.TaskStore, task *domain.Task, trace *domain.Trace, traceID string, payload any) (continueLoop bool, err error) {
	qaPayload := domain.QAResultPayload{}
	if p, ok := payload.(domain.QAResultPayload); ok {
		qaPayload = p
	}

	wf, _ := store.LoadWorkflowState(ctx)
	route := Route(RouteInput{
		MessageType: domain.MsgQAReject,
		FromAgent:   domain.AgentQA,
		QA:          &qaPayload,
		Workflow:    wf,
	})

	if route.Failed {
		_ = e.transitionTask(ctx, store, task.ID, task.Status, domain.TaskStatusFailed, task.Progress, route.Reason)
		e.appendWorkflowTrace(trace, TraceTaskFailed, domain.AgentQA, route.Reason)
		return false, nil
	}

	target := ReworkTargetFromQA(qaPayload)
	_ = e.transitionTask(ctx, store, task.ID, task.Status, domain.TaskStatusReworking, task.Progress, "")

	wf.Round++
	wf.RejectionCount++
	wf.NextAgent = string(target)
	_ = store.SaveWorkflowState(ctx, wf)

	issueStrs := make([]string, 0, len(qaPayload.Issues))
	for _, iss := range qaPayload.Issues {
		issueStrs = append(issueStrs, fmt.Sprintf("[%s] %s", iss.Category, iss.Problem))
	}
	_ = store.SaveWorkflowRework(ctx, state.WorkflowRework{
		TargetAgent: string(target),
		Reason:      qaPayload.Reason,
		Issues:      issueStrs,
		RequestedBy: string(domain.AgentQA),
		Round:       wf.Round,
	})

	e.markAgentRejected(task, target)
	_ = e.tasks.Update(ctx, *task)

	e.appendWorkflowTrace(trace, TraceQARejected, domain.AgentQA, qaPayload.Reason)
	e.appendWorkflowTrace(trace, TraceWorkflowReworked, target, fmt.Sprintf("round %d -> %s", wf.Round, target))

	e.hub.Publish(task.ID, "rejection", map[string]string{
		"fromAgent": string(domain.AgentQA),
		"toAgent":   string(target),
		"reason":    qaPayload.Reason,
	})
	e.emitTaskStatus(task.ID, domain.TaskStatusReworking, task.Progress)
	_ = e.router.PublishQAReject(ctx, task.ID, traceID, qaPayload)

	return true, nil
}

// Cancel 取消任务（持久化标记 + 状态驱动）。
func (e *Engine) Cancel(taskID string) {
	e.markCancelled(context.Background(), taskID)
}

func (e *Engine) cancelTask(ctx context.Context, taskID string, store state.TaskStore, trace domain.Trace, traceID, reason string) error {
	task, err := e.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status == domain.TaskStatusCompleted || task.Status == domain.TaskStatusFailed {
		return context.Canceled
	}
	_ = e.transitionTask(ctx, store, taskID, task.Status, domain.TaskStatusCancelled, task.Progress, reason)
	e.appendWorkflowTrace(&trace, TraceTaskFailed, domain.AgentCoordinator, reason)
	_ = e.traces.Save(ctx, trace)
	task.Status = domain.TaskStatusCancelled
	task.ErrorMessage = reason
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	_ = e.tasks.Update(ctx, task)
	e.hub.Publish(taskID, "task_failed", map[string]string{"message": reason})
	e.releaseBlackboard(taskID)
	return context.Canceled
}

// SubmitClarification 已废弃：请通过 TaskService.Clarify + outbox。
func (e *Engine) SubmitClarification(taskID, answer string) error {
	return e.applyClarification(context.Background(), taskID, answer)
}

func (e *Engine) transitionTask(ctx context.Context, store state.TaskStore, taskID string, from, to domain.TaskStatus, progress int, errMsg string) error {
	if from != to {
		if err := ValidateTransition(from, to); err != nil {
			return err
		}
	}
	task, err := e.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = to
	task.Progress = progress
	if errMsg != "" {
		task.ErrorMessage = errMsg
	}
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := e.tasks.Update(ctx, task); err != nil {
		return err
	}
	return store.SaveTaskStatus(ctx, state.TaskStatusSnapshot{
		Status: to, Progress: progress, ErrorMessage: errMsg,
	})
}

func (e *Engine) markAgentRejected(task *domain.Task, target domain.AgentName) {
	now := time.Now().Format(time.RFC3339)
	for i := range task.AgentStates {
		if task.AgentStates[i].Name != target {
			continue
		}
		task.AgentStates[i].Status = domain.AgentRunRejected
		task.AgentStates[i].FinishedAt = now
	}
}

func (e *Engine) runAgentWithRetry(ctx context.Context, ag domain.Agent, input domain.RunInput, bb state.Blackboard) (domain.RunOutput, error) {
	var lastErr error
	var lastOut domain.RunOutput
	for i := 0; i < e.maxAgentRetries; i++ {
		retryInput := input
		retryInput.Attempt = input.Attempt + i
		out, err := ag.Run(ctx, retryInput, bb)
		if err == nil {
			return out, nil
		}
		lastOut = out
		lastErr = err
		var ae *domain.AgentError
		if errors.As(err, &ae) {
			if ae.Kind == domain.ErrorKindClarificationRequired {
				return out, err
			}
			if ae.Retryable || ae.Kind == domain.ErrorKindRetryable {
				continue
			}
		}
		if out.NeedsRetry || out.Retryable {
			continue
		}
		return out, err
	}
	if lastErr != nil {
		return lastOut, lastErr
	}
	return lastOut, fmt.Errorf("agent %s failed after %d retries", ag.Name(), e.maxAgentRetries)
}

func (e *Engine) publishTaskStarted(taskID string, task domain.Task) {
	e.hub.Publish(taskID, "task_started", map[string]any{
		"status":      task.Status,
		"progress":    task.Progress,
		"agentStates": task.AgentStates,
	})
}

func (e *Engine) emitTaskStatus(taskID string, status domain.TaskStatus, progress int) {
	e.hub.Publish(taskID, "task_status", map[string]any{
		"status":   status,
		"progress": progress,
	})
}

func (e *Engine) emitAgentState(taskID string, name domain.AgentName, status domain.AgentRunStatus, progress int, message string) {
	e.hub.Publish(taskID, "agent_state", map[string]any{
		"name": name, "agent": name, "status": status,
		"progress": progress, "message": message,
	})
}

func (e *Engine) completeTask(ctx context.Context, taskID string, store state.TaskStore, trace domain.Trace, traceID string) error {
	report, err := store.LoadReportFinal(ctx)
	if err != nil {
		return err
	}
	report.TaskID = taskID
	if err := e.reports.Save(ctx, report); err != nil {
		return err
	}
	e.appendWorkflowTrace(&trace, TraceTaskCompleted, domain.AgentQA, "task completed")
	if err := e.traces.Save(ctx, trace); err != nil {
		return err
	}
	_ = e.router.PublishTaskCompleted(ctx, taskID, traceID, domain.TaskCompletedPayload{
		TaskID: taskID, Title: report.Title, QAScore: report.QAScore,
	})

	task, err := e.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	_ = e.transitionTask(ctx, store, taskID, task.Status, domain.TaskStatusCompleted, 100, "")
	task, _ = e.tasks.Get(ctx, taskID)
	for i := range task.AgentStates {
		task.AgentStates[i].Status = domain.AgentRunCompleted
	}
	_ = e.tasks.Update(ctx, task)

	e.hub.Publish(taskID, "task_complete", map[string]any{
		"status": domain.TaskStatusCompleted, "progress": 100,
	})
	metrics.IncTaskCompleted()
	e.ingestTaskCompletion(ctx, taskID, store, report.QAScore)
	e.releaseBlackboard(taskID)
	return nil
}

func (e *Engine) failTask(ctx context.Context, store state.TaskStore, task domain.Task, trace domain.Trace, traceID, agentName, msg string) error {
	_ = e.transitionTask(ctx, store, task.ID, task.Status, domain.TaskStatusFailed, task.Progress, msg)
	task.Status = domain.TaskStatusFailed
	task.ErrorMessage = msg
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	for i := range task.AgentStates {
		if task.AgentStates[i].Status == domain.AgentRunRunning {
			task.AgentStates[i].Status = domain.AgentRunFailed
		}
	}
	e.appendWorkflowTrace(&trace, TraceTaskFailed, domain.AgentName(agentName), msg)
	_ = e.traces.Save(ctx, trace)
	_ = e.router.PublishTaskFailed(ctx, task.ID, traceID, agentName, msg)
	if err := e.tasks.Update(ctx, task); err != nil {
		return err
	}
	e.hub.Publish(task.ID, "task_failed", map[string]string{"message": msg})
	metrics.IncTaskFailed()
	e.releaseBlackboard(task.ID)
	return nil
}

// releaseBlackboard 任务终态时释放内存 Blackboard（Redis 模式无需操作）。
func (e *Engine) releaseBlackboard(taskID string) {
	if !e.useRedisBB {
		state.GlobalRegistry().Release(taskID)
	}
}

func (e *Engine) setAgentRunning(_ context.Context, task *domain.Task, name domain.AgentName) {
	now := time.Now().Format(time.RFC3339)
	for i := range task.AgentStates {
		if task.AgentStates[i].Name != name {
			continue
		}
		if task.AgentStates[i].Status == domain.AgentRunRejected {
			task.AgentStates[i].StartedAt = now
		} else if task.AgentStates[i].StartedAt == "" {
			task.AgentStates[i].StartedAt = now
		}
		task.AgentStates[i].Status = domain.AgentRunRunning
		task.AgentStates[i].Message = ""
	}
	task.UpdatedAt = now
}

func (e *Engine) applyAgentStatus(_ context.Context, task *domain.Task, name domain.AgentName, out domain.RunOutput) {
	now := time.Now().Format(time.RFC3339)
	st := domain.AgentRunCompleted
	if out.Status == domain.AgentRunRejected {
		st = domain.AgentRunRejected
	}
	for i := range task.AgentStates {
		if task.AgentStates[i].Name != name {
			continue
		}
		task.AgentStates[i].Status = st
		task.AgentStates[i].Message = out.Summary
		task.AgentStates[i].FinishedAt = now
	}
	task.Progress = progressFor(name)
	task.UpdatedAt = now
}

func progressFor(name domain.AgentName) int {
	switch name {
	case domain.AgentCoordinator:
		return 15
	case domain.AgentCollector:
		return 40
	case domain.AgentAnalyst:
		return 65
	case domain.AgentWriter:
		return 85
	case domain.AgentQA:
		return 95
	default:
		return 50
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (e *Engine) appendMessageTrace(trace *domain.Trace, msg domain.MessageEnvelope, phase string) {
	action := domain.TraceAction(msg.MessageType, phase)
	node := domain.TraceNode{
		ID: fmt.Sprintf("m%d", len(trace.Nodes)+1), Agent: domain.AgentName(msg.FromAgent),
		Label: action, Status: domain.AgentRunCompleted, Output: string(msg.MessageType),
		Metadata: map[string]interface{}{
			"trace.action": action, "message_id": msg.MessageID,
			"message_type": string(msg.MessageType), "to_agent": msg.ToAgent,
		},
	}
	if phase == "failed" {
		node.Status = domain.AgentRunFailed
	}
	trace.Nodes = append(trace.Nodes, node)
}

func (e *Engine) appendWorkflowTrace(trace *domain.Trace, label string, agent domain.AgentName, output string) {
	trace.Nodes = append(trace.Nodes, domain.TraceNode{
		ID:     fmt.Sprintf("w%d", len(trace.Nodes)+1),
		Agent:  agent,
		Label:  label,
		Status: domain.AgentRunCompleted,
		Output: truncate(output, 300),
		Metadata: map[string]interface{}{
			"trace.action": label,
		},
	})
}
