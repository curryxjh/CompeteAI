package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolRegistry 持有 Eino MCP 工具，供 Chat 与 HTTP API 调用。
type ToolRegistry struct {
	tools  []tool.BaseTool
	lookup map[string]tool.InvokableTool
}

func NewToolRegistry(tools []tool.BaseTool) *ToolRegistry {
	lookup := make(map[string]tool.InvokableTool, len(tools))
	for _, t := range tools {
		if inv, ok := t.(tool.InvokableTool); ok {
			info, err := t.Info(context.Background())
			if err != nil {
				continue
			}
			lookup[info.Name] = inv
		}
	}
	return &ToolRegistry{tools: tools, lookup: lookup}
}

func (r *ToolRegistry) Enabled() bool {
	return r != nil && len(r.lookup) > 0
}

func (r *ToolRegistry) Tools() []tool.BaseTool {
	if r == nil {
		return nil
	}
	return r.tools
}

func (r *ToolRegistry) ListToolInfos(ctx context.Context) ([]*schema.ToolInfo, error) {
	if !r.Enabled() {
		return nil, nil
	}
	infos := make([]*schema.ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (r *ToolRegistry) Invoke(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	if !r.Enabled() {
		return "", fmt.Errorf("tool registry is empty")
	}
	inv, ok := r.lookup[name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal arguments: %w", err)
	}
	return inv.InvokableRun(ctx, string(raw))
}

// ConnectFirecrawlMCP 通过 stdio 连接 Firecrawl MCP Server，并发现可用工具。
func ConnectFirecrawlMCP(ctx context.Context, command string, args []string) (*ToolRegistry, error) {
	apiKey := strings.Trim(os.Getenv("FIRECRAWL_API_KEY"), "\"'")
	if apiKey == "" {
		return nil, fmt.Errorf("FIRECRAWL_API_KEY is not set")
	}
	if command == "" {
		command = "npx"
	}
	if len(args) == 0 {
		args = []string{"-y", "firecrawl-mcp"}
	}

	cli, err := client.NewStdioMCPClient(command, append(os.Environ(), "FIRECRAWL_API_KEY="+apiKey), args...)
	if err != nil {
		return nil, fmt.Errorf("start firecrawl mcp: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "CompeteAI",
		Version: "1.0.0",
	}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("initialize firecrawl mcp: %w", err)
	}

	tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})
	if err != nil {
		return nil, fmt.Errorf("list firecrawl mcp tools: %w", err)
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("firecrawl mcp returned no tools")
	}
	return NewToolRegistry(tools), nil
}
