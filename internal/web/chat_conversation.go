package web

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChatConversationHandler struct {
	svc service.ChatConversationService
}

func NewChatConversationHandler(svc service.ChatConversationService) *ChatConversationHandler {
	return &ChatConversationHandler{svc: svc}
}

func (h *ChatConversationHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/chat/conversations")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Rename)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/messages", h.AppendMessages)
}

func (h *ChatConversationHandler) List(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	list, err := h.svc.List(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if list == nil {
		list = []domain.ChatConversation{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *ChatConversationHandler) Create(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	var req domain.CreateConversationPayload
	_ = c.ShouldBindJSON(&req)
	conv, err := h.svc.Create(c.Request.Context(), uid, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

func (h *ChatConversationHandler) Get(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	detail, err := h.svc.Get(c.Request.Context(), uid, c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *ChatConversationHandler) Rename(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	var req domain.UpdateConversationPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	if err := h.svc.Rename(c.Request.Context(), uid, c.Param("id"), req.Title); err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ChatConversationHandler) Delete(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), uid, c.Param("id")); err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ChatConversationHandler) AppendMessages(c *gin.Context) {
	uid, ok := uidFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "login required"})
		return
	}
	var req domain.AppendChatMessagesPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	if err := h.svc.AppendMessages(c.Request.Context(), uid, c.Param("id"), req.Messages); err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
