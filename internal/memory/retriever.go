package memory

import (
	"CompeteAI/internal/repository/dao"
	"context"
	"encoding/json"
)

type mysqlRetriever struct {
	dao      *dao.MemoryDao
	embedder Embedder
	ranker   *Ranker
}

func NewMySQLRetriever(d *dao.MemoryDao, emb Embedder) Retriever {
	return &mysqlRetriever{dao: d, embedder: emb, ranker: NewRanker()}
}

func (r *mysqlRetriever) RetrieveFacts(ctx context.Context, req RetrievalRequest) ([]FactHit, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 12
	}
	rows, err := r.dao.SearchFactsBySubject(ctx, req.ScopeIDs, req.Competitors, limit*3)
	if err != nil {
		return nil, err
	}
	var qVec []float32
	if req.Query != "" {
		vecs, _ := r.embedder.Embed(ctx, []string{req.Query})
		if len(vecs) > 0 {
			qVec = vecs[0]
		}
	}
	var hits []FactHit
	for _, row := range rows {
		f := factFromEntity(row)
		semantic := KeywordScore(req.Query, f.Summary+" "+f.ObjectText)
		if qVec != nil {
			embRows, _ := r.dao.ListAllEmbeddings(ctx, "fact", 500)
			for _, er := range embRows {
				if er.ObjectID == f.ID {
					var vec []float32
					_ = json.Unmarshal([]byte(er.EmbeddingJSON), &vec)
					semantic = CosineSimilarity(qVec, vec)
					break
				}
			}
		}
		entityScore := 0.0
		for _, c := range req.Competitors {
			if NormalizeName(c) == NormalizeName(f.Subject) {
				entityScore = 1.0
				break
			}
		}
		scopeScore := 0.7
		score := r.ranker.ScoreFact(f, semantic, scopeScore, entityScore)
		if score < 0 {
			continue
		}
		f.FinalScore = score
		hits = append(hits, FactHit{Fact: f, SemanticScore: semantic})
	}
	hits = sortFactHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	facts := make([]Fact, len(hits))
	for i, h := range hits {
		facts[i] = h.Fact
	}
	facts = LimitByPredicate(facts, 3)
	out := make([]FactHit, len(facts))
	for i, f := range facts {
		out[i] = FactHit{Fact: f}
	}
	return out, nil
}

func (r *mysqlRetriever) RetrieveEvidence(ctx context.Context, req RetrievalRequest) ([]EvidenceHit, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	var allChunks []dao.MemoryChunkEntity
	var err error
	if req.TaskID != "" {
		allChunks, err = r.dao.ListChunksByTask(ctx, req.TaskID, 200)
	} else {
		allChunks, err = r.dao.ListRecentChunks(ctx, 200)
	}
	if err != nil {
		return nil, err
	}

	var qVec []float32
	if req.Query != "" {
		vecs, _ := r.embedder.Embed(ctx, []string{req.Query})
		if len(vecs) > 0 {
			qVec = vecs[0]
		}
	}

	sourceCache := map[string]dao.MemorySourceEntity{}
	var hits []EvidenceHit
	domainCount := map[string]int{}
	for _, ch := range allChunks {
		var vec []float32
		_ = json.Unmarshal([]byte(ch.EmbeddingJSON), &vec)
		semantic := KeywordScore(req.Query, ch.ContentText)
		if qVec != nil && len(vec) > 0 {
			semantic = CosineSimilarity(qVec, vec)
		}
		src, ok := sourceCache[ch.SourceID]
		if !ok {
			src, _ = r.dao.GetSource(ctx, ch.SourceID)
			sourceCache[ch.SourceID] = src
		}
		chunk := EvidenceChunk{
			ID: ch.ID, SourceID: ch.SourceID, SourceURL: src.SourceURL,
			SourceDomain: src.SourceDomain, ContentText: ch.ContentText,
			TokenCount: ch.TokenCount, SupportScore: 0.7,
		}
		score := r.ranker.ScoreEvidence(chunk, semantic)
		chunk.FinalScore = score
		dom := chunk.SourceDomain
		if dom == "" {
			dom = "unknown"
		}
		if domainCount[dom] >= 2 {
			continue
		}
		domainCount[dom] = domainCount[dom] + 1
		hits = append(hits, EvidenceHit{Chunk: chunk, SemanticScore: semantic})
		_ = ok
	}
	hits = sortEvidenceHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (r *mysqlRetriever) RetrieveEpisodes(ctx context.Context, req RetrievalRequest) ([]EpisodeHit, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 4
	}
	var hits []EpisodeHit
	for _, sid := range req.ScopeIDs {
		rows, err := r.dao.ListEpisodes(ctx, sid, limit*2)
		if err != nil {
			continue
		}
		for _, row := range rows {
			ep := episodeFromEntity(row)
			semantic := KeywordScore(req.Query, ep.Summary+" "+ep.QueryText)
			score := r.ranker.ScoreEpisode(ep, semantic)
			ep.FinalScore = score
			hits = append(hits, EpisodeHit{Episode: ep, SemanticScore: semantic})
		}
	}
	hits = sortEpisodeHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func factFromEntity(row dao.MemoryFactEntity) Fact {
	var obj map[string]any
	_ = json.Unmarshal([]byte(row.ObjectValueJSON), &obj)
	entID := ""
	if row.EntityID != nil {
		entID = *row.EntityID
	}
	return Fact{
		ID: row.ID, ScopeID: row.ScopeID, EntityID: entID,
		FactType: row.FactType, Subject: row.Subject, Predicate: row.Predicate,
		ObjectValue: obj, ObjectText: row.ObjectText, Summary: row.Summary,
		ConfidenceScore: row.ConfidenceScore, FreshnessScore: row.FreshnessScore,
		ImportanceScore: row.ImportanceScore, VerificationStatus: row.VerificationStatus,
		Status: row.Status, SourceCount: row.SourceCount,
	}
}

func episodeFromEntity(row dao.MemoryEpisodeEntity) Episode {
	var comps, dims, lessons []string
	_ = json.Unmarshal([]byte(row.CompetitorsJSON), &comps)
	_ = json.Unmarshal([]byte(row.DimensionsJSON), &dims)
	_ = json.Unmarshal([]byte(row.LessonsJSON), &lessons)
	qa := 0
	if row.QAScore != nil {
		qa = *row.QAScore
	}
	return Episode{
		ID: row.ID, TaskID: row.TaskID, Title: row.Title,
		QueryText: row.QueryText, Summary: row.Summary,
		Competitors: comps, Dimensions: dims, Outcome: row.Outcome,
		QAScore: qa, Lessons: lessons,
	}
}

func sortFactHits(hits []FactHit) []FactHit {
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Fact.FinalScore > hits[i].Fact.FinalScore {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	return hits
}

func sortEvidenceHits(hits []EvidenceHit) []EvidenceHit {
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Chunk.FinalScore > hits[i].Chunk.FinalScore {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	return hits
}

func sortEpisodeHits(hits []EpisodeHit) []EpisodeHit {
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Episode.FinalScore > hits[i].Episode.FinalScore {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	return hits
}
