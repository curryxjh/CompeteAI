package dao

import (
	"time"
)

// MemoryScopeEntity 记忆作用域。
type MemoryScopeEntity struct {
	ID          string    `gorm:"column:id;primaryKey;size:36"`
	ScopeType   string    `gorm:"column:scope_type;size:32;not null;uniqueIndex:uk_scope,priority:1"`
	ScopeKey    string    `gorm:"column:scope_key;size:128;not null;uniqueIndex:uk_scope,priority:2"`
	DisplayName string    `gorm:"column:display_name;size:255;not null;default:''"`
	OwnerUserID *int64    `gorm:"column:owner_user_id"`
	OwnerOrgID  *string   `gorm:"column:owner_org_id;size:64"`
	ProjectID   *string   `gorm:"column:project_id;size:64"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryScopeEntity) TableName() string { return "memory_scopes" }

// MemoryEntityEntity 长期知识实体锚点。
type MemoryEntityEntity struct {
	ID               string    `gorm:"column:id;primaryKey;size:36"`
	CanonicalName    string    `gorm:"column:canonical_name;size:255;not null"`
	EntityType       string    `gorm:"column:entity_type;size:64;not null;uniqueIndex:uk_entity,priority:1"`
	NormalizedName   string    `gorm:"column:normalized_name;size:255;not null;uniqueIndex:uk_entity,priority:2"`
	AliasesJSON      string    `gorm:"column:aliases_json;type:json"`
	ExternalRefsJSON string    `gorm:"column:external_refs_json;type:json"`
	ConfidenceScore  float64   `gorm:"column:confidence_score;type:decimal(5,4);not null;default:0.8000"`
	Status           string    `gorm:"column:status;size:32;not null;default:'active'"`
	FirstSeenAt      time.Time `gorm:"column:first_seen_at"`
	LastSeenAt       time.Time `gorm:"column:last_seen_at"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryEntityEntity) TableName() string { return "memory_entities" }

// MemoryFactEntity 长期事实。
type MemoryFactEntity struct {
	ID                 string     `gorm:"column:id;primaryKey;size:36"`
	ScopeID            string     `gorm:"column:scope_id;size:36;not null;index:idx_fact_scope,priority:1"`
	EntityID           *string    `gorm:"column:entity_id;size:36;index:idx_fact_entity,priority:1"`
	FactType           string     `gorm:"column:fact_type;size:64;not null"`
	Subject            string     `gorm:"column:subject;size:255;not null"`
	Predicate          string     `gorm:"column:predicate;size:128;not null;index:idx_fact_pred,priority:1"`
	ObjectValueJSON    string     `gorm:"column:object_value_json;type:json;not null"`
	ObjectText         string     `gorm:"column:object_text;type:text;not null"`
	NormalizedHash     string     `gorm:"column:normalized_hash;size:64;not null;uniqueIndex:uk_fact_dedup,priority:2"`
	Summary            string     `gorm:"column:summary;type:text;not null"`
	ConfidenceScore    float64    `gorm:"column:confidence_score;type:decimal(5,4);not null;default:0.7000"`
	FreshnessScore     float64    `gorm:"column:freshness_score;type:decimal(5,4);not null;default:0.7000"`
	ImportanceScore    float64    `gorm:"column:importance_score;type:decimal(5,4);not null;default:0.5000"`
	SourceCount        int        `gorm:"column:source_count;not null;default:0"`
	InferenceType      string     `gorm:"column:inference_type;size:32;not null;default:'extracted'"`
	VerificationStatus string     `gorm:"column:verification_status;size:32;not null;default:'pending';index:idx_fact_scope,priority:3"`
	Status             string     `gorm:"column:status;size:32;not null;default:'active';index:idx_fact_scope,priority:2;index:idx_fact_entity,priority:2;index:idx_fact_pred,priority:2"`
	FirstSeenAt        time.Time  `gorm:"column:first_seen_at"`
	LastSeenAt         time.Time  `gorm:"column:last_seen_at"`
	LastVerifiedAt     *time.Time `gorm:"column:last_verified_at"`
	InvalidatedAt      *time.Time `gorm:"column:invalidated_at"`
	CreatedByTaskID    *string    `gorm:"column:created_by_task_id;size:64"`
	UpdatedByTaskID    *string    `gorm:"column:updated_by_task_id;size:64"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryFactEntity) TableName() string { return "memory_facts" }

// MemorySourceEntity 证据来源。
type MemorySourceEntity struct {
	ID              string    `gorm:"column:id;primaryKey;size:36"`
	TaskID          string    `gorm:"column:task_id;size:64;not null;index:idx_source_task"`
	SourceURL       string    `gorm:"column:source_url;type:text;not null"`
	SourceDomain    string    `gorm:"column:source_domain;size:255;not null;index:idx_source_domain"`
	Title           string    `gorm:"column:title;size:512;not null;default:''"`
	Excerpt         string    `gorm:"column:excerpt;type:text"`
	ContentHash     string    `gorm:"column:content_hash;size:64;not null;uniqueIndex:uk_source_hash"`
	CollectedAt     time.Time `gorm:"column:collected_at"`
	PublishedAt     *time.Time `gorm:"column:published_at"`
	ReliabilityTier string    `gorm:"column:reliability_tier;size:32;not null;default:'unknown'"`
	Language        string    `gorm:"column:language;size:16;not null;default:'zh'"`
	MetadataJSON    string    `gorm:"column:metadata_json;type:json"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemorySourceEntity) TableName() string { return "memory_sources" }

