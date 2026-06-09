package memory

import (
	"CompeteAI/internal/repository/dao"
	"CompeteAI/settings"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// NoopService 记忆关闭时的空实现。
type NoopService struct{}

func (NoopService) Enabled() bool { return false }
func (NoopService) EnsureScopes(context.Context, EnsureScopesRequest) error { return nil }
func (NoopService) BuildAgentContext(context.Context, BuildContextRequest) (AgentMemoryContext, error) {
	return AgentMemoryContext{}, nil
}
func (NoopService) IngestCollectorOutput(context.Context, IngestCollectorRequest) error { return nil }
func (NoopService) IngestAnalysisOutput(context.Context, IngestAnalysisRequest) error   { return nil }
func (NoopService) IngestQAResult(context.Context, IngestQARequest) error { return nil }
func (NoopService) IngestTaskCompletion(context.Context, IngestEpisodeRequest) error  { return nil }
func (NoopService) UpsertPreference(context.Context, UpsertPreferenceRequest) error   { return nil }
func (NoopService) SearchFacts(context.Context, SearchFactsRequest) ([]Fact, error)   { return nil, nil }
func (NoopService) SearchEvidence(context.Context, SearchEvidenceRequest) ([]EvidenceChunk, error) {
	return nil, nil
}
func (NoopService) ResolveEntities(context.Context, ResolveEntitiesRequest) ([]Entity, error) {
	return nil, nil
}
func (NoopService) InvalidateFact(context.Context, string, string) error { return nil }
func (NoopService) ListPreferences(context.Context, ScopeType, string) ([]Preference, error) {
	return nil, nil
}

type service struct {
	dao        *dao.MemoryDao
	cfg        *settings.MemoryConfig
	extractor  Extractor
	embedder   Embedder
	retriever  Retriever
	governance Governance
	explicit   *ExplicitLoader
	assembler  *Assembler
	resolver   *Resolver
	ranker     *Ranker
	scopeCache sync.Map
}

// NewService 构建长期记忆服务。
func NewService(dbDao *dao.MemoryDao, cfg *settings.MemoryConfig) MemoryService {
	if cfg == nil || !cfg.Enabled {
		return NoopService{}
	}
	emb := NewHashEmbedder(128)
	gov := NewGovernance(dbDao, cfg)
	resolver := NewResolver(dbDao)
	ext := NewExtractor(resolver)
	ret := NewMySQLRetriever(dbDao, emb)
	return &service{
		dao: dbDao, cfg: cfg,
		extractor: ext, embedder: emb, retriever: ret,
		governance: gov, explicit: NewExplicitLoader(cfg.ExplicitFile),
		assembler: NewAssembler(ret, dbDao, cfg), resolver: resolver, ranker: NewRanker(),
	}
}

func (s *service) Enabled() bool { return true }

func (s *service) EnsureScopes(ctx context.Context, req EnsureScopesRequest) error {
	projectID := req.ProjectID
	if projectID == "" {
		projectID = s.cfg.ProjectID
	}
	if projectID == "" {
		projectID = defaultProjectID
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = s.cfg.WorkspaceID
	}
	if wsID == "" {
		wsID = defaultWorkspaceID
	}
	now := Now()
	scopes := []dao.MemoryScopeEntity{
		{ID: uuid.NewString(), ScopeType: string(ScopeGlobal), ScopeKey: "default", DisplayName: "Global", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.NewString(), ScopeType: string(ScopeProject), ScopeKey: projectID, DisplayName: projectID, ProjectID: &projectID, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.NewString(), ScopeType: string(ScopeWorkspace), ScopeKey: wsID, DisplayName: wsID, CreatedAt: now, UpdatedAt: now},
	}
	if req.UserID > 0 {
		uid := req.UserID
		key := fmt.Sprintf("user:%d", uid)
		scopes = append(scopes, dao.MemoryScopeEntity{
			ID: uuid.NewString(), ScopeType: string(ScopeUser), ScopeKey: key,
			DisplayName: key, OwnerUserID: &uid, CreatedAt: now, UpdatedAt: now,
		})
	}
	for _, c := range req.Competitors {
		norm := NormalizeName(c)
		scopes = append(scopes, dao.MemoryScopeEntity{
			ID: uuid.NewString(), ScopeType: string(ScopeEntity), ScopeKey: "entity:"+norm,
			DisplayName: c, CreatedAt: now, UpdatedAt: now,
		})
		_, _ = s.resolver.EnsureEntity(ctx, "competitor", c)
	}
	for _, sc := range scopes {
		if err := s.dao.UpsertScope(ctx, sc); err != nil {
			return err
		}
	}
	return s.syncExplicitFile(ctx, projectID)
}

func (s *service) syncExplicitFile(ctx context.Context, projectID string) error {
	prefs, err := s.explicit.Load()
	if err != nil || len(prefs) == 0 {
		return err
	}
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(ScopeProject), projectID)
	if err != nil {
		return nil
	}
	for _, p := range prefs {
		_ = s.dao.UpsertPreference(ctx, dao.MemoryPreferenceEntity{
			ID: uuid.NewString(), ScopeID: scope.ID, PrefKey: p.Key,
			PrefValueJSON: dao.MarshalJSON(p.Value), Priority: p.Priority,
			SourceType: "file", Status: "active",
		})
	}
	return nil
}

