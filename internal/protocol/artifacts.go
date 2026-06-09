package protocol

import "time"

// 工件类型（§9）。
const (
	ArtifactTaskBrief         = "task_brief"
	ArtifactPlan              = "plan"
	ArtifactSourceRef         = "source_ref"
	ArtifactCollectedMaterial = "collected_material"
	ArtifactAnalysisResult    = "analysis_result"
	ArtifactReportDraft       = "report_draft"
	ArtifactReportFinal       = "report_final"
	ArtifactQAReview          = "qa_review"
)

// ArtifactRef 工件引用，大数据走 Blackboard。
type ArtifactRef struct {
	Type      string `json:"type"`
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

// NewArtifactRef 构造工件引用。
func NewArtifactRef(artifactType, key string, version int) ArtifactRef {
	return ArtifactRef{
		Type:      artifactType,
		Key:       key,
		Version:   version,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}
