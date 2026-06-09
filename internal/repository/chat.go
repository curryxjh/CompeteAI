package repository

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrConversationNotFound = errors.New("conversation not found")

type ChatRepository interface {
	ListConversations(ctx context.Context, userID int64) ([]domain.ChatConversation, error)
	CreateConversation(ctx context.Context, userID int64, title string) (domain.ChatConversation, error)
	GetConversation(ctx context.Context, userID int64, id string) (domain.ChatConversationDetail, error)
	UpdateTitle(ctx context.Context, userID int64, id, title string) error
	DeleteConversation(ctx context.Context, userID int64, id string) error
	AppendMessages(ctx context.Context, userID int64, conversationID string, msgs []domain.ChatMessageRecord) error
}

type chatRepository struct {
	dao dao.ChatConversationDao
}

func NewChatRepository(d dao.ChatConversationDao) ChatRepository {
	return &chatRepository{dao: d}
}

func (r *chatRepository) ListConversations(ctx context.Context, userID int64) ([]domain.ChatConversation, error) {
	rows, err := r.dao.ListConversations(ctx, userID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChatConversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toConversationDTO(row))
	}
	return out, nil
}

func (r *chatRepository) CreateConversation(ctx context.Context, userID int64, title string) (domain.ChatConversation, error) {
	if title == "" {
		title = "新对话"
	}
	now := time.Now()
	entity := dao.ChatConversationEntity{
		ID: uuid.NewString(), UserID: userID, Title: title,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := r.dao.InsertConversation(ctx, entity); err != nil {
		return domain.ChatConversation{}, err
	}
	return toConversationDTO(entity), nil
}

func (r *chatRepository) GetConversation(ctx context.Context, userID int64, id string) (domain.ChatConversationDetail, error) {
	conv, err := r.dao.FindConversation(ctx, id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ChatConversationDetail{}, ErrConversationNotFound
		}
		return domain.ChatConversationDetail{}, err
	}
	msgRows, err := r.dao.ListMessages(ctx, id)
	if err != nil {
		return domain.ChatConversationDetail{}, err
	}
	msgs := make([]domain.ChatMessageRecord, 0, len(msgRows))
	for _, m := range msgRows {
		msgs = append(msgs, toMessageDTO(m))
	}
	return domain.ChatConversationDetail{
		ChatConversation: toConversationDTO(conv),
		Messages:         msgs,
	}, nil
}

func (r *chatRepository) UpdateTitle(ctx context.Context, userID int64, id, title string) error {
	conv, err := r.dao.FindConversation(ctx, id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	conv.Title = title
	return r.dao.UpdateConversation(ctx, conv)
}

func (r *chatRepository) DeleteConversation(ctx context.Context, userID int64, id string) error {
	err := r.dao.DeleteConversation(ctx, id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrConversationNotFound
	}
	return err
}

func (r *chatRepository) AppendMessages(ctx context.Context, userID int64, conversationID string, msgs []domain.ChatMessageRecord) error {
	conv, err := r.dao.FindConversation(ctx, conversationID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	existing, err := r.dao.ListMessages(ctx, conversationID)
	if err != nil {
		return err
	}
	sortBase := len(existing)
	entities := make([]dao.ChatMessageEntity, 0, len(msgs))
	for i, m := range msgs {
		id := m.ID
		if id == "" {
			id = uuid.NewString()
		}
		payload, _ := json.Marshal(m.Payload)
		if string(payload) == "null" {
			payload = []byte("{}")
		}
		entities = append(entities, dao.ChatMessageEntity{
			ID: id, ConversationID: conversationID,
			Role: m.Role, Content: m.Content,
			PayloadJSON: string(payload), SortOrder: sortBase + i,
		})
	}
	if err := r.dao.InsertMessages(ctx, entities); err != nil {
		return err
	}
	if conv.Title == "新对话" || conv.Title == "" {
		for _, m := range msgs {
			if m.Role == "user" && m.Content != "" {
				conv.Title = truncateRunes(m.Content, 30)
				break
			}
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Content != "" {
			conv.Preview = truncateRunes(msgs[i].Content, 80)
			break
		}
	}
	return r.dao.UpdateConversation(ctx, conv)
}

func toConversationDTO(e dao.ChatConversationEntity) domain.ChatConversation {
	return domain.ChatConversation{
		ID: e.ID, Title: e.Title, Preview: e.Preview,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
}

func toMessageDTO(e dao.ChatMessageEntity) domain.ChatMessageRecord {
	payload := map[string]any{}
	if e.PayloadJSON != "" && e.PayloadJSON != "{}" {
		_ = json.Unmarshal([]byte(e.PayloadJSON), &payload)
	}
	return domain.ChatMessageRecord{
		ID: e.ID, Role: e.Role, Content: e.Content, Payload: payload,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
	}
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
