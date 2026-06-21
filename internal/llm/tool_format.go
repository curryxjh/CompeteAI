package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatToolResultForUI 将 MCP 工具原始 JSON 转为可读摘要，供前端展示。
func FormatToolResultForUI(toolName, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if summary := formatFirecrawlPayload(toolName, raw); summary != "" {
		return summary
	}
	return truncateForUI(raw, 2000)
}

func formatFirecrawlPayload(toolName, raw string) string {
	text := extractMCPToolText(raw)
	if text == "" {
		text = raw
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return truncateForUI(text, 1500)
	}

	switch {
	case strings.Contains(toolName, "search"):
		return formatSearchResult(payload)
	case strings.Contains(toolName, "scrape"):
		return formatScrapeResult(payload)
	case strings.Contains(toolName, "map"):
		return formatLinksResult(payload, "站点链接")
	case strings.Contains(toolName, "crawl"):
		return formatLinksResult(payload, "爬取页面")
	default:
		return formatGenericResult(payload)
	}
}

func extractMCPToolText(raw string) string {
	var wrapper struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil || len(wrapper.Content) == 0 {
		return ""
	}
	for _, block := range wrapper.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text
		}
	}
	return wrapper.Content[0].Text
}

func formatSearchResult(payload map[string]interface{}) string {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return formatGenericResult(payload)
	}
	web, _ := data["web"].([]interface{})
	if len(web) == 0 {
		return "搜索完成，未找到相关网页结果。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 **%d** 条搜索结果：\n\n", len(web)))
	for i, item := range web {
		if i >= 8 {
			sb.WriteString(fmt.Sprintf("\n… 还有 %d 条未展示", len(web)-8))
			break
		}
		row, _ := item.(map[string]interface{})
		title := asString(row["title"])
		url := asString(row["url"])
		desc := asString(row["description"])
		if title == "" {
			title = url
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, title))
		if url != "" {
			sb.WriteString(fmt.Sprintf("   - %s\n", url))
		}
		if desc != "" {
			sb.WriteString(fmt.Sprintf("   - %s\n", truncateForUI(desc, 160)))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func formatScrapeResult(payload map[string]interface{}) string {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return formatGenericResult(payload)
	}
	title := asString(data["title"])
	url := asString(data["url"])
	md := asString(data["markdown"])
	if md == "" {
		md = asString(data["content"])
	}

	var sb strings.Builder
	if title != "" {
		sb.WriteString(fmt.Sprintf("**%s**\n\n", title))
	}
	if url != "" {
		sb.WriteString(fmt.Sprintf("来源：%s\n\n", url))
	}
	if md != "" {
		sb.WriteString(truncateForUI(md, 1200))
	} else {
		sb.WriteString("页面抓取成功，但未返回 Markdown 正文。")
	}
	return strings.TrimSpace(sb.String())
}

func formatLinksResult(payload map[string]interface{}, label string) string {
	data, _ := payload["data"].(map[string]interface{})
	if data == nil {
		return formatGenericResult(payload)
	}
	links, _ := data["links"].([]interface{})
	if len(links) == 0 {
		return fmt.Sprintf("%s完成，未发现链接。", label)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 **%d** 个%s：\n\n", len(links), label))
	for i, link := range links {
		if i >= 12 {
			sb.WriteString(fmt.Sprintf("\n… 还有 %d 个未展示", len(links)-12))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s\n", asString(link)))
	}
	return strings.TrimSpace(sb.String())
}

func formatGenericResult(payload map[string]interface{}) string {
	if msg := asString(payload["error"]); msg != "" {
		return "错误：" + msg
	}
	if success, ok := payload["success"].(bool); ok && !success {
		pretty, _ := json.MarshalIndent(payload, "", "  ")
		return truncateForUI(string(pretty), 1500)
	}
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return truncateForUI(fmt.Sprintf("%v", payload), 1500)
	}
	return truncateForUI(string(pretty), 1500)
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

// FormatToolArgsForUI 将工具参数 JSON 转为前端可读摘要。
func FormatToolArgsForUI(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return raw
	}

	parts := make([]string, 0, len(args))
	if q := asString(args["query"]); q != "" {
		parts = append(parts, "query: "+truncateForUI(q, 120))
	}
	if u := asString(args["url"]); u != "" {
		parts = append(parts, "url: "+u)
	}
	if lim, ok := args["limit"]; ok {
		parts = append(parts, fmt.Sprintf("limit: %v", lim))
	}
	if len(parts) == 0 {
		pretty, _ := json.MarshalIndent(args, "", "  ")
		return truncateForUI(string(pretty), 500)
	}
	return strings.Join(parts, "\n")
}

func truncateForUI(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...(truncated)"
}
