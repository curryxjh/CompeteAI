package workflow

import (
	"CompeteAI/internal/domain"
	"errors"
	"fmt"
)

const defaultMaxRounds = 3

// ErrInvalidTransition 非法任务状态转换。
var ErrInvalidTransition = errors.New("invalid task status transition")

// pipelineOrder 主流程 Agent 顺序。
var pipelineOrder = []domain.AgentName{
	domain.AgentCoordinator,
	domain.AgentCollector,
	domain.AgentAnalyst,
	domain.AgentWriter,
	domain.AgentQA,
}

// taskTransitions §15.1 合法 TaskStatus 转换表。
var taskTransitions = map[domain.TaskStatus][]domain.TaskStatus{
	domain.TaskStatusPending:           {domain.TaskStatusQueued, domain.TaskStatusRunning, domain.TaskStatusCancelled},
	domain.TaskStatusQueued:            {domain.TaskStatusRunning, domain.TaskStatusCancelled, domain.TaskStatusFailed},
	domain.TaskStatusRunning:           {domain.TaskStatusClarifying, domain.TaskStatusReworking, domain.TaskStatusWaitingReply, domain.TaskStatusCompleted, domain.TaskStatusFailed, domain.TaskStatusCancelled, domain.TaskStatusAttentionRequired},
	domain.TaskStatusClarifying:        {domain.TaskStatusRunning, domain.TaskStatusFailed, domain.TaskStatusCancelled},
	domain.TaskStatusReworking:         {domain.TaskStatusRunning, domain.TaskStatusCompleted, domain.TaskStatusFailed, domain.TaskStatusCancelled},
	domain.TaskStatusWaitingReply:      {domain.TaskStatusRunning, domain.TaskStatusFailed, domain.TaskStatusCancelled},
	domain.TaskStatusAttentionRequired: {domain.TaskStatusRunning, domain.TaskStatusFailed, domain.TaskStatusCancelled},
	domain.TaskStatusCompleted:         {},
	domain.TaskStatusFailed:            {},
	domain.TaskStatusCancelled:         {},
}

// CanTransition 校验任务状态是否允许转换。
func CanTransition(from, to domain.TaskStatus) bool {
	if from == to {
		return true
	}
	allowed, ok := taskTransitions[from]
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

// ValidateTransition 非法转换时返回错误。
func ValidateTransition(from, to domain.TaskStatus) error {
	if CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
}

// AgentIndex 返回 Agent 在流水线中的索引，未知 Agent 返回 -1。
func AgentIndex(name domain.AgentName) int {
	for i, n := range pipelineOrder {
		if n == name {
			return i
		}
	}
	return -1
}

// ShouldRunAgent §12 QA 打回后局部重跑：Coordinator 始终执行，其余从 target 开始。
func ShouldRunAgent(name, target domain.AgentName, isRework bool) bool {
	if !isRework {
		return true
	}
	if name == domain.AgentCoordinator {
		return true
	}
	if target == "" {
		return true
	}
	ti := AgentIndex(target)
	ni := AgentIndex(name)
	if ti < 0 || ni < 0 {
		return true
	}
	return ni >= ti
}

// NextAgentInPipeline 正常主流程下一跳（§12.1）。
func NextAgentInPipeline(current domain.AgentName) (domain.AgentName, bool) {
	idx := AgentIndex(current)
	if idx < 0 || idx >= len(pipelineOrder)-1 {
		return "", false
	}
	return pipelineOrder[idx+1], true
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
