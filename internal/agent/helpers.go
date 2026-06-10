package agent

import (
	"CompeteAI/internal/domain"
	"encoding/json"
	"strings"
)

func parseJSONFromLLM(raw string, dest interface{}) error {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	return json.Unmarshal([]byte(raw), dest)
}

func extractSearchURLs(raw string) []string {
	text := raw
	var wrapper struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err == nil && len(wrapper.Content) > 0 {
		text = wrapper.Content[0].Text
	}
	var payload struct {
		Data struct {
			Web []struct {
				URL string `json:"url"`
			} `json:"web"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	urls := make([]string, 0, len(payload.Data.Web))
	for _, w := range payload.Data.Web {
		if w.URL != "" {
			urls = append(urls, w.URL)
		}
	}
	return urls
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func ptrAgent(name domain.AgentName) *domain.AgentName {
	n := name
	return &n
}
