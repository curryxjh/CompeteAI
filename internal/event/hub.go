package event

import (
	"sync"
)

// TaskEvent SSE 任务事件。
type TaskEvent struct {
	Type string
	Data any
}

// TaskHub 任务 SSE 事件中心。
type TaskHub struct {
	mu      sync.RWMutex
	subs    map[string]map[chan TaskEvent]struct{}
	history map[string][]TaskEvent
}

const maxHistory = 32

func NewTaskHub() *TaskHub {
	return &TaskHub{
		subs:    make(map[string]map[chan TaskEvent]struct{}),
		history: make(map[string][]TaskEvent),
	}
}

func (h *TaskHub) Subscribe(taskID string) (<-chan TaskEvent, func()) {
	ch := make(chan TaskEvent, 16)
	h.mu.Lock()
	if h.subs[taskID] == nil {
		h.subs[taskID] = make(map[chan TaskEvent]struct{})
	}
	h.subs[taskID][ch] = struct{}{}
	past := append([]TaskEvent{}, h.history[taskID]...)
	h.mu.Unlock()

	for _, ev := range past {
		select {
		case ch <- ev:
		default:
		}
	}

	unsub := func() {
		h.mu.Lock()
		if m := h.subs[taskID]; m != nil {
			delete(m, ch)
			if len(m) == 0 {
				delete(h.subs, taskID)
			}
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

func (h *TaskHub) Publish(taskID, eventType string, data any) {
	ev := TaskEvent{Type: eventType, Data: data}
	h.mu.Lock()
	buf := append(h.history[taskID], ev)
	if len(buf) > maxHistory {
		buf = buf[len(buf)-maxHistory:]
	}
	h.history[taskID] = buf
	subs := h.subs[taskID]
	channels := make([]chan TaskEvent, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	h.mu.Unlock()
	for _, ch := range channels {
		select {
		case ch <- ev:
		default:
		}
	}
}
