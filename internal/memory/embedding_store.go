package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"CompeteAI/internal/repository/dao"

	"github.com/google/uuid"
)

// EmbeddingStore 封装 embedding 的双写逻辑：
//   - MySQL 为主存储（持久化、可重建）
//   - Milvus 为向量索引（ANN 检索，写失败时异步补偿）
type EmbeddingStore interface {
	// Write 将 embedding 写入 MySQL 和 Milvus（Milvus 写失败不阻断主流程）
	Write(ctx context.Context, req WriteEmbeddingRequest) error
	// DeleteByObject 删除指定对象的 embedding（fact 失效时调用）
	DeleteByObject(ctx context.Context, objectType, objectID string) error
	// RebuildMilvus 从 MySQL 全量重建 Milvus（运维恢复用）
	RebuildMilvus(ctx context.Context) error
	// Close 关闭后台 goroutine
	Close()
}

// WriteEmbeddingRequest embedding 写入请求。
type WriteEmbeddingRequest struct {
	// MySQLID MySQL memory_embeddings.id，若为空则自动生成 UUID
	MySQLID     string
	ObjectType  string    // "fact" / "chunk" / "episode"
	ObjectID    string    // 对应实体的 ID
	ScopeType   string
	ScopeKey    string
	ContentText string
	Vector      []float32
}

// embeddingStore 实现 EmbeddingStore。
type embeddingStore struct {
	dao      *dao.MemoryDao
	milvus   *MilvusClient // nil = MySQL-only 模式
	repairCh chan string    // objectID 待补写队列
	closeCh  chan struct{}  // 关闭信号
	wg       sync.WaitGroup
}

// NewEmbeddingStore 构建 EmbeddingStore。
// milvus 为 nil 时退化为纯 MySQL 模式。
func NewEmbeddingStore(d *dao.MemoryDao, milvus *MilvusClient) EmbeddingStore {
	s := &embeddingStore{
		dao:      d,
		milvus:   milvus,
		repairCh: make(chan string, 1000),
		closeCh:  make(chan struct{}),
	}
	if milvus != nil {
		s.wg.Add(1)
		go s.runRepair()
	}
	return s
}

// Write 执行双写。MySQL 失败时整体失败；Milvus 失败时记入 repair 队列。
func (s *embeddingStore) Write(ctx context.Context, req WriteEmbeddingRequest) error {
	id := req.MySQLID
	if id == "" {
		id = uuid.NewString()
	}
	// Step 1: 写 MySQL（主链路，失败则整体失败）
	entity := dao.MemoryEmbeddingEntity{
		ID:            id,
		ObjectType:    req.ObjectType,
		ObjectID:      req.ObjectID,
		ScopeType:     req.ScopeType,
		ScopeKey:      req.ScopeKey,
		ContentText:   req.ContentText,
		EmbeddingJSON: marshalVector(req.Vector),
	}
	if err := s.dao.UpsertEmbedding(ctx, entity); err != nil {
		return fmt.Errorf("embedding mysql upsert (objectID=%s): %w", req.ObjectID, err)
	}

	// Step 2: 写 Milvus（辅助链路，失败只入 repair 队列）
	if s.milvus != nil {
		mReq := MilvusInsertRequest{
			ID:          id,
			ObjectType:  req.ObjectType,
			ObjectID:    req.ObjectID,
			ScopeType:   req.ScopeType,
			ScopeKey:    req.ScopeKey,
			ContentText: req.ContentText,
			Vector:      req.Vector,
		}
		if err := s.milvus.Insert(ctx, []MilvusInsertRequest{mReq}); err != nil {
			// 非阻塞写入 repair 队列
			select {
			case s.repairCh <- req.ObjectID:
			default:
				// 队列已满，记录日志，数据已在 MySQL 可通过 RebuildMilvus 恢复
			}
		}
	}
	return nil
}

// DeleteByObject 删除 MySQL 和 Milvus 中指定对象的 embedding。
func (s *embeddingStore) DeleteByObject(ctx context.Context, objectType, objectID string) error {
	if err := s.dao.DeleteEmbeddingByObjectID(ctx, objectType, objectID); err != nil {
		return err
	}
	if s.milvus != nil {
		_ = s.milvus.DeleteByObjectID(ctx, objectType, objectID)
	}
	return nil
}

// RebuildMilvus 从 MySQL 全量重建 Milvus（运维用，分页批量）。
func (s *embeddingStore) RebuildMilvus(ctx context.Context) error {
	if s.milvus == nil {
		return nil
	}
	const batchSize = 100
	offset := 0
	for {
		rows, err := s.dao.ListEmbeddingsPaged(ctx, offset, batchSize)
		if err != nil {
			return fmt.Errorf("list embeddings page=%d: %w", offset, err)
		}
		if len(rows) == 0 {
			break
		}
		reqs := make([]MilvusInsertRequest, 0, len(rows))
		for _, r := range rows {
			vec := unmarshalVector(r.EmbeddingJSON)
			if len(vec) == 0 {
				continue
			}
			reqs = append(reqs, MilvusInsertRequest{
				ID:          r.ID,
				ObjectType:  r.ObjectType,
				ObjectID:    r.ObjectID,
				ScopeType:   r.ScopeType,
				ScopeKey:    r.ScopeKey,
				ContentText: r.ContentText,
				Vector:      vec,
			})
		}
		if len(reqs) > 0 {
			if err := s.milvus.Insert(ctx, reqs); err != nil {
				return fmt.Errorf("milvus batch insert offset=%d: %w", offset, err)
			}
		}
		offset += batchSize
		if len(rows) < batchSize {
			break
		}
	}
	return nil
}

// Close 优雅关闭后台 repair goroutine。
func (s *embeddingStore) Close() {
	close(s.closeCh)
	s.wg.Wait()
}

// runRepair 后台消费 repair 队列，对 Milvus 写失败的 objectID 重试。
func (s *embeddingStore) runRepair() {
	defer s.wg.Done()
	for {
		select {
		case <-s.closeCh:
			return
		case objectID := <-s.repairCh:
			s.repairOne(objectID)
		}
	}
}

func (s *embeddingStore) repairOne(objectID string) {
	ctx := context.Background()
	rows, err := s.dao.GetEmbeddingByObjectID(ctx, objectID)
	if err != nil || len(rows) == 0 {
		return
	}
	var reqs []MilvusInsertRequest
	for _, r := range rows {
		vec := unmarshalVector(r.EmbeddingJSON)
		if len(vec) == 0 {
			continue
		}
		reqs = append(reqs, MilvusInsertRequest{
			ID:          r.ID,
			ObjectType:  r.ObjectType,
			ObjectID:    r.ObjectID,
			ScopeType:   r.ScopeType,
			ScopeKey:    r.ScopeKey,
			ContentText: r.ContentText,
			Vector:      vec,
		})
	}
	if len(reqs) == 0 {
		return
	}
	for retry := 0; retry < 3; retry++ {
		if err := s.milvus.Insert(ctx, reqs); err == nil {
			return
		}
		time.Sleep(time.Duration(1<<retry) * time.Second)
	}
}

// marshalVector 将向量序列化为 JSON 字符串。
func marshalVector(vec []float32) string {
	b, _ := json.Marshal(vec)
	return string(b)
}

// unmarshalVector 从 JSON 字符串反序列化向量。
func unmarshalVector(s string) []float32 {
	var vec []float32
	_ = json.Unmarshal([]byte(s), &vec)
	return vec
}
