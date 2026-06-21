package state

import "CompeteAI/internal/domain"

// Versioned 带轮次与版本的 Blackboard 数据。
type Versioned struct {
	Round   int `json:"round,omitempty"`
	Version int `json:"version,omitempty"`
}

// --- task.* ---

// TaskMeta 任务基础信息（§6.1）。
type TaskMeta struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Competitors []string `json:"competitors"`
	Dimensions  []string `json:"dimensions"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

func TaskMetaFromDomain(t domain.Task) TaskMeta {
	return TaskMeta{
		ID:          t.ID,
		Title:       t.Title,
		Competitors: t.Competitors,
		Dimensions:  t.Dimensions,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// TaskStatusSnapshot 任务运行态（§6.1 task:status）。
type TaskStatusSnapshot struct {
	Status       domain.TaskStatus `json:"status"`
	Progress     int               `json:"progress"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Versioned
}

// --- workflow.* ---

// WorkflowState 路由与轮次（§6.2 workflow:state）。
type WorkflowState struct {
	CurrentAgent          string `json:"currentAgent"`
	NextAgent             string `json:"nextAgent"`
	Round                 int    `json:"round"`
	MaxRounds             int    `json:"maxRounds"`
	NeedsClarification    bool   `json:"needsClarification"`
	ClarificationResolved bool   `json:"clarificationResolved"`
	RejectionCount        int    `json:"rejectionCount"`
	QAQueryAttempts       int    `json:"qaQueryAttempts,omitempty"`
	TraceID               string `json:"traceId,omitempty"`
	Attempt               int    `json:"attempt,omitempty"`
	Plan                  string `json:"plan,omitempty"`
	Versioned
}

// WorkflowSnapshot 兼容旧名。
type WorkflowSnapshot = WorkflowState

// WorkflowPlan 协调计划（§6.2 workflow:plan）。
type WorkflowPlan struct {
	Summary      string   `json:"summary"`
	Deliverables []string `json:"deliverables"`
	Dimensions   []string `json:"dimensions"`
	Competitors  []string `json:"competitors,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Versioned
}

// WorkflowRework QA 打回路由（§6.2 workflow:rework）。
type WorkflowRework struct {
	TargetAgent string   `json:"targetAgent"`
	Reason      string   `json:"reason"`
	Issues      []string `json:"issues,omitempty"`
	RequestedBy string   `json:"requestedBy,omitempty"`
	Round       int      `json:"round,omitempty"`
	Versioned
}

// --- collector.* ---

type CollectorQuery struct {
	Query string `json:"query"`
	Lang  string `json:"lang,omitempty"`
	Limit int    `json:"limit,omitempty"`
	Versioned
}

type CollectorURLs struct {
	URLs []string `json:"urls"`
	Versioned
}

type CollectorSources struct {
	Items map[string]domain.SourceRef `json:"items"`
	Versioned
}

type MaterialItem struct {
	SourceID string   `json:"sourceId"`
	Content  string   `json:"content"`
	Summary  string   `json:"summary,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type CollectorMaterials struct {
	Items []MaterialItem `json:"items"`
	Versioned
}

type CollectorSummary struct {
	SourceCount       int    `json:"sourceCount"`
	UsableSourceCount int    `json:"usableSourceCount"`
	Notes             string `json:"notes,omitempty"`
	Versioned
}

// CollectorOutput 聚合视图，供 Analyst 读取完整采集上下文。
type CollectorOutput struct {
	Query     string                      `json:"query"`
	URLs      []string                    `json:"urls"`
	Sources   map[string]domain.SourceRef `json:"sources"`
	Materials string                      `json:"materials"`
	Summary   string                      `json:"summary"`
	Versioned
}

// --- analysis.* ---

type AnalysisOutput struct {
	Summary     string                              `json:"summary"`
	SWOT        map[string]domain.SWOTAnalysis      `json:"swot"`
	Features    []domain.FeatureRow                   `json:"features"`
	FeatureTree map[string][]domain.FeatureTreeNode `json:"featureTree,omitempty"`
	Pricing     []domain.PricingInfo                  `json:"pricing"`
	Personas    []domain.UserPersona                  `json:"personas"`
	Versioned
}

type AnalysisReviewNotes struct {
	Notes []string `json:"notes"`
	Round int      `json:"round"`
	Versioned
}

type AnalysisHistory struct {
	Entries []AnalysisOutput `json:"entries"`
}

// --- report.* ---

type ReportDraft struct {
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Sections  []string `json:"sections,omitempty"`
	SourceIDs []string `json:"sourceIds,omitempty"`
	Versioned
}

type ReportMeta struct {
	GeneratedAt string `json:"generatedAt"`
	WrittenBy   string `json:"writtenBy,omitempty"`
	Versioned
}

// --- qa.* ---

type QARecord struct {
	Score       int      `json:"score"`
	Result      string   `json:"result"`
	Issues      []string `json:"issues,omitempty"`
	IssuesFull  []Issue  `json:"issuesFull,omitempty"`
	TargetAgent string   `json:"targetAgent,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Versioned
}

// Issue 与 domain.Issue 对齐的 Blackboard 问题结构。
type Issue struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Location   string `json:"location"`
	Problem    string `json:"problem"`
	Suggestion string `json:"suggestion"`
	Severity   string `json:"severity"`
}

type QAHistoryEntry struct {
	Round  int      `json:"round"`
	Score  int      `json:"score"`
	Result string   `json:"result"`
	Issues []string `json:"issues,omitempty"`
}

type QAHistory struct {
	Entries []QAHistoryEntry `json:"entries"`
}

// ClarificationQuestion 待澄清问题（§7.1）。
type ClarificationQuestion struct {
	Question string `json:"question"`
	Agent    string `json:"agent,omitempty"`
}

// ClarificationAnswer 用户澄清答复。
type ClarificationAnswer struct {
	Answer string `json:"answer"`
}
