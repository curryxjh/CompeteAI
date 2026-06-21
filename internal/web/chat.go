package web

import (
	"CompeteAI/internal/domain"
	llmsvc "CompeteAI/internal/llm"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ handler = (*ChatHandler)(nil)

type ChatHandler struct {
	chat *llmsvc.ChatService
}

func NewChatHandler(chat *llmsvc.ChatService) *ChatHandler {
	return &ChatHandler{chat: chat}
}

func (h *ChatHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/chat")
	g.POST("/completions", h.Completions)
	g.POST("/stream", h.Stream)
}

type chatReq struct {
	Messages []domain.ChatMessage `json:"messages" binding:"required"`
}

type streamEvent struct {
	Type       string `json:"type,omitempty"`
	Content    string `json:"content,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolArgs   string `json:"tool_args,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
	Status     string `json:"status,omitempty"`
	Error      string `json:"error,omitempty"`
	Done       bool   `json:"done,omitempty"`
}

func (h *ChatHandler) Completions(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	content, err := h.chat.Chat(c.Request.Context(), req.Messages)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": content,
		"model":   h.chat.Model(),
	})
}

func (h *ChatHandler) Stream(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
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

	writeEvent := func(ev streamEvent) {
		data, _ := json.Marshal(ev)
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(data)
		_, _ = c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}

	err := h.chat.StreamChat(c.Request.Context(), req.Messages, func(ev llmsvc.StreamEvent) error {
		writeEvent(streamEvent{
			Type:       string(ev.Type),
			Content:    ev.Content,
			ToolName:   ev.ToolName,
			ToolArgs:   ev.ToolArgs,
			ToolResult: ev.ToolResult,
			Status:     ev.Status,
		})
		return nil
	})
	if err != nil {
		writeEvent(streamEvent{Error: err.Error()})
		return
	}
	writeEvent(streamEvent{Done: true})
}