func (s *service) BuildAgentContext(ctx context.Context, req BuildContextRequest) (AgentMemoryContext, error) {
	start := time.Now()
	defer func() { recordBuildLatency(time.Since(start)) }()
	if req.TokenBudget <= 0 {
		if b, ok := AgentTokenBudget[req.Agent]; ok {
			req.TokenBudget = b
		} else {
			req.TokenBudget = 800
		}
	}
	return s.assembler.Build(ctx, req)
}

func (s *service) IngestCollectorOutput(ctx context.Context, req IngestCollectorRequest) error {
	start := time.Now()
	defer func() { recordIngestLatency(time.Since(start)) }()
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(ScopeProject), projectID)
	if err != nil {
		return err
	}
	now := Now()
	sourceDBIDs := map[string]string{}
	for refKey, src := range req.Sources {
		if src.URL == "" {
			continue
		}
		hash := ContentHash(src.URL + src.Excerpt)
		domainName := extractDomain(src.URL)
		tier := "unknown"
		if strings.Contains(domainName, ".") {
			tier = "web"
		}
		existing, err := s.dao.GetSourceByHash(ctx, hash)
		var sourceID string
		if err == nil {
			sourceID = existing.ID
		} else {
			sourceID = uuid.NewString()
			entity := dao.MemorySourceEntity{
				ID: sourceID, TaskID: req.TaskID, SourceURL: src.URL,
				SourceDomain: domainName, Title: src.Title, Excerpt: src.Excerpt,
				ContentHash: hash, CollectedAt: now, ReliabilityTier: tier,
				MetadataJSON: dao.MarshalJSON(map[string]string{"title": src.Title, "ref_key": refKey}),
			}
			if err := s.dao.UpsertSource(ctx, entity); err != nil {
				return err
			}
		}
		sourceDBIDs[refKey] = sourceID
		chunks := chunkText(src.Excerpt, 512)
		if len(chunks) == 0 && src.Excerpt != "" {
			chunks = []string{src.Excerpt}
		}
		vecs, _ := s.embedder.Embed(ctx, chunks)
		for i, ch := range chunks {
			vec := []float32{}
			if i < len(vecs) {
				vec = vecs[i]
			}
			_ = s.dao.CreateChunk(ctx, dao.MemoryChunkEntity{
				ID: uuid.NewString(), SourceID: sourceID, TaskID: req.TaskID,
				ChunkIndex: i, ContentText: ch, TokenCount: estimateTokens(ch),
				MetadataJSON: dao.MarshalJSON(map[string]any{"url": src.URL, "ref_key": refKey}),
				EmbeddingJSON: dao.MarshalJSON(vec),
			})
		}
	}
	for _, c := range req.Competitors {
		_, _ = s.resolver.EnsureEntity(ctx, "competitor", c)
	}
	candidates, err := s.extractor.ExtractFacts(ctx, ExtractFactsRequest{
		TaskID: req.TaskID, Sources: req.Sources, SourceDBIDs: sourceDBIDs,
		Materials: req.Materials, Competitors: req.Competitors,
	})
	if err != nil {
		return err
	}
	deduped, _ := s.governance.Dedup(ctx, candidates)
	return s.governance.Merge(ctx, scope.ID, req.TaskID, deduped)
}

