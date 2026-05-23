package eino

import (
	"context"
	"errors"
	"io"

	"CompeteAI/internal/llm"

	"github.com/cloudwego/eino/components/model"
)

type ChatService struct {
	chatModel model.ToolCallingChatModel
	modelName string
}

func NewChatService(chatModel model.ToolCallingChatModel, modelName string) *ChatService {
	return &ChatService{
		chatModel: chatModel,
		modelName: modelName,
	}
}

func (s *ChatService) Model() string {
	return s.modelName
}

func (s *ChatService) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	resp, err := s.chatModel.Generate(ctx, ToSchemaMessages(messages))
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Content == "" {
		return "", errors.New("empty response from model")
	}
	return resp.Content, nil
}

func (s *ChatService) StreamChat(ctx context.Context, messages []llm.Message, onDelta func(string) error) error {
	reader, err := s.chatModel.Stream(ctx, ToSchemaMessages(messages))
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if chunk == nil || chunk.Content == "" {
			continue
		}
		if err := onDelta(chunk.Content); err != nil {
			return err
		}
	}
}
