package llm

// Message is the HTTP API DTO for chat roles and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
