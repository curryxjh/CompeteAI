package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type TraceDao interface {
	Upsert(ctx context.Context, t TraceEntity) error
	FindByTaskID(ctx context.Context, taskID string) (TraceEntity, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

type TraceEntity struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID    string    `gorm:"uniqueIndex;size:36"`
	Payload   string    `gorm:"type:json"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type GORMTraceDao struct {
	db *gorm.DB
}

func NewTraceDao(db *gorm.DB) TraceDao {
	return &GORMTraceDao{db: db}
}

func (d *GORMTraceDao) Upsert(ctx context.Context, t TraceEntity) error {
	var existing TraceEntity
	err := d.db.WithContext(ctx).Where("task_id = ?", t.TaskID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return d.db.WithContext(ctx).Create(&t).Error
	}
	if err != nil {
		return err
	}
	t.ID = existing.ID
	t.CreatedAt = existing.CreatedAt
	return d.db.WithContext(ctx).Omit("CreatedAt").Save(&t).Error
}

func (d *GORMTraceDao) FindByTaskID(ctx context.Context, taskID string) (TraceEntity, error) {
	var t TraceEntity
	err := d.db.WithContext(ctx).Where("task_id = ?", taskID).First(&t).Error
	return t, err
}

func (d *GORMTraceDao) DeleteByTaskID(ctx context.Context, taskID string) error {
	return d.db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&TraceEntity{}).Error
}
