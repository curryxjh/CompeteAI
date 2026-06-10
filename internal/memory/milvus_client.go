package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"CompeteAI/settings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	milvusDefaultDim             = 1024
	milvusDefaultHNSWM           = 16
	milvusDefaultHNSWEfConstruct = 200
	milvusDefaultEfSearch        = 64
	milvusDefaultTopK            = 50
	milvusContentTextMaxLen      = 2048
)

// MilvusClient 封装 Milvus 连接和 Collection 生命周期管理。
type MilvusClient struct {
	mu         sync.Mutex
	milvus     client.Client
	cfg        *settings.MilvusConfig
	collection string
}

// NewMilvusClient 创建并初始化 MilvusClient。
// 成功时 collection 已存在且加载到内存，可立即执行搜索。
func NewMilvusClient(cfg *settings.MilvusConfig) (*MilvusClient, error) {
	if cfg == nil || cfg.Address == "" {
		return nil, fmt.Errorf("milvus config is nil or address is empty")
	}
	dim := cfg.Dim
	if dim <= 0 {
		dim = milvusDefaultDim
	}
	// 修复配置默认值
	if cfg.HNSWM <= 0 {
		cfg.HNSWM = milvusDefaultHNSWM
	}
	if cfg.HNSWEfConstruct <= 0 {
		cfg.HNSWEfConstruct = milvusDefaultHNSWEfConstruct
	}
	if cfg.CollectionPrefix == "" {
		cfg.CollectionPrefix = "compete_ai"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := client.NewClient(ctx, client.Config{Address: cfg.Address})
	if err != nil {
		return nil, fmt.Errorf("milvus connect %s: %w", cfg.Address, err)
	}

	mc := &MilvusClient{
		milvus:     c,
		cfg:        cfg,
		collection: cfg.CollectionPrefix + "_embeddings",
	}
	if err := mc.ensureCollection(context.Background()); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("milvus ensure collection: %w", err)
	}
	return mc, nil
}

// ensureCollection 幂等创建 collection + 索引 + 加载到内存。
func (mc *MilvusClient) ensureCollection(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	exists, err := mc.milvus.HasCollection(ctx, mc.collection)
	if err != nil {
		return fmt.Errorf("HasCollection: %w", err)
	}
	if !exists {
		schema := mc.buildSchema()
		if err := mc.milvus.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return fmt.Errorf("CreateCollection: %w", err)
		}
		// 向量字段索引
		idx, err := entity.NewIndexHNSW(mc.metricType(), mc.cfg.HNSWM, mc.cfg.HNSWEfConstruct)
		if err != nil {
			return fmt.Errorf("NewIndexHNSW: %w", err)
		}
		if err := mc.milvus.CreateIndex(ctx, mc.collection, "embedding", idx, false); err != nil {
			return fmt.Errorf("CreateIndex embedding: %w", err)
		}
	}
	// 确保 collection 加载到内存（幂等）
	if err := mc.milvus.LoadCollection(ctx, mc.collection, false); err != nil {
		return fmt.Errorf("LoadCollection: %w", err)
	}
	return nil
}

func (mc *MilvusClient) buildSchema() *entity.Schema {
	dim := mc.cfg.Dim
	if dim <= 0 {
		dim = milvusDefaultDim
	}
	return entity.NewSchema().
		WithName(mc.collection).
		WithField(entity.NewField().
			WithName("id").
			WithDataType(entity.FieldTypeVarChar).
			WithIsPrimaryKey(true).
			WithIsAutoID(false).
			WithMaxLength(36)).
		WithField(entity.NewField().
			WithName("object_type").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(64)).
		WithField(entity.NewField().
			WithName("object_id").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(36)).
		WithField(entity.NewField().
			WithName("scope_type").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(32)).
		WithField(entity.NewField().
			WithName("scope_key").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(128)).
		WithField(entity.NewField().
			WithName("content_text").
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(milvusContentTextMaxLen)).
		WithField(entity.NewField().
			WithName("embedding").
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(dim)))
}

func (mc *MilvusClient) metricType() entity.MetricType {
	switch strings.ToUpper(mc.cfg.MetricType) {
	case "IP":
		return entity.IP
	case "L2":
		return entity.L2
	default:
		return entity.COSINE
	}
}

// AnnSearchRequest 向量近似搜索请求。
type AnnSearchRequest struct {
	QueryVector []float32
	ObjectType  string // "" = 不限类型
	ScopeKey    string // "" = 不限 scope
	TopK        int    // 默认 milvusDefaultTopK
	EfSearch    int    // HNSW 搜索参数，默认 milvusDefaultEfSearch
}

// AnnHit 向量搜索命中结果。
type AnnHit struct {
	ObjectType string
	ObjectID   string
	Score      float32
}

