package workflow

import "CompeteAI/internal/domain"

const defaultMaxRounds = 3

// CanTransition 校验任务状态是否允许转换（委托给 domain.CanTransitionTo）。
func CanTransition(from, to domain.TaskStatus) bool {
	if from == to {
		return true
	}
	allowed, ok := domain.TaskTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition 非法转换时返回错误（委托给 domain.ErrInvalidTransition）。
func ValidateTransition(from, to domain.TaskStatus) error {
	if CanTransition(from, to) {
		return nil
	}
	return domain.ErrInvalidTransition
}

// ShouldRunAgent QA 打回后局部重跑（委托给 domain.ShouldRunAgent）。
func ShouldRunAgent(name, target domain.AgentName, isRework bool) bool {
	return domain.ShouldRunAgent(name, target, isRework)
}

// NextAgentInPipeline 正常主流程下一跳（委托给 domain.NextAgentInPipeline）。
func NextAgentInPipeline(current domain.AgentName) (domain.AgentName, bool) {
	return domain.NextAgentInPipeline(current)
}

// TraceEvent 工作流 Trace 标签（§17）。
const (
	TraceTaskCreated      = "task_created"
	TraceCoordinatorStart = "coordinator_started"
	TraceAgentCompleted   = "agent_completed"
	TraceQARejected       = "qa_rejected"
	TraceWorkflowReworked = "workflow_reworked"
	TraceTaskCompleted    = "task_completed"
	TraceTaskFailed       = "task_failed"
	TraceClarification    = "clarification_requested"
)