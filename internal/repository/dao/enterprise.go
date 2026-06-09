package dao

import (
	"time"

	"gorm.io/gorm"
)

type MessageLogEntity struct {
	ID           uint           `gorm:"primaryKey"`
	MessageID    string         `gorm:"size:64;uniqueIndex;not null"`
	TaskID       string         `gorm:"size:64;index;not null"`
	TraceID      string         `gorm:"size:64;index"`
	Topic        string         `gorm:"size:128;index"`
	Kind         string         `gorm:"size:32"`
	Name         string         `gorm:"size:64"`
	FromAgent    string         `gorm:"size:32"`
	ToAgent      string         `gorm:"size:32"`
	Attempt      int            `gorm:"default:0"`
	Status       string         `gorm:"size:32;index"`
	PayloadJSON  string         `gorm:"type:longtext"`
	MetadataJSON string         `gorm:"type:text"`
	CreatedAt    time.Time      `gorm:"index"`
	AckedAt      *time.Time
	FailedAt     *time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

type EventLogEntity struct {
	ID          uint      `gorm:"primaryKey"`
	TaskID      string    `gorm:"size:64;index;not null"`
	TraceID     string    `gorm:"size:64;index"`
	EventType   string    `gorm:"size:64;index;not null"`
	Agent       string    `gorm:"size:32"`
	PayloadJSON string    `gorm:"type:longtext"`
	SequenceNo  int64     `gorm:"index"`
	CreatedAt   time.Time `gorm:"index"`
}

type TaskCheckpointEntity struct {
	TaskID           string    `gorm:"primaryKey;size:64"`
	TraceID          string    `gorm:"size:64;index"`
	CurrentAgent     string    `gorm:"size:32"`
	CurrentMessageID string    `gorm:"size:64"`
	Round            int       `gorm:"default:1"`
	RetryCount       int       `gorm:"default:0"`
	CheckpointJSON   string    `gorm:"type:longtext"`
	UpdatedAt        time.Time `gorm:"index"`
}

type WorkerLeaseEntity struct {
	ID         uint      `gorm:"primaryKey"`
	TaskID     string    `gorm:"size:64;uniqueIndex:idx_task_agent;not null"`
	Agent      string    `gorm:"size:32;uniqueIndex:idx_task_agent;not null"`
	WorkerID   string    `gorm:"size:128"`
	LeaseToken string    `gorm:"size:64"`
	ExpiresAt  time.Time `gorm:"index"`
	UpdatedAt  time.Time
}

type DeadLetterEntity struct {
	ID          uint      `gorm:"primaryKey"`
	TaskID      string    `gorm:"size:64;index"`
	MessageID   string    `gorm:"size:64;index"`
	Topic       string    `gorm:"size:128"`
	Agent       string    `gorm:"size:32"`
	Reason      string    `gorm:"type:text"`
	PayloadJSON string    `gorm:"type:longtext"`
	Attempt     int
	CreatedAt   time.Time `gorm:"index"`
}

type OutboxMessageEntity struct {
	ID           uint       `gorm:"primaryKey"`
	TaskID       string     `gorm:"size:64;index;not null"`
	MessageID    string     `gorm:"size:64;uniqueIndex;not null"`
	Topic        string     `gorm:"size:128;not null"`
	PayloadJSON  string     `gorm:"type:longtext;not null"`
	Status       string     `gorm:"size:32;index;default:pending"`
	RetryCount   int        `gorm:"default:0"`
	PublishedAt  *time.Time
	CreatedAt    time.Time  `gorm:"index"`
}
