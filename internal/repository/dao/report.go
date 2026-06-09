package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type ReportDao interface {
	Upsert(ctx context.Context, r ReportEntity) error
	FindByTaskID(ctx context.Context, taskID string) (ReportEntity, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

type ReportEntity struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID    string    `gorm:"uniqueIndex;size:36"`
	Payload   string    `gorm:"type:json"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type GORMReportDao struct {
	db *gorm.DB
}

func NewReportDao(db *gorm.DB) ReportDao {
	return &GORMReportDao{db: db}
}

func (d *GORMReportDao) Upsert(ctx context.Context, r ReportEntity) error {
	var existing ReportEntity
	err := d.db.WithContext(ctx).Where("task_id = ?", r.TaskID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return d.db.WithContext(ctx).Create(&r).Error
	}
	if err != nil {
		return err
	}
	r.ID = existing.ID
	return d.db.WithContext(ctx).Save(&r).Error
}

func (d *GORMReportDao) FindByTaskID(ctx context.Context, taskID string) (ReportEntity, error) {
	var r ReportEntity
	err := d.db.WithContext(ctx).Where("task_id = ?", taskID).First(&r).Error
	return r, err
}

func (d *GORMReportDao) DeleteByTaskID(ctx context.Context, taskID string) error {
	return d.db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&ReportEntity{}).Error
}
