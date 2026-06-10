package memory

import (
	"fmt"
	"strings"
)

// FormatForPrompt 将 AgentMemoryContext 格式化为可注入 LLM 的文本。
func FormatForPrompt(ctx AgentMemoryContext) string {
	if len(ctx.Preferences) == 0 && len(ctx.Facts) == 0 && len(ctx.Episodes) == 0 && len(ctx.Evidence) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n--- 长期记忆上下文 ---\n")
	if sec := FormatExplicitSection(ctx.Preferences); sec != "" {
		b.WriteString(sec)
	}
	if len(ctx.Facts) > 0 {
		b.WriteString("## 已验证/候选事实\n")
		for _, f := range ctx.Facts {
			b.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", f.VerificationStatus, f.Summary, f.Subject))
		}
	}
	if len(ctx.Episodes) > 0 {
		b.WriteString("## 历史任务经验\n")
		for _, ep := range ctx.Episodes {
			b.WriteString(fmt.Sprintf("- %s (QA=%d): %s\n", ep.Title, ep.QAScore, ep.Summary))
			for _, l := range ep.Lessons {
				b.WriteString("  · " + l + "\n")
			}
		}
	}
	if len(ctx.Evidence) > 0 {
		b.WriteString("## 相关证据片段\n")
		for _, ev := range ctx.Evidence {
			b.WriteString(fmt.Sprintf("- %s\n", truncate(ev.ContentText, 180)))
		}
	}
	b.WriteString("---\n")
	return b.String()
}
