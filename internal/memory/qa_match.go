package memory

import (
	"CompeteAI/internal/repository/dao"
	"strings"
	"unicode"
)

// claimMatchesIssue 判断 claim 是否被某条 QA issue 命中。
func claimMatchesIssue(claimText string, iss QAIssueDetail) bool {
	c := strings.ToLower(strings.TrimSpace(claimText))
	if c == "" {
		return false
	}
	for _, part := range []string{iss.Location, iss.Problem, iss.Category} {
		p := strings.ToLower(strings.TrimSpace(part))
		if len(p) < 3 {
			continue
		}
		if strings.Contains(c, p) || strings.Contains(p, c) {
			return true
		}
		if tokenOverlap(c, p) >= 0.4 {
			return true
		}
	}
	return false
}

func factMatchesIssue(f dao.MemoryFactEntity, iss QAIssueDetail) bool {
	text := strings.ToLower(f.Subject + " " + f.Summary + " " + f.ObjectText + " " + iss.Location)
	p := strings.ToLower(strings.TrimSpace(iss.Problem + " " + iss.Location))
	if p == "" {
		return false
	}
	return strings.Contains(text, p) || tokenOverlap(text, p) >= 0.35
}

func claimMatchedByAnyIssue(claimText string, issues []QAIssueDetail) (QAIssueDetail, bool) {
	for _, iss := range issues {
		if claimMatchesIssue(claimText, iss) {
			return iss, true
		}
	}
	return QAIssueDetail{}, false
}

func tokenOverlap(a, b string) float64 {
	tokensA := tokenSet(a)
	tokensB := tokenSet(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}
	hits := 0
	for t := range tokensA {
		if tokensB[t] {
			hits++
		}
	}
	denom := len(tokensA)
	if len(tokensB) < denom {
		denom = len(tokensB)
	}
	if denom == 0 {
		return 0
	}
	return float64(hits) / float64(denom)
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	var cur []rune
	flush := func() {
		if len(cur) >= 2 {
			out[string(cur)] = true
		}
		cur = cur[:0]
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > 127 {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func qaStatusForIssue(iss QAIssueDetail, passed bool) (verification, qaResult string) {
	if passed {
		return "verified", "pass"
	}
	cat := strings.ToLower(iss.Category)
	switch {
	case strings.Contains(cat, "unsupported"), strings.Contains(cat, "incomplete"):
		return "rejected", "fail"
	case strings.Contains(cat, "source"):
		return "rejected", "fail"
	default:
		return "needs_review", "partial"
	}
}
