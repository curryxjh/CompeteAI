package workflow

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrAwaitingClarification = fmt.Errorf("awaiting user clarification")

// contextKeyUserID 用于在 context 中传递登录用户 ID。
type contextKeyUserID struct{}

// WithUserID 将 userID 注入 context，供后续记忆作用域使用。
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, contextKeyUserID{}, userID)
}

// UserIDFromCtx 从 context 取出 userID（未设置时返回 0）。
func UserIDFromCtx(ctx context.Context) int64 {
	v, _ := ctx.Value(contextKeyUserID{}).(int64)
	return v
}

func taskCancelKey(taskID string) string {
	return fmt.Sprintf("compete:task:%s:cancelled", taskID)
}

func (e *Engine) blackboard(taskID string) state.Blackboard {
	return state.NewBlackboard(e.useRedisBB, e.redis, taskID)
}

func (e *Engine) isCancelled(ctx context.Context, taskID string) bool {
	if e.redis == nil {
		return false
	}
	v, err := e.redis.Get(ctx, taskCancelKey(taskID)).Result()
	return err == nil && v == "1"
}

func (e *Engine) markCancelled(ctx context.Context, taskID string) {
	if e.redis == nil {
		return
	}
	_ = e.redis.Set(ctx, taskCancelKey(taskID), "1", 24*time.Hour).Err()
}

func (e *Engine) clearCancelled(ctx context.Context, taskID string) {
	if e.redis == nil {
		return
	}
	_ = e.redis.Del(ctx, taskCancelKey(taskID)).Err()
}

func (e *Engine) withIdempotent(ctx context.Context, msg protocol.MessageEnvelope, fn func() error) error {
	if e.redis != nil && msg.MessageID != "" {
		ok, err := e.redis.SetNX(ctx, bus.DedupKey(msg.MessageID), "1", 24*time.Hour).Result()
		if err == nil && !ok {
			return nil
		}
	}
	return fn()
}

// PrepareTask 初始化 Blackboard（Worker 消费时调用）。
func (e *Engine) PrepareTask(ctx context.Context, task domain.Task, traceID string) (string, error) {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	// 将任务归属用户注入 context，使记忆模块能关联用户级作用域
	if task.UserID > 0 {
		ctx = WithUserID(ctx, task.UserID)
	}
	bb := e.blackboard(task.ID)
	store := state.ForTask(bb, task.ID)
	_ = store.SaveTaskMeta(ctx, state.TaskMetaFromDomain(task))
	_ = store.SaveWorkflowState(ctx, state.WorkflowState{
		TraceID: traceID, Round: 1, MaxRounds: e.maxRounds,
		CurrentAgent: string(domain.AgentCoordinator),
	})
	_ = e.transitionTask(ctx, store, task.ID, task.Status, domain.TaskStatusQueued, task.Progress, "")
	e.clearCancelled(ctx, task.ID)
	e.ensureMemoryScopes(ctx, task)
	return traceID, nil
}

// TraceIDForTask 读取 blackboard 中的 traceID（API 命令用）。
func (e *Engine) TraceIDForTask(ctx context.Context, taskID string) string {
	store := state.ForTask(e.blackboard(taskID), taskID)
	wf, err := store.LoadWorkflowState(ctx)
	if err == nil && wf.TraceID != "" {
		return wf.TraceID
	}
	return uuid.NewString()
}

// ProcessClarification 消费 clarify 命令（Worker）。
func (e *Engine) ProcessClarification(ctx context.Context, msg protocol.MessageEnvelope) error {
	var payload protocol.ClarificationPayload
	if err := msg.DecodePayload(&payload); err != nil {
		return err
	}
	return e.applyClarification(ctx, msg.TaskID, payload.Answer)
}

// ProcessCancel 消费 cancel 命令（Worker）。
func (e *Engine) ProcessCancel(ctx context.Context, taskID string) error {
	e.markCancelled(ctx, taskID)
	bb := e.blackboard(taskID)
	store := state.ForTask(bb, taskID)
	trace, _ := e.loadOrInitTrace(ctx, taskID)
	wf, _ := store.LoadWorkflowState(ctx)
	return e.cancelTask(ctx, taskID, store, trace, wf.TraceID, "用户取消")
}

