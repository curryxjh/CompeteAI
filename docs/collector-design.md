# Collector Agent 详细设计

Collector 是整个系统的数据入口。输入是 Coordinator 确认后的竞品列表，输出是结构化的竞品数据（符合 Schema）。评分标准中"信息溯源"和"采集合规"直接落脚在这个 Agent。

## 1. 内部 Pipeline（多阶段流水线）

Collector 不是一次 LLM 调用，而是内部包含一个多阶段流水线：

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│ 1.Search │──→│ 2.Fetch  │──→│ 3.Extract│──→│ 4.Enrich │──→│ 5.Validate│
│  搜索发现 │   │  页面抓取 │   │ 结构化提取│   │ 知识补充 │   │  校验去重 │
└──────────┘   └──────────┘   └──────────┘   └──────────┘   └──────────┘
     │              │              │              │              │
     ▼              ▼              ▼              ▼              ▼
  搜索API       go-rod/colly   LLM解析HTML    go-rag+Milvus   规则+LLM
  获取URL列表   渲染+抓取       → Schema化     语义检索补充     交叉验证
```

## 2. 各阶段详解

### Stage 1: Search（搜索发现）

**目标**：根据竞品名称，获取相关信息的 URL 列表。

**实现方案**：

```
输入: competitor = { name: "Cursor", website: "https://cursor.com" }
      search_aspects = ["功能特性", "定价", "用户评价", "技术架构"]

输出: [{ url: "https://...", title: "...", snippet: "...", relevance: 0.9 }, ...]
```

| 搜索目标 | 搜索 Query 示例 | 预期来源 |
|---|---|---|
| 官网信息 | `Cursor AI editor features pricing` | cursor.com |
| 用户评价 | `Cursor vs Copilot review 2025` | 知乎、Reddit、V2EX、ProductHunt |
| 技术分析 | `Cursor AI editor architecture tech stack` | 技术博客、GitHub |
| 定价信息 | `Cursor pricing plans subscription` | 官网/pricing 页面 |
| 竞品对比 | `Cursor vs Windsurf vs Copilot comparison` | 对比评测文章 |

**Go 实现**：

```go
type SearchStage struct {
    llmClient   *llm.Client   // 豆包 API，让 LLM 生成搜索 query
    searchTool  SearchTool    // 搜索接口（可替换实现）
}

type SearchTool interface {
    Search(ctx context.Context, query string, num int) ([]SearchResult, error)
}

type SearchResult struct {
    URL         string  `json:"url"`
    Title       string  `json:"title"`
    Snippet     string  `json:"snippet"`
    Relevance   float64 `json:"relevance"`
}
```

**SearchTool 的可替换实现**：

- **方案 A**（推荐）：让 LLM 使用 `web_search` tool（豆包支持 function calling + web_search），直接返回搜索结果
- **方案 B**：调用 SerpAPI / Bing Search API，Go 的 `net/http` 直接调
- **方案 C**（降级）：用预设 URL 列表 + go-rag 做本地文档检索，不依赖外部搜索 API

> 方案 A 最简：无需额外 API Key，豆包原生支持。方案 C 可做 Demo 回放模式（预采集数据），应对演示时网络不稳定。

**合规检查**（在搜索阶段预先过滤）：

```go
func (s *SearchStage) filterByCompliance(urls []SearchResult) []SearchResult {
    var filtered []SearchResult
    for _, u := range urls {
        // 1. 排除已知不合规站点（内网、登录墙后）
        if isRestrictedDomain(u.URL) { continue }
        // 2. 优先级：官方来源 > 权威评测 > 社区讨论
        u.Relevance = s.scoreSourceAuthority(u.URL)
        filtered = append(filtered, u)
    }
    return filtered
}
```

### Stage 2: Fetch（页面抓取）

**目标**：访问 Stage 1 获取的 URL，抓取页面内容。

**Go 技术选型**：

| 工具 | 适用场景 | 说明 |
|---|---|---|
| `net/http` + `goquery` | 静态 HTML 页面 | 轻量，适合大部分官网和博客 |
| `go-rod` | JavaScript 渲染页面（SPA） | Headless Chrome 驱动，适合 React/Vue 渲染的页面 |
| `colly` | 批量爬取 | 内置速率控制、robot.txt 检查、并发管理 |

**实现**：

```go
type FetchStage struct {
    httpClient  *http.Client       // 静态页面
    rodClient   *rod.Browser       // JS 渲染（按需启动，资源重）
    colly       *colly.Collector   // 批量爬取
    rateLimiter *RateLimiter       // 速率控制
}