func (s *service) IngestAnalysisOutput(ctx context.Context, req IngestAnalysisRequest) error {
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(ScopeProject), projectID)
	if err != nil {
		return err
	}
	agentName := req.AgentName
	if agentName == "" {
		agentName = "analyst"
	}
	for _, tier := range req.Analysis.Pricing {
		claimText := fmt.Sprintf("%s pricing: %d tiers", tier.Competitor, len(tier.Tiers))
		hash := ClaimHash(req.TaskID, claimText)
		_ = s.dao.UpsertClaim(ctx, dao.MemoryClaimEntity{
			ID: uuid.NewString(), TaskID: req.TaskID, AgentName: agentName,
			ClaimText: claimText, ClaimHash: hash, ClaimType: "analysis",
			VerificationStatus: "pending",
		})
	}
	for comp, swot := range req.Analysis.SWOT {
		claimText := fmt.Sprintf("%s SWOT strengths=%d weaknesses=%d", comp, len(swot.Strengths), len(swot.Weaknesses))
		hash := ClaimHash(req.TaskID, claimText)
		_ = s.dao.UpsertClaim(ctx, dao.MemoryClaimEntity{
			ID: uuid.NewString(), TaskID: req.TaskID, AgentName: agentName,
			ClaimText: claimText, ClaimHash: hash, ClaimType: "analysis",
			VerificationStatus: "pending",
		})
	}
	if len(req.Analysis.Sources) > 0 && s.cfg.WritePolicy.AllowPendingInference {
		var candidates []FactCandidate
		for comp, swot := range req.Analysis.SWOT {
			for _, sItem := range swot.Strengths {
				if sItem.Text == "" {
					continue
				}
				candidates = append(candidates, FactCandidate{
					FactType: "swot_strength", Subject: comp, Predicate: "has_strength",
					ObjectText: sItem.Text, Summary: sItem.Text,
					ConfidenceScore: 0.55, InferenceType: "verified_inference",
				})
			}
		}
		deduped, _ := s.governance.Dedup(ctx, candidates)
		_ = s.governance.Merge(ctx, scope.ID, req.TaskID, deduped)
	}
	return nil
}

