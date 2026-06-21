package ioc

import (
	"context"
	"os"

	llmsvc "CompeteAI/internal/llm"
	"CompeteAI/internal/pkg/logger"
	"CompeteAI/settings"
)

func InitFirecrawlTools() *llmsvc.ToolRegistry {
	if os.Getenv("SKIP_FIRECRAWL") == "1" {
		logger.L().Info("firecrawl mcp skipped (SKIP_FIRECRAWL=1)")
		return llmsvc.NewToolRegistry(nil)
	}
	cfg := settings.Conf.FirecrawlMCPConfig
	if cfg == nil || !cfg.Enabled {
		logger.L().Info("firecrawl mcp disabled")
		return llmsvc.NewToolRegistry(nil)
	}

	registry, err := llmsvc.ConnectFirecrawlMCP(context.Background(), cfg.Command, cfg.Args)
	if err != nil {
		logger.L().Warn("firecrawl mcp init failed", logger.String("error", err.Error()))
		return llmsvc.NewToolRegistry(nil)
	}

	names := make([]string, 0, len(registry.Tools()))
	for _, t := range registry.Tools() {
		if info, infoErr := t.Info(context.Background()); infoErr == nil {
			names = append(names, info.Name)
		}
	}
	logger.L().Info("firecrawl mcp tools ready", logger.Any("tools", names))
	return registry
}
