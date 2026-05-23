package provider

import (
	"CompeteAI/internal/mcp/protocol"
)

// WebSearchProvider Web 搜索工具提供者（占位，待对接具体搜索服务）
type WebSearchProvider struct {
	apiKey  string
	baseURL string
}

func NewWebSearchProvider(apiKey, baseURL string) *WebSearchProvider {
	return &WebSearchProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *WebSearchProvider) Name() string {
	return "web_search"
}

func (p *WebSearchProvider) Tools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "web_search",
			Description: "搜索互联网获取竞品信息、用户评价、定价等数据",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"query": {
						Type: "string",
					},
					"max_results": {
						Type: "integer",
					},
				},
				Required: []string{"query"},
			},
		},
	}
}

func (p *WebSearchProvider) CallTool(name string, args map[string]interface{}) (*protocol.CallToolResult, error) {
	// TODO: 对接具体搜索 API (Tavily / Brave / 自建)
	return &protocol.CallToolResult{
		Content: []protocol.ContentBlock{
			{Type: "text", Text: "web_search stub"},
		},
		IsError: false,
	}, nil
}

func (p *WebSearchProvider) Close() error {
	return nil
}
