package orchestrator

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/dlq"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/eventlog"
	"CompeteAI/internal/metrics"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/workflow"
	"context"
	"errors"
	"log"
	"time"
)

// Runtime 编排运行时：包装 workflow.Engine，接入 bus ACK 语义与持久化。
type Runtime struct {
	engine   *workflow.Engine
	bus      bus.Bus
	store    *Store
	events   *eventlog.Publisher
	msgLog   *dao.MessageLogDao
	dlq      *dlq.Handler
	tasks    repository.TaskRepository
	workerID string
	role     string
	useRedis bool
}

type Config struct {
	Engine   *workflow.Engine
	Bus      bus.Bus
	Store    *Store
	Events   *eventlog.Publisher
	MsgLog   *dao.MessageLogDao
	DLQ      *dlq.Handler
	Tasks    repository.TaskRepository
	WorkerID string
	Role     string
	UseRedis bool
}

func NewRuntime(cfg Config) *Runtime {
	return &Runtime{
		engine: cfg.Engine, bus: cfg.Bus, store: cfg.Store,
		events: cfg.Events, msgLog: cfg.MsgLog, dlq: cfg.DLQ, tasks: cfg.Tasks,
		workerID: cfg.WorkerID, role: cfg.Role, useRedis: cfg.UseRedis,
	}
}

func (r *Runtime) SetRole(role string) {
	if role != "" {
		r.role = role
	}
}

func (r *Runtime) RegisterHandlers() {
	topics := bus.AgentRoleTopics(r.role)
	if len(topics) == 0 {
		topics = bus.AgentRoleTopics("all")
	}
	for _, topic := range topics {
		t := topic
		r.bus.Register(t, r.wrapHandler(t))
	}
}

func (r *Runtime) wrapHandler(topic string) bus.Handler {
	return func(ctx context.Context, d bus.Delivery) error {
		env := d.Envelope
		if done, err := r.store.Idempotent(ctx, env); err != nil {
			return err
		} else if done {
			return nil
		}

		agent := agentForTopic(topic)
		if agent != "" {
			ok, err := r.store.AcquireLease(ctx, env.TaskID, agent, r.workerID)
			if err != nil || !ok {
				return err
			}
			defer r.store.HeartbeatLease(ctx, env.TaskID, agent, r.workerID)
		}

		_ = r.msgLog.Create(ctx, dao.MessageLogEntity{
			MessageID: env.MessageID, TaskID: env.TaskID, TraceID: env.TraceID,
			Topic: topic, Kind: env.Kind, Name: env.Name,
			FromAgent: env.FromAgent, ToAgent: env.ToAgent,
			Attempt: env.Attempt, Status: "processing",
			PayloadJSON: string(env.Payload),
		})

		metrics.IncMessageConsumed()
		start := time.Now()
		err := r.engine.HandleDelivery(ctx, topic, env)
		metrics.RecordAgentRun(string(agent), time.Since(start), err)

		if err == nil || errors.Is(err, workflow.ErrAwaitingClarification) {
			_ = r.msgLog.MarkAcked(ctx, env.MessageID)
			_ = r.store.SaveCheckpoint(ctx, Checkpoint{
				TaskID: env.TaskID, TraceID: env.TraceID,
				CurrentAgent: string(agent), CurrentMessageID: env.MessageID,
				LastPublishedTopic: topic, UpdatedAt: time.Now().Format(time.RFC3339),
			})
			return nil
		}

		metrics.IncMessageRetry()
		nextAttempt := env.Attempt + 1
		if nextAttempt >= dlq.MaxAttempts && r.dlq != nil {
			reason := err.Error()
			_ = r.dlq.MoveToDLQ(ctx, agent, topic, env, reason)
			_ = r.msgLog.MarkFailed(ctx, env.MessageID, reason)
			metrics.IncTaskFailed()
			r.markAttentionRequired(ctx, env.TaskID, reason)
			log.Printf("[dlq] task=%s msg=%s agent=%s: %s", env.TaskID, env.MessageID, agent, reason)
			return nil
		}
		time.Sleep(bus.RetryBackoff(nextAttempt))
		return err
	}
}

func (r *Runtime) markAttentionRequired(ctx context.Context, taskID, reason string) {
	if r.tasks == nil || taskID == "" {
		return
	}
	task, err := r.tasks.Get(ctx, taskID)
	if err != nil {
		return
	}
	task.Status = domain.TaskStatusAttentionRequired
	task.ErrorMessage = reason
	_ = r.tasks.Update(ctx, task)
}

func agentForTopic(topic string) domain.AgentName {
	switch bus.NormalizeTopic(topic) {
	case bus.TopicAgentCoordinator, bus.TopicTaskCreate:
		return domain.AgentCoordinator
	case bus.TopicAgentCollector:
		return domain.AgentCollector
	case bus.TopicAgentAnalyst, bus.TopicQAQuery:
		return domain.AgentAnalyst
	case bus.TopicAgentWriter:
		return domain.AgentWriter
	case bus.TopicAgentQA, bus.TopicAnalystReply:
		return domain.AgentQA
	default:
		return ""
	}
}

func (r *Runtime) PublishEvent(ctx context.Context, taskID, traceID, eventType, agent string, payload any) {
	if r.events == nil {
		return
	}
	_, _ = r.events.Publish(ctx, taskID, traceID, eventlog.MapLegacyEvent(eventType), agent, payload)
}

type Recovery struct {
	tasks  repository.TaskRepository
	bus    bus.Bus
	store  *Store
	engine *workflow.Engine
}

func NewRecovery(tasks repository.TaskRepository, b bus.Bus, store *Store, engine *workflow.Engine) *Recovery {
	return &Recovery{tasks: tasks, bus: b, store: store, engine: engine}
}

func (rec *Recovery) RunOnce(ctx context.Context) {
	metrics.IncRecovery()
	tasks, err := rec.tasks.List(ctx)
	if err != nil {
		return
	}
	staleBefore := time.Now().Add(-2 * time.Minute)
	cps, _ := rec.store.checkpoints.ListStale(ctx, staleBefore)
	cpMap := map[string]dao.TaskCheckpointEntity{}
	for _, cp := range cps {
		cpMap[cp.TaskID] = cp
	}
	for _, t := range tasks {
		switch t.Status {
		case domain.TaskStatusRunning, domain.TaskStatusReworking, domain.TaskStatusWaitingReply, domain.TaskStatusQueued:
			cp, ok := cpMap[t.ID]
			agent := domain.AgentCoordinator
			traceID := orchestratorTraceID(cp, ok)
			topic := bus.AgentCommandTopic(agent)
			env := protocol.NewEnvelope(t.ID, traceID, "recovery", string(agent), protocol.MsgTaskCreated, nil)
			_ = rec.bus.Publish(ctx, topic, env)
		}
	}
	if reclaimer, ok := rec.bus.(interface {
		ReclaimPending(ctx context.Context, topic string, minIdle time.Duration, count int64) (int, error)
	}); ok {
		for _, topic := range bus.AgentRoleTopics("all") {
			_, _ = reclaimer.ReclaimPending(ctx, topic, 2*time.Minute, 10)
		}
	}
}

func orchestratorTraceID(cp dao.TaskCheckpointEntity, ok bool) string {
	if ok && cp.TraceID != "" {
		return cp.TraceID
	}
	return NewTraceID()
}
