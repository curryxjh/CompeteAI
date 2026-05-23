package provider

import "CompeteAI/internal/mcp/protocol"

// RAGProvider RAG 向量检索工具提供者（占位，待对接 Milvus 等向量数据库）
type RAGProvider struct{}

func NewRAGProvider() *RAGProvider {
	return &RAGProvider{}
}

func (p *RAGProvider) Name() string {
	return "rag"
}

func (p *RAGProvider) Tools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "rag_search",
			Description: "在已索引的竞品知识库中做语义检索，返回相关文档片段",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"query": {
						Type: "string",
					},
					"top_k": {
						Type: "integer",
					},
				},
				Required: []string{"query"},
			},
		},
	}
}

func (p *RAGProvider) CallTool(name string, args map[string]interface{}) (*protocol.CallToolResult, error) {
	// TODO: 对接 Milvus / 其他向量数据库
	return &protocol.CallToolResult{
		Content: []protocol.ContentBlock{
			{Type: "text", Text: "rag_search stub"},
		},
		IsError: false,
	}, nil
}

func (p *RAGProvider) Close() error {
	return nil
}
