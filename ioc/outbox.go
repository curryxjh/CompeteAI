package ioc

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/outbox"
	"CompeteAI/internal/repository/dao"

	"gorm.io/gorm"
)

func InitOutboxPublisher(db *gorm.DB, b bus.Bus) *outbox.Publisher {
	return outbox.NewPublisher(dao.NewOutboxDao(db), b)
}
