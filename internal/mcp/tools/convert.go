package tools

import (
	"encoding/json"
	"fmt"

	"CompeteAI/internal/mcp/protocol"

	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

// ToSchemaTools 将 MCP 工具定义转换为 Eino ToolInfo
func ToSchemaTools(mcpTools []protocol.Tool) ([]*schema.ToolInfo, error) {
	out := make([]*schema.ToolInfo, 0, len(mcpTools))
	for _, tool := range mcpTools {
		info, err := ToSchemaTool(tool)
		if err != nil {
			return nil, fmt.Errorf("convert tool %s: %w", tool.Name, err)
		}
		out = append(out, info)
	}
	return out, nil
}

func ToSchemaTool(tool protocol.Tool) (*schema.ToolInfo, error) {
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, err
	}

	var js jsonschema.Schema
	if err := json.Unmarshal(raw, &js); err != nil {
		return nil, err
	}

	return &schema.ToolInfo{
		Name:        tool.Name,
		Desc:        tool.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&js),
	}, nil
}
