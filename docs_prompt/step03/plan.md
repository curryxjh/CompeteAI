# Plan: Milvus 混合检索实施计划

更新时间：2026-06-09

---

## 总体策略

**先搭基础设施 → 再实现存储层 → 再实现检索层 → 修复 Fallback → 接入验证**

分四个 Phase，各 Phase 独立可验证：

| Phase | 内容 | 预计工时 |
|---|---|---|
| P1 | 基础设施：Milvus docker-compose + 配置结构 + SDK 依赖 | 0.5 天 |
| P2 | 存储层：MilvusClient + EmbeddingStore 双写 | 1 天 |
| P3 | 检索层：MilvusRetriever + HybridRetriever + MySQLRetriever 修复 | 1.5 天 |
| P4 | 接入与测试：service.go 工厂 + bootstrap + 单元测试 | 1 天 |

---

## Phase 1：基础设施（0.5 天）

### P1-1 更新 docker-compose.yaml

在现有 `mysql`、`redis` 服务后追加 etcd、minio、milvus 三个服务（spec §2.1）。

注意事项：
- etcd 镜像指定 `v3.5.14`，避免与 Milvus 不兼容
- minio 镜像使用固定版本 `RELEASE.2024-05-10T01-41-38Z`
- milvus 通过 `depends_on` + `condition: service_healthy` 保证启动顺序
- 三个服务均加入 `compete-ai-net` 网络，与现有 api/worker 通信
- 添加三个 volume：`etcd_data`、`minio_data`、`milvus_data`

**API / Worker 服务** 中新增环境变量：
```yaml
- MILVUS_ADDRESS=milvus:19530
```

### P1-2 配置结构扩展

**文件**：`settings/settings.go`

在 `MemoryConfig` 末尾追加：
```go
Milvus *MilvusConfig `mapstructure:"milvus"`
```

新增 `MilvusConfig` 结构体（spec §3.1）。

在 `Init()` 函数末尾追加环境变量覆盖：
```go
if addr := os.Getenv("MILVUS_ADDRESS"); addr != "" {
    if Conf.MemoryConfig == nil { Conf.MemoryConfig = &MemoryConfig{} }
    if Conf.MemoryConfig.Milvus == nil { Conf.MemoryConfig.Milvus = &MilvusConfig{} }
    Conf.MemoryConfig.Milvus.Address = addr
}
```

**文件**：`config/dev.yaml`

在 `memory` 段末尾追加（spec §3.2）：
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

### P1-3 添加 Go SDK 依赖

```bash
cd /path/to/CompeteAI
go get github.com/milvus-io/milvus-sdk-go/v2@latest
go mod tidy
```

验证：`go build ./...` 无错误（此时新代码文件还未写，只是依赖先到位）。

---

## Phase 2：存储层（1 天）

### P2-1 实现 MilvusClient

**新建文件**：`internal/memory/milvus_client.go`

实现内容（按顺序）：

```go
package memory

import (
    "context"
    "fmt"

    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
    "CompeteAI/settings"
)

type MilvusClient struct {
    milvus     client.Client
    cfg        *settings.MilvusConfig
    collection string
}
```

**`NewMilvusClient`**：
1. 调用 `client.NewClient(ctx, client.Config{Address: cfg.Address})`
2. 成功后调用 `mc.ensureCollection(ctx)`

**`ensureCollection`**（幂等）：
1. `HasCollection` → 若不存在：`CreateCollection(schema)` → `CreateIndex("embedding", HNSW)`
2. `LoadCollection`（无论是否新建，都确保加载到内存）

**`buildSchema`**（spec §4.2 字段清单）：
```go
schema := &entity.Schema{
    CollectionName: mc.collection,
    Fields: []*entity.Field{
        {Name: "id",           DataType: entity.FieldTypeVarChar, PrimaryKey: true, AutoID: false, TypeParams: map[string]string{"max_length": "36"}},
        {Name: "object_type",  DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
        {Name: "object_id",    DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "36"}},
        {Name: "scope_type",   DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "32"}},
        {Name: "scope_key",    DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}},
        {Name: "content_text", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "2048"}},
        {Name: "embedding",    DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": strconv.Itoa(mc.cfg.Dim)}},
    },
}
```

