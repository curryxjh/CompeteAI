package agent

import "context"

type agentProgressCtxKey struct{}

// AgentProgressEvent Agent 执行过程事件（path / note / thinking / output / analysis）。
type AgentProgressEvent struct {
	Kind    string
	Content string
	Status  string // running | done
}

// AgentProgressReporter 进度回调。
type AgentProgressReporter func(ev AgentProgressEvent)

func WithAgentProgressReporter(ctx context.Context, rep AgentProgressReporter) context.Context {
	if rep == nil {
		return ctx
	}
	return context.WithValue(ctx, agentProgressCtxKey{}, rep)
}

func agentProgressFrom(ctx context.Context) (AgentProgressReporter, bool) {
	rep, ok := ctx.Value(agentProgressCtxKey{}).(AgentProgressReporter)
	return rep, ok && rep != nil
}

// EmitProgress 向上下文中的 Reporter 发送进度（无 Reporter 时静默）。
func EmitProgress(ctx context.Context, kind, content, status string) {
	if rep, ok := agentProgressFrom(ctx); ok {
		rep(AgentProgressEvent{Kind: kind, Content: content, Status: status})
	}
}
