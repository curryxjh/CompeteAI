package memory

import (
	"context"
	"encoding/json"

	"CompeteAI/internal/repository/dao"
)

// HybridRetriever 混合检索器：Milvus ANN + MySQL 关键词，两路结果通过 RRF 合并。
// 实现 Retriever 接口，对上层 MemoryService 完全透明。
// 当 milvus 为 nil 时，自动退回到 mysqlRetriever（完全兼容旧行为）。
type HybridRetriever struct {
	milvus   *MilvusRetriever
	mysql    *mysqlRetriever
	embedder Embedder
}

// NewHybridRetriever 构建 HybridRetriever。
// milvus 为 nil 时退化为纯 MySQL 模式。
func NewHybridRetriever(milvus *MilvusRetriever, mysql *mysqlRetriever, embedder Embedder) Retriever {
	return &HybridRetriever{milvus: milvus, mysql: mysql, embedder: embedder}
}

// RetrieveFacts 混合检索 Fact。
func (r *HybridRetriever) RetrieveFacts(ctx context.Context, req RetrievalRequest) ([]FactHit, error) {
	// Milvus 不可用时直接委托 MySQL
	if r.milvus == nil {
		return r.mysql.RetrieveFacts(ctx, req)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 12
	}

	// Step 1: 生成查询向量
	var qVec []float32
	if req.Query != "" && r.embedder != nil {
		vecs, _ := r.embedder.Embed(ctx, []string{req.Query})
		if len(vecs) > 0 {
			qVec = vecs[0]
		}
	}

	// Step 2: Milvus ANN 召回（按 scope_key 过滤）
	var milvusIDs []string
	milvusScores := map[string]float32{}
	if qVec != nil {
		hits, err := r.milvus.Search(ctx, AnnSearchRequest{
			QueryVector: qVec,
			ObjectType:  "fact",
			ScopeKey:    primaryScopeKey(req.ScopeKeys),
			TopK:        limit * 5,
		})
		if err == nil {
			for _, h := range hits {
				milvusIDs = append(milvusIDs, h.ObjectID)
				milvusScores[h.ObjectID] = h.Score
			}
		}
	}

	// Step 3: MySQL 关键词召回
	kwRows, _ := r.mysql.dao.SearchFactsBySubject(ctx, req.ScopeIDs, req.Competitors, limit*3)
	kwIDs := make([]string, 0, len(kwRows))
	kwMap := make(map[string]dao.MemoryFactEntity, len(kwRows))
	for _, row := range kwRows {
		kwIDs = append(kwIDs, row.ID)
		kwMap[row.ID] = row
	}

	// Step 4: 批量拉取 Milvus 召回的 Fact 完整数据
	milvusFacts, _ := r.mysql.dao.GetFactsByIDs(ctx, milvusIDs)
	for _, f := range milvusFacts {
		if _, ok := kwMap[f.ID]; !ok {
			kwMap[f.ID] = f
		}
	}

	// Step 5: 构建 rank map，计算 RRF 分数
	mRankMap := rankMap(milvusIDs)
	kRankMap := rankMap(kwIDs)

	type scored struct {
		entity   dao.MemoryFactEntity
		rrfScore float64
		semantic float64
	}
	scoredFacts := make([]scored, 0, len(kwMap))
	for id, row := range kwMap {
		rs := rrfCombineScore(mRankMap[id]-1, kRankMap[id]-1, 60)
		sem := float64(milvusScores[id])
		if sem == 0 && req.Query != "" {
			f := factFromEntity(row)
			sem = KeywordScore(req.Query, f.Summary+" "+f.ObjectText)
		}
		scoredFacts = append(scoredFacts, scored{entity: row, rrfScore: rs, semantic: sem})
	}

	// Step 6: 按 RRF 分数排序
	for i := 0; i < len(scoredFacts); i++ {
		for j := i + 1; j < len(scoredFacts); j++ {
			if scoredFacts[j].rrfScore > scoredFacts[i].rrfScore {
				scoredFacts[i], scoredFacts[j] = scoredFacts[j], scoredFacts[i]
			}
		}
	}
	if len(scoredFacts) > limit {
		scoredFacts = scoredFacts[:limit]
	}

	// Step 7: 转换为 FactHit
	hits := make([]FactHit, 0, len(scoredFacts))
	for _, sf := range scoredFacts {
		f := factFromEntity(sf.entity)
		f.FinalScore = sf.rrfScore
		hits = append(hits, FactHit{Fact: f, SemanticScore: sf.semantic})
	}
	return hits, nil
}

