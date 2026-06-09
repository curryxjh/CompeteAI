package ioc

import (
	"CompeteAI/internal/eventlog"
	"CompeteAI/internal/repository/dao"

	"gorm.io/gorm"
)

func InitEventHub(db *gorm.DB) *eventlog.HybridHub {
	return eventlog.NewHybridHub(dao.NewEventLogDao(db))
}
