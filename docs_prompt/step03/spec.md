# Spec: Milvus 混合检索长期记忆系统技术规格

更新时间：2026-06-09

---

## 1. 整体架构

```
写入路径
  Agent/Service
      │
      ▼
  EmbeddingStore.Save(ctx, objectType, objectID, scopeType, scopeKey, text)
      ├─► MySQL: memory_embeddings (关系元数据 + 向量 JSON，用于重建)
      └─► Milvus: compete_ai_embeddings collection (向量索引，用于 ANN 检索)

读取路径
  MemoryService.Retrieve(ctx, req)
      │
      ▼
  HybridRetriever
      ├─► MilvusRetriever.AnnSearch(scopeFilter + queryVec) → []objectID
      ├─► MySQLRetriever.FetchByIDs(objectIDs)           → []Fact/Chunk
      └─► Merger.RRF(milvusHits, keywordHits)            → []Hit(sorted)
```

---

## 2. docker-compose 配置

### 2.1 新增服务

Milvus Standalone 依赖 etcd 和 MinIO，需同时声明：

```yaml
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.14
    container_name: compete-ai-etcd
    environment:
      - ETCD_AUTO_COMPACTION_MODE=revision
      - ETCD_AUTO_COMPACTION_RETENTION=1000
      - ETCD_QUOTA_BACKEND_BYTES=4294967296
      - ETCD_SNAPSHOT_COUNT=50000
    volumes:
      - etcd_data:/etcd
    command: >
      etcd
      --advertise-client-urls=http://127.0.0.1:2379
      --listen-client-urls=http://0.0.0.0:2379
      --data-dir=/etcd
    healthcheck:
      test: ["CMD", "etcdctl", "endpoint", "health"]
      interval: 30s
      timeout: 20s
      retries: 3
    networks:
      - compete-ai-net

  minio:
    image: minio/minio:RELEASE.2024-05-10T01-41-38Z
    container_name: compete-ai-minio
    environment:
      MINIO_ACCESS_KEY: minioadmin
      MINIO_SECRET_KEY: minioadmin
    volumes:
      - minio_data:/data
    command: minio server /data --console-address ":9001"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3
    networks:
      - compete-ai-net

  milvus:
    image: milvusdb/milvus:v2.4.9
    container_name: compete-ai-milvus
    command: ["milvus", "run", "standalone"]
    environment:
      ETCD_ENDPOINTS: etcd:2379
      MINIO_ADDRESS: minio:9000
    volumes:
      - milvus_data:/var/lib/milvus
    ports:
      - "19530:19530"
      - "9091:9091"
    depends_on:
      etcd:
        condition: service_healthy
      minio:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9091/healthz"]
      interval: 30s
      timeout: 20s
      retries: 5
    networks:
      - compete-ai-net

volumes:
  etcd_data:
  minio_data:
  milvus_data:
```

### 2.2 API / Worker 服务新增环境变量

```yaml
environment:
  - MILVUS_ADDRESS=milvus:19530
```

---

## 3. 配置结构

### 3.1 settings/settings.go 新增

```go
type MilvusConfig struct {
    Address          string `mapstructure:"address"`           // "localhost:19530"
    CollectionPrefix string `mapstructure:"collection_prefix"` // "compete_ai"
    Dim              int    `mapstructure:"dim"`               // 1024（与 VolcanoEmbedder 一致）
    IndexType        string `mapstructure:"index_type"`        // "HNSW"（默认）
    MetricType       string `mapstructure:"metric_type"`       // "COSINE"（默认）
    HNSWM            int    `mapstructure:"hnsw_m"`            // 16（默认）
    HNSWEfConstruct  int    `mapstructure:"hnsw_ef_construct"` // 200（默认）
    Enabled          bool   `mapstructure:"enabled"`           // 总开关
}

// MemoryConfig 新增字段
type MemoryConfig struct {
    // ... 现有字段 ...
    Milvus *MilvusConfig `mapstructure:"milvus"`
}
```

### 3.2 config/dev.yaml 新增

```yaml
memory:
  milvus:
    enabled: true
    address: "localhost:19530"
    collection_prefix: "compete_ai"
    dim: 1024
    index_type: "HNSW"
    metric_type: "COSINE"
    hnsw_m: 16
    hnsw_ef_construct: 200
```

---

## 4. Milvus Collection Schema

### 4.1 Collection 名称

```
{collection_prefix}_embeddings   →   compete_ai_embeddings
```

### 4.2 字段定义

