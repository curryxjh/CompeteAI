package repository

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrReportNotFound = errors.New("report not found")
	ErrTraceNotFound  = errors.New("trace not found")
)

type TaskRepository interface {
	Create(ctx context.Context, task domain.Task) error
	Update(ctx context.Context, task domain.Task) error
	Get(ctx context.Context, id string) (domain.Task, error)
	List(ctx context.Context) ([]domain.Task, error)
	Delete(ctx context.Context, id string) error
}

type ReportRepository interface {
	Save(ctx context.Context, report domain.Report) error
	GetByTaskID(ctx context.Context, taskID string) (domain.Report, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

type TraceRepository interface {
	Save(ctx context.Context, trace domain.Trace) error
	GetByTaskID(ctx context.Context, taskID string) (domain.Trace, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

type taskRepository struct {
	dao dao.TaskDao
}

func NewTaskRepository(d dao.TaskDao) TaskRepository {
	return &taskRepository{dao: d}
}

func (r *taskRepository) Create(ctx context.Context, task domain.Task) error {
	entity, err := taskToEntity(task)
	if err != nil {
		return err
	}
	return r.dao.Insert(ctx, entity)
}

func (r *taskRepository) Update(ctx context.Context, task domain.Task) error {
	entity, err := taskToEntity(task)
	if err != nil {
		return err
	}
	return r.dao.Update(ctx, entity)
}

func (r *taskRepository) Get(ctx context.Context, id string) (domain.Task, error) {
	entity, err := r.dao.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Task{}, ErrTaskNotFound
	}
	if err != nil {
		return domain.Task{}, err
	}
	return entityToTask(entity)
}

func (r *taskRepository) List(ctx context.Context) ([]domain.Task, error) {
	entities, err := r.dao.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Task, 0, len(entities))
	for _, e := range entities {
		t, err := entityToTask(e)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *taskRepository) Delete(ctx context.Context, id string) error {
	return r.dao.Delete(ctx, id)
}

type reportRepository struct {
	dao dao.ReportDao
}

func NewReportRepository(d dao.ReportDao) ReportRepository {
	return &reportRepository{dao: d}
}

func (r *reportRepository) Save(ctx context.Context, report domain.Report) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return r.dao.Upsert(ctx, dao.ReportEntity{
		TaskID:  report.TaskID,
		Payload: string(raw),
	})
}

func (r *reportRepository) GetByTaskID(ctx context.Context, taskID string) (domain.Report, error) {
	entity, err := r.dao.FindByTaskID(ctx, taskID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Report{}, ErrReportNotFound
	}
	if err != nil {
		return domain.Report{}, err
	}
	var report domain.Report
	if err := json.Unmarshal([]byte(entity.Payload), &report); err != nil {
		return domain.Report{}, err
	}
	return report, nil
}

func (r *reportRepository) DeleteByTaskID(ctx context.Context, taskID string) error {
	return r.dao.DeleteByTaskID(ctx, taskID)
}

type traceRepository struct {
	dao dao.TraceDao
}

func NewTraceRepository(d dao.TraceDao) TraceRepository {
	return &traceRepository{dao: d}
}

func (r *traceRepository) Save(ctx context.Context, trace domain.Trace) error {
	raw, err := json.Marshal(trace)
	if err != nil {
		return err
	}
	return r.dao.Upsert(ctx, dao.TraceEntity{
		TaskID:  trace.TaskID,
		Payload: string(raw),
	})
}

func (r *traceRepository) GetByTaskID(ctx context.Context, taskID string) (domain.Trace, error) {
	entity, err := r.dao.FindByTaskID(ctx, taskID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Trace{}, ErrTraceNotFound
	}
	if err != nil {
		return domain.Trace{}, err
	}
	var trace domain.Trace
	if err := json.Unmarshal([]byte(entity.Payload), &trace); err != nil {
		return domain.Trace{}, err
	}
	return trace, nil
}

func (r *traceRepository) DeleteByTaskID(ctx context.Context, taskID string) error {
	return r.dao.DeleteByTaskID(ctx, taskID)
}

func taskToEntity(task domain.Task) (dao.TaskEntity, error) {
	competitors, err := json.Marshal(task.Competitors)
	if err != nil {
		return dao.TaskEntity{}, err
	}
	dimensions, err := json.Marshal(task.Dimensions)
	if err != nil {
		return dao.TaskEntity{}, err
	}
	agentStates, err := json.Marshal(task.AgentStates)
	if err != nil {
		return dao.TaskEntity{}, err
	}
	return dao.TaskEntity{
		ID:           task.ID,
		Title:        task.Title,
		Competitors:  string(competitors),
		Dimensions:   string(dimensions),
		Status:       string(task.Status),
		Progress:     task.Progress,
		AgentStates:  string(agentStates),
		ErrorMessage: task.ErrorMessage,
	}, nil
}

func entityToTask(entity dao.TaskEntity) (domain.Task, error) {
	var competitors []string
	var dimensions []string
	var agentStates []domain.AgentState
	if err := json.Unmarshal([]byte(entity.Competitors), &competitors); err != nil {
		return domain.Task{}, err
	}
	if err := json.Unmarshal([]byte(entity.Dimensions), &dimensions); err != nil {
		return domain.Task{}, err
	}
	if entity.AgentStates != "" {
		if err := json.Unmarshal([]byte(entity.AgentStates), &agentStates); err != nil {
			return domain.Task{}, err
		}
	}
	task := domain.Task{
		ID:           entity.ID,
		Title:        entity.Title,
		Competitors:  competitors,
		Dimensions:   dimensions,
		Status:       domain.TaskStatus(entity.Status),
		Progress:     entity.Progress,
		AgentStates:  agentStates,
		ErrorMessage: entity.ErrorMessage,
		CreatedAt:    entity.CreatedAt.Format(time.RFC3339),
	}
	if !entity.UpdatedAt.IsZero() {
		task.UpdatedAt = entity.UpdatedAt.Format(time.RFC3339)
	}
	return task, nil
}
