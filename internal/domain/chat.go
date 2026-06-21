// Package domain 定义所有核心领域模型。
// 本文件包含对话会话聚合根及其值对象，用于 ChatView 自由对话功能。
package domain

// ChatConversation 对话会话摘要，用于列表展示。
type ChatConversation struct {
	ID        string `json:"id"`                    // 会话唯一标识（UUID）
	Title     string `json:"title"`                  // 会话标题（自动生成或用户设置）
	Preview   string `json:"preview,omitempty"`       // 最新消息预览文本
	CreatedAt string `json:"createdAt"`              // 创建时间
	UpdatedAt string `json:"updatedAt"`              // 最后更新时间
}

// ChatMessageRecord 持久化的单条对话消息，存储用户和助手的对话内容。
type ChatMessageRecord struct {
	ID        string         `json:"id"`                    // 消息唯一标识（UUID）
	Role      string         `json:"role"`                  // 角色：user / assistant / system
	Content   string         `json:"content"`                // 消息文本内容
	Payload   map[string]any `json:"payload,omitempty"`      // 扩展载荷（如 tool_call 结果）
	CreatedAt string         `json:"createdAt,omitempty"`   // 创建时间
}

// ChatConversationDetail 对话详情，包含完整消息列表。
// 用于 GET /api/chat/conversations/:id 响应。
type ChatConversationDetail struct {
	ChatConversation                          // 嵌入会话摘要
	Messages        []ChatMessageRecord `json:"messages"` // 消息列表
}

// CreateConversationPayload 创建对话会话的请求载荷。
type CreateConversationPayload struct {
	Title string `json:"title"` // 会话标题（可选）
}

// UpdateConversationPayload 更新对话会话的请求载荷。
type UpdateConversationPayload struct {
	Title string `json:"title" binding:"required"` // 新标题（必填）
}

// AppendChatMessagesPayload 追加消息到对话会话的请求载荷。
type AppendChatMessagesPayload struct {
	Messages []ChatMessageRecord `json:"messages" binding:"required,min=1"` // 消息列表（至少 1 条）
}

// ChatMessage LLM 对话请求中的单条消息，用于 Chat API 和 Agent 调用。
type ChatMessage struct {
	Role    string `json:"role"`    // 角色：user / assistant / system
	Content string `json:"content"` // 消息内容
}