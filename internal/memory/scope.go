package memory

import (
	"CompeteAI/internal/repository/dao"
	"context"
	"fmt"
)

// ScopeResolver 解析多层 scope ID（global/project/workspace/user/entity）。
type ScopeResolver struct {
	dao *dao.MemoryDao
}

func NewScopeResolver(d *dao.MemoryDao) *ScopeResolver {
	return &ScopeResolver{dao: d}
}

type ScopeResolveRequest struct {
	ProjectID   string
	WorkspaceID string
	UserID      int64
	Competitors []string
}

func (r *ScopeResolver) ResolveIDs(ctx context.Context, req ScopeResolveRequest) []string {
	if r.dao == nil {
		return nil
	}
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = defaultWorkspaceID
	}

	seen := map[string]struct{}{}
	var ids []string
	add := func(scopeType, key string) {
		row, err := r.dao.GetScopeByTypeKey(ctx, scopeType, key)
		if err != nil {
			return
		}
		if _, ok := seen[row.ID]; ok {
			return
		}
		seen[row.ID] = struct{}{}
		ids = append(ids, row.ID)
	}

	add(string(ScopeGlobal), "default")
	add(string(ScopeProject), projectID)
	add(string(ScopeWorkspace), wsID)
	if req.UserID > 0 {
		add(string(ScopeUser), fmt.Sprintf("user:%d", req.UserID))
	}
	for _, comp := range req.Competitors {
		add(string(ScopeEntity), "entity:"+NormalizeName(comp))
	}
	return ids
}

// ResolveKeys 返回 ScopeResolver 解析到的 scope_key 列表（供 Milvus 过滤用）。
// 与 ResolveIDs 返回的 UUID 列表一一对应。
func (r *ScopeResolver) ResolveKeys(ctx context.Context, req ScopeResolveRequest) []string {
	if r.dao == nil {
		return nil
	}
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = defaultWorkspaceID
	}

	type entry struct{ scopeType, key string }
	entries := []entry{
		{string(ScopeGlobal), "default"},
		{string(ScopeProject), projectID},
		{string(ScopeWorkspace), wsID},
	}
	if req.UserID > 0 {
		entries = append(entries, entry{string(ScopeUser), fmt.Sprintf("user:%d", req.UserID)})
	}
	for _, comp := range req.Competitors {
		entries = append(entries, entry{string(ScopeEntity), "entity:" + NormalizeName(comp)})
	}

	seen := map[string]struct{}{}
	var keys []string
	for _, e := range entries {
		row, err := r.dao.GetScopeByTypeKey(ctx, e.scopeType, e.key)
		if err != nil {
			continue
		}
		if _, ok := seen[row.ScopeKey]; ok {
			continue
		}
		seen[row.ScopeKey] = struct{}{}
		keys = append(keys, row.ScopeKey)
	}
	return keys
}

// ScopeIDsForPreferences 返回用于加载偏好的 scope ID（含类型信息）。
func (r *ScopeResolver) ScopeIDsForPreferences(ctx context.Context, req ScopeResolveRequest) []scopePrefRef {
	if r.dao == nil {
		return nil
	}
	projectID := req.ProjectID
	if projectID == "" {
		projectID = defaultProjectID
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = defaultWorkspaceID
	}

	type pair struct {
		scopeType ScopeType
		key       string
		rank      int
	}
	pairs := []pair{
		{ScopeGlobal, "default", 500},
		{ScopeWorkspace, wsID, 400},
		{ScopeProject, projectID, 200},
	}
	if req.UserID > 0 {
		pairs = append(pairs, pair{ScopeUser, fmt.Sprintf("user:%d", req.UserID), 300})
	}

	var out []scopePrefRef
	seen := map[string]struct{}{}
	for _, p := range pairs {
		row, err := r.dao.GetScopeByTypeKey(ctx, string(p.scopeType), p.key)
		if err != nil {
			continue
		}
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		out = append(out, scopePrefRef{ID: row.ID, ScopeType: p.scopeType, Rank: p.rank})
	}
	return out
}

type scopePrefRef struct {
	ID        string
	ScopeType ScopeType
	Rank      int
}
