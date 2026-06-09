package web

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/eventlog"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/service"
	ijwt "CompeteAI/internal/web/jwt"
	"CompeteAI/internal/workflow"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	svc service.TaskService
	log *eventlog.Reader
}

func NewTaskHandler(svc service.TaskService, hub *eventlog.HybridHub) *TaskHandler {
	var reader *eventlog.Reader
	if hub != nil {
		reader = hub.Reader()
	}
	return &TaskHandler{svc: svc, log: reader}
}

func (h *TaskHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/tasks")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.GET("/:id/stream", h.Stream)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/clarify", h.Clarify)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req domain.CreateTaskPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	// 将登录用户 ID 注入 context，供记忆作用域关联使用
	ctx := c.Request.Context()
	if claims, ok := c.Get("claims"); ok {
		if uc, ok := claims.(*ijwt.UserClaims); ok && uc.Uid > 0 {
			ctx = workflow.WithUserID(ctx, uc.Uid)
		}
	}
	task, err := h.svc.Create(ctx, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) List(c *gin.Context) {
	tasks, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []domain.Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) Get(c *gin.Context) {
	task, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Clarify(c *gin.Context) {
	var req domain.ClarifyPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	if err := h.svc.Clarify(c.Request.Context(), c.Param("id"), req.Answer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Stream SSE 仅读 event_logs（蓝图 §11.1）。
func (h *TaskHandler) Stream(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.svc.Get(c.Request.Context(), taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "streaming not supported"})
		return
	}

	writeEvent := func(eventType string, data any) {
		raw, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", eventType)
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
		flusher.Flush()
	}

	var lastSeq int64
	emitFromLog := func() bool {
		if h.log == nil {
			return false
		}
		rows, err := h.log.ListSince(c.Request.Context(), taskID, lastSeq)
		if err != nil {
			return false
		}
		for _, rec := range rows {
			writeEvent(eventlog.SSEEventName(rec.EventType), rec.Payload)
			lastSeq = rec.SequenceNo
			if rec.EventType == "task_completed" || rec.EventType == "task_failed" {
				return true
			}
		}
		return false
	}

	if emitFromLog() {
		return
	}

	writeEvent("task_started", map[string]any{
		"status": task.Status, "progress": task.Progress, "agentStates": task.AgentStates,
	})
	if task.Status == domain.TaskStatusCompleted {
		writeEvent("task_complete", map[string]any{"status": task.Status, "progress": task.Progress})
		return
	}
	if task.Status == domain.TaskStatusFailed || task.Status == domain.TaskStatusAttentionRequired {
		writeEvent("task_failed", map[string]string{"message": task.ErrorMessage})
		return
	}

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if emitFromLog() {
				return
			}
			cur, err := h.svc.Get(c.Request.Context(), taskID)
			if err != nil {
				return
			}
			if cur.Status == domain.TaskStatusCompleted {
				writeEvent("task_complete", map[string]any{"status": cur.Status, "progress": cur.Progress})
				return
			}
			if cur.Status == domain.TaskStatusFailed || cur.Status == domain.TaskStatusAttentionRequired {
				writeEvent("task_failed", map[string]string{"message": cur.ErrorMessage})
				return
			}
		}
	}
}