| 字段名 | 类型 | 说明 |
|---|---|---|
| `id` | VARCHAR(36), primary key | UUID，与 MySQL memory_embeddings.id 对应 |
| `object_type` | VARCHAR(64) | "fact" / "chunk" / "episode" |
| `object_id` | VARCHAR(36) | 对应 MySQL 中记录的 ID |
| `scope_type` | VARCHAR(32) | "user" / "workspace" / "project" |
| `scope_key` | VARCHAR(128) | 如 "user:42" / "project:abc" |
| `content_text` | VARCHAR(2048) | 原文摘要（截断），用于关键词回退 |
| `embedding` | FloatVector(dim) | 语义向量 |

### 4.3 索引配置

```go
// HNSW 向量索引
IndexParam{
    IndexType:  "HNSW",
    MetricType: "COSINE",
    ExtraParams: map[string]any{
        "M":              16,
        "efConstruction": 200,
    },
}

// 标量字段索引（用于 pre-filter）
// object_type: STL_SORT
// scope_type + scope_key: STL_SORT（分别建）
```

### 4.4 Collection 加载策略

- 启动时 `AutoCreate`（若不存在则创建 + 建索引 + 加载到内存）
- 使用 `LoadCollection` 确保集合在内存中，支持搜索

---

## 5. EmbeddingStore 接口

**文件**：`internal/memory/embedding_store.go`

```go
// EmbeddingStore 封装双写逻辑：MySQL 持久化 + Milvus 向量索引。
type EmbeddingStore interface {
    // Save 保存文本的向量表示（双写）
    Save(ctx context.Context, req SaveEmbeddingRequest) error
    // DeleteByObject 删除特定对象的 embedding（如 fact 被废弃）
    DeleteByObject(ctx context.Context, objectType, objectID string) error
    // RebuildMilvus 从 MySQL 重建 Milvus collection（用于运维恢复）
    RebuildMilvus(ctx context.Context, scopeKey string) error
}

type SaveEmbeddingRequest struct {
    ID          string    // UUID
    ObjectType  string    // "fact" / "chunk" / "episode"
    ObjectID    string    // 对应实体的 ID
    ScopeType   string
    ScopeKey    string
    ContentText string    // 原文（截断到 2048 字符）
    Vector      []float32 // 已计算好的向量
}
```

### 5.1 双写实现策略

```go
func (s *embeddingStore) Save(ctx context.Context, req SaveEmbeddingRequest) error {
    // Step 1：写 MySQL（主链路，失败则整体失败）
    if err := s.dao.UpsertEmbedding(ctx, toEntity(req)); err != nil {
        return fmt.Errorf("mysql embedding write: %w", err)
    }
    // Step 2：写 Milvus（辅助链路，失败只记录日志，不阻断主流程）
    if s.milvus != nil {
        if err := s.milvus.Insert(ctx, req); err != nil {
            log.Warnf("milvus embedding insert failed (objectID=%s): %v", req.ObjectID, err)
            // 后台异步补偿（下节说明）
            s.scheduleRepair(req.ObjectID)
        }
    }
    return nil
}
```

### 5.2 异步补偿（Repair）

- `embeddingStore` 内部维护一个 `repairCh chan string`（objectID 队列）
- 后台 goroutine 消费队列，读 MySQL 重试写 Milvus，最多 3 次，间隔指数退避
- 进程重启后遗失队列时：可通过 `RebuildMilvus` 管理接口从 MySQL 全量重建

---

## 6. MilvusRetriever

**文件**：`internal/memory/milvus_retriever.go`

```go
type MilvusRetriever struct {
    client *MilvusClient
    cfg    *settings.MilvusConfig
}

type AnnSearchRequest struct {
    QueryVector []float32
    ObjectType  string   // "" = 不限
    ScopeType   string
    ScopeKey    string
    TopK        int      // 默认 50
    EfSearch    int      // HNSW 搜索参数，默认 64
}

type AnnHit struct {
    ObjectType string
    ObjectID   string
    Score      float32 // cosine 相似度 [0, 1]
}

func (r *MilvusRetriever) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error)
```

### 6.1 Search 实现

