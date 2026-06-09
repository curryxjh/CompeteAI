package eventlog

import (
	"CompeteAI/internal/events"
	"CompeteAI/internal/repository/dao"
	"context"
)

// HybridHub 双写：内存 fan-out（实时）+ DB event_logs（持久化 / SSE 回放）。
type HybridHub struct {
	*events.TaskHub
	pub *Publisher
}

func NewHybridHub(eventDao *dao.EventLogDao) *HybridHub {
	h := &HybridHub{TaskHub: events.NewTaskHub()}
	if eventDao != nil {
		h.pub = NewPublisher(eventDao)
	}
	return h
}

func (h *HybridHub) Publish(taskID, eventType string, data any) {
	h.TaskHub.Publish(taskID, eventType, data)
	if h.pub != nil {
		_, _ = h.pub.Publish(context.Background(), taskID, "", MapLegacyEvent(eventType), "", data)
	}
}

func (h *HybridHub) Reader() *Reader {
	if h.pub == nil {
		return nil
	}
	return NewReader(h.pub.dao)
}
