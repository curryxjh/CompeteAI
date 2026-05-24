package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"CompeteAI/internal/mcp/client/transport"
	"CompeteAI/internal/mcp/protocol"
)

// Manager MCP 客户端管理器 — 管理多个 MCP Server 连接
type Manager struct {
	mu              sync.RWMutex
	servers         map[string]transport.Transport
	tools           map[string]*protocol.Tool
	toolSrc         map[string]string
	requestTimeout  time.Duration
	nextRequestID   atomic.Int64
}

func NewManager(requestTimeout time.Duration) *Manager {
	if requestTimeout <= 0 {
		requestTimeout = 60 * time.Second
	}
	return &Manager{
		servers:        make(map[string]transport.Transport),
		tools:          make(map[string]*protocol.Tool),
		toolSrc:        make(map[string]string),
		requestTimeout: requestTimeout,
	}
}

func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers) > 0
}

// Connect 连接一个 MCP Server 并完成初始化握手
func (m *Manager) Connect(cfg protocol.ServerConfig) error {
	if cfg.URL == "" && cfg.Transport != "stdio" {
		return fmt.Errorf("server %s: url is required", cfg.Name)
	}

	var t transport.Transport
	switch cfg.Transport {
	case "http":
		t = transport.NewHTTPTransport(cfg.URL, m.requestTimeout)
	case "sse":
		t = transport.NewSSETransport(cfg.URL, m.requestTimeout)
	case "stdio":
		t = transport.NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	default:
		return fmt.Errorf("unsupported transport: %s", cfg.Transport)
	}

	if err := t.Connect(); err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.Name, err)
	}

	if err := m.initialize(t); err != nil {
		t.Close()
		return fmt.Errorf("initialize %s: %w", cfg.Name, err)
	}

	if err := m.fetchTools(t, cfg.Name); err != nil {
		t.Close()
		return fmt.Errorf("fetch tools from %s: %w", cfg.Name, err)
	}

	m.mu.Lock()
	m.servers[cfg.Name] = t
	m.mu.Unlock()
	return nil
}

func (m *Manager) initialize(t transport.Transport) error {
	params := protocol.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: protocol.ClientInfo{
			Name:    "CompeteAI",
			Version: "1.0.0",
		},
		Capabilities: protocol.Capabilities{
			Tools: &protocol.ToolsCapability{},
		},
	}

	req, err := protocol.NewRequest(m.nextID(), "initialize", params)
	if err != nil {
		return err
	}

	resp, err := t.Send(req)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	notif := &protocol.Notification{
		JSONRPC: protocol.Version,
		Method:  "notifications/initialized",
	}
	return t.SendNotification(notif)
}

func (m *Manager) fetchTools(t transport.Transport, name string) error {
	req, err := protocol.NewRequest(m.nextID(), "tools/list", nil)
	if err != nil {
		return err
	}

	resp, err := t.Send(req)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	var result protocol.ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("unmarshal tools: %w", err)
	}

	m.mu.Lock()
	for i := range result.Tools {
		tool := result.Tools[i]
		m.tools[tool.Name] = &tool
		m.toolSrc[tool.Name] = name
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) nextID() int64 {
	return m.nextRequestID.Add(1)
}

// ListTools 返回所有已注册的工具
func (m *Manager) ListTools() []protocol.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]protocol.Tool, 0, len(m.tools))
	for _, t := range m.tools {
		tools = append(tools, *t)
	}
	return tools
}

// CallTool 调用指定工具
func (m *Manager) CallTool(name string, args map[string]interface{}) (*protocol.CallToolResult, error) {
	m.mu.RLock()
	src, ok := m.toolSrc[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	m.mu.RLock()
	t, ok := m.servers[src]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found for tool %s", name)
	}

	params := protocol.CallToolParams{
		Name:      name,
		Arguments: args,
	}

	req, err := protocol.NewRequest(m.nextID(), "tools/call", params)
	if err != nil {
		return nil, err
	}

	resp, err := t.Send(req)
	if err != nil {
		return nil, fmt.Errorf("call tool %s: %w", name, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tool %s error: %s", name, resp.Error.Message)
	}

	var result protocol.CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w", err)
	}
	if result.IsError {
		return &result, fmt.Errorf("tool %s returned error: %s", name, FormatToolResult(&result))
	}
	return &result, nil
}

// FormatToolResult 将 MCP 工具结果格式化为文本
func FormatToolResult(result *protocol.CallToolResult) string {
	if result == nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// Close 关闭所有连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, t := range m.servers {
		if err := t.Close(); err != nil {
			return fmt.Errorf("close %s: %w", name, err)
		}
	}
	return nil
}
