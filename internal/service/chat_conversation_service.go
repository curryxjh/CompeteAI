package service

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository"
	"context"
)

type ChatConversationService interface {
	List(ctx context.Context, userID int64) ([]domain.ChatConversation, error)
	Create(ctx context.Context, userID int64, title string) (domain.ChatConversation, error)
	Get(ctx context.Context, userID int64, id string) (domain.ChatConversationDetail, error)
	Rename(ctx context.Context, userID int64, id, title string) error
	Delete(ctx context.Context, userID int64, id string) error
	AppendMessages(ctx context.Context, userID int64, id string, msgs []domain.ChatMessageRecord) error
}

type chatConversationService struct {
	repo repository.ChatRepository
}

func NewChatConversationService(repo repository.ChatRepository) ChatConversationService {
	return &chatConversationService{repo: repo}
}

func (s *chatConversationService) List(ctx context.Context, userID int64) ([]domain.ChatConversation, error) {
	return s.repo.ListConversations(ctx, userID)
}

func (s *chatConversationService) Create(ctx context.Context, userID int64, title string) (domain.ChatConversation, error) {
	return s.repo.CreateConversation(ctx, userID, title)
}

func (s *chatConversationService) Get(ctx context.Context, userID int64, id string) (domain.ChatConversationDetail, error) {
	return s.repo.GetConversation(ctx, userID, id)
}

func (s *chatConversationService) Rename(ctx context.Context, userID int64, id, title string) error {
	return s.repo.UpdateTitle(ctx, userID, id, title)
}

func (s *chatConversationService) Delete(ctx context.Context, userID int64, id string) error {
	return s.repo.DeleteConversation(ctx, userID, id)
}

func (s *chatConversationService) AppendMessages(ctx context.Context, userID int64, id string, msgs []domain.ChatMessageRecord) error {
	return s.repo.AppendMessages(ctx, userID, id, msgs)
}
