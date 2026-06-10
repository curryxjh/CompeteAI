package service

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/metrics"
	"CompeteAI/internal/outbox"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/workflow"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// userIDFromCtx 从 context 取出登录用户 ID，依赖 workflow 包的实现。
func userIDFromCtx(ctx context.Context) int64 {
	return workflow.UserIDFromCtx(ctx)
}

type TaskService interface {
	Create(ctx context.Context, payload domain.CreateTaskPayload) (domain.Task, error)
	List(ctx context.Context) ([]domain.Task, error)
	Get(ctx context.Context, id string) (domain.Task, error)
	Delete(ctx context.Context, id string) error
	Clarify(ctx context.Context, taskID, answer string) error
	Cancel(ctx context.Context, taskID string) error
}

type ReportService interface {
	GetByTaskID(ctx context.Context, taskID string) (domain.Report, error)
}

type TraceService interface {
	GetByTaskID(ctx context.Context, taskID string) (domain.Trace, error)
}

type OpsService interface {
	ListDeadLetters(ctx context.Context, taskID string) ([]dao.DeadLetterEntity, error)
	ReplayDeadLetter(ctx context.Context, id uint) error
}

type taskService struct {
	db      *gorm.DB
	tasks   repository.TaskRepository
	reports repository.ReportRepository
	traces  repository.TraceRepository
	engine  *workflow.Engine
	outbox  *outbox.Publisher
}

type opsService struct {
	dlq    *dao.DeadLetterDao
	outbox *outbox.Publisher
}

func NewTaskService(
	db *gorm.DB,
	tasks repository.TaskRepository,
	reports repository.ReportRepository,
	traces repository.TraceRepository,
	engine *workflow.Engine,
	outboxPub *outbox.Publisher,
) TaskService {
	return &taskService{
		db: db, tasks: tasks, reports: reports, traces: traces,
		engine: engine, outbox: outboxPub,
	}
}

func NewOpsService(dlq *dao.DeadLetterDao, outboxPub *outbox.Publisher) OpsService {
	return &opsService{dlq: dlq, outbox: outboxPub}
}

func (s *taskService) Create(ctx context.Context, payload domain.CreateTaskPayload) (domain.Task, error) {
	if len(payload.Competitors) == 0 {
		return domain.Task{}, fmt.Errorf("competitors required")
	}
	if len(payload.Dimensions) == 0 {
		return domain.Task{}, fmt.Errorf("dimensions required")
	}
	title := payload.Title
	if title == "" {
		title = strings.Join(payload.Competitors, " vs ") + " 竞品分析"
	}
	now := time.Now().Format(time.RFC3339)
	task := domain.Task{
		ID: uuid.NewString(), Title: title,
		Competitors: payload.Competitors, Dimensions: payload.Dimensions,
		Status: domain.TaskStatusQueued, Progress: 0,
		AgentStates: domain.DefaultAgentStates(),
		CreatedAt: now, UpdatedAt: now,
		UserID: userIDFromCtx(ctx),
	}
	traceID := uuid.NewString()
	if err := repository.CreateTaskWithOutbox(ctx, s.db, task, traceID); err != nil {
		return domain.Task{}, err
	}
	metrics.IncTaskCreated()
	return task, nil
}

func (s *taskService) Clarify(ctx context.Context, taskID, answer string) error {
	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != domain.TaskStatusClarifying {
		return fmt.Errorf("task %s is not awaiting clarification", taskID)
	}
	traceID := s.engine.TraceIDForTask(ctx, taskID)
	return s.outbox.EnqueueClarify(ctx, taskID, traceID, answer)
}

func (s *taskService) Cancel(ctx context.Context, taskID string) error {
	if _, err := s.tasks.Get(ctx, taskID); err != nil {
		return err
	}
	traceID := s.engine.TraceIDForTask(ctx, taskID)
	return s.outbox.EnqueueCancel(ctx, taskID, traceID)
}

func (s *taskService) Delete(ctx context.Context, id string) error {
	_ = s.Cancel(ctx, id)
	_ = s.reports.DeleteByTaskID(ctx, id)
	_ = s.traces.DeleteByTaskID(ctx, id)
	return s.tasks.Delete(ctx, id)
}

func (s *taskService) List(ctx context.Context) ([]domain.Task, error) {
	return s.tasks.List(ctx)
}

func (s *taskService) Get(ctx context.Context, id string) (domain.Task, error) {
	return s.tasks.Get(ctx, id)
}

func (s *opsService) ListDeadLetters(ctx context.Context, taskID string) ([]dao.DeadLetterEntity, error) {
	return s.dlq.List(ctx, taskID, 100)
}

func (s *opsService) ReplayDeadLetter(ctx context.Context, id uint) error {
	dl, err := s.dlq.Get(ctx, id)
	if err != nil {
		return err
	}
	var env protocol.MessageEnvelope
	if err := json.Unmarshal([]byte(dl.PayloadJSON), &env); err != nil {
		return err
	}
	return s.outbox.ReplayEnvelope(ctx, dl.Topic, env)
}

func NewReportService(reports repository.ReportRepository) ReportService {
	return &reportService{reports: reports}
}

func NewTraceService(traces repository.TraceRepository) TraceService {
	return &traceService{traces: traces}
}

type reportService struct{ reports repository.ReportRepository }
type traceService struct{ traces repository.TraceRepository }

func (s *reportService) GetByTaskID(ctx context.Context, taskID string) (domain.Report, error) {
	return s.reports.GetByTaskID(ctx, taskID)
}

func (s *traceService) GetByTaskID(ctx context.Context, taskID string) (domain.Trace, error) {
	return s.traces.GetByTaskID(ctx, taskID)
}

type AnalysisService = workflow.Engine

func NewAnalysisService(engine *workflow.Engine) *AnalysisService {
	return engine
}
