package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/llm"
	"context"
	"errors"
	"strings"
)

// Registry 管理全部 Agent 实例。
type Registry struct {
	order  []domain.AgentName
	agents map[domain.AgentName]domain.Agent
}

func NewRegistry(deps Deps) *Registry {
	tools := deps.Tools
	chat := deps.Chat
	r := &Registry{
		order: []domain.AgentName{
			domain.AgentCoordinator,
			domain.AgentCollector,
			domain.AgentAnalyst,
			domain.AgentWriter,
			domain.AgentQA,
		},
		agents: map[domain.AgentName]domain.Agent{},
	}
	r.agents[domain.AgentCoordinator] = NewCoordinator()
	r.agents[domain.AgentCollector] = NewCollector(tools)
	r.agents[domain.AgentAnalyst] = NewAnalyst(chat)
	r.agents[domain.AgentWriter] = NewWriter(chat)
	r.agents[domain.AgentQA] = NewQA()
	return r
}

func (r *Registry) Get(name domain.AgentName) (domain.Agent, bool) {
	a, ok := r.agents[name]
	return a, ok
}

func (r *Registry) PipelineOrder() []domain.AgentName {
	out := make([]domain.AgentName, len(r.order))
	copy(out, r.order)
	return out
}

func (r *Registry) Cards() []WebAgentCard {
	out := make([]WebAgentCard, 0, len(r.order))
	for _, name := range r.order {
		if a, ok := r.agents[name]; ok {
			if carder, ok := a.(interface{ Card() AgentCard }); ok {
				out = append(out, carder.Card().ToWeb())
			}
		}
	}
	return out
}

func (r *Registry) Card(name domain.AgentName) (WebAgentCard, bool) {
	a, ok := r.agents[name]
	if !ok {
		return WebAgentCard{}, false
	}
	if carder, ok := a.(interface{ Card() AgentCard }); ok {
		return carder.Card().ToWeb(), true
	}
	return WebAgentCard{}, false
}

// EinoChatAdapter 将 ChatService 适配为 ChatClient。
type EinoChatAdapter struct {
	svc *llm.ChatService
}

func NewEinoChatAdapter(svc *llm.ChatService) *EinoChatAdapter {
	return &EinoChatAdapter{svc: svc}
}

func (a *EinoChatAdapter) Chat(ctx context.Context, system, user string) (string, error) {
	return a.StreamChat(ctx, system, user, nil)
}

func (a *EinoChatAdapter) StreamChat(
	ctx context.Context,
	system, user string,
	onChunk func(eventType, content string),
) (string, error) {
	var full strings.Builder
	err := a.svc.StreamChat(ctx, []domain.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, func(ev llm.StreamEvent) error {
		switch ev.Type {
		case llm.StreamEventThinking:
			if onChunk != nil && ev.Content != "" {
				onChunk("thinking", ev.Content)
			}
		case llm.StreamEventContent:
			full.WriteString(ev.Content)
			if onChunk != nil && ev.Content != "" {
				onChunk("content", ev.Content)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if full.Len() == 0 {
		return "", errors.New("empty response from model")
	}
	return full.String(), nil
}

// EinoToolAdapter 将 ToolRegistry 适配为 ToolInvoker。
type EinoToolAdapter struct {
	reg *llm.ToolRegistry
}

func NewEinoToolAdapter(reg *llm.ToolRegistry) *EinoToolAdapter {
	return &EinoToolAdapter{reg: reg}
}

func (a *EinoToolAdapter) Enabled() bool {
	return a.reg != nil && a.reg.Enabled()
}

func (a *EinoToolAdapter) Invoke(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	if a.reg == nil {
		return "", nil
	}
	if rep, ok := toolStepReporterFrom(ctx); ok {
		argsText := formatToolArgs(args)
		rep(ToolStepEvent{ToolName: name, Args: argsText, Status: "running"})
		raw, err := a.reg.Invoke(ctx, name, args)
		status := "done"
		if err != nil {
			status = "error"
		}
		rep(ToolStepEvent{ToolName: name, Args: argsText, Result: raw, Status: status})
		return raw, err
	}
	return a.reg.Invoke(ctx, name, args)
}
