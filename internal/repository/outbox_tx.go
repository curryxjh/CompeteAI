package repository

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateTaskWithOutbox 同一事务写入 tasks + outbox（蓝图 §7.3）。
func CreateTaskWithOutbox(ctx context.Context, db *gorm.DB, task domain.Task, traceID string) error {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	entity, err := taskToEntity(task)
	if err != nil {
		return err
	}
	msg := protocol.NewTaskCreatedMessage(task, traceID).WithStatus(protocol.MessageStatusPending)
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entity).Error; err != nil {
			return err
		}
		return tx.Create(&dao.OutboxMessageEntity{
			TaskID: task.ID, MessageID: msg.MessageID,
			Topic: bus.TopicTaskCreate, PayloadJSON: string(raw),
			Status: "pending",
		}).Error
	})
}
