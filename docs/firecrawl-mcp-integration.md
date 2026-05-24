# Firecrawl MCP 接入指南

本文档记录 CompeteAI 项目将 **Firecrawl MCP Server** 接入 **Eino 框架** 的完整流程，包括架构设计、配置方式、代码结构与验证方法。

---

## 1. 背景与目标

CompeteAI 的 Collector Agent 需要联网搜索与网页抓取能力。Firecrawl 提供托管的 MCP Server，可将 URL 转为 LLM 友好的 Markdown / JSON，并支持 search、crawl、extract 等工具。

**接入目标：**

- CompeteAI 作为 **MCP Host（客户端）**，通过 stdio 拉起 Firecrawl MCP 子进程
- 使用 Eino 官方 MCP 组件将 Firecrawl 工具暴露为 `tool.BaseTool`
- 豆包 LLM 通过 **Tool Calling** 自动调用 `firecrawl_scrape` 等工具
- 提供 HTTP 调试接口，便于独立验证 MCP 工具

---

## 2. 方案选型

| 方案 | 说明 | 结论 |
|------|------|------|
| Path A：Firecrawl CLI | Agent 终端临时执行 `firecrawl scrape` | 不适合产品运行时 |
| Path B：REST API 直连 | Go 直接调 `api.firecrawl.dev/v2/scrape` | 可行，但需自封装 HTTP |
| Path C：自写 MCP Client + HTTP | 连 `mcp.firecrawl.dev/{key}/v2/mcp` | 实测 HTTP 406，协议不匹配 |
| **Path D：stdio + Eino 官方 MCP 组件** | `npx firecrawl-mcp` + `eino-ext/components/tool/mcp` | **最终采用** |

> 说明：部分文档提到的 `officialmcp` 包在当前 `eino-ext` 版本中不存在，实际使用的是  
> `github.com/cloudwego/eino-ext/components/tool/mcp` 的 `GetTools` API。

---

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     CompeteAI Go 后端                        │
│                                                             │
│  main.go → wire.InitWebServer()                             │
│       ↓                                                     │
│  ioc.InitFirecrawlTools()                                   │
│       ↓ stdio 子进程                                          │
│  npx -y firecrawl-mcp  ←── FIRECRAWL_API_KEY (.env)        │
│       ↓ MCP 协议 (initialize → tools/list)                  │
│  eino-ext/mcp.GetTools() → []tool.BaseTool                  │
│       ↓                                                     │
│  ToolRegistry (internal/eino/tool_registry.go)              │
│       ├─→ ChatService：绑定豆包 Tool Calling               │
│       └─→ MCPHandler：/api/mcp/tools、/api/mcp/call         │
└─────────────────────────────────────────────────────────────┘
```

**运行时数据流（Chat 对话）：**

```
用户消息
  → ChatService.generate()
  → 豆包 LLM（WithTools 绑定 Firecrawl 工具）
  → 模型返回 tool_calls（如 firecrawl_scrape）
  → ToolRegistry.Invoke() → MCP 子进程执行
  → 工具结果塞回 messages
  → 豆包继续生成最终回复（最多 5 轮 tool loop）
```

---

## 4. 依赖

```bash
go get github.com/cloudwego/eino-ext/components/tool/mcp@latest
```

间接依赖：

- `github.com/mark3labs/mcp-go` — MCP 协议客户端（stdio 传输）
- `github.com/cloudwego/eino` — Tool 接口与 Tool Calling

**运行环境：**

- Node.js（用于 `npx -y firecrawl-mcp`）
- Firecrawl API Key（环境变量 `FIRECRAWL_API_KEY`）

---

## 5. 配置

### 5.1 环境变量（`.env`）

项目启动时通过 `godotenv` 自动加载项目根目录 `.env`：

```dotenv
ARK_API_KEY="ark-xxxxxxxx"
FIRECRAWL_API_KEY="fc-xxxxxxxx"
```

| 变量 | 用途 |
|------|------|
| `ARK_API_KEY` | 豆包 LLM API Key |
| `FIRECRAWL_API_KEY` | Firecrawl MCP 子进程认证 |

### 5.2 应用配置（`config/dev.yaml`）

```yaml
firecrawl_mcp:
  enabled: true
  command: npx
  args:
    - -y
    - firecrawl-mcp
```

| 字段 | 说明 |
|------|------|
| `enabled` | 是否启用 Firecrawl MCP |
| `command` | stdio 子进程命令，默认 `npx` |
| `args` | 子进程参数，默认 `["-y", "firecrawl-mcp"]` |

对应结构体定义见 `settings/settings.go` 中的 `FirecrawlMCPConfig`。

### 5.3 Windows 注意事项

若 `npx` 启动失败，可尝试在配置中改为：

```yaml
firecrawl_mcp:
  enabled: true
  command: cmd
  args:
    - /c
    - npx -y firecrawl-mcp
