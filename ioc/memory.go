package ioc

import (
	"CompeteAI/internal/memory"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/settings"

	"gorm.io/gorm"
)

func InitMemoryService(db *gorm.DB) memory.MemoryService {
	cfg := settings.Conf.MemoryConfig
	if cfg == nil || !cfg.Enabled {
		return memory.NoopService{}
	}
	return memory.NewService(dao.NewMemoryDao(db), cfg)
}
