package dao

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// jsonUnmarshal 是 json.Unmarshal 的本地别名，避免 import cycle。
var jsonUnmarshal = json.Unmarshal

type MemoryDao struct{ db *gorm.DB }

func NewMemoryDao(db *gorm.DB) *MemoryDao { return &MemoryDao{db: db} }

func (d *MemoryDao) UpsertScope(ctx context.Context, e MemoryScopeEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope_type"}, {Name: "scope_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "updated_at"}),
	}).Create(&e).Error
}

func (d *MemoryDao) GetScopeByTypeKey(ctx context.Context, scopeType, scopeKey string) (MemoryScopeEntity, error) {
	var row MemoryScopeEntity
	err := d.db.WithContext(ctx).Where("scope_type = ? AND scope_key = ?", scopeType, scopeKey).First(&row).Error
	return row, err
}

func (d *MemoryDao) UpsertEntity(ctx context.Context, e MemoryEntityEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "entity_type"}, {Name: "normalized_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"canonical_name", "aliases_json", "last_seen_at", "updated_at"}),
	}).Create(&e).Error
}

func (d *MemoryDao) FindEntityByNormalized(ctx context.Context, entityType, normalized string) (MemoryEntityEntity, error) {
	var row MemoryEntityEntity
	err := d.db.WithContext(ctx).Where("entity_type = ? AND normalized_name = ? AND status = 'active'", entityType, normalized).First(&row).Error
	return row, err
}

func (d *MemoryDao) ListEntitiesByType(ctx context.Context, entityType string, limit int) ([]MemoryEntityEntity, error) {
	var rows []MemoryEntityEntity
	q := d.db.WithContext(ctx).Where("entity_type = ? AND status = 'active'", entityType)
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Find(&rows).Error
}

func (d *MemoryDao) UpsertFact(ctx context.Context, e MemoryFactEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope_id"}, {Name: "normalized_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"summary", "confidence_score", "freshness_score", "source_count",
			"last_seen_at", "updated_by_task_id", "updated_at", "verification_status", "status",
		}),
	}).Create(&e).Error
}

func (d *MemoryDao) GetFact(ctx context.Context, id string) (MemoryFactEntity, error) {
	var row MemoryFactEntity
	return row, d.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
}

func (d *MemoryDao) GetFactByHash(ctx context.Context, scopeID, hash string) (MemoryFactEntity, error) {
	var row MemoryFactEntity
	return row, d.db.WithContext(ctx).Where("scope_id = ? AND normalized_hash = ?", scopeID, hash).First(&row).Error
}

func (d *MemoryDao) ListFactsByTask(ctx context.Context, taskID string) ([]MemoryFactEntity, error) {
	var rows []MemoryFactEntity
	return rows, d.db.WithContext(ctx).
		Where("created_by_task_id = ? AND status != 'invalid'", taskID).
		Find(&rows).Error
}

func (d *MemoryDao) GetSource(ctx context.Context, id string) (MemorySourceEntity, error) {
	var row MemorySourceEntity
	return row, d.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
}

func (d *MemoryDao) CountEvidenceLinks(ctx context.Context, factID string) (int64, error) {
	var n int64
	return n, d.db.WithContext(ctx).Model(&MemoryEvidenceLinkEntity{}).Where("fact_id = ?", factID).Count(&n).Error
}

func (d *MemoryDao) UpdateClaimVerification(ctx context.Context, id, status string, qaResult *string, factID *string) error {
	updates := map[string]any{"verification_status": status, "updated_at": time.Now()}
	if qaResult != nil {
		updates["qa_result"] = *qaResult
	}
	if factID != nil {
		updates["fact_id"] = *factID
	}
	return d.db.WithContext(ctx).Model(&MemoryClaimEntity{}).Where("id = ?", id).Updates(updates).Error
}

