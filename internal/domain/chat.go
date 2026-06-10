package domain

// ChatConversation 对话会话摘要。
type ChatConversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ChatMessageRecord 持久化的单条对话消息。
type ChatMessageRecord struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt string         `json:"createdAt,omitempty"`
}

// ChatConversationDetail 对话详情（含消息列表）。
type ChatConversationDetail struct {
	ChatConversation
	Messages []ChatMessageRecord `json:"messages"`
}

type CreateConversationPayload struct {
	Title string `json:"title"`
}

type UpdateConversationPayload struct {
	Title string `json:"title" binding:"required"`
}

type AppendChatMessagesPayload struct {
	Messages []ChatMessageRecord `json:"messages" binding:"required,min=1"`
}
