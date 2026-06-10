# Checklist: Milvus 混合检索实施清单

更新时间：2026-06-09

---

## Phase 1：基础设施

### P1-1 docker-compose.yaml 更新

- [ ] 新增 `etcd` 服务：
  - [ ] 镜像：`quay.io/coreos/etcd:v3.5.14`
  - [ ] 环境变量：`ETCD_AUTO_COMPACTION_MODE=revision`，`ETCD_QUOTA_BACKEND_BYTES=4294967296`
  - [ ] healthcheck：`etcdctl endpoint health`
  - [ ] volume：`etcd_data:/etcd`
  - [ ] 网络：`compete-ai-net`

- [ ] 新增 `minio` 服务：
  - [ ] 镜像：`minio/minio:RELEASE.2024-05-10T01-41-38Z`
  - [ ] 环境变量：`MINIO_ACCESS_KEY=minioadmin`，`MINIO_SECRET_KEY=minioadmin`
  - [ ] healthcheck：`curl -f http://localhost:9000/minio/health/live`
  - [ ] volume：`minio_data:/data`
  - [ ] 网络：`compete-ai-net`

- [ ] 新增 `milvus` 服务：
  - [ ] 镜像：`milvusdb/milvus:v2.4.9`
  - [ ] 命令：`milvus run standalone`
  - [ ] 环境变量：`ETCD_ENDPOINTS=etcd:2379`，`MINIO_ADDRESS=minio:9000`
  - [ ] 端口映射：`19530:19530`，`9091:9091`
  - [ ] depends_on：etcd（healthy），minio（healthy）
  - [ ] healthcheck：`curl -f http://localhost:9091/healthz`
  - [ ] volume：`milvus_data:/var/lib/milvus`
  - [ ] 网络：`compete-ai-net`

- [ ] `api` 和 `worker` 服务 `environment` 新增：`MILVUS_ADDRESS=milvus:19530`
- [ ] `volumes` 段新增：`etcd_data`，`minio_data`，`milvus_data`
- [ ] 验证：`docker-compose up etcd minio milvus -d && docker-compose ps` 三个服务均 healthy

### P1-2 配置结构扩展

- [ ] `settings/settings.go` 新增 `MilvusConfig` 结构体：
  - [ ] 字段：`Address`, `CollectionPrefix`, `Dim`, `IndexType`, `MetricType`, `HNSWM`, `HNSWEfConstruct`, `Enabled`
  - [ ] 所有字段有 `mapstructure` tag 和合理默认值注释

- [ ] `MemoryConfig` 末尾新增：`Milvus *MilvusConfig mapstructure:"milvus"`

- [ ] `settings.Init()` 末尾添加环境变量覆盖逻辑（`MILVUS_ADDRESS`）

- [ ] `config/dev.yaml` 的 `memory` 段新增 `milvus` 子配置：
  - [ ] `enabled: true`
  - [ ] `address: "localhost:19530"`
  - [ ] `collection_prefix: "compete_ai"`
  - [ ] `dim: 1024`
  - [ ] `index_type: "HNSW"`，`metric_type: "COSINE"`
  - [ ] `hnsw_m: 16`，`hnsw_ef_construct: 200`

### P1-3 添加 Go SDK 依赖

- [ ] 执行：`go get github.com/milvus-io/milvus-sdk-go/v2@latest`
- [ ] 执行：`go mod tidy`
- [ ] `go build ./...` 无编译错误（此时还没有新代码，只是依赖到位）
- [ ] 确认 `go.mod` 中有 `github.com/milvus-io/milvus-sdk-go/v2` 条目

---

## Phase 2：存储层

### P2-1 实现 MilvusClient

- [ ] 新建 `internal/memory/milvus_client.go`

- [ ] `MilvusClient` 结构体：`milvus client.Client`，`cfg *settings.MilvusConfig`，`collection string`

- [ ] `NewMilvusClient(cfg) (*MilvusClient, error)` 实现：
  - [ ] `client.NewClient(ctx, Config{Address: cfg.Address})`
  - [ ] 调用 `ensureCollection`
  - [ ] 连接失败返回 error

