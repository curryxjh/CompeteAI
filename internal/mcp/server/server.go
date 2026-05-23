package server

import (
	"encoding/json"
	"fmt"
	"sync"

	"CompeteAI/internal/mcp/protocol"
)

// ToolHandler 工具处理函数
type ToolHandler func(args map[string]interface{}) (*protocol.CallToolResult, error)

// Server MCP Server — 将 CompeteAI 的能力暴露为 MCP 工具
type Server struct {
	mu       sync.RWMutex
	name     string
	version  string
	tools    map[string]*protocol.Tool
	handlers map[string]ToolHandler
}

func New(name, version string) *Server {
	return &Server{
		name:     name,
		version:  version,
		tools:    make(map[string]*protocol.Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// RegisterTool 注册一个工具及其处理函数
func (s *Server) RegisterTool(tool protocol.Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = &tool
	s.handlers[tool.Name] = handler
}

// HandleRequest 处理 JSON-RPC 请求
func (s *Server) HandleRequest(data []byte) ([]byte, error) {
	var req protocol.Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(&req)
	case "tools/list":
		return s.handleListTools(&req)
	case "tools/call":
		return s.handleCallTool(&req)
	default:
		return json.Marshal(protocol.NewErrorResponse(req.ID, -32601, "method not found"))
	}
}

func (s *Server) handleInitialize(req *protocol.Request) ([]byte, error) {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return json.Marshal(protocol.NewErrorResponse(req.ID, -32602, "invalid params"))
	}

	result := protocol.InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: protocol.Capabilities{
			Tools: &protocol.ToolsCapability{},
		},
		ServerInfo: protocol.ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}

	resp, err := protocol.NewResponse(req.ID, result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

func (s *Server) handleListTools(req *protocol.Request) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]protocol.Tool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, *t)
	}

	result := protocol.ListToolsResult{Tools: tools}
	resp, err := protocol.NewResponse(req.ID, result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

func (s *Server) handleCallTool(req *protocol.Request) ([]byte, error) {
	var params protocol.CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return json.Marshal(protocol.NewErrorResponse(req.ID, -32602, "invalid params"))
	}

	s.mu.RLock()
	handler, ok := s.handlers[params.Name]
	s.mu.RUnlock()

	if !ok {
		return json.Marshal(protocol.NewErrorResponse(req.ID, -32602, "tool not found: "+params.Name))
	}

	result, err := handler(params.Arguments)
	if err != nil {
		return json.Marshal(protocol.NewErrorResponse(req.ID, -32000, err.Error()))
	}

	resp, err := protocol.NewResponse(req.ID, result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}
