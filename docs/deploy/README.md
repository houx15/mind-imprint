# 部署总览 · Mind Imprint Deployment

> 目标：把平台以 Docker 部署到一台 ECS，对外提供 HTTPS。当前阶段 ≤10 人同时使用。

## 架构（重要）

前端 **不是三个独立应用**。`apps/web` 是**一个按角色路由的 SPA**：登录后由后端返回的 `role`
决定渲染学生端（StudentApp）还是教师/管理员控制台（ConsoleShell）。因此只需部署 **两个** 对外服务：

| 对外域名 | 服务 | 说明 |
|---|---|---|
| `mind-api.uni-robot.cn` | Go API（`apps/api`） | 唯一持有密钥、访问数据库与模型的单元 |
| `mind-web.uni-robot.cn` | 单个 SPA（`apps/web`） | 学生/教师/管理员共用一份构建产物 |

`mind-stu / mind-teacher / mind-admin` 暂不需要（一份 bundle 已覆盖全部角色）。如果将来要按角色分域，
可把它们 CNAME/反代到同一个 web 容器即可，无需改代码。

## 拓扑

```
                Internet (HTTPS)
                      │
             ┌────────┴─────────┐
             │   host nginx     │   ← TLS 终止 (certbot)，与服务器上已有站点共存
             │  (47.93.151.131) │
             └───┬──────────┬───┘
   mind-api ─────┘          └───── mind-web
   127.0.0.1:8090            127.0.0.1:8091
        │                         │
  ┌─────┴─────┐             ┌─────┴──────┐
  │ api (Go)  │             │ web (nginx │
  │  :8080    │             │  static)   │
  └─────┬─────┘             └────────────┘
        │  compose network
  ┌─────┴─────┐
  │ db (pg16) │  ← 仅 compose 网络内可达，不发布主机端口
  └───────────┘
```

浏览器直接从 `https://mind-web...` 加载 SPA，API 请求直连 `https://mind-api...`（origin 已在构建时打进
bundle）。两者同属 `uni-robot.cn`（same-site），会话 Cookie 为 `SameSite=Lax; Secure; HttpOnly`，
配合 API 的 CORS 白名单（`CORS_ORIGINS`）即可跨子域携带凭证。

## 文件清单（本仓库，均不含密钥）

- `deploy/docker-compose.prod.yml` — db + api + web 三服务
- `deploy/env.prod.example` — 环境变量样例（复制为 `deploy/.env.prod` 后填真值；`.env.prod` 已被 git 忽略）
- `apps/api/Dockerfile` — Go 多阶段构建（distroless）
- `apps/web/Dockerfile` + `apps/web/nginx.conf` — Node 构建 → nginx 静态托管
- `deploy/nginx/mind-api.uni-robot.cn.conf` / `mind-web.uni-robot.cn.conf` — 宿主机 nginx 反代站点
- 运行手册：`docs/deploy/api.md`、`docs/deploy/web.md`、`docs/deploy/tls.md`

## 服务器前置条件（当前 ECS 已满足）

- Ubuntu 24.04，2 vCPU / 3.4 GB RAM（+ 已加 4 GB swap 兜底构建），Docker 29 + Compose v5
- `deploy` 用户已在 `docker` 组（无需 sudo 跑 docker）
- 宿主机已装 nginx + certbot，已托管其它站点（本部署为**增量**，勿动既有站点）
- 仓库已在 `/home/deploy/mind-imprint`，`git pull` 可用
- DNS：`mind-api` 与 `mind-web` 均已解析到 `47.93.151.131`

## 部署顺序（详见各分册）

1. **API**（`docs/deploy/api.md`）：`git pull` → 写 `deploy/.env.prod` → 起 db → 迁移+种子 → 起 api → 校验 `/healthz` `/readyz`
2. **Web**（`docs/deploy/web.md`）：以 `VITE_API_BASE_URL=https://mind-api.uni-robot.cn` 构建并启动 web 容器
3. **HTTPS**（`docs/deploy/tls.md`）：宿主机 nginx 反代两个站点 → certbot 签发证书 → 端到端校验登录

## 中国网络注意（镜像与依赖源）

服务器在国内，若首次构建报 `not found` / `gcr.io` 拉不动：

- **基础镜像**：先跑一次 `bash deploy/pull-base-images.sh`——它经 DaoCloud 代理拉取
  postgres/nginx/node/golang/distroless 并重打成规范名，构建/compose 直接用本地副本（不改宿主机 docker 配置）。
- **Go 模块**：API 的 Dockerfile 已设 `GOPROXY=https://goproxy.cn,direct`。
- **npm 包**：web 构建走 npmjs（较慢但可用）；如需提速可在构建阶段设 `npm_config_registry=https://registry.npmmirror.com`。
- **pnpm**：web 的 Dockerfile 已 `corepack prepare pnpm@10.29.3 --activate` 固定版本，确保 `pnpm-workspace.yaml`
  的 `onlyBuiltDependencies`（放行 esbuild 构建脚本）生效。

## 资源与容量

平台在本地资源上很轻——算力（LLM）外包给 DeepSeek/Volcano，Go API 多数时间在等网络。
瓶颈是**构建时内存**（vite/go build 峰值可达 1–2 GB）与 **DeepSeek 速率/成本**，不是运行时。
若构建 OOM：已加的 4 GB swap 通常足够；否则把 ECS 升到 8 GB。
