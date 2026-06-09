package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/state"
	"testing"
)

func TestEnrichAnalysisSources(t *testing.T) {
	sources := map[string]domain.SourceRef{
		"s1": {URL: "https://a.com", Title: "Product A pricing", Excerpt: "A supports collaboration and API access"},
		"s2": {URL: "https://b.com", Title: "Product B", Excerpt: "B has advanced analytics dashboard"},
	}
	out := state.AnalysisOutput{
		Summary: "test",
		SWOT: map[string]domain.SWOTAnalysis{
			"A": {
				Strengths: []domain.SWOTItem{{Text: "collaboration API"}},
			},
		},
		Features: []domain.FeatureRow{{
			Feature: "API",
			Values:  map[string]interface{}{"A": true},
		}},
	}
	enrichAnalysisSources(&out, sources)
	if len(out.SWOT["A"].Strengths[0].SourceIDs) == 0 {
		t.Fatal("expected auto-linked sourceIds for SWOT")
	}
	if len(out.Features[0].SourceIDs["A"]) == 0 {
		t.Fatal("expected auto-linked sourceIds for feature row")
	}
}

func TestSanitizeSourceIDs(t *testing.T) {
	valid := map[string]struct{}{"s1": {}, "s2": {}}
	got := sanitizeSourceIDs([]string{"s1", "bad", "s1"}, valid)
	if len(got) != 1 || got[0] != "s1" {
		t.Fatalf("unexpected sanitize result: %v", got)
	}
}

func TestCountSourcedConclusions(t *testing.T) {
	report := domain.Report{
		SWOT: map[string]domain.SWOTAnalysis{
			"X": {Strengths: []domain.SWOTItem{
				{Text: "a", SourceIDs: []string{"s1"}},
				{Text: "b"},
			}},
		},
		Features: []domain.FeatureRow{{
			Feature: "f",
			SourceIDs: map[string][]string{"X": {"s1"}},
		}},
	}
	s, total := countSourcedConclusions(report)
	if s != 2 || total != 3 {
		t.Fatalf("got sourced=%d total=%d", s, total)
	}
}