func (d *MemoryDao) ListFacts(ctx context.Context, scopeID string, limit int) ([]MemoryFactEntity, error) {
	var rows []MemoryFactEntity
	q := d.db.WithContext(ctx).Where("scope_id = ? AND status = 'active'", scopeID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Order("importance_score DESC, confidence_score DESC").Find(&rows).Error
}

func (d *MemoryDao) SearchFactsBySubject(ctx context.Context, scopeIDs []string, subjects []string, limit int) ([]MemoryFactEntity, error) {
	if len(scopeIDs) == 0 {
		return nil, nil
	}
	var rows []MemoryFactEntity
	q := d.db.WithContext(ctx).Where("scope_id IN ? AND status = 'active' AND verification_status IN ('verified','pending')", scopeIDs)
	if len(subjects) > 0 {
		var conds []string
		var args []any
		for _, s := range subjects {
			conds = append(conds, "LOWER(subject) LIKE ?")
			args = append(args, "%"+strings.ToLower(s)+"%")
		}
		q = q.Where(strings.Join(conds, " OR "), args...)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Order("verification_status = 'verified' DESC, importance_score DESC").Find(&rows).Error
}

func (d *MemoryDao) UpdateFactStatus(ctx context.Context, id, status, verification string) error {
	updates := map[string]any{"status": status, "updated_at": time.Now()}
	if verification != "" {
		updates["verification_status"] = verification
	}
	if status == "invalid" {
		now := time.Now()
		updates["invalidated_at"] = &now
	}
	return d.db.WithContext(ctx).Model(&MemoryFactEntity{}).Where("id = ?", id).Updates(updates).Error
}

func (d *MemoryDao) UpsertSource(ctx context.Context, e MemorySourceEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "content_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "excerpt", "collected_at"}),
	}).Create(&e).Error
}

func (d *MemoryDao) GetSourceByHash(ctx context.Context, hash string) (MemorySourceEntity, error) {
	var row MemorySourceEntity
	return row, d.db.WithContext(ctx).Where("content_hash = ?", hash).First(&row).Error
}

func (d *MemoryDao) CreateEvidenceLink(ctx context.Context, e MemoryEvidenceLinkEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&e).Error
}

func (d *MemoryDao) UpsertClaim(ctx context.Context, e MemoryClaimEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "claim_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{"verification_status", "qa_result", "evidence_summary", "updated_at"}),
	}).Create(&e).Error
}

func (d *MemoryDao) ListClaimsByTask(ctx context.Context, taskID string) ([]MemoryClaimEntity, error) {
	var rows []MemoryClaimEntity
	return rows, d.db.WithContext(ctx).Where("task_id = ?", taskID).Find(&rows).Error
}

func (d *MemoryDao) UpsertEpisode(ctx context.Context, e MemoryEpisodeEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"summary", "outcome", "qa_score", "lessons_json", "artifact_refs_json"}),
	}).Create(&e).Error
}

func (d *MemoryDao) ListEpisodes(ctx context.Context, scopeID string, limit int) ([]MemoryEpisodeEntity, error) {
	var rows []MemoryEpisodeEntity
	q := d.db.WithContext(ctx).Where("scope_id = ?", scopeID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Order("created_at DESC").Find(&rows).Error
}

func (d *MemoryDao) UpsertPreference(ctx context.Context, e MemoryPreferenceEntity) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope_id"}, {Name: "pref_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"pref_value_json", "priority", "source_type", "status", "updated_at"}),
	}).Create(&e).Error
}

func (d *MemoryDao) ListPreferences(ctx context.Context, scopeIDs []string) ([]MemoryPreferenceEntity, error) {
	if len(scopeIDs) == 0 {
		return nil, nil
	}
	var rows []MemoryPreferenceEntity
	return rows, d.db.WithContext(ctx).
		Where("scope_id IN ? AND status = 'active'", scopeIDs).
		Order("priority ASC").Find(&rows).Error
}

