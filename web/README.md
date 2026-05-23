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
| `/login` | 登录（对接 Go `/users/login`） |
| `/signup` | 注册（对接 Go `/users/signup`） |

## 认证联调

1. 启动 Go 后端：`go run .`（端口 **8084**，需 MySQL + Redis）
2. 启动前端：`npm run dev`
3. 浏览器打开 `/signup` 注册，或 `/login` 登录
4. 登录成功后 Token 存于 `localStorage`，后续 `/api/*` 请求自动带 `Authorization`
5. 顶栏可 **测试 Ping**（`GET /ping`，需 JWT）和 **退出登录**

Vite 代理：`/api`、`/users`、`/ping` → `http://localhost:8084`

- `VITE_USE_MOCK=true`（默认）：任务等页面仍用 Mock，**不强制登录**
- `VITE_USE_MOCK=false`：访问主站需先登录，业务 API 会带 Token

```bash
cd web
npm install --legacy-peer-deps
npm run dev
```

默认 `VITE_USE_MOCK=true`，无后端也可演示。联调 Go API 时在 `.env.development` 设置：

```
VITE_USE_MOCK=false
```

Vite 已将 `/api`、`/users`、`/ping` 代理到 `http://localhost:8084`。

## 构建

```bash
npm run build
```
