package ioc

import (
	"context"

	einosvc "CompeteAI/internal/eino"
	"CompeteAI/settings"
)

func InitChatService() *einosvc.ChatService {
	ctx := context.Background()
	chatModel, err := einosvc.NewChatModel(ctx)
	if err != nil {
		panic("init eino chat model: " + err.Error())
	}

	modelName := ""
	if cfg := settings.Conf.LLMConfig; cfg != nil {
		modelName = cfg.Model
	}
	return einosvc.NewChatService(chatModel, modelName)
}
