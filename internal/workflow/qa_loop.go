package workflow

import (
	"CompeteAI/internal/agent"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/eino"
	"CompeteAI/internal/bus"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"
)

func (e *Engine) executeAgent(ctx context.Context, taskID string, name domain.AgentName, input agent.RunInput, bb state.Blackboard) (agent.RunOutput, error) {
	ag, ok := e.registry.Get(name)
	if !ok {
		return agent.RunOutput{}, fmt.Errorf("agent %s not registered", name)
	}
	agentCtx, traceSess := agent.WithTraceSession(ctx)
	agentCtx = agent.WithToolStepReporter(agentCtx, func(ev agent.ToolStepEvent) {
		args := ev.Args
		if formatted := eino.FormatToolArgsForUI(ev.Args); formatted != "" {
			args = formatted
		}
		result := ev.Result
		if ev.Status == "done" && ev.Result != "" {
			result = eino.FormatToolResultForUI(ev.ToolName, ev.Result)
		}
		e.hub.Publish(taskID, "tool_step", map[string]any{
			"tool_name": ev.ToolName, "tool_args": args,
			"tool_result": result, "status": ev.Status, "agent": string(name),
		})
		traceSess.AppendStep("tool", args, ev.Status, ev.ToolName)
		if ev.Status == "done" && result != "" {
			traceSess.AppendStep("tool_result", truncate(result, 500), "done", ev.ToolName)
		}
	})
	agentCtx = agent.WithAgentProgressReporter(agentCtx, func(ev agent.AgentProgressEvent) {
		e.hub.Publish(taskID, "agent_thinking", map[string]any{
			"agent": string(name), "kind": ev.Kind, "content": ev.Content, "status": ev.Status,
		})
		traceSess.AppendStep(ev.Kind, ev.Content, ev.Status, "")
	})
	out, err := e.runAgentWithRetry(agentCtx, ag, input, bb)
	if traceSess != nil {
		if out.Metadata == nil {
			out.Metadata = map[string]any{}
		}
		out.Metadata["trace_steps"] = traceSess.Steps
		out.Metadata["duration_ms"] = traceSess.DurationMs()
		tokenCount := 0
		if hint, ok := out.Metadata["token_hint"].(int); ok {
			tokenCount = hint
		}
		if tokenCount == 0 {
			tokenCount = estimateTokens(out.Summary)
		}
		out.Metadata["token_count"] = tokenCount
	}
	return out, err
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len([]rune(text)) / 2
}

// handleQAQuery QA 对单条结论发起追问 → Analyst 局部回复。
func (e *Engine) handleQAQuery(ctx context.Context, msg protocol.MessageEnvelope) error {
	var payload protocol.QAQueryPayload
	if err := msg.DecodePayload(&payload); err != nil {
		return err
	}
	bb := e.blackboard(msg.TaskID)
	store := state.ForTask(bb, msg.TaskID)
	_ = store.SaveAnalysisReviewNotes(ctx, state.AnalysisReviewNotes{
		Notes: []string{payload.Question},
	})
	input := agent.RunInput{
		TaskID: msg.TaskID, TraceID: msg.TraceID, Attempt: 1,
		TriggerType: "qa_query",
		Payload:     map[string]any{"query": payload.Question, "claim": payload.Claim},
	}
	out, err := e.executeAgent(ctx, msg.TaskID, domain.AgentAnalyst, input, bb)
	if err != nil {
		return err
	}
	resp := protocol.NewEnvelope(msg.TaskID, msg.TraceID, string(domain.AgentAnalyst), string(domain.AgentQA),
		protocol.MsgAnalysisReady, protocol.AnalystResponsePayload{
			Claim: payload.Claim, Evidence: out.Summary, Explanation: out.Summary,
		})
	return e.bus.Publish(ctx, bus.TopicAnalystReply, resp)
}

// handleAnalystResponse QA 收到 Analyst 证据回复后合并报告并重新质检。
func (e *Engine) handleAnalystResponse(ctx context.Context, msg protocol.MessageEnvelope) error {
	var payload protocol.AnalystResponsePayload
	if err := msg.DecodePayload(&payload); err != nil {
		return err
	}
	bb := e.blackboard(msg.TaskID)
	store := state.ForTask(bb, msg.TaskID)
	report, err := store.LoadReportFinal(ctx)
	if err == nil && payload.Evidence != "" {
		report.Summary = strings.TrimSpace(report.Summary) + "\n\n> 证据补充：" + payload.Evidence
		_ = store.SaveReportFinal(ctx, report)
	}
	e.hub.Publish(msg.TaskID, "agent_thinking", map[string]any{
		"agent": string(domain.AgentQA), "kind": "note",
		"content": "已收到 Analyst 证据回复，重新质检…", "status": "done",
	})
	next := protocol.NewEnvelope(msg.TaskID, msg.TraceID, string(domain.AgentAnalyst), string(domain.AgentQA),
		protocol.MsgAnalystResponse, payload)
	return e.handleAgentInput(ctx, domain.AgentQA, next)
}