func (e *Engine) applyClarification(ctx context.Context, taskID, answer string) error {
	bb := e.blackboard(taskID)
	store := state.ForTask(bb, taskID)
	var question state.ClarificationQuestion
	if err := bb.Get(ctx, state.ClarificationQuestionKey(taskID), &question); err != nil {
		return fmt.Errorf("task %s is not awaiting clarification", taskID)
	}
	_ = store.SaveClarificationAnswer(ctx, state.ClarificationAnswer{Answer: answer})
	meta, _ := store.LoadTaskMeta(ctx)
	meta = state.MergeClarificationIntoMeta(meta, state.ClarificationAnswer{Answer: answer}, question)
	_ = store.SaveTaskMeta(ctx, meta)
	wf, _ := store.LoadWorkflowState(ctx)
	wf.NeedsClarification = false
	wf.ClarificationResolved = true
	_ = store.SaveWorkflowState(ctx, wf)
	task, _ := e.tasks.Get(ctx, taskID)
	_ = e.transitionTask(ctx, store, taskID, task.Status, domain.TaskStatusRunning, task.Progress, "")
	e.emitTaskStatus(taskID, domain.TaskStatusRunning, task.Progress)
	next := protocol.NewEnvelope(
		taskID, wf.TraceID, "user", string(domain.AgentCoordinator),
		protocol.MsgClarificationAnswered,
		protocol.ClarificationPayload{Answer: answer},
	).WithStatus(protocol.MessageStatusCompleted)
	return e.forwardToAgent(ctx, domain.AgentCoordinator, next, protocol.TriggerClarificationAnswered)
}

// EnqueueTask 兼容：Prepare + 直接 Publish（Worker/Recovery 内部用）。
func (e *Engine) EnqueueTask(ctx context.Context, task domain.Task) error {
	traceID, err := e.PrepareTask(ctx, task, "")
	if err != nil {
		return err
	}
	e.publishTaskStarted(task.ID, task)
	return e.router.PublishTaskCreated(ctx, task, traceID)
}

func (e *Engine) handleTaskCreated(ctx context.Context, msg protocol.MessageEnvelope) error {
	if e.isCancelled(ctx, msg.TaskID) {
		return nil
	}
	bb := e.blackboard(msg.TaskID)
	store := state.ForTask(bb, msg.TaskID)
	task, err := e.tasks.Get(ctx, msg.TaskID)
	if err != nil {
		return err
	}
	wf, _ := store.LoadWorkflowState(ctx)
	if wf.TraceID == "" {
		_, _ = e.PrepareTask(ctx, task, msg.TraceID)
		wf, _ = store.LoadWorkflowState(ctx)
	}
	if task.Status == domain.TaskStatusQueued || task.Status == domain.TaskStatusPending {
		_ = e.transitionTask(ctx, store, msg.TaskID, task.Status, domain.TaskStatusRunning, 5, "")
		e.publishTaskStarted(msg.TaskID, task)
	}
	if msg.TraceID == "" && wf.TraceID != "" {
		msg.TraceID = wf.TraceID
	}
	return e.handleAgentInput(ctx, domain.AgentCoordinator, msg)
}

