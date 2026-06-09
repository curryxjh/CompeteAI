package web

import (
	"CompeteAI/internal/repository"
	"CompeteAI/internal/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ handler = (*TraceHandler)(nil)

type TraceHandler struct {
	svc service.TraceService
}

func NewTraceHandler(svc service.TraceService) *TraceHandler {
	return &TraceHandler{svc: svc}
}

func (h *TraceHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/traces")
	g.GET("/:taskId", h.Get)
	g.GET("/:taskId/replay", h.Replay)
}

func (h *TraceHandler) Get(c *gin.Context) {
	trace, err := h.svc.GetByTaskID(c.Request.Context(), c.Param("taskId"))
	if err != nil {
		if errors.Is(err, repository.ErrTraceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "trace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, trace)
}

func (h *TraceHandler) Replay(c *gin.Context) {
	h.Get(c)
}
