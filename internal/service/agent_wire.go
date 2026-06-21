package service

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/llm"
	"CompeteAI/internal/event"
	"CompeteAI/internal/bus"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/workflow"
	"CompeteAI/settings"

	"github.com/redis/go-redis/v9"
)

// NewAgentRegistry 构建 5 Agent 注册表。
func NewAgentRegistry(chat *llm.ChatService, tools *llm.ToolRegistry) *agent.Registry {
	return agent.NewRegistry(agent.Deps{
		Chat:  agent.NewEinoChatAdapter(chat),
		Tools: agent.NewEinoToolAdapter(tools),
	})
}

// NewWorkflowEngine 构建工作流引擎。
func NewWorkflowEngine(
	registry *agent.Registry,
	tasks repository.TaskRepository,
	reports repository.ReportRepository,
	traces repository.TraceRepository,
	hub *event.HybridHub,
	bus bus.Bus,
	router *bus.Router,
	redisClient redis.Cmdable,
	mem memory.MemoryService,
) *workflow.Engine {
	maxRetry := 3
	maxRounds := 0
	useRedisBB := true
	if cfg := settings.Conf.WorkflowConfig; cfg != nil {
		if cfg.MaxAgentRetries > 0 {
			maxRetry = cfg.MaxAgentRetries
		}
		maxRounds = cfg.MaxRounds
		useRedisBB = cfg.UseRedisBlackboard
	}
	return workflow.NewEngine(registry, tasks, reports, traces, hub, bus, router, redisClient, useRedisBB, maxRetry, maxRounds, mem)
}