func (f *FetchStage) Fetch(ctx context.Context, url string) (*FetchedPage, error) {
    // 1. 检查 robots.txt（评分合规项）
    if !f.allowByRobots(url) {
        return nil, fmt.Errorf("blocked by robots.txt: %s", url)
    }

    // 2. 速率控制：同域名最少间隔 2s
    f.rateLimiter.Wait(url)

    // 3. 先尝试静态抓取（快）
    content, err := f.fetchStatic(ctx, url)
    if err == nil && len(content) > 500 {  // 内容够长，说明不是 SPA 壳
        return content, nil
    }

    // 4. 静态抓取内容过短 → 用 go-rod 渲染 JS
    return f.fetchWithRod(ctx, url)
}

type FetchedPage struct {
    URL         string    `json:"url"`
    Title       string    `json:"title"`
    HTML        string    `json:"html"`
    Text        string    `json:"text"`         // 提取后的纯文本
    FetchAt     time.Time `json:"fetch_at"`
    ContentType string    `json:"content_type"`
}
```

### Stage 3: Extract（结构化提取 — 核心）

**目标**：将抓取的 HTML/文本，用 LLM 提取为符合 Schema 的结构化 JSON。这是整个采集流程最关键的一步。

**为什么用 LLM 而非规则提取？**
- 每个竞品网站结构不同，写 CSS selector 规则不可扩展
- LLM 能理解语义，自动识别"这段文字描述的是定价信息还是功能特性"
- 豆包 API 支持 JSON Mode，可直接约束输出格式

**实现**：

```go
type ExtractStage struct {
    llmClient *llm.Client
    prompt    *PromptTemplate   // prompts/collector_extract.txt
}

func (e *ExtractStage) Extract(ctx context.Context, page *FetchedPage) (*ExtractedData, error) {
    // 1. 文本过长时分片处理
    chunks := e.splitText(page.Text, maxChunkSize)  // 每片 ~8K tokens

    // 2. 逐片调用 LLM 提取
    var allResults []*ExtractedData
    for _, chunk := range chunks {
        result, err := e.extractChunk(ctx, chunk, page.URL)
        if err != nil { continue }
        allResults = append(allResults, result)
    }

    // 3. 合并去重（同一竞品的多条信息合并）
    return e.merge(allResults), nil
}
```

**Extract Prompt 设计要点**（`prompts/collector_extract.txt`）：

```text
你是一个竞品信息提取器。从以下网页内容中提取竞品信息，输出严格符合 JSON Schema 的结构化数据。

## 输出 Schema
{
  "product_name": "产品名称",
  "features": [{ "name": "功能名", "category": "分类", "description": "描述", "source_text": "原文摘录" }],
  "pricing": { "free_tier": "...", "paid_tiers": [...] },
  "target_users": ["用户类型1"],
  "differentiators": ["差异化特点"]
}

## 关键规则
1. 每条提取的信息必须包含 source_text（原文摘录），不可凭空编造
2. 如果网页中没有某个字段的信息，该字段留空，不要编造
3. 不确定的信息标记 confidence: "low"

## 网页内容
{chunk_text}
```

### Stage 4: Enrich（知识补充 — RAG）

**目标**：用已采集的文档库做语义检索，补充 Stage 3 可能遗漏的信息。

```
已采集文档 → go-rag 分块 → Milvus 存储向量 → 语义检索 → 补充到结构化数据
```

**实现**：

```go
type EnrichStage struct {
    ragClient   *rag.Client        // go-rag
    embedder    *Embedder          // Embedding 生成（豆包 Embedding API）
    milvusCli   *milvus.Client     // Milvus 向量检索
}

