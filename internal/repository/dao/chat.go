package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type ChatConversationDao interface {
	InsertConversation(ctx context.Context, c ChatConversationEntity) error
	UpdateConversation(ctx context.Context, c ChatConversationEntity) error
	FindConversation(ctx context.Context, id string, userID int64) (ChatConversationEntity, error)
	ListConversations(ctx context.Context, userID int64, limit int) ([]ChatConversationEntity, error)
	DeleteConversation(ctx context.Context, id string, userID int64) error
	InsertMessages(ctx context.Context, msgs []ChatMessageEntity) error
	ListMessages(ctx context.Context, conversationID string) ([]ChatMessageEntity, error)
	DeleteMessages(ctx context.Context, conversationID string) error
}

type ChatConversationEntity struct {
	ID        string    `gorm:"column:id;primaryKey;size:36"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_chat_conv_user"`
	Title     string    `gorm:"column:title;size:255;not null;default:''"`
	Preview   string    `gorm:"column:preview;size:512;not null;default:''"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ChatConversationEntity) TableName() string { return "chat_conversations" }

type ChatMessageEntity struct {
	ID             string    `gorm:"column:id;primaryKey;size:36"`
	ConversationID string    `gorm:"column:conversation_id;size:36;not null;index:idx_chat_msg_conv"`
	Role           string    `gorm:"column:role;size:16;not null"`
	Content        string    `gorm:"column:content;type:text"`
	PayloadJSON    string    `gorm:"column:payload_json;type:json"`
	SortOrder      int       `gorm:"column:sort_order;not null;default:0;index:idx_chat_msg_conv"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ChatMessageEntity) TableName() string { return "chat_messages" }

type GORMChatDao struct {
	db *gorm.DB
}

func NewChatDao(db *gorm.DB) ChatConversationDao {
	return &GORMChatDao{db: db}
}

func (d *GORMChatDao) InsertConversation(ctx context.Context, c ChatConversationEntity) error {
	return d.db.WithContext(ctx).Create(&c).Error
}

func (d *GORMChatDao) UpdateConversation(ctx context.Context, c ChatConversationEntity) error {
	return d.db.WithContext(ctx).Model(&ChatConversationEntity{}).
		Where("id = ? AND user_id = ?", c.ID, c.UserID).
		Updates(map[string]any{
			"title": c.Title, "preview": c.Preview, "updated_at": time.Now(),
		}).Error
}

func (d *GORMChatDao) FindConversation(ctx context.Context, id string, userID int64) (ChatConversationEntity, error) {
	var c ChatConversationEntity
	err := d.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&c).Error
	return c, err
}

func (d *GORMChatDao) ListConversations(ctx context.Context, userID int64, limit int) ([]ChatConversationEntity, error) {
	if limit <= 0 {
		limit = 50
	}
	var list []ChatConversationEntity
	err := d.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("updated_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

func (d *GORMChatDao) DeleteConversation(ctx context.Context, id string, userID int64) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", id).Delete(&ChatMessageEntity{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&ChatConversationEntity{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (d *GORMChatDao) InsertMessages(ctx context.Context, msgs []ChatMessageEntity) error {
	if len(msgs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).Create(&msgs).Error
}

func (d *GORMChatDao) ListMessages(ctx context.Context, conversationID string) ([]ChatMessageEntity, error) {
	var list []ChatMessageEntity
	err := d.db.WithContext(ctx).Where("conversation_id = ?", conversationID).
		Order("sort_order ASC, created_at ASC").Find(&list).Error
	return list, err
}

func (d *GORMChatDao) DeleteMessages(ctx context.Context, conversationID string) error {
	return d.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Delete(&ChatMessageEntity{}).Error
}
