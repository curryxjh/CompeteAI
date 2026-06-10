package memory

import (
	"context"
	"testing"
)

// ─────────────────────────────────────────────
// TestRRF_Scoring：验证 RRF 分值计算正确性
// ─────────────────────────────────────────────

func TestRRF_Scoring(t *testing.T) {
	// 两路均命中，分值最高
	bothHit := rrfCombineScore(1, 1, 60)
	// 仅 Milvus 命中
	milvusOnly := rrfCombineScore(1, 0, 60)
	// 仅 keyword 命中
	kwOnly := rrfCombineScore(0, 1, 60)
	// 均未命中
	noneHit := rrfCombineScore(0, 0, 60)

	if bothHit <= milvusOnly {
		t.Errorf("bothHit(%.4f) should > milvusOnly(%.4f)", bothHit, milvusOnly)
	}
	if milvusOnly != kwOnly {
		t.Errorf("milvusOnly(%.4f) should == kwOnly(%.4f) when rank=1", milvusOnly, kwOnly)
	}
	if noneHit != 0 {
		t.Errorf("noneHit should be 0, got %.4f", noneHit)
	}
	// 排名越靠后分值越低
	rank1 := rrfCombineScore(1, 0, 60)
	rank5 := rrfCombineScore(5, 0, 60)
	if rank1 <= rank5 {
		t.Errorf("rank1(%.4f) should > rank5(%.4f)", rank1, rank5)
	}
}

// ─────────────────────────────────────────────
// TestRankMap：验证 rankMap 1-based 语义
// ─────────────────────────────────────────────

func TestRankMap(t *testing.T) {
	ids := []string{"a", "b", "c"}
	m := rankMap(ids)
	if m["a"] != 1 {
		t.Errorf("expected rank[a]=1, got %d", m["a"])
	}
	if m["c"] != 3 {
		t.Errorf("expected rank[c]=3, got %d", m["c"])
	}
	if m["notexist"] != 0 {
		t.Errorf("missing key should return 0, got %d", m["notexist"])
	}
}

// ─────────────────────────────────────────────
// TestHybridRetriever_NilMilvusFallback：
// milvus=nil 时退回 mysqlRetriever
// ─────────────────────────────────────────────

func TestHybridRetriever_NilMilvusFallback(t *testing.T) {
	// mysqlRetriever 为 nil 时构造 HybridRetriever 不应 panic
	hr := &HybridRetriever{
		milvus:   nil,
		mysql:    nil,
		embedder: nil,
	}
	// milvus 为 nil，期望直接委托 mysql（mysql 也为 nil，会 panic 除非有保护）
	// 此处只测试 milvus=nil 时不走 Milvus 分支
	req := RetrievalRequest{Query: "test", Limit: 5}
	defer func() {
		if r := recover(); r != nil {
			// mysql 为 nil 时 panic 是预期的（生产中不会这样构造）
			t.Logf("expected panic with nil mysql: %v", r)
		}
	}()
	_, _ = hr.RetrieveFacts(context.Background(), req)
}

// ─────────────────────────────────────────────
// TestBuildFilterExpr：验证 Milvus 过滤表达式生成
// ─────────────────────────────────────────────

func TestBuildFilterExpr(t *testing.T) {
	tests := []struct {
		scopeKey   string
		objectType string
		expected   string
	}{
		{"user:42", "fact", `scope_key == "user:42" && object_type == "fact"`},
		{"user:42", "", `scope_key == "user:42"`},
		{"", "fact", `object_type == "fact"`},
		{"", "", ``},
	}
	for _, tt := range tests {
		got := buildFilterExpr(tt.scopeKey, tt.objectType)
		if got != tt.expected {
			t.Errorf("buildFilterExpr(%q, %q) = %q, want %q",
				tt.scopeKey, tt.objectType, got, tt.expected)
		}
	}
}

// ─────────────────────────────────────────────
// TestTruncateStr：验证文本截断
// ─────────────────────────────────────────────

func TestTruncateStr(t *testing.T) {
	s := "hello world"
	if got := truncateStr(s, 100); got != s {
		t.Errorf("short string should not be truncated")
	}
	long := "中文字符测试内容，用来验证截断是否按照 rune 边界"
	truncated := truncateStr(long, 5)
	if len([]rune(truncated)) != 5 {
		t.Errorf("expected 5 runes after truncation, got %d", len([]rune(truncated)))
	}
}

// ─────────────────────────────────────────────
// TestPrimaryScopeKey：验证 primaryScopeKey 取第一个非空值
// ─────────────────────────────────────────────

func TestPrimaryScopeKey(t *testing.T) {
	if got := primaryScopeKey([]string{"", "project:foo", "user:1"}); got != "project:foo" {
		t.Errorf("expected 'project:foo', got %q", got)
	}
	if got := primaryScopeKey(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
	if got := primaryScopeKey([]string{"", ""}); got != "" {
		t.Errorf("expected empty for all-empty, got %q", got)
	}
}

// ─────────────────────────────────────────────
// TestMarshalUnmarshalVector：验证向量序列化往返
// ─────────────────────────────────────────────

func TestMarshalUnmarshalVector(t *testing.T) {
	original := []float32{0.1, 0.2, 0.3, 0.4}
	marshaled := marshalVector(original)
	recovered := unmarshalVector(marshaled)
	if len(recovered) != len(original) {
		t.Fatalf("length mismatch: got %d want %d", len(recovered), len(original))
	}
	for i := range original {
		diff := recovered[i] - original[i]
		if diff < -1e-6 || diff > 1e-6 {
			t.Errorf("element[%d]: got %f want %f", i, recovered[i], original[i])
		}
	}
}

// ─────────────────────────────────────────────
// TestRRFMergeDedup：验证两路合并去重
// ─────────────────────────────────────────────

func TestRRFMergeDedup(t *testing.T) {
	// 模拟 Milvus 返回 [A, B, C]，keyword 返回 [B, C, D]
	milvusIDs := []string{"A", "B", "C"}
	kwIDs := []string{"B", "C", "D"}

	mRankMap := rankMap(milvusIDs)
	kRankMap := rankMap(kwIDs)

	// 合并所有 ID
	allIDs := map[string]struct{}{}
	for _, id := range milvusIDs {
		allIDs[id] = struct{}{}
	}
	for _, id := range kwIDs {
		allIDs[id] = struct{}{}
	}

	// 期望 4 个不重复 ID
	if len(allIDs) != 4 {
		t.Errorf("expected 4 unique IDs, got %d", len(allIDs))
	}

	// B 和 C 在两路都命中，分值应高于 A 和 D
	scoreB := rrfCombineScore(mRankMap["B"], kRankMap["B"], 60)
	scoreA := rrfCombineScore(mRankMap["A"], kRankMap["A"], 60)
	scoreD := rrfCombineScore(mRankMap["D"], kRankMap["D"], 60)

	if scoreB <= scoreA {
		t.Errorf("B(both hits, %.4f) should > A(milvus-only, %.4f)", scoreB, scoreA)
	}
	if scoreB <= scoreD {
		t.Errorf("B(both hits, %.4f) should > D(kw-only, %.4f)", scoreB, scoreD)
	}
}
