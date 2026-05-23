package server

import "CompeteAI/internal/mcp/protocol"

// RegisterAllTools 注册所有 CompeteAI 对外暴露的 MCP 工具
func (s *Server) RegisterAllTools() {
	registerReportTool(s)
	registerSWOTTool(s)
	registerFeatureCompareTool(s)
}

func registerReportTool(s *Server) {
	// TODO: 实现报告生成工具
	s.RegisterTool(
		protocol.Tool{
			Name:        "generate_competitive_report",
			Description: "对指定竞品列表生成完整的竞品分析报告（含 SWOT、功能矩阵、用户画像、定价对比）",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"competitors": {
						Type: "array",
						Items: &protocol.JSONSchema{Type: "string"},
					},
					"scope": {
						Type: "string",
					},
				},
				Required: []string{"competitors"},
			},
		},
		func(args map[string]interface{}) (*protocol.CallToolResult, error) {
			return &protocol.CallToolResult{
				Content: []protocol.ContentBlock{
					{Type: "text", Text: "generate_competitive_report stub"},
				},
			}, nil
		},
	)
}

func registerSWOTTool(s *Server) {
	// TODO: 实现 SWOT 分析工具
	s.RegisterTool(
		protocol.Tool{
			Name:        "swot_analysis",
			Description: "对指定产品做 SWOT 分析（优势、劣势、机会、威胁）",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"product": {Type: "string"},
				},
				Required: []string{"product"},
			},
		},
		func(args map[string]interface{}) (*protocol.CallToolResult, error) {
			return &protocol.CallToolResult{
				Content: []protocol.ContentBlock{
					{Type: "text", Text: "swot_analysis stub"},
				},
			}, nil
		},
	)
}

func registerFeatureCompareTool(s *Server) {
	// TODO: 实现功能对比工具
	s.RegisterTool(
		protocol.Tool{
			Name:        "feature_compare",
			Description: "对多个竞品按功能维度做对比矩阵分析",
			InputSchema: protocol.JSONSchema{
				Type: "object",
				Properties: map[string]protocol.JSONSchema{
					"products": {
						Type:  "array",
						Items: &protocol.JSONSchema{Type: "string"},
					},
					"dimensions": {
						Type:  "array",
						Items: &protocol.JSONSchema{Type: "string"},
					},
				},
				Required: []string{"products"},
			},
		},
		func(args map[string]interface{}) (*protocol.CallToolResult, error) {
			return &protocol.CallToolResult{
				Content: []protocol.ContentBlock{
					{Type: "text", Text: "feature_compare stub"},
				},
			}, nil
		},
	)
}
