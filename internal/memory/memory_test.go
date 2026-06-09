package memory

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	if got := NormalizeName("GitHub Copilot"); got != "githubcopilot" {
		t.Fatalf("normalize failed: %s", got)
	}
}

func TestFactHashStable(t *testing.T) {
	h1 := FactHash("scope1", "Notion", "has_pricing", "$10/mo")
	h2 := FactHash("scope1", "notion", "has_pricing", "$10/mo")
	if h1 != h2 {
		t.Fatal("fact hash should normalize subject/object")
	}
}

func TestRankerVerifiedBoost(t *testing.T) {
	r := NewRanker()
	f := Fact{ConfidenceScore: 0.7, FreshnessScore: 0.7, VerificationStatus: "verified", Status: "active"}
	s := r.ScoreFact(f, 0.8, 0.7, 1.0)
	if s <= 0 {
		t.Fatal("score should be positive")
	}
	invalid := Fact{VerificationStatus: "invalid", Status: "invalid"}
	if r.ScoreFact(invalid, 1, 1, 1) >= 0 {
		t.Fatal("invalid fact should be filtered")
	}
}

func TestParseExplicitMarkdown(t *testing.T) {
	prefs := parseExplicitMarkdown(`- ` + "`report.style`" + `: 专业客观`)
	if len(prefs) != 1 || prefs[0].Key != "report.style" {
		t.Fatalf("parse failed: %+v", prefs)
	}
}

func TestFormatForPromptEmpty(t *testing.T) {
	if FormatForPrompt(AgentMemoryContext{}) != "" {
		t.Fatal("empty context should produce empty string")
	}
}

func TestClaimMatchesIssue(t *testing.T) {
	iss := QAIssueDetail{Problem: "SWOT 未覆盖全部竞品", Location: "report.swot", Category: "analysis_incomplete"}
	if !claimMatchesIssue("Notion SWOT strengths=2 weaknesses=1", iss) {
		t.Fatal("expected SWOT claim to match issue")
	}
	if claimMatchesIssue("unrelated pricing tiers", iss) {
		t.Fatal("unrelated claim should not match")
	}
}

func TestScopeResolverKeys(t *testing.T) {
	n := NormalizeName("GitHub Copilot")
	if n != "githubcopilot" {
		t.Fatalf("bad normalize: %s", n)
	}
}