func (s *service) IngestQAResult(ctx context.Context, req IngestQARequest) error {
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	issues := req.IssueDetails
	if len(issues) == 0 {
		for _, text := range req.Issues {
			issues = append(issues, QAIssueDetail{Problem: text})
		}
	}

	claims, _ := s.dao.ListClaimsByTask(ctx, req.TaskID)
	taskFacts, _ := s.dao.ListFactsByTask(ctx, req.TaskID)

	for _, cl := range claims {
		verification := "pending"
		qaResult := "partial"
		if req.Passed {
			if _, hit := claimMatchedByAnyIssue(cl.ClaimText, issues); hit {
				verification = "needs_review"
				qaResult = "partial"
			} else {
				verification = "verified"
				qaResult = "pass"
			}
		} else if iss, hit := claimMatchedByAnyIssue(cl.ClaimText, issues); hit {
			verification, qaResult = qaStatusForIssue(iss, false)
		} else if req.Score < 70 {
			verification = "rejected"
			qaResult = "fail"
		}

		qr := qaResult
		_ = s.dao.UpdateClaimVerification(ctx, cl.ID, verification, &qr, cl.FactID)

		if verification == "verified" && cl.FactID != nil {
			_ = s.dao.UpdateFactStatus(ctx, *cl.FactID, "active", "verified")
		}
		if verification == "rejected" && cl.FactID != nil {
			_ = s.governance.Invalidate(ctx, *cl.FactID, "qa_rejected:"+cl.ClaimText)
		}
	}

	for _, f := range taskFacts {
		linkCount, _ := s.dao.CountEvidenceLinks(ctx, f.ID)
		for _, iss := range issues {
			if !factMatchesIssue(f, iss) {
				continue
			}
			cat := strings.ToLower(iss.Category)
			if strings.Contains(cat, "source") || strings.Contains(strings.ToLower(iss.Problem), "来源") {
				if linkCount == 0 {
					_ = s.dao.UpdateFactStatus(ctx, f.ID, "active", "needs_review")
				}
				continue
			}
			if !req.Passed && (strings.Contains(cat, "unsupported") || strings.Contains(cat, "incomplete")) {
				_ = s.governance.Invalidate(ctx, f.ID, "qa:"+iss.Problem)
			}
		}
		if req.Passed && f.VerificationStatus == "pending" && linkCount > 0 {
			_ = s.dao.UpdateFactStatus(ctx, f.ID, "active", "verified")
		}
	}
	return nil
}

func (s *service) IngestTaskCompletion(ctx context.Context, req IngestEpisodeRequest) error {
	if s.cfg.WritePolicy.EpisodeOnCompletionOnly && req.Outcome != "completed" {
		return nil
	}
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(ScopeProject), projectID)
	if err != nil {
		return err
	}
	ep, err := s.extractor.ExtractEpisode(ctx, ExtractEpisodeRequest{
		TaskID: req.TaskID, Title: req.Title, Competitors: req.Competitors,
		Dimensions: req.Dimensions, Outcome: req.Outcome, QAScore: req.QAScore,
		Lessons: req.Lessons, Summary: req.Summary,
	})
	if err != nil {
		return err
	}
	score := req.QAScore
	entity := dao.MemoryEpisodeEntity{
		ID: uuid.NewString(), ScopeID: scope.ID, TaskID: req.TaskID,
		Title: ep.Title, QueryText: ep.QueryText, Summary: ep.Summary,
		CompetitorsJSON: dao.MarshalJSON(ep.Competitors),
		DimensionsJSON:  dao.MarshalJSON(ep.Dimensions),
		Outcome: req.Outcome, QAScore: &score,
		LessonsJSON: dao.MarshalJSON(ep.Lessons),
	}
	if err := s.dao.UpsertEpisode(ctx, entity); err != nil {
		return err
	}
	vec, _ := s.embedder.Embed(ctx, []string{ep.Summary})
	if len(vec) > 0 {
		_ = s.dao.CreateEmbedding(ctx, dao.MemoryEmbeddingEntity{
			ID: uuid.NewString(), ObjectType: "episode", ObjectID: entity.ID,
			ScopeType: string(ScopeProject), ScopeKey: projectID,
			ContentText: ep.Summary, EmbeddingJSON: dao.MarshalJSON(vec[0]),
		})
	}
	_ = s.dao.CreateEvent(ctx, dao.MemoryEventEntity{
		ID: uuid.NewString(), AggregateType: "episode", AggregateID: entity.ID,
		EventType: "episode.created", PayloadJSON: dao.MarshalJSON(ep),
		CreatedByTaskID: &req.TaskID,
	})
	return nil
}

