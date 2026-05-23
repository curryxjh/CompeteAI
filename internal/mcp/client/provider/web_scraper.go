package provider

import "CompeteAI/internal/mcp/protocol"

// WebScraperProvider 网页抓取工具提供者（占位，待对接具体抓取服务）
type WebScraperProvider struct{}

func NewWebScraperProvider() *WebScraperProvider {
	return &WebScraperProvider{}
}

func (p *WebScraperProvider) Name() string {
	return "web_scraper"
}

func (p *WebScraperProvider) Tools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "web_scrape",
			Description: "抓取指定 URL 的网页内容并提取文本",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"url": {
						Type: "string",
					},
				},
				Required: []string{"url"},
			},
		},
	}
}

func (p *WebScraperProvider) CallTool(name string, args map[string]interface{}) (*protocol.CallToolResult, error) {
	// TODO: 对接具体抓取服务 (Firecrawl / Jina / 自建)
	return &protocol.CallToolResult{
		Content: []protocol.ContentBlock{
			{Type: "text", Text: "web_scrape stub"},
		},
		IsError: false,
	}, nil
}

func (p *WebScraperProvider) Close() error {
	return nil
}
