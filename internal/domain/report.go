// Package domain 定义所有核心领域模型。
// 本文件包含报告聚合根及其值对象（SWOT、功能矩阵、定价、用户画像、源引用）。
package domain

// ──────────────────────────────────────────────────────
// SWOT 分析值对象
// ──────────────────────────────────────────────────────

// SWOTItem 单条 SWOT 项，携带来源引用以支持可追溯性。
type SWOTItem struct {
	Text      string   `json:"text"`                // 项内容描述
	SourceIDs []string `json:"sourceIds,omitempty"` // 关联的来源 ID 列表
}

// SWOTAnalysis 某个竞品的 SWOT 四象限分析结果。
type SWOTAnalysis struct {
	Strengths     []SWOTItem `json:"strengths"`     // 优势
	Weaknesses    []SWOTItem `json:"weaknesses"`    // 劣势
	Opportunities []SWOTItem `json:"opportunities"` // 机会
	Threats       []SWOTItem `json:"threats"`       // 威胁
}

// ──────────────────────────────────────────────────────
// 功能对比值对象
// ──────────────────────────────────────────────────────

// FeatureRow 功能对比矩阵的一行：一个功能特性 × 各竞品的值。
// Values 的 key 是竞品名，value 是该功能在该竞品上的描述。
type FeatureRow struct {
	Feature   string                  `json:"feature"`             // 功能特性名称
	Values    map[string]interface{}  `json:"values"`              // 竞品 → 功能值
	SourceIDs map[string][]string    `json:"sourceIds,omitempty"` // 竞品 → 来源 ID 列表
}

// FeatureTreeNode 层级功能树节点，支持多级分类结构。
// 用于将扁平的 FeatureRow 组织成树形结构展示。
type FeatureTreeNode struct {
	Name        string            `json:"name"`                  // 节点名称
	Description string            `json:"description,omitempty"` // 节点描述
	Supported   *bool             `json:"supported,omitempty"`   // 是否支持（nil 表示不适用）
	SourceIDs   []string          `json:"sourceIds,omitempty"`   // 来源引用 ID 列表
	Children    []FeatureTreeNode `json:"children,omitempty"`    // 子节点
}

// ──────────────────────────────────────────────────────
// 定价对比值对象
// ──────────────────────────────────────────────────────

// PricingTier 单个定价方案（如"基础版"、"专业版"）。
type PricingTier struct {
	Name     string   `json:"name"`     // 方案名称
	Price    string   `json:"price"`    // 价格描述（如"¥99/月"）
	Features []string `json:"features"` // 该方案包含的功能列表
}

// PricingInfo 某个竞品的定价信息，包含多个定价方案。
type PricingInfo struct {
	Competitor string        `json:"competitor"`           // 竞品名称
	Tiers      []PricingTier `json:"tiers"`               // 定价方案列表
	SourceIDs  []string      `json:"sourceIds,omitempty"`  // 来源引用 ID 列表
}

// ──────────────────────────────────────────────────────
// 用户画像值对象
// ──────────────────────────────────────────────────────

// UserPersona 某个竞品的用户画像，描述目标用户群体的特征。
type UserPersona struct {
	Competitor string   `json:"competitor"`            // 竞品名称
	Segments   []string `json:"segments"`               // 用户群体标签（如"中小企业"、"开发者"）
	PainPoints []string `json:"painPoints"`             // 痛点列表
	UseCases   []string `json:"useCases"`                // 典型使用场景
	SourceIDs  []string `json:"sourceIds,omitempty"`      // 来源引用 ID 列表
}

// ──────────────────────────────────────────────────────
// 来源引用
// ──────────────────────────────────────────────────────

// SourceRef 采集来源的引用信息，嵌入 Report.Sources 中。
// Key 是来源标识（如 "s1"），用于 SWOT/Feature/Pricing 中的 SourceIDs 引用关联。
type SourceRef struct {
	URL         string `json:"url"`                    // 来源 URL
	Excerpt     string `json:"excerpt"`                 // 来源摘要片段
	CollectedAt string `json:"collectedAt"`             // 采集时间（ISO 3339）
	Title       string `json:"title,omitempty"`         // 网页标题
}

// ──────────────────────────────────────────────────────
// 报告聚合根
// ──────────────────────────────────────────────────────

// Report 竞品分析报告聚合根。
// 由 Writer Agent 生成，QA Agent 评分，是整个流水线的最终产出物。
// 与 frontend/src/types/index.ts 的 Report 类型对齐。
type Report struct {
	TaskID      string                       `json:"taskId"`              // 关联的任务 ID
	Title       string                       `json:"title"`               // 报告标题
	GeneratedAt string                       `json:"generatedAt"`          // 生成时间（ISO 3339）
	QAScore     int                          `json:"qaScore"`              // QA 评分（0-100，≥70 通过）
	Summary     string                       `json:"summary"`              // 执行摘要
	SWOT        map[string]SWOTAnalysis      `json:"swot"`                // 竞品 → SWOT 分析
	Features    []FeatureRow                 `json:"features"`             // 功能对比矩阵
	FeatureTree map[string][]FeatureTreeNode `json:"featureTree,omitempty"` // 竞品 → 功能树
	Pricing     []PricingInfo                `json:"pricing"`              // 定价对比
	Personas    []UserPersona                `json:"personas"`             // 用户画像
	Sources     map[string]SourceRef         `json:"sources"`              // 来源引用（key 为来源 ID）
}