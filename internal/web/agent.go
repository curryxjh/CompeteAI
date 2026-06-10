package web

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/domain"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ handler = (*AgentHandler)(nil)

type AgentHandler struct {
	registry *agent.Registry
}

func NewAgentHandler(registry *agent.Registry) *AgentHandler {
	return &AgentHandler{registry: registry}
}

func (h *AgentHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/agents")
	g.GET("", h.List)
	g.GET("/:name", h.Get)
}

func (h *AgentHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.registry.Cards())
}

func (h *AgentHandler) Get(c *gin.Context) {
	name := domain.AgentName(c.Param("name"))
	card, ok := h.registry.Card(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, card)
}