func (e *Engine) handleAgentInput(ctx context.Context, name domain.AgentName, msg protocol.MessageEnvelope) error {
	if e.isCancelled(ctx, msg.TaskID) {
		bb := e.blackboard(msg.TaskID)
		store := state.ForTask(bb, msg.TaskID)
		trace, _ := e.loadOrInitTrace(ctx, msg.TaskID)
		return e.cancelTask(ctx, msg.TaskID, store, trace, msg.TraceID, "任务已取消")
	}

	bb := e.blackboard(msg.TaskID)
	store := state.ForTask(bb, msg.TaskID)
	trace, _ := e.loadOrInitTrace(ctx, msg.TaskID)

	wf, _ := store.LoadWorkflowState(ctx)
	traceID := wf.TraceID
	if traceID == "" {
		traceID = msg.TraceID
	}

	trigger := triggerForMessage(msg.MessageType)
	isRework := wf.Round > 1 || msg.MessageType == protocol.MsgQAReject

	if isRework && msg.MessageType == protocol.MsgQAReject {
		trigger = protocol.TriggerQAReject
		task, _ := e.tasks.Get(ctx, msg.TaskID)
		if task.Status == domain.TaskStatusReworking {
			_ = e.transitionTask(ctx, store, msg.TaskID, domain.TaskStatusReworking, domain.TaskStatusRunning, task.Progress, "")
			e.emitTaskStatus(msg.TaskID, domain.TaskStatusRunning, task.Progress)
		}
	}

	rework, _ := store.LoadWorkflowRework(ctx)
	target := domain.AgentName(rework.TargetAgent)
	if isRework && target != "" && name != domain.AgentCoordinator {
		if !ShouldRunAgent(name, target, true) {
			return e.forwardToAgent(ctx, target, msg, trigger)
		}
	}

	ag, ok := e.registry.Get(name)
	if !ok {
		return fmt.Errorf("agent %s not found", name)
	}

	task, _ := e.tasks.Get(ctx, msg.TaskID)
	e.setAgentRunning(ctx, &task, name)
	_ = e.tasks.Update(ctx, task)
	e.emitAgentState(msg.TaskID, name, domain.AgentRunRunning, task.Progress, "")

	input := e.buildRunInput(ctx, msg, trigger, store, isRework, name)
	out, runErr := e.executeAgent(ctx, msg.TaskID, name, input, bb)

	if runErr != nil {
		if handled, herr := e.handleAgentErrorAsync(ctx, msg.TaskID, traceID, name, bb, store, runErr); handled {
			return herr
		}
		trace.Nodes = append(trace.Nodes, domain.TraceNode{
			ID: fmt.Sprintf("n%d", len(trace.Nodes)+1), Agent: name,
			Label: ag.Card().DisplayName, Status: domain.AgentRunFailed,
			Output: runErr.Error(),
		})
		_ = e.traces.Save(ctx, trace)
		task, _ = e.tasks.Get(ctx, msg.TaskID)
		return e.failTask(ctx, store, task, trace, traceID, string(name), runErr.Error())
	}

	trace.Nodes = append(trace.Nodes, buildTraceNode(
		fmt.Sprintf("n%d", len(trace.Nodes)+1), name, ag.Card().DisplayName, out.Status, out.Summary, out.Metadata,
	))
	_ = e.traces.Save(ctx, trace)

	agentMsg := protocol.NewAgentMessage(msg.TaskID, traceID, name, protocol.MessageType(out.MessageType), out.Payload, out.Artifacts)

	task, _ = e.tasks.Get(ctx, msg.TaskID)
	e.applyAgentStatus(ctx, &task, name, out)
	if name == domain.AgentCoordinator && trigger == protocol.TriggerClarificationAnswered {
		if meta, err := store.LoadTaskMeta(ctx); err == nil {
			task.Competitors = meta.Competitors
			task.Dimensions = meta.Dimensions
			if meta.Title != "" {
				task.Title = meta.Title
			}
		}
	}
	wf.CurrentAgent = string(name)
	if out.NextAgent != nil {
		wf.NextAgent = string(*out.NextAgent)
	}
	_ = store.SaveWorkflowState(ctx, wf)
	_ = e.tasks.Update(ctx, task)
	e.emitAgentState(msg.TaskID, name, out.Status, task.Progress, out.Summary)
	e.ingestAfterAgent(ctx, string(name), msg.TaskID, store)

	if name == domain.AgentQA && out.MessageType == string(protocol.MsgQAReject) {
		qaPayload := protocol.QAResultPayload{}
		if p, ok := out.Payload.(protocol.QAResultPayload); ok {
			qaPayload = p
		}
		if e.tryQAQueryLoop(ctx, msg.TaskID, traceID, store, wf, qaPayload) {
			return nil
		}
		cont, err := e.handleQAReject(ctx, store, &task, &trace, traceID, out.Payload)
		if err != nil {
			return err
		}
		if !cont {
			task, _ = e.tasks.Get(ctx, msg.TaskID)
			return e.failTask(ctx, store, task, trace, traceID, string(domain.AgentQA), "超过最大打回轮次")
		}
		return nil
	}

	if name == domain.AgentQA && out.MessageType == string(protocol.MsgQAPass) {
		return e.completeTask(ctx, msg.TaskID, store, trace, traceID)
	}

	if out.NextAgent != nil {
		return e.forwardToAgent(ctx, *out.NextAgent, agentMsg, trigger)
	}
	return e.router.Publish(ctx, agentMsg)
}

func (e *Engine) tryQAQueryLoop(ctx context.Context, taskID, traceID string, store state.TaskStore, wf state.WorkflowState, qa protocol.QAResultPayload) bool {
	if wf.QAQueryAttempts >= 1 {
		return false
	}
	var claim string
	for _, iss := range qa.Issues {
		if iss.Category == protocol.IssueUnsupportedClaim {
			claim = iss.Problem
			if claim == "" {
				claim = iss.Location
			}
			break
		}
	}
	if claim == "" {
		return false
	}
	wf.QAQueryAttempts++
	_ = store.SaveWorkflowState(ctx, wf)
	e.hub.Publish(taskID, "agent_thinking", map[string]any{
		"agent": string(domain.AgentQA), "kind": "note",
		"content": "摘要缺少来源支撑，向 Analyst 发起证据追问…", "status": "done",
	})
	_ = e.router.PublishQAQuery(ctx, taskID, traceID, protocol.QAQueryPayload{
		Claim: claim, Question: "请提供该结论的来源依据与引用片段",
	})
	return true
}

