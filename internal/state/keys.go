package state

import "fmt"

// Key 命名：task:{taskID}:{scope}:{field}（§5）

func TaskPrefix(taskID string) string {
	return fmt.Sprintf("task:%s:", taskID)
}

func TaskMetaKey(taskID string) string {
	return fmt.Sprintf("task:%s:task:meta", taskID)
}

func TaskStatusKey(taskID string) string {
	return fmt.Sprintf("task:%s:task:status", taskID)
}

func WorkflowStateKey(taskID string) string {
	return fmt.Sprintf("task:%s:workflow:state", taskID)
}

func WorkflowPlanKey(taskID string) string {
	return fmt.Sprintf("task:%s:workflow:plan", taskID)
}

func WorkflowReworkKey(taskID string) string {
	return fmt.Sprintf("task:%s:workflow:rework", taskID)
}

func CollectorQueryKey(taskID string) string {
	return fmt.Sprintf("task:%s:collector:query", taskID)
}

func CollectorURLsKey(taskID string) string {
	return fmt.Sprintf("task:%s:collector:urls", taskID)
}

func CollectorSourcesKey(taskID string) string {
	return fmt.Sprintf("task:%s:collector:sources", taskID)
}

func CollectorMaterialsKey(taskID string) string {
	return fmt.Sprintf("task:%s:collector:materials", taskID)
}

func CollectorSummaryKey(taskID string) string {
	return fmt.Sprintf("task:%s:collector:summary", taskID)
}

func AnalysisResultKey(taskID string) string {
	return fmt.Sprintf("task:%s:analysis:result", taskID)
}

func AnalysisReviewNotesKey(taskID string) string {
	return fmt.Sprintf("task:%s:analysis:review_notes", taskID)
}

func AnalysisHistoryKey(taskID string) string {
	return fmt.Sprintf("task:%s:analysis:history", taskID)
}

func ReportDraftKey(taskID string) string {
	return fmt.Sprintf("task:%s:report:draft", taskID)
}

func ReportFinalKey(taskID string) string {
	return fmt.Sprintf("task:%s:report:final", taskID)
}

func ReportMetaKey(taskID string) string {
	return fmt.Sprintf("task:%s:report:meta", taskID)
}

func QAResultKey(taskID string) string {
	return fmt.Sprintf("task:%s:qa:result", taskID)
}

func QAHistoryKey(taskID string) string {
	return fmt.Sprintf("task:%s:qa:history", taskID)
}

func ClarificationAnswerKey(taskID string) string {
	return fmt.Sprintf("task:%s:clarification:answer", taskID)
}

func ClarificationQuestionKey(taskID string) string {
	return fmt.Sprintf("task:%s:clarification:question", taskID)
}

func VersionedKey(base string, version int) string {
	return fmt.Sprintf("%s:v%d", base, version)
}