// Search 执行 ANN 向量搜索。
func (mc *MilvusClient) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error) {
	topK := req.TopK
	if topK <= 0 {
		topK = milvusDefaultTopK
	}
	efSearch := req.EfSearch
	if efSearch <= 0 {
		efSearch = milvusDefaultEfSearch
	}

	sp, err := entity.NewIndexHNSWSearchParam(efSearch)
	if err != nil {
		return nil, fmt.Errorf("NewIndexHNSWSearchParam: %w", err)
	}

	expr := buildFilterExpr(req.ScopeKey, req.ObjectType)
	vectors := []entity.Vector{entity.FloatVector(req.QueryVector)}

	results, err := mc.milvus.Search(
		ctx,
		mc.collection,
		nil, // partition names
		expr,
		[]string{"object_type", "object_id"},
		vectors,
		"embedding",
		mc.metricType(),
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	var hits []AnnHit
	for _, result := range results {
		// result.Fields 包含 output fields
		var objectTypes []string
		var objectIDs []string
		for _, col := range result.Fields {
			switch col.Name() {
			case "object_type":
				if c, ok := col.(*entity.ColumnVarChar); ok {
					objectTypes = c.Data()
				}
			case "object_id":
				if c, ok := col.(*entity.ColumnVarChar); ok {
					objectIDs = c.Data()
				}
			}
		}
		for i := 0; i < result.ResultCount; i++ {
			hit := AnnHit{Score: result.Scores[i]}
			if i < len(objectTypes) {
				hit.ObjectType = objectTypes[i]
			}
			if i < len(objectIDs) {
				hit.ObjectID = objectIDs[i]
			}
			if hit.ObjectID != "" {
				hits = append(hits, hit)
			}
		}
	}
	return hits, nil
}

// MilvusInsertRequest 单条 embedding 写入请求。
type MilvusInsertRequest struct {
	ID          string
	ObjectType  string
	ObjectID    string
	ScopeType   string
	ScopeKey    string
	ContentText string
	Vector      []float32
}

// Insert 向 Milvus 写入一批 embedding。
// 若记录 ID 已存在会先 Delete 再 Insert（Upsert 语义）。
func (mc *MilvusClient) Insert(ctx context.Context, reqs []MilvusInsertRequest) error {
	if len(reqs) == 0 {
		return nil
	}

	ids := make([]string, len(reqs))
	objectTypes := make([]string, len(reqs))
	objectIDs := make([]string, len(reqs))
	scopeTypes := make([]string, len(reqs))
	scopeKeys := make([]string, len(reqs))
	contentTexts := make([]string, len(reqs))
	vecs := make([][]float32, len(reqs))

	dim := mc.cfg.Dim
	if dim <= 0 {
		dim = milvusDefaultDim
	}

	for i, r := range reqs {
		ids[i] = r.ID
		objectTypes[i] = r.ObjectType
		objectIDs[i] = r.ObjectID
		scopeTypes[i] = r.ScopeType
		scopeKeys[i] = r.ScopeKey
		contentTexts[i] = truncateStr(r.ContentText, milvusContentTextMaxLen)
		vecs[i] = r.Vector
	}

	// Upsert 语义：先删除已有 ID，再插入
	exprParts := make([]string, len(ids))
	for i, id := range ids {
		exprParts[i] = `"` + id + `"`
	}
	deleteExpr := "id in [" + strings.Join(exprParts, ",") + "]"
	// 忽略 delete 错误（记录可能不存在）
	_ = mc.milvus.Delete(ctx, mc.collection, "", deleteExpr)

	_, err := mc.milvus.Insert(ctx, mc.collection, "",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnVarChar("object_type", objectTypes),
		entity.NewColumnVarChar("object_id", objectIDs),
		entity.NewColumnVarChar("scope_type", scopeTypes),
		entity.NewColumnVarChar("scope_key", scopeKeys),
		entity.NewColumnVarChar("content_text", contentTexts),
		entity.NewColumnFloatVector("embedding", dim, vecs),
	)
	if err != nil {
		return fmt.Errorf("milvus insert: %w", err)
	}
	// 异步 flush：让数据在后台持久化，不阻塞写入路径
	go func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = mc.milvus.Flush(flushCtx, mc.collection, false)
	}()
	return nil
}

// DeleteByObjectID 删除指定对象的 embedding。
func (mc *MilvusClient) DeleteByObjectID(ctx context.Context, objectType, objectID string) error {
	expr := fmt.Sprintf(`object_type == "%s" && object_id == "%s"`, objectType, objectID)
	return mc.milvus.Delete(ctx, mc.collection, "", expr)
}

// Close 关闭 Milvus 连接。
func (mc *MilvusClient) Close() error {
	if mc.milvus != nil {
		return mc.milvus.Close()
	}
	return nil
}

// buildFilterExpr 根据 scopeKey 和 objectType 构造 Milvus 标量过滤表达式。
func buildFilterExpr(scopeKey, objectType string) string {
	var parts []string
	if scopeKey != "" {
		parts = append(parts, fmt.Sprintf(`scope_key == "%s"`, scopeKey))
	}
	if objectType != "" {
		parts = append(parts, fmt.Sprintf(`object_type == "%s"`, objectType))
	}
	return strings.Join(parts, " && ")
}

// truncateStr 截断字符串到 maxLen 个字节（按 rune 边界）。
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