func (e *EnrichStage) Enrich(ctx context.Context, extracted *ExtractedData) error {
    // 1. 对每次采集的文档做分块 + Embedding + 存入 Milvus
    for _, page := range extracted.SourcePages {
        chunks := e.ragClient.Split(page.Text, chunkSize, overlap)
        embeddings := e.embedder.Embed(ctx, chunks)
        e.milvusCli.Insert(ctx, "competitor_docs", embeddings, chunks)
    }

    // 2. 用竞品名 + 关键方面做语义检索，补充遗漏信息
    queries := []string{
        extracted.ProductName + " 定价",
        extracted.ProductName + " 用户评价",
        extracted.ProductName + " 技术架构",
    }
    for _, q := range queries {
        results := e.milvusCli.Search(ctx, "competitor_docs", q, topK=5)
        // 合并新发现的信息到 extracted 中
    }
    return nil
}
```

> Enrich 阶段在首次采集时效果有限（文档库还小），但随着系统使用，已采集的文档越来越多，RAG 的效果会越来越好。演示时可以预采集一些文档入库。

### Stage 5: Validate（校验去重）

**目标**：交叉验证多源数据，标记冲突，过滤低质量信息。

```go
type ValidateStage struct {
    llmClient *llm.Client
}

type ValidationResult struct {
    Conflicts   []Conflict  `json:"conflicts"`    // 矛盾信息
    LowConfidence []string  `json:"low_confidence"` // 低置信度信息
    MissingAspects []string `json:"missing_aspects"` // 未覆盖的分析维度
}

type Conflict struct {
    Field    string   `json:"field"`     // "pricing.pro_plan.price"
    Values   []string `json:"values"`     // ["$20/month", "$25/month"]
    Sources  []string `json:"sources"`    // 各自的来源 URL
    Suggested string  `json:"suggested"`  // 建议采用的值
}
```

## 3. 采集合规策略（评分硬指标）

```go
type ComplianceChecker struct{}

func (c *ComplianceChecker) Check(url string) error {
    // 1. robots.txt 检查
    robotsURL := getRobotsURL(url)
    robots, _ := fetchRobots(robotsURL)
    if !robots.IsAllowed(url) {
        return ErrBlockedByRobots
    }

    // 2. 速率控制：同域名请求间隔 ≥ 2s
    // 3. User-Agent 标识：明确标注为自动化竞品分析研究用途
    // 4. 不爬登录墙后内容
    // 5. 不采集用户个人隐私数据（评分明确要求数据脱敏）
    return nil
}
```

## 4. Collector 输出到 Shared State

```go
// 写入 Shared State，供下游 Agent 读取
type CollectorOutput struct {
    TaskID      string                `json:"task_id"`
    Products    []*CompetitorProfile  `json:"products"`      // 符合 Schema
    SourcePages []*FetchedPage        `json:"source_pages"`  // 原始页面快照
    TraceRefs   []TraceRef            `json:"trace_refs"`    // 溯源引用（每条结论 → URL）
    Validated   *ValidationResult     `json:"validated"`     // 校验结果
    CollectedAt time.Time             `json:"collected_at"`
    Stats       CollectStats          `json:"stats"`         // 采集统计
}

type CollectStats struct {
    PagesFetched   int `json:"pages_fetched"`
    PagesExtracted int `json:"pages_extracted"`
    ConflictsFound int `json:"conflicts_found"`
    DurationMs     int64 `json:"duration_ms"`
    TokenUsed      int `json:"token_used"`
}
```

## 5. 降级与回放模式

为了应对演示时网络不稳定或 API 限流，Collector 支持两种运行模式：

| 模式 | 数据来源 | 适用场景 |
|---|---|---|
| **Live 模式**（默认） | 实时搜索 + 抓取 | 正常使用 |
| **Replay 模式** | 从 MySQL 读取预采集的 FetchedPage 快照，直接进入 Extract 阶段 | Demo 演示、网络不稳定时 |

```go
type CollectorAgent struct {
    mode       CollectMode  // "live" / "replay"
    search     *SearchStage
    fetch      *FetchStage
    extract    *ExtractStage
    enrich     *EnrichStage
    validate   *ValidateStage
    pageStore  *PageStore   // MySQL 存储的预采集页面
}

func (a *CollectorAgent) Run(ctx context.Context, task *Task) (*CollectorOutput, error) {
    switch a.mode {
    case ModeLive:
        return a.runLive(ctx, task)
    case ModeReplay:
        return a.runReplay(ctx, task)  // 从 pageStore 加载预采集数据
    }
}
```
