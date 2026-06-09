package service

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/eino"
	"CompeteAI/internal/eventlog"
	"CompeteAI/internal/kafka"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/workflow"
	"CompeteAI/settings"

	"github.com/redis/go-redis/v9"
)

// NewAgentRegistry 构建 5 Agent 注册表。
func NewAgentRegistry(chat *eino.ChatService, tools *eino.ToolRegistry) *agent.Registry {
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
	hub *eventlog.HybridHub,
	bus kafka.Bus,
	router *kafka.Router,
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