// MemoryEvidenceLinkEntity 事实-证据链接。
type MemoryEvidenceLinkEntity struct {
	ID           string    `gorm:"column:id;primaryKey;size:36"`
	FactID       string    `gorm:"column:fact_id;size:36;not null;index:idx_ev_fact;uniqueIndex:uk_fact_quote,priority:1"`
	SourceID     string    `gorm:"column:source_id;size:36;not null;index:idx_ev_source"`
	QuoteText    string    `gorm:"column:quote_text;type:text;not null"`
	QuoteHash    string    `gorm:"column:quote_hash;size:64;not null;uniqueIndex:uk_fact_quote,priority:2"`
	SupportType  string    `gorm:"column:support_type;size:32;not null;default:'direct'"`
	SupportScore float64   `gorm:"column:support_score;type:decimal(5,4);not null;default:0.7000"`
	StartOffset  *int      `gorm:"column:start_offset"`
	EndOffset    *int      `gorm:"column:end_offset"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemoryEvidenceLinkEntity) TableName() string { return "memory_evidence_links" }

// MemoryClaimEntity 模型结论（待验证）。
type MemoryClaimEntity struct {
	ID                 string    `gorm:"column:id;primaryKey;size:36"`
	TaskID             string    `gorm:"column:task_id;size:64;not null;index:idx_claim_task;uniqueIndex:uk_claim,priority:1"`
	FactID             *string   `gorm:"column:fact_id;size:36;index:idx_claim_fact"`
	AgentName          string    `gorm:"column:agent_name;size:64;not null"`
	ClaimText          string    `gorm:"column:claim_text;type:text;not null"`
	ClaimHash          string    `gorm:"column:claim_hash;size:64;not null;uniqueIndex:uk_claim,priority:2"`
	ClaimType          string    `gorm:"column:claim_type;size:32;not null;default:'analysis'"`
	VerificationStatus string    `gorm:"column:verification_status;size:32;not null;default:'pending'"`
	QAResult           *string   `gorm:"column:qa_result;size:32"`
	EvidenceSummary    string    `gorm:"column:evidence_summary;type:text"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryClaimEntity) TableName() string { return "memory_claims" }

// MemoryEpisodeEntity 任务经验。
type MemoryEpisodeEntity struct {
	ID               string    `gorm:"column:id;primaryKey;size:36"`
	ScopeID          string    `gorm:"column:scope_id;size:36;not null;index:idx_episode_scope,priority:1"`
	TaskID           string    `gorm:"column:task_id;size:64;not null;uniqueIndex:uk_episode_task"`
	Title            string    `gorm:"column:title;size:512;not null"`
	QueryText        string    `gorm:"column:query_text;type:text;not null"`
	Summary          string    `gorm:"column:summary;type:text;not null"`
	CompetitorsJSON  string    `gorm:"column:competitors_json;type:json"`
	DimensionsJSON   string    `gorm:"column:dimensions_json;type:json"`
	Outcome          string    `gorm:"column:outcome;size:32;not null"`
	QAScore          *int      `gorm:"column:qa_score"`
	LessonsJSON      string    `gorm:"column:lessons_json;type:json"`
	ArtifactRefsJSON string    `gorm:"column:artifact_refs_json;type:json"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime;index:idx_episode_scope,priority:2"`
}

