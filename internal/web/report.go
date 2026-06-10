package web

import (
	"CompeteAI/internal/repository"
	"CompeteAI/internal/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var _ handler = (*ReportHandler)(nil)

type ReportHandler struct {
	svc service.ReportService
}

func NewReportHandler(svc service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

func (h *ReportHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/reports")
	g.GET("/:taskId", h.Get)
	g.GET("/:taskId/export", h.Export)
	g.POST("/:taskId/annotate", h.Annotate)
}

func (h *ReportHandler) Get(c *gin.Context) {
	report, err := h.svc.GetByTaskID(c.Request.Context(), c.Param("taskId"))
	if err != nil {
		if errors.Is(err, repository.ErrReportNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "report not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *ReportHandler) Export(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "export not implemented yet"})
}

func (h *ReportHandler) Annotate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