**`Insert`**：
```go
func (mc *MilvusClient) Insert(ctx context.Context, req SaveEmbeddingRequest) error {
    // 构造各字段的 ColumnData
    // 调用 mc.milvus.Insert(ctx, mc.collection, "", columns...)
    // Flush 确保数据可搜索（生产可改为异步 flush）
}
```

**`Search`**（ANN 查询）：
```go
func (mc *MilvusClient) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error) {
    // 构造 HNSW SearchParam（efSearch=req.EfSearch 或默认 64）
    // 调用 milvus.Search()，output fields: object_type, object_id
    // 解析 SearchResult → []AnnHit
}
```

**`Delete`** / **`Close`** 方法。

### P2-2 实现 EmbeddingStore

**新建文件**：`internal/memory/embedding_store.go`

```go
type embeddingStore struct {
    dao      *dao.MemoryDao
    milvus   *MilvusClient       // nil = MySQL-only 模式
    repairCh chan string          // objectID 待补写队列
    once     sync.Once
}

func NewEmbeddingStore(dao *dao.MemoryDao, milvus *MilvusClient) EmbeddingStore {
    s := &embeddingStore{dao: dao, milvus: milvus, repairCh: make(chan string, 1000)}
    if milvus != nil {
        go s.runRepair()  // 启动后台补偿 goroutine
    }
    return s
}
```

**`Save`** 双写逻辑（spec §5.1）：
1. 写 MySQL `UpsertEmbedding`（失败则返回 error）
2. 写 Milvus `Insert`（失败则写 `repairCh`，不返回 error）

**`runRepair`** 后台补偿：
```go
func (s *embeddingStore) runRepair() {
    for objectID := range s.repairCh {
        for retry := 0; retry < 3; retry++ {
            emb, err := s.dao.GetEmbeddingByObjectID(ctx, objectID)
            if err != nil { break }
            req := toSaveRequest(emb)
            if err := s.milvus.Insert(ctx, req); err == nil { break }
            time.Sleep(time.Duration(1<<retry) * time.Second)  // 指数退避
        }
    }
}
```

**`RebuildMilvus`**：分页从 MySQL 读取全部 embedding，批量写入 Milvus（用于运维恢复）。

### P2-3 新增 DAO 方法

**文件**：`internal/repository/dao/memory.go`（或现有 MemoryDao 文件）

新增两个方法：
```go
// 按 objectIDs 批量查询 embedding（修复嵌套扫描）
func (d *MemoryDao) GetEmbeddingsByObjectIDs(ctx context.Context,
    objectType string, ids []string) (map[string][]float32, error)

// 按单个 objectID 查询 embedding（用于 repair）
func (d *MemoryDao) GetEmbeddingByObjectID(ctx context.Context,
    objectID string) (MemoryEmbeddingEntity, error)
```

---

## Phase 3：检索层（1.5 天）

### P3-1 实现 MilvusRetriever

**新建文件**：`internal/memory/milvus_retriever.go`

```go
type MilvusRetriever struct {
    client *MilvusClient
    cfg    *settings.MilvusConfig
}

func NewMilvusRetriever(c *MilvusClient, cfg *settings.MilvusConfig) *MilvusRetriever {
    return &MilvusRetriever{client: c, cfg: cfg}
}

// Search 执行向量 ANN 检索，返回 objectID 列表（按相似度降序）
func (r *MilvusRetriever) Search(ctx context.Context, req AnnSearchRequest) ([]AnnHit, error) {
    return r.client.Search(ctx, req)
}
```

### P3-2 实现 HybridRetriever

**新建文件**：`internal/memory/hybrid_retriever.go`

**`RetrieveFacts`** 实现步骤（spec §7.2）：

