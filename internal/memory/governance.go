package memory

import (
	"CompeteAI/internal/repository/dao"
	"CompeteAI/settings"
	"context"
	"time"

	"github.com/google/uuid"
)

type governance struct {
	dao *dao.MemoryDao
	cfg *settings.MemoryConfig
}

func NewGovernance(d *dao.MemoryDao, cfg *settings.MemoryConfig) Governance {
	return &governance{dao: d, cfg: cfg}
}

func (g *governance) Dedup(ctx context.Context, facts []FactCandidate) ([]FactCandidate, error) {
	seen := map[string]struct{}{}
	var out []FactCandidate
	for _, f := range facts {
		key := NormalizeName(f.Subject) + "|" + f.Predicate + "|" + NormalizeName(f.ObjectText)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out, nil
}

func (g *governance) Merge(ctx context.Context, scopeID, taskID string, facts []FactCandidate) error {
	minConf := 0.65
	if g.cfg != nil && g.cfg.WritePolicy.MinConfidence > 0 {
		minConf = g.cfg.WritePolicy.MinConfidence
	}
	requireSource := true
	if g.cfg != nil {
		requireSource = g.cfg.WritePolicy.RequireSourceForFact
	}
	now := Now()
	for _, f := range facts {
		if f.ConfidenceScore < minConf {
			continue
		}
		if requireSource && f.QuoteText == "" && f.InferenceType == "extracted" {
			continue
		}
		hash := FactHash(scopeID, f.Subject, f.Predicate, f.ObjectText)
		factID := uuid.NewString()
		srcCount := 1
		if existing, err := g.dao.GetFactByHash(ctx, scopeID, hash); err == nil {
			factID = existing.ID
			srcCount = existing.SourceCount + 1
		}
		var entityID *string
		if f.EntityID != "" {
			entityID = &f.EntityID
		}
		tid := taskID
		row := dao.MemoryFactEntity{
			ID: factID, ScopeID: scopeID, EntityID: entityID,
			FactType: f.FactType, Subject: f.Subject, Predicate: f.Predicate,
			ObjectValueJSON: dao.MarshalJSON(f.ObjectValue), ObjectText: f.ObjectText,
			NormalizedHash: hash, Summary: f.Summary,
			ConfidenceScore: f.ConfidenceScore, FreshnessScore: 0.85,
			ImportanceScore: 0.5, SourceCount: srcCount, InferenceType: f.InferenceType,
			VerificationStatus: "pending", Status: "active",
			FirstSeenAt: now, LastSeenAt: now,
			CreatedByTaskID: &tid, UpdatedByTaskID: &tid,
		}
		if err := g.dao.UpsertFact(ctx, row); err != nil {
			return err
		}
		linkEvidence(ctx, g.dao, factID, f)
		_ = g.dao.CreateEvent(ctx, dao.MemoryEventEntity{
			ID: uuid.NewString(), AggregateType: "fact", AggregateID: factID,
			EventType: "fact.upserted", PayloadJSON: dao.MarshalJSON(f),
			CreatedByTaskID: &taskID,
		})
	}
	return nil
}

func (g *governance) Decay(ctx context.Context) error {
	decayDays := 90
	staleDays := 180
	if g.cfg != nil {
		if g.cfg.Governance.DecayAfterDays > 0 {
			decayDays = g.cfg.Governance.DecayAfterDays
		}
		if g.cfg.Governance.StaleAfterDays > 0 {
			staleDays = g.cfg.Governance.StaleAfterDays
		}
	}
	_ = decayDays
	cutoff := time.Now().AddDate(0, 0, -staleDays)
	rows, _ := g.dao.ListFacts(ctx, "", 500)
	for _, f := range rows {
		if f.LastSeenAt.Before(cutoff) && f.Status == "active" {
			_ = g.dao.UpdateFactStatus(ctx, f.ID, "stale", f.VerificationStatus)
		}
	}
	return nil
}

func (g *governance) Invalidate(ctx context.Context, factID, reason string) error {
	if factID == "" {
		return nil
	}
	if err := g.dao.UpdateFactStatus(ctx, factID, "invalid", "rejected"); err != nil {
		return err
	}
	tid := reason
	_ = g.dao.CreateEvent(ctx, dao.MemoryEventEntity{
		ID: uuid.NewString(), AggregateType: "fact", AggregateID: factID,
		EventType: "fact.invalidated", PayloadJSON: dao.MarshalJSON(map[string]string{"reason": reason}),
		CreatedByTaskID: &tid,
	})
	recordInvalidated()
	return nil
}
