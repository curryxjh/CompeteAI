package workflow

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/state"
	"CompeteAI/settings"
	"context"
)

func (e *Engine) memorySvc() memory.MemoryService {
	return e.mem
}

func (e *Engine) ensureMemoryScopes(ctx context.Context, task domain.Task) {
	mem := e.memorySvc()
	if mem == nil || !mem.Enabled() {
		return
	}
	projectID := defaultMemoryProjectID()
	_ = mem.EnsureScopes(ctx, memory.EnsureScopesRequest{
		TaskID: task.ID, ProjectID: projectID,
		Competitors: task.Competitors,
	})
}

func (e *Engine) buildMemoryContext(ctx context.Context, taskID, agentName string, store state.TaskStore, trigger string) memory.AgentMemoryContext {
	mem := e.memorySvc()
	if mem == nil || !mem.Enabled() {
		return memory.AgentMemoryContext{}
	}
	meta, _ := store.LoadTaskMeta(ctx)
	req := memory.BuildContextRequest{
		TaskID: taskID, Agent: agentName, TriggerType: trigger,
		Competitors: meta.Competitors, Dimensions: meta.Dimensions,
		Query: meta.Title, ProjectID: defaultMemoryProjectID(),
	}
	ctxOut, err := mem.BuildAgentContext(ctx, req)
	if err != nil {
		return memory.AgentMemoryContext{}
	}
	return ctxOut
}

func (e *Engine) ingestAfterAgent(ctx context.Context, agentName, taskID string, store state.TaskStore) {
	mem := e.memorySvc()
	if mem == nil || !mem.Enabled() {
		return
	}
	projectID := defaultMemoryProjectID()
	meta, _ := store.LoadTaskMeta(ctx)
	switch agentName {
	case string(domain.AgentCollector):
		out, err := store.LoadCollectorOutput(ctx)
		if err != nil {
			return
		}
		_ = mem.IngestCollectorOutput(ctx, memory.IngestCollectorRequest{
			TaskID: taskID, ProjectID: projectID,
			Competitors: meta.Competitors, Sources: out.Sources,
			Materials: out.Materials, Query: out.Query,
		})
	case string(domain.AgentAnalyst):
		analysis, aerr := store.LoadAnalysisOutput(ctx)
		if aerr != nil {
			return
		}
		report := domain.Report{
			SWOT: analysis.SWOT, Features: analysis.Features,
			Pricing: analysis.Pricing, Summary: analysis.Summary,
		}
		coll, _ := store.LoadCollectorOutput(ctx)
		sources := map[string]domain.SourceRef{}
		if coll.Sources != nil {
			sources = coll.Sources
		}
		_ = mem.IngestAnalysisOutput(ctx, memory.IngestAnalysisRequest{
			TaskID: taskID, ProjectID: projectID, AgentName: agentName,
			Competitors: meta.Competitors, Analysis: report, Sources: sources,
		})
	case string(domain.AgentQA):
		qa, err := store.LoadQAResult(ctx)
		if err != nil {
			return
		}
		var issues []string
		issues = append(issues, qa.Issues...)
		var issueDetails []memory.QAIssueDetail
		for _, iss := range qa.IssuesFull {
			issueDetails = append(issueDetails, memory.QAIssueDetail{
				Location: iss.Location, Problem: iss.Problem,
				Category: iss.Category, Severity: iss.Severity,
			})
		}
		if len(issueDetails) == 0 {
			for _, text := range qa.Issues {
				issueDetails = append(issueDetails, memory.QAIssueDetail{Problem: text})
			}
		}
		passed := qa.Result == "pass"
		_ = mem.IngestQAResult(ctx, memory.IngestQARequest{
			TaskID: taskID, ProjectID: projectID, AgentName: agentName,
			Score: qa.Score, Passed: passed, Issues: issues, IssueDetails: issueDetails,
		})
	}
}

func (e *Engine) ingestTaskCompletion(ctx context.Context, taskID string, store state.TaskStore, qaScore int) {
	mem := e.memorySvc()
	if mem == nil || !mem.Enabled() {
		return
	}
	meta, _ := store.LoadTaskMeta(ctx)
	report, _ := store.LoadReportFinal(ctx)
	_ = mem.IngestTaskCompletion(ctx, memory.IngestEpisodeRequest{
		TaskID: taskID, ProjectID: defaultMemoryProjectID(),
		Title: meta.Title, Query: meta.Title,
		Competitors: meta.Competitors, Dimensions: meta.Dimensions,
		Outcome: "completed", QAScore: qaScore, Summary: report.Summary,
	})
}

func defaultMemoryProjectID() string {
	if settings.Conf != nil && settings.Conf.MemoryConfig != nil && settings.Conf.MemoryConfig.ProjectID != "" {
		return settings.Conf.MemoryConfig.ProjectID
	}
	return "compete-ai"
}
