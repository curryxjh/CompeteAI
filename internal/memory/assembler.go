package memory

import (
	"CompeteAI/internal/repository/dao"
	"CompeteAI/settings"
	"context"
	"sort"
	"strings"
)

// Assembler 组装 AgentMemoryContext（§11）。
type Assembler struct {
	retriever Retriever
	dao       *dao.MemoryDao
	scopes    *ScopeResolver
	cfg       *settings.MemoryConfig
	ranker    *Ranker
	explicit  *ExplicitLoader
}

func NewAssembler(ret Retriever, d *dao.MemoryDao, cfg *settings.MemoryConfig) *Assembler {
	path := "COMPETE.md"
	if cfg != nil && cfg.ExplicitFile != "" {
		path = cfg.ExplicitFile
	}
	return &Assembler{
		retriever: ret, dao: d, scopes: NewScopeResolver(d),
		cfg: cfg, ranker: NewRanker(), explicit: NewExplicitLoader(path),
	}
}

func (a *Assembler) Build(ctx context.Context, req BuildContextRequest) (AgentMemoryContext, error) {
	trace := RetrievalTrace{Query: req.Query}
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = defaultWorkspaceID
	}
	scopeReq := ScopeResolveRequest{
		ProjectID: projectID, WorkspaceID: wsID,
		UserID: req.UserID, Competitors: req.Competitors,
	}
	scopeIDs := a.scopes.ResolveIDs(ctx, scopeReq)

	topFacts := 12
	topEp := 4
	topEv := 8
	if a.cfg != nil {
		if a.cfg.Retrieval.TopKFacts > 0 {
			topFacts = a.cfg.Retrieval.TopKFacts
		}
		if a.cfg.Retrieval.TopKEpisodes > 0 {
			topEp = a.cfg.Retrieval.TopKEpisodes
		}
		if a.cfg.Retrieval.TopKEvidence > 0 {
			topEv = a.cfg.Retrieval.TopKEvidence
		}
	}

	query := req.Query
	if query == "" {
		query = strings.Join(req.Competitors, " vs ")
	}
	trace.RewrittenQuery = query

	prefs := a.loadPreferences(ctx, scopeReq)

	factHits, _ := a.retriever.RetrieveFacts(ctx, RetrievalRequest{
		Query: query, ScopeIDs: scopeIDs, Competitors: req.Competitors, Limit: topFacts,
	})
	epHits, _ := a.retriever.RetrieveEpisodes(ctx, RetrievalRequest{
		Query: query, ScopeIDs: scopeIDs, Limit: topEp,
	})
	evHits, _ := a.retriever.RetrieveEvidence(ctx, RetrievalRequest{
		Query: query, TaskID: req.TaskID, Limit: topEv,
	})

	ctxOut := AgentMemoryContext{Preferences: prefs, Debug: trace}
	for _, h := range factHits {
		ctxOut.Facts = append(ctxOut.Facts, h.Fact)
	}
	for _, h := range epHits {
		ctxOut.Episodes = append(ctxOut.Episodes, h.Episode)
	}
	for _, h := range evHits {
		ctxOut.Evidence = append(ctxOut.Evidence, h.Chunk)
	}

	ctxOut = a.trimByAgent(req.Agent, req.TokenBudget, ctxOut)
	ctxOut.Debug.CandidateFacts = len(factHits)
	ctxOut.Debug.CandidateEvidence = len(evHits)
	ctxOut.Debug.InjectedTokens = estimateContextTokens(ctxOut)
	recordInjectedTokens(ctxOut.Debug.InjectedTokens)
	return ctxOut, nil
}

// loadPreferences 合并 COMPETE.md + DB 多层 scope 偏好（§15.3）。
func (a *Assembler) loadPreferences(ctx context.Context, req ScopeResolveRequest) []Preference {
	merged := map[string]Preference{}

	// COMPETE.md — 优先级 100
	if filePrefs, err := a.explicit.Load(); err == nil {
		for _, p := range filePrefs {
			merged[p.Key] = Preference{
				Key: p.Key, Value: p.Value, Priority: 100, Source: "COMPETE.md",
			}
		}
	}

	if a.dao == nil {
		return sortPreferences(merged)
	}

	refs := a.scopes.ScopeIDsForPreferences(ctx, req)
	for _, ref := range refs {
		rows, err := a.dao.ListPreferences(ctx, []string{ref.ID})
		if err != nil {
			continue
		}
		for _, row := range rows {
			effectivePri := ref.Rank + row.Priority
			cur, exists := merged[row.PrefKey]
			if exists && cur.Priority <= effectivePri {
				continue
			}
			merged[row.PrefKey] = Preference{
				ID: row.ID, ScopeType: ref.ScopeType, ScopeKey: ref.ID,
				Key: row.PrefKey, Value: parseJSONMap(row.PrefValueJSON),
				Priority: effectivePri, Source: row.SourceType,
			}
		}
	}
	return sortPreferences(merged)
}

func sortPreferences(m map[string]Preference) []Preference {
	out := make([]Preference, 0, len(m))
	for _, p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].Key < out[j].Key
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func (a *Assembler) trimByAgent(agent string, budget int, ctx AgentMemoryContext) AgentMemoryContext {
	switch agent {
	case "coordinator":
		ctx.Facts = nil
		ctx.Evidence = nil
	case "collector":
		ctx.Episodes = trimEpisodes(ctx.Episodes, 2)
	case "writer":
		ctx.Evidence = nil
		ctx.Facts = trimFacts(ctx.Facts, 5)
	case "qa":
		// keep facts + evidence
	case "analyst":
		// keep all layers
	}
	for estimateContextTokens(ctx) > budget && len(ctx.Evidence) > 0 {
		ctx.Evidence = ctx.Evidence[:len(ctx.Evidence)-1]
	}
	for estimateContextTokens(ctx) > budget && len(ctx.Facts) > 0 {
		ctx.Facts = ctx.Facts[:len(ctx.Facts)-1]
	}
	for estimateContextTokens(ctx) > budget && len(ctx.Episodes) > 0 {
		ctx.Episodes = ctx.Episodes[:len(ctx.Episodes)-1]
	}
	return ctx
}

func trimFacts(facts []Fact, n int) []Fact {
	if len(facts) <= n {
		return facts
	}
	return facts[:n]
}

func trimEpisodes(eps []Episode, n int) []Episode {
	if len(eps) <= n {
		return eps
	}
	return eps[:n]
}

func estimateContextTokens(ctx AgentMemoryContext) int {
	return estimateTokens(FormatForPrompt(ctx))
}