```go
func (r *MilvusRetriever) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error) {
    // 构造 scalar pre-filter
    // expr 示例: scope_key == "user:42" && object_type == "fact"
    expr := buildFilterExpr(req.ScopeType, req.ScopeKey, req.ObjectType)
    
    param := entity.NewIndexHNSWSearchParam(req.EfSearch)
    results, err := r.client.milvus.Search(
        ctx,
        r.collectionName(),
        nil,           // partition names
        expr,
        []string{"object_type", "object_id"}, // output fields
        []entity.Vector{entity.FloatVector(req.QueryVector)},
        "embedding",
        entity.COSINE,
        req.TopK,
        param,
    )
    // 解析 results → []AnnHit
}

func buildFilterExpr(scopeType, scopeKey, objectType string) string {
    parts := []string{}
    if scopeKey != "" {
        parts = append(parts, fmt.Sprintf(`scope_key == "%s"`, scopeKey))
    }
    if objectType != "" {
        parts = append(parts, fmt.Sprintf(`object_type == "%s"`, objectType))
    }
    return strings.Join(parts, " && ")
}
```

---

## 7. HybridRetriever

**文件**：`internal/memory/hybrid_retriever.go`

### 7.1 接口

```go
// HybridRetriever 结合 Milvus ANN + MySQL 关键词，实现混合检索。
// 实现 Retriever 接口，对上层透明。
type HybridRetriever struct {
    milvus   *MilvusRetriever
    mysql    *mysqlRetriever
    embedder Embedder
}
```

### 7.2 RetrieveFacts 流程

```
1. 用 embedder.Embed(query) 获取查询向量 qVec

2. Milvus 路：
   milvusHits = milvus.Search(scope_key, object_type="fact", qVec, topK=50)
   → 得到 [(objectID, score), ...]

3. MySQL 关键词路：
   keywordFacts = dao.SearchFactsBySubject(scopeIDs, competitors, limit*3)
   → 关键词匹配的候选集

4. 合并：
   a. 以 milvusHits 的 objectID 集合为主，从 MySQL 拉取完整 Fact 数据
   b. 将 keywordFacts 中不在 milvusHits 的条目补入（union）
   c. 为每条 Fact 赋 semanticScore：
      - 在 milvusHits 中 → 使用 Milvus cosine score
      - 仅在 keyword 路 → 使用 KeywordScore

5. RRF 重排（Reciprocal Rank Fusion）：
   score_rrf = Σ 1/(k + rank_i)   k=60（标准参数）

6. 截断到 limit，返回
```

### 7.3 RRF 实现

```go
// RRF 将多路排序结果合并为统一评分
// k=60 是标准参数，对排名靠后的结果自动衰减
func rrf(lists [][]string, k int) map[string]float64 {
    scores := map[string]float64{}
    for _, list := range lists {
        for rank, id := range list {
            scores[id] += 1.0 / float64(k + rank + 1)
        }
    }
    return scores
}
```

### 7.4 Fallback 策略

```go
// 若 Milvus 不可用（连接失败 / 未启用），自动退回 MySQLRetriever
func (r *HybridRetriever) RetrieveFacts(ctx context.Context, req RetrievalRequest) ([]FactHit, error) {
    if r.milvus == nil {
        return r.mysql.RetrieveFacts(ctx, req)
    }
    // ... hybrid logic ...
}
```

---

## 8. 修复 MySQLRetriever（作为 Fallback）

当前 `RetrieveFacts` 嵌套全表扫描问题，在此次改版中一并修复：

```go
// 修复前：对每个 fact 全扫 500 条 embedding
embRows, _ := r.dao.ListAllEmbeddings(ctx, "fact", 500)

// 修复后：按 objectIDs 批量查询
factIDs := make([]string, len(rows))
for i, row := range rows { factIDs[i] = row.ID }
embMap, _ := r.dao.GetEmbeddingsByObjectIDs(ctx, "fact", factIDs) // 批量 IN 查询
```

**新增 DAO 方法**：`GetEmbeddingsByObjectIDs(ctx, objectType string, ids []string) (map[string][]float32, error)`

---

## 9. MemoryService 工厂更新

**文件**：`internal/memory/service.go`

```go
func NewService(dbDao *dao.MemoryDao, cfg *settings.MemoryConfig) MemoryService {
    if cfg == nil || !cfg.Enabled {
        return NoopService{}
    }
    
    // 构建 Embedder
    var emb Embedder
    if cfg.Embedder != nil && cfg.Embedder.Type == "volcano" {
        emb = NewVolcanoEmbedder(cfg.Embedder)
    } else {
        emb = NewHashEmbedder(hashEmbedDim)
    }
    
    // 构建 EmbeddingStore（双写）
    var milvusClient *MilvusClient
    if cfg.Milvus != nil && cfg.Milvus.Enabled {
        mc, err := NewMilvusClient(cfg.Milvus)
        if err != nil {
            log.Warnf("milvus client init failed, fallback to mysql-only: %v", err)
        } else {
            milvusClient = mc
        }
    }
    embStore := NewEmbeddingStore(dbDao, milvusClient)
    
    // 构建 Retriever
    var ret Retriever
    mysqlRet := NewMySQLRetriever(dbDao, emb)
    if milvusClient != nil {
        milvusRet := NewMilvusRetriever(milvusClient, cfg.Milvus)
        ret = NewHybridRetriever(milvusRet, mysqlRet, emb)
    } else {
        ret = mysqlRet
    }
    
    return &service{
        dao:        dbDao,
        embedder:   emb,
        embStore:   embStore,
        retriever:  ret,
        extractor:  NewExtractor(),
        assembler:  NewAssembler(),
        governance: NewGovernance(dbDao),
    }
}
```

