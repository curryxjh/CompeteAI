package web

import (
	"CompeteAI/internal/memory"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type MemoryHandler struct {
	svc memory.MemoryService
}

func NewMemoryHandler(svc memory.MemoryService) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

func (h *MemoryHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/memory")
	g.GET("/preferences", h.ListPreferences)
	g.POST("/preferences", h.UpsertPreference)
	g.GET("/facts/search", h.SearchFacts)
	g.POST("/facts/:id/invalidate", h.InvalidateFact)
}

func (h *MemoryHandler) ListPreferences(c *gin.Context) {
	scopeType := memory.ScopeType(c.DefaultQuery("scope_type", "project"))
	scopeKey := c.DefaultQuery("scope_key", "compete-ai")
	rows, err := h.svc.ListPreferences(c.Request.Context(), scopeType, scopeKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *MemoryHandler) UpsertPreference(c *gin.Context) {
	var req struct {
		ScopeType string         `json:"scopeType"`
		ScopeKey  string         `json:"scopeKey"`
		Key       string         `json:"key"`
		Value     map[string]any `json:"value"`
		Priority  int            `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	if err := h.svc.UpsertPreference(c.Request.Context(), memory.UpsertPreferenceRequest{
		ScopeType: memory.ScopeType(req.ScopeType),
		ScopeKey:  req.ScopeKey,
		Key:       req.Key,
		Value:     req.Value,
		Priority:  req.Priority,
		Source:    "api",
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *MemoryHandler) SearchFacts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	facts, err := h.svc.SearchFacts(c.Request.Context(), memory.SearchFactsRequest{
		Query: c.Query("q"),
		Limit: limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, facts)
}

func (h *MemoryHandler) InvalidateFact(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.InvalidateFact(c.Request.Context(), c.Param("id"), req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invalidated"})
}
