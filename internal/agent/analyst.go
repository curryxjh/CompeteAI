package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/protocol"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"
)

type Analyst struct {
	chat ChatClient
}

func NewAnalyst(chat ChatClient) *Analyst {
	return &Analyst{chat: chat}
}

func (a *Analyst) Name() domain.AgentName { return domain.AgentAnalyst }

func (a *Analyst) Card() AgentCard {
	return AgentCard{
		Name:            domain.AgentAnalyst,
		DisplayName:     "Analyst",
		Description:     "分析者：功能矩阵、SWOT、定价与用户画像结构化分析",
		Skills:          []string{"竞品对比", "SWOT 生成", "定价建模"},
		Tools:           []string{"FeatureMatrix", "SWOTGenerator"},
		DependsOn:       []string{"collector"},
		InputArtifacts:  []string{"collector:materials", "collector:sources", "task:meta"},
		OutputArtifacts: []string{"analysis:result"},
	}
}

func (a *Analyst) Run(ctx context.Context, input RunInput, bb state.Blackboard) (RunOutput, error) {
	store := state.ForTask(bb, input.TaskID)

	meta, err := store.LoadTaskMeta(ctx)
	if err != nil {
		return RunOutput{}, &protocol.AgentError{Kind: protocol.ErrorKindFatal, Message: "task meta not found"}
	}
	coll, err := store.LoadCollectorOutput(ctx)
	if err != nil {
		return RunOutput{}, &protocol.AgentError{Kind: protocol.ErrorKindFatal, Message: "collector output not found"}
	}

	rework := ""
	if input.Payload != nil {
		if r, ok := input.Payload["rework_reason"].(string); ok {
			rework = r
		}
	}
	if input.TriggerType == "qa_query" {
		return a.runQAQuery(ctx, input, store, coll)
	}
	if rework != "" {
		_ = store.SaveAnalysisReviewNotes(ctx, state.AnalysisReviewNotes{
			Notes: []string{rework},
		})
	}

	systemPrompt := `你是竞品分析专家。输出严格 JSON，不要 markdown 代码块。
{
  "summary": "执行摘要",
  "swot": { "竞品名": { "strengths":[{"text":"..."}], "weaknesses":[], "opportunities":[], "threats":[] } },
  "features": [{ "feature": "功能", "values": { "A": true, "B": false } }],
  "pricing": [{ "competitor": "名", "tiers": [{ "name": "套餐", "price": "$10/月", "features": [] }] }],
  "personas": [{ "competitor": "名", "segments": [], "painPoints": [], "useCases": [] }]
}`

	userPrompt := fmt.Sprintf(`任务：%s
竞品：%s
维度：%s
修正要求：%s

资料：
%s%s`, meta.Title, strings.Join(meta.Competitors, ", "), strings.Join(meta.Dimensions, ", "), rework, coll.Materials, MemoryPromptSuffix(input))

	EmitProgress(ctx, "path", "读取资料 → 功能矩阵 → SWOT → 定价 → 用户画像", "done")
	EmitProgress(ctx, "note", fmt.Sprintf("已加载 %d 个来源，竞品：%s", len(coll.Sources), strings.Join(meta.Competitors, " vs ")), "done")
	if rework != "" {
		EmitProgress(ctx, "note", "QA 修正要求："+rework, "done")
	}
	EmitProgress(ctx, "note", "开始调用 LLM，流式生成分析 JSON…", "running")

	var outputBuf strings.Builder
	raw, err := a.chat.StreamChat(ctx, systemPrompt, userPrompt, func(eventType, content string) {
		if content == "" {
			return
		}
		switch eventType {
		case "thinking":
			EmitProgress(ctx, "thinking", content, "running")
		case "content":
			outputBuf.WriteString(content)
			EmitProgress(ctx, "output", content, "running")
		}
	})
	if err != nil {
		return RunOutput{}, &protocol.AgentError{
			Kind:      protocol.ErrorKindRetryable,
			Message:   err.Error(),
			Retryable: true,
		}
	}

	EmitProgress(ctx, "note", fmt.Sprintf("模型输出完成（约 %d 字），正在解析 JSON…", outputBuf.Len()), "done")

	var partial state.AnalysisOutput
	if err := parseJSONFromLLM(raw, &partial); err != nil {
		return RunOutput{}, &protocol.AgentError{Kind: protocol.ErrorKindFatal, Message: "parse analysis json: " + err.Error()}
	}
	if partial.SWOT == nil {
		partial.SWOT = map[string]domain.SWOTAnalysis{}
	}

	if err := store.SaveAnalysisOutput(ctx, partial); err != nil {
		return RunOutput{}, err
	}

	summary := truncate(partial.Summary, 120)
	if summary == "" {
		summary = "结构化分析完成"
	}
	EmitProgress(ctx, "note", fmt.Sprintf("分析完成：SWOT %d 项、功能 %d 行、定价 %d 组",
		len(partial.SWOT), len(partial.Features), len(partial.Pricing)), "done")
	EmitProgress(ctx, "analysis", FormatAnalysisMarkdown(meta.Competitors, partial), "done")

	swotKeys := make([]string, 0, len(partial.SWOT))
	for k := range partial.SWOT {
		swotKeys = append(swotKeys, k)
	}
	analysisPayload := protocol.AnalysisPayload{
		Summary:      summary,
		SWOTKeys:     swotKeys,
		FeatureCount: len(partial.Features),
		PricingCount: len(partial.Pricing),
		PersonaCount: len(partial.Personas),
	}

	return RunOutput{
		Status:      domain.AgentRunCompleted,
		NextAgent:   ptrAgent(domain.AgentWriter),
		MessageType: string(protocol.MsgAnalysisReady),
		Payload:     analysisPayload,
		Summary:     summary,
		Artifacts: []protocol.ArtifactRef{
			protocol.NewArtifactRef(protocol.ArtifactAnalysisResult, state.AnalysisResultKey(input.TaskID), partial.Version),
		},
		Metadata: map[string]any{"token_hint": len(raw)},
	}, nil
}

func (a *Analyst) runQAQuery(ctx context.Context, input RunInput, store state.TaskStore, coll state.CollectorOutput) (RunOutput, error) {
	claim, _ := input.Payload["claim"].(string)
	query, _ := input.Payload["query"].(string)
	if claim == "" {
		claim = query
	}
	EmitProgress(ctx, "note", "QA 追问："+query, "done")

	var evidence []string
	for _, src := range coll.Sources {
		excerpt := strings.ToLower(src.Excerpt + " " + src.Title)
		for _, w := range strings.Fields(claim) {
			if len([]rune(w)) >= 2 && strings.Contains(excerpt, strings.ToLower(w)) {
				evidence = append(evidence, fmt.Sprintf("[%s] %s", src.Title, truncate(src.Excerpt, 200)))
				break
			}
		}
		if len(evidence) >= 3 {
			break
		}
	}
	answer := "未在已采集来源中找到直接依据，建议改写表述或补充采集。"
	if len(evidence) > 0 {
		answer = strings.Join(evidence, "\n")
	}
	EmitProgress(ctx, "note", "证据检索完成", "done")
	return RunOutput{
		Status: domain.AgentRunCompleted, Summary: answer,
		MessageType: string(protocol.MsgAnalysisReady),
		Payload: protocol.AnalystResponsePayload{
			Claim: claim, Evidence: answer, Explanation: answer,
		},
	}, nil
}
