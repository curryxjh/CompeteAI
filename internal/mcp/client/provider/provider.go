package provider

import "CompeteAI/internal/mcp/protocol"

// Provider 工具提供者接口 — 每个外部服务实现此接口
type Provider interface {
	// Name 服务名称
	Name() string
	// Tools 该服务提供的工具列表
	Tools() []protocol.Tool
	// CallTool 调用工具
	CallTool(name string, args map[string]interface{}) (*protocol.CallToolResult, error)
	// Close 关闭连接
	Close() error
}