```

并确保 `FIRECRAWL_API_KEY` 已在系统环境或 `.env` 中设置。

---

## 6. 代码结构

| 文件 | 职责 |
|------|------|
| `internal/eino/tool_registry.go` | stdio 连接 Firecrawl MCP；`ConnectFirecrawlMCP()`；工具注册表 |
| `internal/eino/chat_service.go` | Tool Calling 循环；调用 `ToolRegistry` 执行工具 |
| `ioc/firecrawl.go` | `InitFirecrawlTools()` 启动入口 |
| `ioc/eino.go` | `InitChatService()` 注入 ToolRegistry |
| `internal/web/mcp.go` | HTTP 调试 API |
| `wire.go` / `wire_gen.go` | 依赖注入：`InitFirecrawlTools → InitChatService → MCPHandler` |

### 6.1 核心连接逻辑

`ConnectFirecrawlMCP()` 步骤：

1. 读取 `FIRECRAWL_API_KEY`
2. `client.NewStdioMCPClient("npx", env, "-y", "firecrawl-mcp")` 拉起子进程
3. `cli.Initialize()` MCP 握手
4. `mcpp.GetTools(ctx, &mcpp.Config{Cli: cli})` 发现工具
5. 封装为 `ToolRegistry` 供 Chat 与 HTTP 使用

### 6.2 Chat Tool Calling

`ChatService` 在检测到 `ToolRegistry.Enabled()` 时：

1. 将 MCP 工具转为 `[]*schema.ToolInfo`
2. `chatModel.WithTools(infos)` 绑定给豆包
3. 模型返回 `tool_calls` 时，调用 `ToolRegistry.Invoke()`
4. 将工具结果以 `schema.Tool` 消息追加，继续生成
5. 最多 **5 轮** tool loop（`maxToolRoundTrips = 5`）

> 启用 MCP 工具后，Chat 接口走非流式路径（先完成 tool call，再一次性返回结果）。

### 6.3 降级策略

Firecrawl MCP 连接失败时：

- 记录 `firecrawl mcp init failed` 警告日志
- 返回空 `ToolRegistry`，**服务仍可正常启动**
- Chat 退化为纯 LLM 对话（无网页抓取能力）

---

## 7. HTTP 调试接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/mcp/tools` | 列出已注册的 MCP 工具 |
| POST | `/api/mcp/call` | 手动调用指定工具 |

以上路径已在 JWT 中间件中忽略鉴权（见 `ioc/web.go`）。

### 7.1 列出工具

```bash
curl http://127.0.0.1:8084/api/mcp/tools
```

成功响应示例：

```json
{
  "enabled": true,
  "tools": [
    {"name": "firecrawl_scrape", "description": "..."},
    {"name": "firecrawl_search", "description": "..."}
  ]
}
```

### 7.2 手动调用 scrape

```bash
curl -X POST http://127.0.0.1:8084/api/mcp/call \
  -H "Content-Type: application/json" \
  -d '{"name":"firecrawl_scrape","arguments":{"url":"https://firecrawl.dev"}}'
```

### 7.3 Chat 对话（自动 Tool Calling）

```bash
curl -X POST http://127.0.0.1:8084/api/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "请抓取 https://firecrawl.dev 首页并总结主要内容"}
    ]
  }'
```

豆包会自动选择 `firecrawl_scrape` 等工具完成抓取后再回复。

---

## 8. 启动与验证

### 8.1 启动

```powershell
cd "D:\Learning Materials\CompeteAI"
go run .
```

### 8.2 成功日志

控制台或 `logs/compete_ai.log` 中应出现：

```
firecrawl mcp tools ready
tools=["firecrawl_scrape","firecrawl_map","firecrawl_search", ...]
```

实测成功时会注册约 **24 个** Firecrawl 工具。

### 8.3 常见失败与排查

| 现象 | 可能原因 | 处理 |
|------|----------|------|
| `FIRECRAWL_API_KEY is not set` | `.env` 未配置或未加载 | 检查根目录 `.env`，重启服务 |
| `start firecrawl mcp: ...` | Node/npx 未安装 | 安装 Node.js，确认 `npx -v` 可用 |
| `firecrawl mcp init failed` | API Key 无效或网络问题 | 在 [firecrawl.dev](https://firecrawl.dev) 确认 Key |
| `HTTP 406 from MCP server` | 使用了旧的 HTTP 托管 MCP 方案 | 确认已切换到 stdio 方案（本文档方案） |
| Chat 不调用工具 | MCP 未就绪或模型未触发 | 先测 `/api/mcp/tools`；问题描述中明确提到 URL |

---

## 9. 与 Collector Agent 的关系

当前 MCP 接入主要服务于 **Chat 对话** 与 **HTTP 调试**。后续 Collector Agent 五阶段流水线中：

| 阶段 | 计划用法 |
|------|----------|
| Stage 1 Search | `firecrawl_search` |
| Stage 2 Fetch | `firecrawl_scrape` |
| Stage 3 Extract | 豆包 LLM 将 Markdown 转为竞品 Schema |
| Stage 4 Enrich | RAG 补充（独立模块） |
| Stage 5 Validate | 校验 source URL 与字段完整性 |

Collector 可直接复用 `ToolRegistry.Invoke()`，无需重复封装 MCP 连接。

---

## 10. 演进记录

| 阶段 | 做法 | 结果 |
|------|------|------|
| v1 | 自写 `internal/mcp/client` + HTTP 托管 URL | HTTP 406，放弃 |
| v2 | stdio + `eino-ext/components/tool/mcp` + `mark3labs/mcp-go` | 成功，24 个工具就绪 |

---

## 11. 参考链接

- [Eino MCP Tool 组件](https://github.com/cloudwego/eino-ext/tree/main/components/tool/mcp)
- [Firecrawl MCP Server](https://github.com/firecrawl/firecrawl-mcp-server)
- [MCP 协议介绍](https://modelcontextprotocol.io/introduction)
- [mcp-go SDK](https://github.com/mark3labs/mcp-go)
