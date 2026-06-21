package llm

import (
	"context"

	"CompeteAI/settings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
)

func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	cfg := settings.Conf.LLMConfig
	if cfg == nil {
		cfg = &settings.LLMConfig{}
	}

	return ark.NewChatModel(ctx, &ark.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
}
