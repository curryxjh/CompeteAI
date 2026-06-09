package memory

import (
	"CompeteAI/internal/domain"
	"context"
	"time"
)

// ScopeType 记忆作用域类型。
type ScopeType string

const (
	ScopeUser      ScopeType = "user"
	ScopeWorkspace ScopeType = "workspace"
	ScopeProject   ScopeType = "project"
	ScopeEntity    ScopeType = "entity"
	ScopeGlobal    ScopeType = "global"
)

// MemoryService 长期记忆统一入口。
type MemoryService interface {
	Enabled() bool
	EnsureScopes(ctx context.Context, req EnsureScopesRequest) error
	BuildAgentContext(ctx context.Context, req BuildContextRequest) (AgentMemoryContext, error)

	IngestCollectorOutput(ctx context.Context, req IngestCollectorRequest) error
	IngestAnalysisOutput(ctx context.Context, req IngestAnalysisRequest) error
	IngestQAResult(ctx context.Context, req IngestQARequest) error
	IngestTaskCompletion(ctx context.Context, req IngestEpisodeRequest) error

	UpsertPreference(ctx context.Context, req UpsertPreferenceRequest) error
	SearchFacts(ctx context.Context, req SearchFactsRequest) ([]Fact, error)
	SearchEvidence(ctx context.Context, req SearchEvidenceRequest) ([]EvidenceChunk, error)
	ResolveEntities(ctx context.Context, req ResolveEntitiesRequest) ([]Entity, error)
	InvalidateFact(ctx context.Context, factID string, reason string) error
	ListPreferences(ctx context.Context, scopeType ScopeType, scopeKey string) ([]Preference, error)
}

type EnsureScopesRequest struct {
	TaskID      string
	UserID      int64
	WorkspaceID string
	ProjectID   string
	Competitors []string
}

type BuildContextRequest struct {
	TaskID      string
	Agent       string
	Query       string
	Competitors []string
	Dimensions  []string
	TriggerType string
	UserID      int64
	WorkspaceID string
	ProjectID   string
	TokenBudget int
}

type AgentMemoryContext struct {
	Preferences []Preference
	Facts       []Fact
	Episodes    []Episode
	Evidence    []EvidenceChunk
	Debug       RetrievalTrace
}

type RetrievalTrace struct {
	Query            string   `json:"query"`
	RewrittenQuery   string   `json:"rewritten_query,omitempty"`
	CandidateFacts   int      `json:"candidate_facts"`
	CandidateEvidence int     `json:"candidate_evidence"`
	InjectedTokens   int      `json:"injected_tokens"`
	FilterReasons    []string `json:"filter_reasons,omitempty"`
}

type Preference struct {
	ID        string         `json:"id"`
	ScopeType ScopeType      `json:"scopeType"`
	ScopeKey  string         `json:"scopeKey"`
	Key       string         `json:"key"`
	Value     map[string]any `json:"value"`
	Priority  int            `json:"priority"`
	Source    string         `json:"source"`
}

type Entity struct {
	ID             string   `json:"id"`
	CanonicalName  string   `json:"canonicalName"`
	EntityType     string   `json:"entityType"`
	NormalizedName string   `json:"normalizedName"`
	Aliases        []string `json:"aliases,omitempty"`
}

type Fact struct {
	ID                 string         `json:"id"`
	ScopeID            string         `json:"scopeId"`
	EntityID           string         `json:"entityId,omitempty"`
	FactType           string         `json:"factType"`
	Subject            string         `json:"subject"`
	Predicate          string         `json:"predicate"`
	ObjectValue        map[string]any `json:"objectValue"`
	ObjectText         string         `json:"objectText"`
	Summary            string         `json:"summary"`
	ConfidenceScore    float64        `json:"confidenceScore"`
	FreshnessScore     float64        `json:"freshnessScore"`
	ImportanceScore    float64        `json:"importanceScore"`
	VerificationStatus string         `json:"verificationStatus"`
	Status             string         `json:"status,omitempty"`
	SourceCount        int            `json:"sourceCount"`
	FinalScore         float64        `json:"finalScore,omitempty"`
}

type EvidenceChunk struct {
	ID           string  `json:"id"`
	SourceID     string  `json:"sourceId"`
	SourceURL    string  `json:"sourceUrl,omitempty"`
	SourceDomain string  `json:"sourceDomain,omitempty"`
	ContentText  string  `json:"contentText"`
	TokenCount   int     `json:"tokenCount"`
	SupportScore float64 `json:"supportScore,omitempty"`
	FinalScore   float64 `json:"finalScore,omitempty"`
}

