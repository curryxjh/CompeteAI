package dao

import (
	"sync"

	"gorm.io/gorm"
)

var migrateOnce sync.Once
var migrateErr error

func InitTable(db *gorm.DB) error {
	migrateOnce.Do(func() {
		migrateErr = db.AutoMigrate(
			&User{}, &TaskEntity{}, &ReportEntity{}, &TraceEntity{},
			&MessageLogEntity{}, &EventLogEntity{}, &TaskCheckpointEntity{},
			&WorkerLeaseEntity{}, &DeadLetterEntity{}, &OutboxMessageEntity{},
			&MemoryScopeEntity{}, &MemoryEntityEntity{}, &MemoryFactEntity{},
			&MemorySourceEntity{}, &MemoryEvidenceLinkEntity{}, &MemoryClaimEntity{},
			&MemoryEpisodeEntity{}, &MemoryPreferenceEntity{}, &MemoryEventEntity{},
			&MemoryEmbeddingEntity{}, &MemoryChunkEntity{},
			&ChatConversationEntity{}, &ChatMessageEntity{},
		)
	})
	return migrateErr
}
