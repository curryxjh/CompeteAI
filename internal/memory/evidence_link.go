package memory

import (
	"CompeteAI/internal/repository/dao"
	"context"

	"github.com/google/uuid"
)

func linkEvidence(ctx context.Context, d *dao.MemoryDao, factID string, f FactCandidate) {
	if d == nil || factID == "" || f.SourceID == "" || f.QuoteText == "" {
		return
	}
	_ = d.CreateEvidenceLink(ctx, dao.MemoryEvidenceLinkEntity{
		ID: uuid.NewString(), FactID: factID, SourceID: f.SourceID,
		QuoteText: f.QuoteText, QuoteHash: ContentHash(f.QuoteText),
		SupportType: "direct", SupportScore: f.ConfidenceScore,
	})
}
