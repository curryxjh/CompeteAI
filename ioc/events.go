package ioc

import (
	"CompeteAI/internal/event"
	"CompeteAI/internal/repository/dao"

	"gorm.io/gorm"
)

func InitEventHub(db *gorm.DB) *event.HybridHub {
	return event.NewHybridHub(dao.NewEventLogDao(db))
}
