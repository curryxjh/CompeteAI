package orchestrator

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const leaseTTL = 30 * time.Second

// Checkpoint 任务检查点（蓝图 §6.2）。
type Checkpoint struct {
	TaskID             string `json:"task_id"`
	TraceID            string `json:"trace_id"`
	CurrentAgent       string `json:"current_agent"`
	CurrentMessageID   string `json:"current_message_id"`
	LastCompletedStep  string `json:"last_completed_step"`
	LastPublishedTopic string `json:"last_published_topic"`
	Round              int    `json:"round"`
	RetryCount         int    `json:"retry_count"`
	RejectionCount     int    `json:"rejection_count"`
	ClarificationOpen  bool   `json:"clarification_open"`
	WaitingOn          string `json:"waiting_on"`
	LeaseOwner         string `json:"lease_owner"`
	LeaseExpireAt      string `json:"lease_expire_at"`
	UpdatedAt          string `json:"updated_at"`
}

type Store struct {
	checkpoints *dao.CheckpointDao
	redis       redis.Cmdable
}

func NewStore(cp *dao.CheckpointDao, redisClient redis.Cmdable) *Store {
	return &Store{checkpoints: cp, redis: redisClient}
}

func (s *Store) SaveCheckpoint(ctx context.Context, cp Checkpoint) error {
	cp.UpdatedAt = time.Now().Format(time.RFC3339)
	raw := dao.MarshalJSON(cp)
	return s.checkpoints.Upsert(ctx, dao.TaskCheckpointEntity{
		TaskID: cp.TaskID, TraceID: cp.TraceID,
		CurrentAgent: cp.CurrentAgent, CurrentMessageID: cp.CurrentMessageID,
		Round: cp.Round, RetryCount: cp.RetryCount,
		CheckpointJSON: raw, UpdatedAt: time.Now(),
	})
}

func leaseKey(taskID string, agent domain.AgentName) string {
	return fmt.Sprintf("lease:task:%s:%s", taskID, agent)
}

// AcquireLease 获取任务阶段 lease（§6.3）。
func (s *Store) AcquireLease(ctx context.Context, taskID string, agent domain.AgentName, workerID string) (bool, error) {
	if s.redis == nil {
		return true, nil
	}
	key := leaseKey(taskID, agent)
	ok, err := s.redis.SetNX(ctx, key, workerID, leaseTTL).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		owner, _ := s.redis.Get(ctx, key).Result()
		return owner == workerID, nil
	}
	return true, nil
}

func (s *Store) HeartbeatLease(ctx context.Context, taskID string, agent domain.AgentName, workerID string) {
	if s.redis == nil {
		return
	}
	key := leaseKey(taskID, agent)
	val, _ := s.redis.Get(ctx, key).Result()
	if val == workerID {
		_ = s.redis.Expire(ctx, key, leaseTTL).Err()
	}
}

// Idempotent 检查消息是否已处理（§8.3）。
func (s *Store) Idempotent(ctx context.Context, msg protocol.MessageEnvelope) (already bool, err error) {
	if s.redis == nil {
		return false, nil
	}
	key := bus.DedupKey(msg.DedupKey)
	if key == "compete:dedup:" {
		key = bus.DedupKey(msg.MessageID)
	}
	ok, err := s.redis.SetNX(ctx, key, "1", 7*24*time.Hour).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func (s *Store) Blackboard(taskID string, useRedis bool) state.Blackboard {
	return state.NewBlackboard(useRedis, s.redis, taskID)
}

// Router 决定下一跳 topic（不执行 Agent）。
type Router struct {
	bus bus.Bus
}

func NewRouter(b bus.Bus) *Router {
	return &Router{bus: b}
}

func (r *Router) PublishNext(ctx context.Context, env protocol.MessageEnvelope) error {
	topic := topicForEnvelope(env)
	if topic == "" {
		return nil
	}
	return r.bus.Publish(ctx, topic, env)
}

func topicForEnvelope(env protocol.MessageEnvelope) string {
	switch env.MessageType {
	case protocol.MsgTaskCreated:
		return bus.TopicAgentCoordinator
	case protocol.MsgPlanReady:
		return bus.TopicAgentCollector
	case protocol.MsgMaterialsReady:
		return bus.TopicAgentAnalyst
	case protocol.MsgAnalysisReady:
		return bus.TopicAgentWriter
	case protocol.MsgReportReady:
		return bus.TopicAgentQA
	case protocol.MsgQAReject:
		return bus.TopicAgentCoordinator
	case protocol.MsgClarificationAnswered:
		return bus.TopicAgentCoordinator
	default:
		if env.ToAgent != "" {
			return bus.AgentCommandTopic(domain.AgentName(env.ToAgent))
		}
		return ""
	}
}

func NewTaskCreateEnvelope(task domain.Task, traceID string) protocol.MessageEnvelope {
	return protocol.NewTaskCreatedMessage(task, traceID)
}

func NewTraceID() string { return uuid.NewString() }
