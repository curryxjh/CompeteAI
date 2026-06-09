package agent

import (
	"context"
	"encoding/json"
)

type toolStepCtxKey struct{}

// ToolStepEvent 工具执行步骤（供 SSE / Chat 展示）。
type ToolStepEvent struct {
	ToolName string
	Args     string
	Result   string
	Status   string // running | done | error
}

// ToolStepReporter 工具步骤回调。
type ToolStepReporter func(ev ToolStepEvent)

func WithToolStepReporter(ctx context.Context, rep ToolStepReporter) context.Context {
	if rep == nil {
		return ctx
	}
	return context.WithValue(ctx, toolStepCtxKey{}, rep)
}

func toolStepReporterFrom(ctx context.Context) (ToolStepReporter, bool) {
	rep, ok := ctx.Value(toolStepCtxKey{}).(ToolStepReporter)
	return rep, ok && rep != nil
}

func formatToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(raw)
}
