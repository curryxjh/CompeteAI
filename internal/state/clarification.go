package state

import (
	"strings"
)

// MergeClarificationIntoMeta 将用户澄清答复合并进任务元信息。
func MergeClarificationIntoMeta(meta TaskMeta, answer ClarificationAnswer, question ClarificationQuestion) TaskMeta {
	ans := strings.TrimSpace(answer.Answer)
	if ans == "" {
		return meta
	}

	needsCompetitors := len(meta.Competitors) < 1 ||
		strings.Contains(question.Question, "竞品")

	if needsCompetitors {
		if parts := splitClarificationList(ans); len(parts) > 0 {
			meta.Competitors = parts
		}
	}

	if len(meta.Dimensions) < 1 && strings.Contains(question.Question, "维度") {
		if parts := splitClarificationList(ans); len(parts) > 0 {
			meta.Dimensions = parts
		}
	}

	return meta
}

func splitClarificationList(s string) []string {
	s = strings.NewReplacer("，", ",", "、", ",", "；", ",").Replace(s)
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