func (d *MemoryDao) CreateEvent(ctx context.Context, e MemoryEventEntity) error {
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *MemoryDao) CreateEmbedding(ctx context.Context, e MemoryEmbeddingEntity) error {
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *MemoryDao) CreateChunk(ctx context.Context, e MemoryChunkEntity) error {
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *MemoryDao) ListChunksByTask(ctx context.Context, taskID string, limit int) ([]MemoryChunkEntity, error) {
	var rows []MemoryChunkEntity
	q := d.db.WithContext(ctx).Where("task_id = ?", taskID)
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Order("chunk_index ASC").Find(&rows).Error
}

func (d *MemoryDao) ListRecentChunks(ctx context.Context, limit int) ([]MemoryChunkEntity, error) {
	var rows []MemoryChunkEntity
	q := d.db.WithContext(ctx).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Find(&rows).Error
}

func (d *MemoryDao) ListScopesByKeys(ctx context.Context, scopeType string, keys []string) ([]MemoryScopeEntity, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	var rows []MemoryScopeEntity
	return rows, d.db.WithContext(ctx).Where("scope_type = ? AND scope_key IN ?", scopeType, keys).Find(&rows).Error
}

func (d *MemoryDao) ListAllEmbeddings(ctx context.Context, objectType string, limit int) ([]MemoryEmbeddingEntity, error) {
	var rows []MemoryEmbeddingEntity
	q := d.db.WithContext(ctx).Where("object_type = ?", objectType)
	if limit > 0 {
		q = q.Limit(limit)
	}
	return rows, q.Find(&rows).Error
}

// GetEmbeddingsByObjectIDs 按 objectType + objectID IN 批量查询 embedding 向量。
// 返回 map[objectID][]float32，供 MySQLRetriever 替换嵌套全表扫描。
func (d *MemoryDao) GetEmbeddingsByObjectIDs(ctx context.Context, objectType string, ids []string) (map[string][]float32, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []MemoryEmbeddingEntity
	err := d.db.WithContext(ctx).
		Where("object_type = ? AND object_id IN ?", objectType, ids).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string][]float32, len(rows))
	for _, r := range rows {
		var vec []float32
		if jsonErr := jsonUnmarshal([]byte(r.EmbeddingJSON), &vec); jsonErr == nil && len(vec) > 0 {
			result[r.ObjectID] = vec
		}
	}
	return result, nil
}

// GetEmbeddingByObjectID 查询单个 objectID 对应的所有 embedding 行。
func (d *MemoryDao) GetEmbeddingByObjectID(ctx context.Context, objectID string) ([]MemoryEmbeddingEntity, error) {
	var rows []MemoryEmbeddingEntity
	return rows, d.db.WithContext(ctx).
		Where("object_id = ?", objectID).
		Find(&rows).Error
}

// GetFactsByIDs 按 ID IN 批量查询 fact（HybridRetriever 合并结果时使用）。
func (d *MemoryDao) GetFactsByIDs(ctx context.Context, ids []string) ([]MemoryFactEntity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []MemoryFactEntity
	return rows, d.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&rows).Error
}

// UpsertEmbedding 插入或更新 embedding 记录（按 object_id + object_type 唯一）。
func (d *MemoryDao) UpsertEmbedding(ctx context.Context, e MemoryEmbeddingEntity) error {
	return d.db.WithContext(ctx).Save(&e).Error
}

// DeleteEmbeddingByObjectID 删除指定 objectType + objectID 的 embedding 记录。
func (d *MemoryDao) DeleteEmbeddingByObjectID(ctx context.Context, objectType, objectID string) error {
	return d.db.WithContext(ctx).
		Where("object_type = ? AND object_id = ?", objectType, objectID).
		Delete(&MemoryEmbeddingEntity{}).Error
}

// ListEmbeddingsPaged 分页查询全量 embedding（RebuildMilvus 使用）。
func (d *MemoryDao) ListEmbeddingsPaged(ctx context.Context, offset, limit int) ([]MemoryEmbeddingEntity, error) {
	var rows []MemoryEmbeddingEntity
	return rows, d.db.WithContext(ctx).
		Offset(offset).Limit(limit).
		Order("created_at ASC").
		Find(&rows).Error
}
