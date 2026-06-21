package ioc

import (
	"context"

	llmsvc "CompeteAI/internal/llm"
	"CompeteAI/settings"
)

func InitChatService(tools *llmsvc.ToolRegistry) *llmsvc.ChatService {
	ctx := context.Background()
	chatModel, err := llmsvc.NewChatModel(ctx)
	if err != nil {
		panic("init eino chat model: " + err.Error())
	}

	modelName := ""
	if cfg := settings.Conf.LLMConfig; cfg != nil {
		modelName = cfg.Model
	}
	return llmsvc.NewChatService(chatModel, modelName, tools)
}
