package web

import (
	"context"
	"net/http"

	einosvc "CompeteAI/internal/eino"

	"github.com/gin-gonic/gin"
)

type MCPHandler struct {
	tools *einosvc.ToolRegistry
}

func NewMCPHandler(tools *einosvc.ToolRegistry) *MCPHandler {
	return &MCPHandler{tools: tools}
}

func (h *MCPHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/mcp")
	g.GET("/tools", h.ListTools)
	g.POST("/call", h.CallTool)
}

func (h *MCPHandler) ListTools(c *gin.Context) {
	if h.tools == nil || !h.tools.Enabled() {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"tools":   []any{},
		})
		return
	}

	infos, err := h.tools.ListToolInfos(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(infos))
	for _, info := range infos {
		out = append(out, gin.H{
			"name":        info.Name,
			"description": info.Desc,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"tools":   out,
	})
}

type mcpCallReq struct {
	Name      string                 `json:"name" binding:"required"`
	Arguments map[string]interface{} `json:"arguments"`
}

func (h *MCPHandler) CallTool(c *gin.Context) {
	if h.tools == nil || !h.tools.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "firecrawl mcp is not enabled"})
		return
	}

	var req mcpCallReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	content, err := h.tools.Invoke(context.Background(), req.Name, req.Arguments)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}
