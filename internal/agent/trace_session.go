package agent

import (
	"CompeteAI/internal/domain"
	"context"
	"time"
)

type traceSessionKey struct{}

// TraceSession 单次 Agent 运行的可观测性会话（供 workflow 写入 Trace）。
type TraceSession struct {
	Steps      []domain.TraceStep
	StartedAt  time.Time
	TokenCount int
}

func WithTraceSession(ctx context.Context) (context.Context, *TraceSession) {
	s := &TraceSession{StartedAt: time.Now()}
	return context.WithValue(ctx, traceSessionKey{}, s), s
}

func TraceSessionFrom(ctx context.Context) (*TraceSession, bool) {
	s, ok := ctx.Value(traceSessionKey{}).(*TraceSession)
	return s, ok
}

func (s *TraceSession) AppendStep(kind, content, status, toolName string) {
	if s == nil || content == "" {
		return
	}
	s.Steps = append(s.Steps, domain.TraceStep{
		Kind: kind, Content: truncate(content, 2000), Status: status, ToolName: toolName,
	})
}

func (s *TraceSession) DurationMs() int {
	if s == nil {
		return 0
	}
	return int(time.Since(s.StartedAt).Milliseconds())
}