func (s *service) UpsertPreference(ctx context.Context, req UpsertPreferenceRequest) error {
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(req.ScopeType), req.ScopeKey)
	if err != nil {
		now := Now()
		scope = dao.MemoryScopeEntity{
			ID: uuid.NewString(), ScopeType: string(req.ScopeType), ScopeKey: req.ScopeKey,
			DisplayName: req.ScopeKey, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.dao.UpsertScope(ctx, scope); err != nil {
			return err
		}
	}
	src := req.Source
	if src == "" {
		src = "manual"
	}
	pri := req.Priority
	if pri == 0 {
		pri = 100
	}
	return s.dao.UpsertPreference(ctx, dao.MemoryPreferenceEntity{
		ID: uuid.NewString(), ScopeID: scope.ID, PrefKey: req.Key,
		PrefValueJSON: dao.MarshalJSON(req.Value), Priority: pri,
		SourceType: src, Status: "active",
	})
}

func (s *service) SearchFacts(ctx context.Context, req SearchFactsRequest) ([]Fact, error) {
	hits, err := s.retriever.RetrieveFacts(ctx, RetrievalRequest{
		Query: req.Query, ScopeIDs: req.ScopeIDs, Competitors: req.Competitors, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Fact, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Fact)
	}
	return out, nil
}

func (s *service) SearchEvidence(ctx context.Context, req SearchEvidenceRequest) ([]EvidenceChunk, error) {
	hits, err := s.retriever.RetrieveEvidence(ctx, RetrievalRequest{
		Query: req.Query, TaskID: req.TaskID, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]EvidenceChunk, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Chunk)
	}
	return out, nil
}

func (s *service) ResolveEntities(ctx context.Context, req ResolveEntitiesRequest) ([]Entity, error) {
	etype := req.EntityType
	if etype == "" {
		etype = "competitor"
	}
	var out []Entity
	for _, name := range req.Names {
		ent, err := s.resolver.Resolve(ctx, etype, name)
		if err != nil {
			continue
		}
		out = append(out, ent)
	}
	return out, nil
}

func (s *service) InvalidateFact(ctx context.Context, factID, reason string) error {
	return s.governance.Invalidate(ctx, factID, reason)
}

func (s *service) ListPreferences(ctx context.Context, scopeType ScopeType, scopeKey string) ([]Preference, error) {
	scope, err := s.dao.GetScopeByTypeKey(ctx, string(scopeType), scopeKey)
	if err != nil {
		return nil, nil
	}
	rows, err := s.dao.ListPreferences(ctx, []string{scope.ID})
	if err != nil {
		return nil, err
	}
	return mapPreferences(rows), nil
}

// --- helpers ---

func NormalizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ContentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func FactHash(scopeID, subject, predicate, objectText string) string {
	raw := scopeID + "|" + NormalizeName(subject) + "|" + predicate + "|" + NormalizeName(objectText)
	return ContentHash(raw)
}

func ClaimHash(taskID, text string) string {
	return ContentHash(taskID + "|" + text)
}

func extractDomain(url string) string {
	re := regexp.MustCompile(`https?://([^/]+)`)
	m := re.FindStringSubmatch(url)
	if len(m) > 1 {
		return strings.TrimPrefix(m[1], "www.")
	}
	return ""
}

func chunkText(text string, size int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= size {
			chunks = append(chunks, text)
			break
		}
		chunks = append(chunks, text[:size])
		text = text[size:]
	}
	return chunks
}

func estimateTokens(s string) int {
	return len([]rune(s)) / 4
}

func mapPreferences(rows []dao.MemoryPreferenceEntity) []Preference {
	out := make([]Preference, 0, len(rows))
	for _, r := range rows {
		out = append(out, Preference{
			ID: r.ID, Key: r.PrefKey, Priority: r.Priority, Source: r.SourceType,
			Value: parseJSONMap(r.PrefValueJSON),
		})
	}
	return out
}

func parseJSONMap(raw string) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}
