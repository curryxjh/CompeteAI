package memory

import (
	"context"

	"CompeteAI/settings"
)

// MilvusRetriever 使用 Milvus ANN 搜索，返回 objectID 列表（不含完整业务数据）。
type MilvusRetriever struct {
	client *MilvusClient
	cfg    *settings.MilvusConfig
}

// NewMilvusRetriever 构建 MilvusRetriever。
func NewMilvusRetriever(c *MilvusClient, cfg *settings.MilvusConfig) *MilvusRetriever {
	return &MilvusRetriever{client: c, cfg: cfg}
}

// Search 执行向量 ANN 检索，返回命中的 objectID 列表（按相似度降序）。
func (r *MilvusRetriever) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error) {
	if r.client == nil {
		return nil, nil
	}
	return r.client.Search(ctx, req)
}
