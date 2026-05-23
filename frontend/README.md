# CompeteAI Web 前端

基于 [docs/frontend-design.md](../docs/frontend-design.md) 实现的 Vue 3 + Vite 单页应用。

## 技术栈

- Vue 3 + TypeScript + Vite
- Vue Router 4、Pinia、Axios
- Element Plus、ECharts 5
- SSE（`useTaskSSE`）对接后端实时状态

## 页面

| 路由 | 页面 |
|------|------|
| `/` | Dashboard — 任务管理 |
| `/report/:taskId` | 报告查看（SWOT、功能矩阵、溯源） |
| `/trace/:taskId` | Agent 执行追踪回放 |
| `/agents` | Agent 能力与 DAG |

## 开发

```bash
cd frontend
npm install --legacy-peer-deps
npm run dev
```

默认 `VITE_USE_MOCK=true`，无后端也可演示。联调 Go API 时在 `.env.development` 设置：

```
VITE_USE_MOCK=false
```

Vite 已将 `/api` 代理到 `http://localhost:8080`。

## 构建

```bash
npm run build
```
