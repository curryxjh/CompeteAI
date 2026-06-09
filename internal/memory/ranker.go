package memory

import (
	"math"
	"strings"
)

// Ranker 长期记忆检索排序（§12.2）。
type Ranker struct{}

func NewRanker() *Ranker { return &Ranker{} }

func (r *Ranker) ScoreFact(f Fact, semantic, scope, entity float64) float64 {
	conf := f.ConfidenceScore
	fresh := f.FreshnessScore
	reliability := 0.5
	if f.VerificationStatus == "verified" {
		reliability = 0.9
		conf += 0.1
	}
	if f.VerificationStatus == "invalid" || f.Status == "invalid" {
		return -1
	}
	score := 0.40*semantic + 0.20*scope + 0.15*entity + 0.10*fresh + 0.10*conf + 0.05*reliability
	return score
}

func (r *Ranker) ScoreEvidence(ch EvidenceChunk, semantic float64) float64 {
	return 0.6*semantic + 0.4*ch.SupportScore
}

func (r *Ranker) ScoreEpisode(ep Episode, semantic float64) float64 {
	qa := 0.5
	if ep.QAScore > 0 {
		qa = float64(ep.QAScore) / 100.0
	}
	return 0.5*semantic + 0.3*qa + 0.2*float64(len(ep.Lessons))/5.0
}

func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func KeywordScore(query, text string) float64 {
	q := strings.Fields(strings.ToLower(query))
	if len(q) == 0 {
		return 0
	}
	lower := strings.ToLower(text)
	hits := 0
	for _, w := range q {
		if len(w) < 2 {
			continue
		}
		if strings.Contains(lower, w) {
			hits++
		}
	}
	return float64(hits) / float64(len(q))
}

// LimitByPredicate 同一 predicate 最多保留 n 条。
func LimitByPredicate(facts []Fact, n int) []Fact {
	if n <= 0 {
		return facts
	}
	counts := map[string]int{}
	var out []Fact
	for _, f := range facts {
		if counts[f.Predicate] >= n {
			continue
		}
		counts[f.Predicate]++
		out = append(out, f)
	}
	return out
}
