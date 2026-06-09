package memory

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type defaultExtractor struct {
	resolver *Resolver
}

func NewExtractor(resolver *Resolver) Extractor {
	return &defaultExtractor{resolver: resolver}
}

func (e *defaultExtractor) ExtractFacts(ctx context.Context, req ExtractFactsRequest) ([]FactCandidate, error) {
	var out []FactCandidate
	priceRe := regexp.MustCompile(`(?i)(\$[\d.]+\s*/?\s*\w*)`)
	for sid, src := range req.Sources {
		if src.URL == "" {
			continue
		}
		for _, comp := range req.Competitors {
			if !strings.Contains(strings.ToLower(src.Excerpt+src.Title), strings.ToLower(comp)) {
				continue
			}
			ent, _ := e.resolver.Resolve(ctx, "competitor", comp)
			for _, price := range priceRe.FindAllString(src.Excerpt, 3) {
				dbSourceID := req.SourceDBIDs[sid]
				out = append(out, FactCandidate{
					FactType: "pricing", Subject: comp, Predicate: "has_pricing",
					ObjectValue: map[string]any{"price": price, "source_ref": sid},
					ObjectText: price, Summary: comp + " 定价: " + price,
					ConfidenceScore: 0.72, InferenceType: "extracted",
					QuoteText: truncate(src.Excerpt, 200), EntityID: ent.ID,
					SourceID: dbSourceID,
				})
			}
			if strings.Contains(strings.ToLower(src.Excerpt), "feature") || strings.Contains(src.Excerpt, "功能") {
				dbSourceID := req.SourceDBIDs[sid]
				out = append(out, FactCandidate{
					FactType: "feature", Subject: comp, Predicate: "has_feature_mention",
					ObjectText: truncate(src.Excerpt, 120), Summary: comp + " 功能提及",
					ConfidenceScore: 0.65, InferenceType: "extracted",
					QuoteText: truncate(src.Excerpt, 200), EntityID: ent.ID,
					SourceID: dbSourceID,
				})
			}
		}
	}
	return out, nil
}

func (e *defaultExtractor) ExtractEpisode(ctx context.Context, req ExtractEpisodeRequest) (Episode, error) {
	lessons := req.Lessons
	if len(lessons) == 0 {
		lessons = []string{"完成竞品分析任务"}
		if req.QAScore > 0 && req.QAScore < 80 {
			lessons = append(lessons, "QA 分数偏低，需加强来源与结构完整性")
		}
	}
	query := strings.Join(req.Competitors, " vs ")
	if req.Title != "" {
		query = req.Title
	}
	summary := req.Summary
	if summary == "" {
		summary = fmt.Sprintf("任务 %s 结果: %s, QA=%d", req.TaskID, req.Outcome, req.QAScore)
	}
	return Episode{
		TaskID: req.TaskID, Title: req.Title, QueryText: query,
		Summary: summary, Competitors: req.Competitors, Dimensions: req.Dimensions,
		Outcome: req.Outcome, QAScore: req.QAScore, Lessons: lessons,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ExtractClaimsFromReport 从报告结构生成 claim 候选。
func ExtractClaimsFromReport(report domain.Report) []string {
	var claims []string
	for comp, swot := range report.SWOT {
		for _, s := range swot.Strengths {
			if s.Text != "" {
				claims = append(claims, comp+": "+s.Text)
			}
		}
	}
	return claims
}

// Resolver 实体归一。
type Resolver struct {
	dao *dao.MemoryDao
}

func NewResolver(d *dao.MemoryDao) *Resolver { return &Resolver{dao: d} }

func (r *Resolver) Normalize(name string) string { return NormalizeName(name) }

func (r *Resolver) Resolve(ctx context.Context, entityType, name string) (Entity, error) {
	norm := NormalizeName(name)
	row, err := r.dao.FindEntityByNormalized(ctx, entityType, norm)
	if err == nil {
		return entityFromRow(row), nil
	}
	return r.EnsureEntity(ctx, entityType, name)
}

func (r *Resolver) EnsureEntity(ctx context.Context, entityType, name string) (Entity, error) {
	norm := NormalizeName(name)
	row, err := r.dao.FindEntityByNormalized(ctx, entityType, norm)
	if err == nil {
		return entityFromRow(row), nil
	}
	now := Now()
	e := dao.MemoryEntityEntity{
		ID: uuid.NewString(), CanonicalName: strings.TrimSpace(name),
		EntityType: entityType, NormalizedName: norm,
		AliasesJSON: dao.MarshalJSON([]string{name}),
		ConfidenceScore: 0.8, Status: "active",
		FirstSeenAt: now, LastSeenAt: now,
	}
	if err := r.dao.UpsertEntity(ctx, e); err != nil {
		return Entity{}, err
	}
	return entityFromRow(e), nil
}

func entityFromRow(row dao.MemoryEntityEntity) Entity {
	var aliases []string
	_ = jsonUnmarshalAliases(row.AliasesJSON, &aliases)
	return Entity{
		ID: row.ID, CanonicalName: row.CanonicalName,
		EntityType: row.EntityType, NormalizedName: row.NormalizedName,
		Aliases: aliases,
	}
}

func jsonUnmarshalAliases(raw string, dest *[]string) error {
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), dest)
}
