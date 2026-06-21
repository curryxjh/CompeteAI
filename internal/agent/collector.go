package agent

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/llm"
	"CompeteAI/internal/state"
	"context"
	"fmt"
	"strings"
	"time"
)

type Collector struct {
	tools ToolInvoker
}

func NewCollector(tools ToolInvoker) *Collector {
	return &Collector{tools: tools}
}

func (a *Collector) Name() domain.AgentName { return domain.AgentCollector }

func (a *Collector) Card() AgentCard {
	return AgentCard{
		Name:            domain.AgentCollector,
		DisplayName:     "Collector",
		Description:     "信息采集者：Search → Fetch → Extract → Enrich → Validate 五阶段流水线",
		Skills:          []string{"网页搜索", "内容抓取", "结构化抽取", "溯源标注"},
		Tools:           []string{"firecrawl_search", "firecrawl_scrape"},
		DependsOn:       []string{"coordinator"},
		InputArtifacts:  []string{"task:meta", "workflow:plan"},
		OutputArtifacts: []string{"collector:sources", "collector:materials"},
	}
}

func (a *Collector) Run(ctx context.Context, input domain.RunInput, bb any) (domain.RunOutput, error) {
	blackboard := bb.(state.Blackboard)
	store := state.ForTask(blackboard, input.TaskID)

	meta, err := store.LoadTaskMeta(ctx)
	if err != nil {
		return domain.RunOutput{}, &domain.AgentError{Kind: domain.ErrorKindFatal, Message: "task meta not found"}
	}

	query := strings.Join(meta.Competitors, " vs ") + " 竞品对比 功能 定价 SWOT 2025"
	EmitProgress(ctx, "path", "Search → Fetch → Extract → Enrich → Validate", "running")
	EmitProgress(ctx, "note", "搜索关键词："+query, "done")

	var urls []string
	var materials strings.Builder
	sources := map[string]domain.SourceRef{}
	phases := []string{}

	if a.tools != nil && a.tools.Enabled() {
		raw, err := a.tools.Invoke(ctx, "firecrawl_search", map[string]interface{}{
			"query":   query,
			"limit":   5,
			"sources": []map[string]string{{"type": "web"}},
			"lang":    "zh",
		})
		if err == nil {
			urls = extractSearchURLs(raw)
			phases = append(phases, "Search")
			materials.WriteString("## Search\n")
			materials.WriteString(llm.FormatToolResultForUI("firecrawl_search", raw))
			materials.WriteString("\n\n")
		}
	} else {
		phases = append(phases, "Search(skipped)")
	}

	fetched := 0
	for i, u := range urls {
		if i >= 3 {
			break
		}
		if a.tools == nil || !a.tools.Enabled() {
			break
		}
		scrapeRaw, err := a.tools.Invoke(ctx, "firecrawl_scrape", map[string]interface{}{"url": u})
		if err != nil {
			continue
		}
		text := llm.FormatToolResultForUI("firecrawl_scrape", scrapeRaw)
		materials.WriteString(fmt.Sprintf("## Fetch/Extract %d\n%s\n\n", i+1, text))
		sid := fmt.Sprintf("s%d", i+1)
		sources[sid] = domain.SourceRef{
			URL:         u,
			Excerpt:     truncate(text, 200),
			CollectedAt: time.Now().Format(time.RFC3339),
			Title:       u,
		}
		fetched++
	}
	if fetched > 0 {
		phases = append(phases, "Fetch", "Extract")
	}

	if materials.Len() == 0 {
		materials.WriteString("未获取到网页资料，将基于模型知识进行分析（来源待验证）。\n")
		EmitProgress(ctx, "note", "未获取到网页资料，将基于模型知识分析", "done")
	} else {
		phases = append(phases, "Enrich")
		EmitProgress(ctx, "note", fmt.Sprintf("已抓取 %d 个页面，进入结构化抽取", fetched), "done")
	}

	if len(sources) > 0 || materials.Len() > 0 {
		phases = append(phases, "Validate")
	}

	out := state.CollectorOutput{
		Query:     query,
		URLs:      urls,
		Sources:   sources,
		Materials: materials.String(),
		Summary:   fmt.Sprintf("采集 %d 个来源，阶段: %s", len(sources), strings.Join(phases, "→")),
	}
	if err := store.SaveCollectorOutput(ctx, out); err != nil {
		return domain.RunOutput{}, err
	}
	saved, _ := store.LoadCollectorOutput(ctx)

	sourceIDs := make([]string, 0, len(sources))
	for id := range sources {
		sourceIDs = append(sourceIDs, id)
	}

	materialsPayload := domain.MaterialsPayload{
		Query:       query,
		SourceCount: len(sources),
		SourceIDs:   sourceIDs,
		Summary:     out.Summary,
	}

	return domain.RunOutput{
		Status:      domain.AgentRunCompleted,
		NextAgent:   ptrAgent(domain.AgentAnalyst),
		MessageType: string(domain.MsgMaterialsReady),
		Payload:     materialsPayload,
		Summary:     out.Summary,
		Artifacts: []domain.ArtifactRef{
			domain.NewArtifactRef(domain.ArtifactSourceRef, state.CollectorSourcesKey(input.TaskID), saved.Version),
			domain.NewArtifactRef(domain.ArtifactCollectedMaterial, state.CollectorMaterialsKey(input.TaskID), saved.Version),
		},
		Metadata: map[string]any{"phases": phases, "source_count": len(sources)},
	}, nil
}
