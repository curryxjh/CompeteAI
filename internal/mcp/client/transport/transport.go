package transport

import "CompeteAI/internal/mcp/protocol"

// Transport MCP 传输层接口
type Transport interface {
	// Connect 建立连接
	Connect() error
	// Send 发送 JSON-RPC 请求并等待响应
	Send(req *protocol.Request) (*protocol.Response, error)
	// SendNotification 发送通知（无响应）
	SendNotification(notif *protocol.Notification) error
	// Close 关闭连接
	Close() error
	// IsConnected 检查连接状态
	IsConnected() bool
}