```go
func (r *HybridRetriever) RetrieveFacts(ctx context.Context, req RetrievalRequest) ([]FactHit, error) {
    // Step 1: 生成查询向量
    var qVec []float32
    if req.Query != "" {
        vecs, _ := r.embedder.Embed(ctx, []string{req.Query})
        if len(vecs) > 0 { qVec = vecs[0] }
    }

    // Step 2: Milvus ANN 召回 objectID 列表
    var milvusIDs []string
    milvusScores := map[string]float32{}
    if r.milvus != nil && qVec != nil {
        hits, _ := r.milvus.Search(ctx, AnnSearchRequest{
            QueryVector: qVec,
            ObjectType:  "fact",
            ScopeKey:    primaryScopeKey(req.ScopeIDs),
            TopK:        50,
        })
        for _, h := range hits {
            milvusIDs = append(milvusIDs, h.ObjectID)
            milvusScores[h.ObjectID] = h.Score
        }
    }

    // Step 3: MySQL 关键词召回
    kwRows, _ := r.mysql.dao.SearchFactsBySubject(ctx, req.ScopeIDs, req.Competitors, req.Limit*3)
    kwIDs := make([]string, len(kwRows))
    kwMap  := map[string]dao.MemoryFactEntity{}
    for i, row := range kwRows {
        kwIDs[i] = row.ID
        kwMap[row.ID] = row
    }

    // Step 4: 批量拉取 Milvus 召回的 Fact 完整数据（IN 查询）
    milvusFacts, _ := r.mysql.dao.GetFactsByIDs(ctx, milvusIDs)

    // Step 5: Union 合并（去重以 objectID 为 key）
    allFacts := map[string]dao.MemoryFactEntity{}
    for _, f := range milvusFacts { allFacts[f.ID] = f }
    for id, f := range kwMap      { allFacts[id] = f }

    // Step 6: 评分（milvus score 优先，无则用关键词分）
    var hits []FactHit
    milvusRank := rankMap(milvusIDs)
    kwRank     := rankMap(kwIDs)
    for id, row := range allFacts {
        f := factFromEntity(row)
        rrfScore := rrfScore(milvusRank[id], kwRank[id], 60)
        f.FinalScore = rrfScore
        semantic := float64(milvusScores[id])
        hits = append(hits, FactHit{Fact: f, SemanticScore: semantic})
    }

    // Step 7: 排序、截断
    hits = sortFactHits(hits)
    if len(hits) > req.Limit { hits = hits[:req.Limit] }
    return hits, nil
}
```

**辅助函数**：
```go
func rankMap(ids []string) map[string]int {
    m := map[string]int{}
    for i, id := range ids { m[id] = i }
    return m
}

// rrfScore：两路 rank 均为 -1 表示不在该路中，用 len(list)+1 替代
func rrfScore(mRank, kRank, k int) float64 {
    score := 0.0
    if mRank >= 0 { score += 1.0 / float64(k+mRank+1) }
    if kRank >= 0 { score += 1.0 / float64(k+kRank+1) }
    return score
}
```

**`RetrieveEvidence`** 类似逻辑（object_type="chunk"），不赘述。

**`RetrieveEpisodes`** 直接委托 `mysql.RetrieveEpisodes`（episodes 数量少，keyword 已够用）。

### P3-3 修复 MySQLRetriever（嵌套扫描 Bug）

**文件**：`internal/memory/retriever.go`

将 `RetrieveFacts` 中的嵌套扫描替换为批量 IN 查询：

```go
// 修复前（删除）
embRows, _ := r.dao.ListAllEmbeddings(ctx, "fact", 500)
for _, er := range embRows {
    if er.ObjectID == f.ID { ... }
}

// 修复后
factIDs := make([]string, len(rows))
for i, row := range rows { factIDs[i] = row.ID }
embMap, _ := r.dao.GetEmbeddingsByObjectIDs(ctx, "fact", factIDs)

for _, row := range rows {
    f := factFromEntity(row)
    semantic := KeywordScore(req.Query, f.Summary+" "+f.ObjectText)
    if vec, ok := embMap[f.ID]; ok && qVec != nil {
        semantic = CosineSimilarity(qVec, vec)
    }
    // ... 后续评分逻辑不变 ...
}
```

---

## Phase 4：接入与测试（1 天）

### P4-1 更新 service.go 工厂

**文件**：`internal/memory/service.go`

按照 spec §9 重写 `NewService`：
- 根据 `cfg.Milvus.Enabled` 决定是否初始化 `MilvusClient`
- 初始化失败时降级（warn log + 继续使用 MySQLRetriever）
- 将 `embStore` 注入 `service`，后续保存 embedding 通过 `embStore.Save` 而非直接 `dao`

**`MemoryService` 接口新增 `Close() error`**：
```go
type MemoryService interface {
    // ... 现有方法 ...
    Close() error
}
// NoopService.Close() 返回 nil
// service.Close() 调用 milvusClient.Close()（若存在）
```

### P4-2 更新 bootstrap.go

**文件**：`internal/app/bootstrap.go`

在 shutdown handler 中调用 `deps.MemoryService.Close()`。

