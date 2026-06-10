package dao

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type EventLogDao struct{ db *gorm.DB }

func NewEventLogDao(db *gorm.DB) *EventLogDao { return &EventLogDao{db: db} }

func (d *EventLogDao) Append(ctx context.Context, e EventLogEntity) (EventLogEntity, error) {
	var maxSeq int64
	_ = d.db.WithContext(ctx).Model(&EventLogEntity{}).Where("task_id = ?", e.TaskID).
		Select("COALESCE(MAX(sequence_no),0)").Scan(&maxSeq).Error
	e.SequenceNo = maxSeq + 1
	e.CreatedAt = time.Now()
	return e, d.db.WithContext(ctx).Create(&e).Error
}

func (d *EventLogDao) ListSince(ctx context.Context, taskID string, afterSeq int64) ([]EventLogEntity, error) {
	var rows []EventLogEntity
	err := d.db.WithContext(ctx).Where("task_id = ? AND sequence_no > ?", taskID, afterSeq).
		Order("sequence_no asc").Find(&rows).Error
	return rows, err
}

type MessageLogDao struct{ db *gorm.DB }

func NewMessageLogDao(db *gorm.DB) *MessageLogDao { return &MessageLogDao{db: db} }

func (d *MessageLogDao) Create(ctx context.Context, e MessageLogEntity) error {
	e.CreatedAt = time.Now()
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *MessageLogDao) MarkAcked(ctx context.Context, messageID string) error {
	now := time.Now()
	return d.db.WithContext(ctx).Model(&MessageLogEntity{}).Where("message_id = ?", messageID).
		Updates(map[string]any{"status": "acked", "acked_at": now}).Error
}

func (d *MessageLogDao) MarkFailed(ctx context.Context, messageID string, reason string) error {
	now := time.Now()
	return d.db.WithContext(ctx).Model(&MessageLogEntity{}).Where("message_id = ?", messageID).
		Updates(map[string]any{"status": "failed", "failed_at": now, "metadata_json": reason}).Error
}

type CheckpointDao struct{ db *gorm.DB }

func NewCheckpointDao(db *gorm.DB) *CheckpointDao { return &CheckpointDao{db: db} }

func (d *CheckpointDao) Upsert(ctx context.Context, e TaskCheckpointEntity) error {
	e.UpdatedAt = time.Now()
	return d.db.WithContext(ctx).Save(&e).Error
}

func (d *CheckpointDao) Get(ctx context.Context, taskID string) (TaskCheckpointEntity, error) {
	var e TaskCheckpointEntity
	return e, d.db.WithContext(ctx).First(&e, "task_id = ?", taskID).Error
}

func (d *CheckpointDao) ListStale(ctx context.Context, before time.Time) ([]TaskCheckpointEntity, error) {
	var rows []TaskCheckpointEntity
	err := d.db.WithContext(ctx).Where("updated_at < ?", before).Find(&rows).Error
	return rows, err
}

type OutboxDao struct{ db *gorm.DB }

func NewOutboxDao(db *gorm.DB) *OutboxDao { return &OutboxDao{db: db} }

func (d *OutboxDao) Enqueue(ctx context.Context, e OutboxMessageEntity) error {
	e.Status = "pending"
	e.CreatedAt = time.Now()
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *OutboxDao) ListPending(ctx context.Context, limit int) ([]OutboxMessageEntity, error) {
	var rows []OutboxMessageEntity
	err := d.db.WithContext(ctx).Where("status = ?", "pending").Order("id asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func (d *OutboxDao) MarkSent(ctx context.Context, id uint) error {
	now := time.Now()
	return d.db.WithContext(ctx).Model(&OutboxMessageEntity{}).Where("id = ?", id).
		Updates(map[string]any{"status": "sent", "published_at": now}).Error
}

type DeadLetterDao struct{ db *gorm.DB }

func NewDeadLetterDao(db *gorm.DB) *DeadLetterDao { return &DeadLetterDao{db: db} }

func (d *DeadLetterDao) Create(ctx context.Context, e DeadLetterEntity) error {
	e.CreatedAt = time.Now()
	return d.db.WithContext(ctx).Create(&e).Error
}

func (d *DeadLetterDao) List(ctx context.Context, taskID string, limit int) ([]DeadLetterEntity, error) {
	var rows []DeadLetterEntity
	q := d.db.WithContext(ctx).Order("id desc").Limit(limit)
	if taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}
	return rows, q.Find(&rows).Error
}

func (d *DeadLetterDao) Get(ctx context.Context, id uint) (DeadLetterEntity, error) {
	var e DeadLetterEntity
	return e, d.db.WithContext(ctx).First(&e, id).Error
}

func MarshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