- [ ] `ensureCollection(ctx)` 实现：
  - [ ] `HasCollection` → 不存在则 `CreateCollection`
  - [ ] `CreateIndex("embedding", HNSW, {M:cfg.HNSWM, efConstruction:cfg.HNSWEfConstruct})`
  - [ ] `CreateIndex("object_type", STL_SORT, nil)`
  - [ ] `CreateIndex("scope_key", STL_SORT, nil)`
  - [ ] `LoadCollection`（等待 loaded）

- [ ] `buildSchema()` 实现：按 spec §4.2 定义 7 个字段（id/object_type/object_id/scope_type/scope_key/content_text/embedding）

- [ ] `Insert(ctx, SaveEmbeddingRequest) error` 实现：
  - [ ] 构造各字段 `entity.NewColumnVarChar` / `entity.NewColumnFloatVector`
  - [ ] `milvus.Insert` 后调用 `milvus.Flush`

- [ ] `Search(ctx, AnnSearchRequest) ([]AnnHit, error)` 实现：
  - [ ] 构造 `buildFilterExpr(scopeType, scopeKey, objectType)`
  - [ ] `entity.NewIndexHNSWSearchParam(efSearch)`
  - [ ] 调用 `milvus.Search`，output fields：`object_type`，`object_id`
  - [ ] 解析 SearchResult → `[]AnnHit{ObjectType, ObjectID, Score}`

- [ ] `Delete(ctx, objectType, objectID string) error` 实现

- [ ] `Close() error` 实现

- [ ] `buildFilterExpr` 辅助函数：支持 scope_key、object_type 组合过滤

### P2-2 实现 EmbeddingStore

- [ ] 新建 `internal/memory/embedding_store.go`

- [ ] `EmbeddingStore` 接口定义：`Save`，`DeleteByObject`，`RebuildMilvus`

- [ ] `SaveEmbeddingRequest` 结构体：ID, ObjectType, ObjectID, ScopeType, ScopeKey, ContentText, Vector

- [ ] `embeddingStore` 结构体：`dao`，`milvus`，`repairCh chan string`

- [ ] `NewEmbeddingStore(dao, milvus)` 工厂：
  - [ ] milvus != nil 时启动 `go s.runRepair()`

- [ ] `Save(ctx, req)` 实现（spec §5.1）：
  - [ ] MySQL `UpsertEmbedding`（失败返回 error）
  - [ ] Milvus `Insert`（失败写 `repairCh`，不返回 error）
  - [ ] 写 repairCh 时用 non-blocking select，避免阻塞（channel 满则直接丢弃 + warn log）

- [ ] `runRepair()` 实现（spec §5.2）：
  - [ ] 消费 `repairCh`，读 MySQL → 重试写 Milvus
  - [ ] 最多 3 次，指数退避（1s, 2s, 4s）

- [ ] `RebuildMilvus(ctx, scopeKey)` 实现：
  - [ ] 分页（每批 100）从 MySQL 读取 embeddings
  - [ ] 批量写入 Milvus
  - [ ] 打印进度日志

- [ ] `DeleteByObject(ctx, objectType, objectID)` 实现

### P2-3 新增 DAO 方法

- [ ] `internal/repository/dao/memory.go`（定位到 `MemoryDao`）

- [ ] 新增 `GetEmbeddingsByObjectIDs(ctx, objectType string, ids []string) (map[string][]float32, error)`：
  - [ ] ids 为空时返回 nil, nil
  - [ ] `WHERE object_type = ? AND object_id IN ?` 批量查询
  - [ ] 解析 `EmbeddingJSON` → `[]float32` 映射到 map[objectID]vec

- [ ] 新增 `GetEmbeddingByObjectID(ctx, objectID string) (MemoryEmbeddingEntity, error)`:
  - [ ] `WHERE object_id = ?` 单条查询

- [ ] 新增 `GetFactsByIDs(ctx, ids []string) ([]MemoryFactEntity, error)`（HybridRetriever 需要）：
  - [ ] ids 为空时返回 nil, nil
  - [ ] `WHERE id IN ?` 批量查询

- [ ] `go build ./...` 无错误

---

## Phase 3：检索层

### P3-1 实现 MilvusRetriever

- [ ] 新建 `internal/memory/milvus_retriever.go`

- [ ] `AnnSearchRequest` 结构体（QueryVector, ObjectType, ScopeType, ScopeKey, TopK, EfSearch）

