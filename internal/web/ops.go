package web

import (
	"CompeteAI/internal/service"
	"CompeteAI/internal/repository/dao"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type OpsHandler struct {
	svc service.OpsService
}

func NewOpsHandler(svc service.OpsService) *OpsHandler {
	return &OpsHandler{svc: svc}
}

func (h *OpsHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/ops")
	g.GET("/dead-letters", h.ListDeadLetters)
	g.POST("/dead-letters/:id/replay", h.Replay)
}

func (h *OpsHandler) ListDeadLetters(c *gin.Context) {
	taskID := c.Query("task_id")
	rows, err := h.svc.ListDeadLetters(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if rows == nil {
		rows = []dao.DeadLetterEntity{}
	}
	c.JSON(http.StatusOK, rows)
}

func (h *OpsHandler) Replay(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	if err := h.svc.ReplayDeadLetter(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "requeued via outbox"})
}