func (e *Engine) forwardToAgent(ctx context.Context, target domain.AgentName, msg protocol.MessageEnvelope, _ string) error {
	topic := inputTopicForAgent(target)
	if topic == "" {
		return nil
	}
	next := msg
	next.ToAgent = string(target)
	return e.bus.Publish(ctx, topic, next)
}

func triggerForMessage(msgType protocol.MessageType) string {
	switch msgType {
	case protocol.MsgTaskCreated:
		return protocol.TriggerTaskCreated
	case protocol.MsgQAReject:
		return protocol.TriggerQAReject
	case protocol.MsgClarificationAnswered:
		return protocol.TriggerClarificationAnswered
	default:
		return protocol.TriggerTaskCreated
	}
}

func (e *Engine) buildRunInput(ctx context.Context, msg protocol.MessageEnvelope, trigger string, store state.TaskStore, isRework bool, agentName domain.AgentName) agent.RunInput {
	input := agent.RunInput{
		TaskID:      msg.TaskID,
		TraceID:     msg.TraceID,
		Attempt:     1,
		TriggerType: trigger,
		Payload:     map[string]any{},
		Memory:      e.buildMemoryContext(ctx, msg.TaskID, string(agentName), store, trigger),
	}
	if isRework {
		if rework, err := store.LoadWorkflowRework(ctx); err == nil && rework.Reason != "" {
			input.Reason = rework.Reason
			input.Payload["rework_reason"] = rework.Reason
		}
	}
	return input
}

func (e *Engine) loadOrInitTrace(ctx context.Context, taskID string) (domain.Trace, error) {
	trace, err := e.traces.GetByTaskID(ctx, taskID)
	if err != nil {
		trace = domain.Trace{TaskID: taskID, Nodes: []domain.TraceNode{}}
	}
	return trace, nil
}

func (e *Engine) handleAgentErrorAsync(ctx context.Context, taskID, traceID string, name domain.AgentName, bb state.Blackboard, store state.TaskStore, err error) (handled bool, retErr error) {
	var ae *protocol.AgentError
	if !errors.As(err, &ae) || ae.Kind != protocol.ErrorKindClarificationRequired {
		return false, nil
	}

	task, _ := e.tasks.Get(ctx, taskID)
	_ = e.transitionTask(ctx, store, taskID, task.Status, domain.TaskStatusClarifying, task.Progress, "")
	_ = bb.Put(ctx, state.ClarificationQuestionKey(taskID), state.ClarificationQuestion{
		Question: ae.Message,
		Agent:    string(name),
	})
	wf, _ := store.LoadWorkflowState(ctx)
	wf.NeedsClarification = true
	_ = store.SaveWorkflowState(ctx, wf)

	e.hub.Publish(taskID, "clarification", map[string]string{
		"question": ae.Message,
		"agent":    string(name),
	})
	e.emitTaskStatus(taskID, domain.TaskStatusClarifying, task.Progress)
	_ = e.router.PublishClarification(ctx, taskID, traceID, protocol.ClarificationPayload{Question: ae.Message})
	return true, ErrAwaitingClarification
}

func (e *Engine) recoverActiveTasks(ctx context.Context) {
	tasks, err := e.tasks.List(ctx)
	if err != nil {
		return
	}
	for _, t := range tasks {
		switch t.Status {
		case domain.TaskStatusRunning, domain.TaskStatusReworking:
			bb := e.blackboard(t.ID)
			store := state.ForTask(bb, t.ID)
			wf, err := store.LoadWorkflowState(ctx)
			if err != nil {
				_ = e.EnqueueTask(ctx, t)
				continue
			}
			agentName := domain.AgentName(wf.CurrentAgent)
			if agentName == "" {
				agentName = domain.AgentCoordinator
			}
			topic := inputTopicForAgent(agentName)
			if topic == "" {
				continue
			}
			msg := protocol.NewEnvelope(t.ID, wf.TraceID, "recovery", string(agentName), protocol.MsgTaskCreated, nil)
			_ = e.bus.Publish(ctx, topic, msg)
		case domain.TaskStatusClarifying:
			// 等待用户 POST /clarify
		}
	}
}