- [ ] `AnnHit` 结构体（ObjectType, ObjectID string, Score float32）

- [ ] `MilvusRetriever` 结构体 + `NewMilvusRetriever` 工厂

- [ ] `Search(ctx, AnnSearchRequest) ([]AnnHit, error)` 方法：委托 `client.Search`

### P3-2 实现 HybridRetriever

- [ ] 新建 `internal/memory/hybrid_retriever.go`

- [ ] `HybridRetriever` 结构体（milvus *MilvusRetriever, mysql *mysqlRetriever, embedder Embedder）

- [ ] `NewHybridRetriever(milvus, mysql, embedder)` 工厂

- [ ] `RetrieveFacts(ctx, req) ([]FactHit, error)` 实现（plan §P3-2 完整流程）：
  - [ ] 生成查询向量（`embedder.Embed`）
  - [ ] Milvus ANN 召回（object_type="fact"，scope_key 过滤）
  - [ ] MySQL 关键词召回（`SearchFactsBySubject`）
  - [ ] 批量拉取 Milvus 召回的 Fact 完整数据（`GetFactsByIDs`）
  - [ ] Union 合并去重
  - [ ] RRF 评分
  - [ ] 排序截断
  - [ ] Milvus 为 nil 时直接委托 `mysql.RetrieveFacts`

- [ ] `RetrieveEvidence(ctx, req) ([]EvidenceHit, error)` 实现：
  - [ ] 与 RetrieveFacts 类似，object_type="chunk"
  - [ ] Milvus 为 nil 时直接委托 `mysql.RetrieveEvidence`

- [ ] `RetrieveEpisodes(ctx, req) ([]EpisodeHit, error)` 实现：
  - [ ] 直接委托 `mysql.RetrieveEpisodes`（episodes 数量少）

- [ ] `rrfScore(mRank, kRank, k int) float64` 辅助函数
- [ ] `rankMap(ids []string) map[string]int` 辅助函数
- [ ] `primaryScopeKey(scopeIDs []string) string` 辅助函数（取第一个 scope 的 key）

### P3-3 修复 MySQLRetriever 嵌套扫描

- [ ] `internal/memory/retriever.go` 中 `RetrieveFacts` 修改：
  - [ ] 删除 `r.dao.ListAllEmbeddings(ctx, "fact", 500)` 调用
  - [ ] 替换为：先收集所有 `factIDs`，再调用 `r.dao.GetEmbeddingsByObjectIDs(ctx, "fact", factIDs)`
  - [ ] 用 `embMap[f.ID]` 直接取向量，不再循环遍历

- [ ] `RetrieveEvidence` 类似修复：
  - [ ] 确认现有 Evidence 检索中是否有类似问题（chunk 的 EmbeddingJSON 直接存在 chunk 行，无需额外查询，已是正确的）

- [ ] `go build ./...` 无错误

---

## Phase 4：接入与测试

### P4-1 更新 service.go 工厂

- [ ] `MemoryService` 接口新增 `Close() error`

- [ ] `NoopService.Close()` 返回 nil

- [ ] `service` 结构体新增 `milvusClient *MilvusClient` 字段

- [ ] `service.Close()` 调用 `s.milvusClient.Close()`（nil 检查）

- [ ] `service` 结构体新增 `embStore EmbeddingStore` 字段

- [ ] `NewService` 按照 spec §9 重写：
  - [ ] 构建 Embedder（不变）
  - [ ] 构建 MilvusClient（若 cfg.Milvus.Enabled；失败则降级 + warn log）
  - [ ] 构建 EmbeddingStore（`NewEmbeddingStore(dao, milvusClient)`）
  - [ ] 构建 Retriever（milvusClient != nil → HybridRetriever，否则 MySQLRetriever）
  - [ ] `return &service{..., embStore: embStore, retriever: ret, milvusClient: milvusClient}`

- [ ] 确认 `service` 的 `SaveFact` / `SaveChunk` 路径改用 `embStore.Save`（而非直接 dao）

### P4-2 更新 bootstrap.go

- [ ] 在 shutdown hook 中调用 `deps.MemoryService.Close()`
- [ ] 确认 `NewService` 入参传入 `settings.Conf.MemoryConfig`（含 Milvus 配置）