检查 `BuildSharedDeps` 中 `NewService` 的调用，确保传入 `cfg.MemoryConfig`（含 Milvus 配置）。

### P4-3 新增 DAO 方法实现

**文件**：`internal/repository/dao/memory.go`

实现 P2-3 定义的两个新方法：

```go
func (d *MemoryDao) GetEmbeddingsByObjectIDs(ctx context.Context,
    objectType string, ids []string) (map[string][]float32, error) {
    
    if len(ids) == 0 { return nil, nil }
    var rows []MemoryEmbeddingEntity
    err := d.db.WithContext(ctx).
        Where("object_type = ? AND object_id IN ?", objectType, ids).
        Find(&rows).Error
    if err != nil { return nil, err }
    
    result := make(map[string][]float32, len(rows))
    for _, r := range rows {
        var vec []float32
        _ = json.Unmarshal([]byte(r.EmbeddingJSON), &vec)
        result[r.ObjectID] = vec
    }
    return result, nil
}
```

### P4-4 单元测试

**新建文件**：`internal/memory/hybrid_retriever_test.go`

实现 spec §13 的 6 个测试用例：

1. `TestHybridRetriever_MilvusFallback` — milvus nil，委托 mysql
2. `TestHybridRetriever_MergeDedup` — 合并去重验证
3. `TestRRF_Scoring` — RRF 分值正确性
4. `TestMySQLRetriever_NoBulkScan` — 批量 IN 替代嵌套扫描
5. `TestEmbeddingStore_MilvusWriteFailure` — 主链路成功，repair 队列记录
6. `TestBuildFilterExpr` — 过滤表达式生成

### P4-5 集成验证

```bash
# 1. 启动基础设施
docker-compose up mysql redis etcd minio milvus -d

# 2. 等待 milvus 健康（约 30s）
docker-compose ps

# 3. 编译
go build ./...

# 4. 单元测试
go test ./internal/memory/... -v

# 5. 跑完整任务，观察日志
# 期望日志：
# [memory] milvus client initialized, collection=compete_ai_embeddings
# [memory] embedding saved: objectType=fact objectID=xxx milvus=ok
# [memory] hybrid retrieve: milvus_hits=23 keyword_hits=15 merged=28
```

---

## 风险与应对

| 风险 | 应对 |
|---|---|
| Milvus 启动慢（30~60s） | docker-compose healthcheck 确保 API/Worker 等待 milvus healthy 后启动 |
| milvus-sdk-go 与现有 grpc 版本冲突 | 引入后立即 `go build`；若冲突，降级 sdk 版本或 replace |
| Milvus docker image 在低配机器 OOM | Milvus standalone 最低 8GB RAM；开发机内存不足时可 `enabled: false` 退回 MySQL 模式 |
| HybridRetriever 延迟增加 | Milvus ANN 单次 <20ms；MySQL IN 查询按 ID <5ms；总增量可接受 |
| 双写不一致（Milvus 补写失败） | repair goroutine 保证最终一致；可通过 `RebuildMilvus` 手动触发全量修复 |
| milvus-sdk-go 的 gRPC 连接泄漏 | `Close()` 在 shutdown hook 中调用，已覆盖 |

---

## 各 Phase 交付物汇总

| Phase | 文件 | 变更 |
|---|---|---|
| P1 | `docker-compose.yaml` | 新增 etcd + minio + milvus 服务 |
| P1 | `settings/settings.go` | 新增 MilvusConfig，MemoryConfig 引用 |
| P1 | `config/dev.yaml` | 新增 milvus 配置段 |
| P1 | `go.mod` / `go.sum` | 新增 milvus-sdk-go 依赖 |
| P2 | `internal/memory/milvus_client.go` | 新建 |
| P2 | `internal/memory/embedding_store.go` | 新建 |
| P2 | `internal/repository/dao/memory.go` | 新增 2 个查询方法 |
| P3 | `internal/memory/milvus_retriever.go` | 新建 |
| P3 | `internal/memory/hybrid_retriever.go` | 新建 |
| P3 | `internal/memory/retriever.go` | 修复嵌套扫描 Bug |
| P4 | `internal/memory/service.go` | 工厂方法接入 Milvus |
| P4 | `internal/app/bootstrap.go` | Close hook |
| P4 | `internal/memory/hybrid_retriever_test.go` | 新建，6 个测试 |
