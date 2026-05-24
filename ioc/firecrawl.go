package ioc

import (
	"context"

	einosvc "CompeteAI/internal/eino"
	"CompeteAI/internal/pkg/logger"
	"CompeteAI/settings"
)

func InitFirecrawlTools() *einosvc.ToolRegistry {
	cfg := settings.Conf.FirecrawlMCPConfig
	if cfg == nil || !cfg.Enabled {
		logger.L().Info("firecrawl mcp disabled")
		return einosvc.NewToolRegistry(nil)
	}

	registry, err := einosvc.ConnectFirecrawlMCP(context.Background(), cfg.Command, cfg.Args)
	if err != nil {
		logger.L().Warn("firecrawl mcp init failed", logger.String("error", err.Error()))
		return einosvc.NewToolRegistry(nil)
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