### P4-3 单元测试

- [ ] 新建 `internal/memory/hybrid_retriever_test.go`

- [ ] `TestHybridRetriever_MilvusFallback`：
  - [ ] `NewHybridRetriever(nil, mockMySQL, embedder)` 的 `RetrieveFacts` 应调用 mysql
  - [ ] 验证 milvus.Search 没有被调用

- [ ] `TestHybridRetriever_MergeDedup`：
  - [ ] mock milvus 返回 IDs [A, B, C]，mock MySQL 返回 [B, C, D]
  - [ ] 调用 `RetrieveFacts`，验证结果包含 A, B, C, D 各一次

- [ ] `TestRRF_Scoring`：
  - [ ] 直接测试 `rrfScore` 函数
  - [ ] `rrfScore(0, 0, 60)` > `rrfScore(0, -1, 60)` > `rrfScore(-1, -1, 60)`

- [ ] `TestMySQLRetriever_NoBulkScan`：
  - [ ] mock `dao.SearchFactsBySubject` 返回 5 条
  - [ ] mock `dao.GetEmbeddingsByObjectIDs` 记录调用次数
  - [ ] 验证 `GetEmbeddingsByObjectIDs` 调用 1 次（不是 5 次）

- [ ] `TestEmbeddingStore_MilvusWriteFailure`：
  - [ ] mock MySQL UpsertEmbedding 成功
  - [ ] mock milvus Insert 返回 error
  - [ ] `Save` 返回 nil
  - [ ] 等待 10ms 后检查 repairCh 有 1 条

- [ ] `TestBuildFilterExpr`：
  - [ ] `buildFilterExpr("user", "user:42", "fact")` → `scope_key == "user:42" && object_type == "fact"`
  - [ ] `buildFilterExpr("", "", "")` → `""`（空过滤）
  - [ ] `buildFilterExpr("user", "user:42", "")` → 只有 scope_key 条件

- [ ] 运行：`go test ./internal/memory/... -v` 全部通过

### P4-4 全量回归

- [ ] `go build ./...` 无编译错误
- [ ] `go vet ./...` 无警告
- [ ] `go test ./internal/memory/... -v`
- [ ] `go test ./internal/state/... -v -race`
- [ ] `go test ./internal/workflow/... -v`

---

## 集成验证清单

- [ ] `docker-compose up -d` 启动全部服务（mysql + redis + etcd + minio + milvus + api + worker）
- [ ] `docker-compose ps` 确认所有服务 healthy
- [ ] 查看 API 日志：`[memory] milvus client initialized, collection=compete_ai_embeddings`
- [ ] 创建一个竞品分析任务，观察日志：
  - [ ] `[memory] embedding saved: objectType=fact ... milvus=ok`
  - [ ] `[memory] embedding saved: objectType=chunk ... milvus=ok`
- [ ] 第二次跑相同竞品任务，观察 Analyst prompt 中包含 `## 历史知识` 段落
- [ ] 查看日志：`[memory] hybrid retrieve: milvus_hits=N keyword_hits=M merged=K`
- [ ] 关闭 Milvus（`docker-compose stop milvus`），再创建任务：
  - [ ] 任务仍能正常完成（降级到 MySQL 模式）
  - [ ] 日志中出现 `milvus client init failed, fallback to mysql-only`

---

## 全局验收标准

| 验收项 | 方法 | 预期结果 |
|---|---|---|
| Milvus collection 自动创建 | 首次启动后查看 Milvus collection 列表 | `compete_ai_embeddings` 存在，字段正确 |
| 双写成功 | 跑一个任务后查 MySQL + Milvus | 同一 objectID 在两处均有数据 |
| 向量检索优于关键词 | 使用语义相关但无关键词的查询 | Milvus 路能召回，keyword 路召回为空 |
| 降级透明 | Milvus 不可用时 | 任务正常完成，结果无差异（质量略降但不崩溃） |
| 嵌套扫描修复 | 开启 SQL 日志，执行 RetrieveFacts | `ListAllEmbeddings` 不再出现，只有 `IN (?,?,...)` |
| 内存无泄漏 | 运行 100 个任务后检查进程内存 | 内存不持续增长（MilvusClient 连接复用） |