type Episode struct {
	ID          string   `json:"id"`
	TaskID      string   `json:"taskId"`
	Title       string   `json:"title"`
	QueryText   string   `json:"queryText"`
	Summary     string   `json:"summary"`
	Competitors []string `json:"competitors,omitempty"`
	Dimensions  []string `json:"dimensions,omitempty"`
	Outcome     string   `json:"outcome"`
	QAScore     int      `json:"qaScore,omitempty"`
	Lessons     []string `json:"lessons,omitempty"`
	FinalScore  float64  `json:"finalScore,omitempty"`
}

type FactCandidate struct {
	FactType        string
	Subject         string
	Predicate       string
	ObjectValue     map[string]any
	ObjectText      string
	Summary         string
	ConfidenceScore float64
	InferenceType   string
	SourceID        string
	QuoteText       string
	EntityID        string
}

type IngestCollectorRequest struct {
	TaskID      string
	ProjectID   string
	Competitors []string
	Sources     map[string]domain.SourceRef
	Materials   string
	Query       string
}

type IngestAnalysisRequest struct {
	TaskID      string
	ProjectID   string
	AgentName   string
	Competitors []string
	Analysis    domain.Report
	Sources     map[string]domain.SourceRef
}

type IngestQARequest struct {
	TaskID       string
	ProjectID    string
	AgentName    string
	Score        int
	Passed       bool
	Issues       []string
	IssueDetails []QAIssueDetail
}

// QAIssueDetail QA 打回/issue 明细（用于 claim/fact 级精确治理）。
type QAIssueDetail struct {
	Location string
	Problem  string
	Category string
	Severity string
}

type IngestEpisodeRequest struct {
	TaskID      string
	ProjectID   string
	Title       string
	Query       string
	Competitors []string
	Dimensions  []string
	Outcome     string
	QAScore     int
	Lessons     []string
	Summary     string
}

type UpsertPreferenceRequest struct {
	ScopeType ScopeType
	ScopeKey  string
	Key       string
	Value     map[string]any
	Priority  int
	Source    string
}

type SearchFactsRequest struct {
	ScopeIDs    []string
	Competitors []string
	Query       string
	Limit       int
}

type SearchEvidenceRequest struct {
	TaskID string
	Query  string
	Limit  int
}

type ResolveEntitiesRequest struct {
	Names      []string
	EntityType string
}

type ExtractFactsRequest struct {
	TaskID      string
	Sources     map[string]domain.SourceRef
	SourceDBIDs map[string]string // blackboard ref key -> memory_sources.id
	Materials   string
	Competitors []string
}

type ExtractEpisodeRequest struct {
	TaskID      string
	Title       string
	Competitors []string
	Dimensions  []string
	Outcome     string
	QAScore     int
	Lessons     []string
	Summary     string
}

type RetrievalRequest struct {
	Query       string
	TaskID      string
	ScopeIDs    []string
	Competitors []string
	Limit       int
}

type FactHit struct {
	Fact         Fact
	SemanticScore float64
}

type EvidenceHit struct {
	Chunk        EvidenceChunk
	SemanticScore float64
}

type EpisodeHit struct {
	Episode      Episode
	SemanticScore float64
}

// Extractor 从任务产物抽取长期记忆候选。
type Extractor interface {
	ExtractFacts(ctx context.Context, req ExtractFactsRequest) ([]FactCandidate, error)
	ExtractEpisode(ctx context.Context, req ExtractEpisodeRequest) (Episode, error)
}

// Embedder 文本向量化。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
}

// Retriever 向量/关键词检索。
type Retriever interface {
	RetrieveFacts(ctx context.Context, req RetrievalRequest) ([]FactHit, error)
	RetrieveEvidence(ctx context.Context, req RetrievalRequest) ([]EvidenceHit, error)
	RetrieveEpisodes(ctx context.Context, req RetrievalRequest) ([]EpisodeHit, error)
}

// Governance 去重、合并、失效。
type Governance interface {
	Dedup(ctx context.Context, facts []FactCandidate) ([]FactCandidate, error)
	Merge(ctx context.Context, scopeID, taskID string, facts []FactCandidate) error
	Decay(ctx context.Context) error
	Invalidate(ctx context.Context, factID string, reason string) error
}

// Agent token budgets（§11.3）。
var AgentTokenBudget = map[string]int{
	"coordinator": 400,
	"collector":   500,
	"analyst":     1800,
	"writer":      1200,
	"qa":          1500,
}

const defaultProjectID = "compete-ai"
const defaultWorkspaceID = "default"

func Now() time.Time { return time.Now() }