func (MemoryEpisodeEntity) TableName() string { return "memory_episodes" }

// MemoryPreferenceEntity 显式偏好。
type MemoryPreferenceEntity struct {
	ID            string    `gorm:"column:id;primaryKey;size:36"`
	ScopeID       string    `gorm:"column:scope_id;size:36;not null;uniqueIndex:uk_pref,priority:1"`
	PrefKey       string    `gorm:"column:pref_key;size:128;not null;uniqueIndex:uk_pref,priority:2"`
	PrefValueJSON string    `gorm:"column:pref_value_json;type:json;not null"`
	Priority      int       `gorm:"column:priority;not null;default:100"`
	SourceType    string    `gorm:"column:source_type;size:32;not null;default:'manual'"`
	Status        string    `gorm:"column:status;size:32;not null;default:'active'"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryPreferenceEntity) TableName() string { return "memory_preferences" }

// MemoryEventEntity 记忆变更审计。
type MemoryEventEntity struct {
	ID              string    `gorm:"column:id;primaryKey;size:36"`
	AggregateType   string    `gorm:"column:aggregate_type;size:32;not null;index:idx_event_agg,priority:1"`
	AggregateID     string    `gorm:"column:aggregate_id;size:36;not null;index:idx_event_agg,priority:2"`
	EventType       string    `gorm:"column:event_type;size:64;not null"`
	PayloadJSON     string    `gorm:"column:payload_json;type:json;not null"`
	CreatedByTaskID *string   `gorm:"column:created_by_task_id;size:64"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;index:idx_event_agg,priority:3"`
}

func (MemoryEventEntity) TableName() string { return "memory_events" }

// MemoryEmbeddingEntity 向量索引（MySQL JSON 存储，可迁移 pgvector）。
type MemoryEmbeddingEntity struct {
	ID           string    `gorm:"column:id;primaryKey;size:36"`
	ObjectType   string    `gorm:"column:object_type;size:32;not null;index:idx_mem_embed_object,priority:1"`
	ObjectID     string    `gorm:"column:object_id;size:36;not null;index:idx_mem_embed_object,priority:2"`
	ScopeType    string    `gorm:"column:scope_type;size:32;not null;index:idx_mem_embed_scope,priority:1"`
	ScopeKey     string    `gorm:"column:scope_key;size:128;not null;index:idx_mem_embed_scope,priority:2"`
	EntityID     *string   `gorm:"column:entity_id;size:36;index:idx_mem_embed_entity"`
	ContentText  string    `gorm:"column:content_text;type:text;not null"`
	MetadataJSON string    `gorm:"column:metadata_json;type:json"`
	EmbeddingJSON string   `gorm:"column:embedding_json;type:json;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemoryEmbeddingEntity) TableName() string { return "memory_embeddings" }

// MemoryChunkEntity 来源 chunk 向量。
type MemoryChunkEntity struct {
	ID            string    `gorm:"column:id;primaryKey;size:36"`
	SourceID      string    `gorm:"column:source_id;size:36;not null;index:idx_chunk_source,priority:1"`
	TaskID        string    `gorm:"column:task_id;size:64;not null;index:idx_chunk_task"`
	ChunkIndex    int       `gorm:"column:chunk_index;not null;index:idx_chunk_source,priority:2"`
	ContentText   string    `gorm:"column:content_text;type:text;not null"`
	TokenCount    int       `gorm:"column:token_count;not null"`
	MetadataJSON  string    `gorm:"column:metadata_json;type:json"`
	EmbeddingJSON string    `gorm:"column:embedding_json;type:json;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemoryChunkEntity) TableName() string { return "memory_chunks" }
