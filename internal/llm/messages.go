package llm

import (
	"CompeteAI/internal/domain"

	"github.com/cloudwego/eino/schema"
)

func ToSchemaMessages(msgs []domain.ChatMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, &schema.Message{
			Role:    toRole(m.Role),
			Content: m.Content,
		})
	}
	return out
}

func toRole(role string) schema.RoleType {
	switch role {
	case "assistant":
		return schema.Assistant
	case "system":
		return schema.System
	default:
		return schema.User
	}
}
