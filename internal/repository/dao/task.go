package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type TaskDao interface {
	Insert(ctx context.Context, t TaskEntity) error
	Update(ctx context.Context, t TaskEntity) error
	FindByID(ctx context.Context, id string) (TaskEntity, error)
	List(ctx context.Context) ([]TaskEntity, error)
	Delete(ctx context.Context, id string) error
}

type TaskEntity struct {
	ID           string    `gorm:"primaryKey;size:36"`
	Title        string    `gorm:"size:255"`
	Competitors  string    `gorm:"type:json"`
	Dimensions   string    `gorm:"type:json"`
	Status       string    `gorm:"size:20;index"`
	Progress     int
	AgentStates  string    `gorm:"type:json"`
	ErrorMessage string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

type GORMTaskDao struct {
	db *gorm.DB
}

func NewTaskDao(db *gorm.DB) TaskDao {
	return &GORMTaskDao{db: db}
}

func (d *GORMTaskDao) Insert(ctx context.Context, t TaskEntity) error {
	return d.db.WithContext(ctx).Create(&t).Error
}

func (d *GORMTaskDao) Update(ctx context.Context, t TaskEntity) error {
	return d.db.WithContext(ctx).Omit("CreatedAt").Save(&t).Error
}

func (d *GORMTaskDao) FindByID(ctx context.Context, id string) (TaskEntity, error) {
	var t TaskEntity
	err := d.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	return t, err
}

func (d *GORMTaskDao) List(ctx context.Context) ([]TaskEntity, error) {
	var list []TaskEntity
	err := d.db.WithContext(ctx).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (d *GORMTaskDao) Delete(ctx context.Context, id string) error {
	return d.db.WithContext(ctx).Where("id = ?", id).Delete(&TaskEntity{}).Error
}
