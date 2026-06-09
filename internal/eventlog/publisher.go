package eventlog

import (
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Record 持久化事件（SSE 消费）。
type Record struct {
	SequenceNo int64
	EventType  string
	Agent      string
	Payload    any
	CreatedAt  time.Time
}

// Publisher 写 DB event_logs（主存档）+ 可选内存 fan-out（dev）。
type Publisher struct {
	dao   *dao.EventLogDao
	mu    sync.RWMutex
	live  map[string]map[chan Record]struct{}
}

func NewPublisher(eventDao *dao.EventLogDao) *Publisher {
	return &Publisher{
		dao:  eventDao,
		live: make(map[string]map[chan Record]struct{}),
	}
}

func (p *Publisher) Publish(ctx context.Context, taskID, traceID, eventType, agent string, payload any) (Record, error) {
	raw, _ := json.Marshal(payload)
	entity, err := p.dao.Append(ctx, dao.EventLogEntity{
		TaskID: taskID, TraceID: traceID, EventType: eventType,
		Agent: agent, PayloadJSON: string(raw),
	})
	if err != nil {
		return Record{}, err
	}
	rec := Record{
		SequenceNo: entity.SequenceNo,
		EventType:  eventType,
		Agent:      agent,
		Payload:    payload,
		CreatedAt:  entity.CreatedAt,
	}
	p.broadcast(taskID, rec)
	return rec, nil
}

func (p *Publisher) broadcast(taskID string, rec Record) {
	p.mu.RLock()
	subs := p.live[taskID]
	channels := make([]chan Record, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	p.mu.RUnlock()
	for _, ch := range channels {
		select {
		case ch <- rec:
		default:
		}
	}
}

// SubscribeLive 实时订阅（Worker 写 DB 后 fan-out；API SSE 主读 DB）。
func (p *Publisher) SubscribeLive(taskID string) (<-chan Record, func()) {
	ch := make(chan Record, 32)
	p.mu.Lock()
	if p.live[taskID] == nil {
		p.live[taskID] = make(map[chan Record]struct{})
	}
	p.live[taskID][ch] = struct{}{}
	p.mu.Unlock()
	return ch, func() {
		p.mu.Lock()
		if m := p.live[taskID]; m != nil {
			delete(m, ch)
		}
		p.mu.Unlock()
		close(ch)
	}
}

// Reader 按 sequence 增量读取（SSE 持久化源）。
type Reader struct {
	dao *dao.EventLogDao
}

func NewReader(eventDao *dao.EventLogDao) *Reader {
	return &Reader{dao: eventDao}
}

func (r *Reader) ListSince(ctx context.Context, taskID string, afterSeq int64) ([]Record, error) {
	rows, err := r.dao.ListSince(ctx, taskID, afterSeq)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		var payload any
		_ = json.Unmarshal([]byte(row.PayloadJSON), &payload)
		out = append(out, Record{
			SequenceNo: row.SequenceNo,
			EventType:  row.EventType,
			Agent:      row.Agent,
			Payload:    payload,
			CreatedAt:  row.CreatedAt,
		})
	}
	return out, nil
}

// SSEEventName 企业事件名 → 前端 SSE 事件名。
func SSEEventName(stored string) string {
	switch stored {
	case "task_completed":
		return "task_complete"
	case "task_status":
		return "task_status"
	case "agent_progress":
		return "agent_state"
	case "tool_call_completed":
		return "tool_step"
	case "clarification_required":
		return "clarification"
	case "qa_rejected":
		return "rejection"
	default:
		return stored
	}
}

// MapLegacyEvent 将旧 TaskHub 事件名映射为企业事件名（写入 DB）。
func MapLegacyEvent(eventType string) string {
	switch eventType {
	case "task_started":
		return "task_started"
	case "task_status":
		return "task_status"
	case "agent_state":
		return "agent_progress"
	case "agent_thinking":
		return "agent_progress"
	case "tool_step":
		return "tool_call_completed"
	case "task_complete":
		return "task_completed"
	case "task_failed":
		return "task_failed"
	case "clarification":
		return "clarification_required"
	case "rejection":
		return "qa_rejected"
	default:
		return eventType
	}
}
