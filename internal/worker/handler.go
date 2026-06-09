package worker

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/orchestrator"
)

// Role 与蓝图 §8.1 对齐。
type Role string

const (
	RoleCoordinator Role = "coordinator"
	RoleCollector   Role = "collector"
	RoleAnalyst     Role = "analyst"
	RoleWriter      Role = "writer"
	RoleQA          Role = "qa"
	RoleAll         Role = "all"
)

// Handler 角色化 Worker：只注册本角色 Topic。
type Handler struct {
	Runtime *orchestrator.Runtime
	Role    Role
}

func NewHandler(rt *orchestrator.Runtime, role Role) *Handler {
	return &Handler{Runtime: rt, Role: role}
}

func (h *Handler) Register(_ bus.Bus) {
	h.Runtime.SetRole(string(h.Role))
	h.Runtime.RegisterHandlers()
}

func RoleFromEnv(s string) Role {
	switch Role(s) {
	case RoleCoordinator, RoleCollector, RoleAnalyst, RoleWriter, RoleQA:
		return Role(s)
	default:
		return RoleAll
	}
}

func (r Role) Agent() domain.AgentName {
	switch r {
	case RoleCoordinator:
		return domain.AgentCoordinator
	case RoleCollector:
		return domain.AgentCollector
	case RoleAnalyst:
		return domain.AgentAnalyst
	case RoleWriter:
		return domain.AgentWriter
	case RoleQA:
		return domain.AgentQA
	default:
		return ""
	}
}