// RetrieveEvidence 混合检索 Evidence（chunk）。
func (r *HybridRetriever) RetrieveEvidence(ctx context.Context, req RetrievalRequest) ([]EvidenceHit, error) {
	if r.milvus == nil {
		return r.mysql.RetrieveEvidence(ctx, req)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}

	// 生成查询向量
	var qVec []float32
	if req.Query != "" && r.embedder != nil {
		vecs, _ := r.embedder.Embed(ctx, []string{req.Query})
		if len(vecs) > 0 {
			qVec = vecs[0]
		}
	}

	// Milvus ANN 召回 chunk objectID
	milvusChunkIDs := map[string]float32{}
	if qVec != nil {
		hits, err := r.milvus.Search(ctx, AnnSearchRequest{
			QueryVector: qVec,
			ObjectType:  "chunk",
			ScopeKey:    primaryScopeKey(req.ScopeKeys),
			TopK:        limit * 5,
		})
		if err == nil {
			for _, h := range hits {
				milvusChunkIDs[h.ObjectID] = h.Score
			}
		}
	}

	// MySQL 按 taskID / 最近 chunk 召回
	var allChunks []dao.MemoryChunkEntity
	if req.TaskID != "" {
		allChunks, _ = r.mysql.dao.ListChunksByTask(ctx, req.TaskID, 200)
	} else {
		allChunks, _ = r.mysql.dao.ListRecentChunks(ctx, 200)
	}

	sourceCache := map[string]dao.MemorySourceEntity{}
	var hits []EvidenceHit
	domainCount := map[string]int{}

	for _, ch := range allChunks {
		// 语义分：优先 Milvus ANN 分，其次 chunk 内嵌向量余弦相似度，再次关键词匹配
		var semantic float64
		if score, ok := milvusChunkIDs[ch.ID]; ok {
			semantic = float64(score)
		} else {
			var vec []float32
			_ = json.Unmarshal([]byte(ch.EmbeddingJSON), &vec)
			if qVec != nil && len(vec) > 0 {
				semantic = CosineSimilarity(qVec, vec)
			} else {
				semantic = KeywordScore(req.Query, ch.ContentText)
			}
		}

		src, ok := sourceCache[ch.SourceID]
		if !ok {
			src, _ = r.mysql.dao.GetSource(ctx, ch.SourceID)
			sourceCache[ch.SourceID] = src
		}
		chunk := EvidenceChunk{
			ID: ch.ID, SourceID: ch.SourceID, SourceURL: src.SourceURL,
			SourceDomain: src.SourceDomain, ContentText: ch.ContentText,
			TokenCount: ch.TokenCount, SupportScore: 0.7,
		}
		score := r.mysql.ranker.ScoreEvidence(chunk, semantic)
		chunk.FinalScore = score
		dom := chunk.SourceDomain
		if dom == "" {
			dom = "unknown"
		}
		if domainCount[dom] >= 2 {
			continue
		}
		domainCount[dom]++
		hits = append(hits, EvidenceHit{Chunk: chunk, SemanticScore: semantic})
		_ = ok
	}

	hits = sortEvidenceHits(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// RetrieveEpisodes 直接委托 MySQL（episode 数量少，keyword 已足够）。
func (r *HybridRetriever) RetrieveEpisodes(ctx context.Context, req RetrievalRequest) ([]EpisodeHit, error) {
	return r.mysql.RetrieveEpisodes(ctx, req)
}

// ─────────────────────────────────────────────
// RRF 辅助函数
// ─────────────────────────────────────────────

// rankMap 将有序 ID 列表转换为 rank map（1-based，0 表示不在列表中）。
func rankMap(ids []string) map[string]int {
	m := make(map[string]int, len(ids))
	for i, id := range ids {
		m[id] = i + 1 // 1-based rank
	}
	return m
}

// rrfCombineScore 计算 RRF 加权分数。
// rank 为 1-based，0 表示不在该路中。k=60 为标准参数。
func rrfCombineScore(mRank, kRank, k int) float64 {
	score := 0.0
	if mRank > 0 {
		score += 1.0 / float64(k+mRank)
	}
	if kRank > 0 {
		score += 1.0 / float64(k+kRank)
	}
	return score
}

// primaryScopeKey 从 ScopeKeys 列表中取第一个有效值。
func primaryScopeKey(keys []string) string {
	for _, k := range keys {
		if k != "" {
			return k
		}
	}
	return ""
}