---

## 10. MilvusClient 生命周期管理

**文件**：`internal/memory/milvus_client.go`

```go
type MilvusClient struct {
    milvus     client.Client   // milvus-sdk-go v2
    cfg        *settings.MilvusConfig
    collection string          // 完整 collection 名称
}

func NewMilvusClient(cfg *settings.MilvusConfig) (*MilvusClient, error) {
    c, err := client.NewClient(ctx, client.Config{Address: cfg.Address})
    if err != nil { return nil, err }
    mc := &MilvusClient{milvus: c, cfg: cfg, collection: cfg.CollectionPrefix + "_embeddings"}
    if err := mc.ensureCollection(context.Background()); err != nil { return nil, err }
    return mc, nil
}

// ensureCollection 幂等创建 collection + 索引 + 加载
func (mc *MilvusClient) ensureCollection(ctx context.Context) error {
    exists, _ := mc.milvus.HasCollection(ctx, mc.collection)
    if !exists {
        schema := mc.buildSchema()
        mc.milvus.CreateCollection(ctx, schema, 2) // shardNum=2
        mc.milvus.CreateIndex(ctx, mc.collection, "embedding", mc.buildIndex(), false)
    }
    mc.milvus.LoadCollection(ctx, mc.collection, false)
    return nil
}

func (mc *MilvusClient) Close() error { return mc.milvus.Close() }
```

---

## 11. Bootstrap 接入

**文件**：`internal/app/bootstrap.go`

在 `BuildSharedDeps` 的 `MemoryService` 初始化阶段，`NewService` 已内部处理 Milvus Client 的创建，Bootstrap 无需额外修改。

但需在应用退出时关闭 Milvus 连接：

```go
// 在 Server/Worker 的 shutdown hook 中
if deps.MemoryService != nil {
    deps.MemoryService.Close()
}
```

在 `MemoryService` 接口新增 `Close() error`，`NoopService` 返回 `nil`，`service` 委托给 `MilvusClient.Close()`。

---

## 12. 依赖管理

### 12.1 新增 Go 依赖

```bash
go get github.com/milvus-io/milvus-sdk-go/v2@latest
```

`milvus-sdk-go/v2` 引入以下间接依赖（无需手动管理）：
- `google.golang.org/grpc`
- `github.com/grpc-ecosystem/go-grpc-middleware`

### 12.2 go.mod 说明

`milvus-sdk-go/v2` 使用 gRPC 协议与 Milvus 通信，与现有 MySQL/Redis 依赖无冲突。

---

## 13. 单元测试规格

### 13.1 `TestHybridRetriever_MilvusFallback`

- 场景：`milvus=nil`，`HybridRetriever` 应委托 `MySQLRetriever`
- 验证：MySQL Retriever 的 `RetrieveFacts` 被调用

### 13.2 `TestHybridRetriever_MergeDedup`

- 场景：Milvus 返回 IDs [A, B, C]，MySQL 关键词返回 [B, C, D]
- 验证：合并后结果包含 [A, B, C, D]，无重复

### 13.3 `TestRRF_Scoring`

- 场景：两路排序 [A, B, C] 和 [C, A, D]
- 验证：C 的 RRF 分数高于仅在单路出现的 B 和 D

### 13.4 `TestMySQLRetriever_NoBulkScan`

- 场景：Mock DAO，`SearchFactsBySubject` 返回 5 条 fact
- 验证：`GetEmbeddingsByObjectIDs` 被调用 1 次（批量 IN），而非 5 次

### 13.5 `TestEmbeddingStore_MilvusWriteFailure`

- 场景：MySQL 写入成功，Milvus 写入返回 error
- 验证：`Save` 返回 nil（主链路成功），repair 队列中有 objectID

### 13.6 `TestBuildFilterExpr`

- 验证各组合下的 Milvus 过滤表达式生成正确性
