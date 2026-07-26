# 部署分册 · Web（单个角色路由 SPA）

`apps/web` 是一份构建产物，学生 / 教师 / 管理员共用。构建时把 API origin 打进 bundle
（`VITE_API_BASE_URL`），运行时由容器内 nginx 静态托管，对外通过宿主机 nginx 反代
`mind-web.uni-robot.cn` → `127.0.0.1:8091`。

## 关键点

- **构建上下文是 repo 根**（SPA 依赖 `@mind-imprint/contracts` workspace 包）。compose 已配置
  `build.context: ..` + `dockerfile: apps/web/Dockerfile`，无需手动处理。
- `VITE_API_BASE_URL` 在**构建期**注入，改它必须**重新构建** web 镜像。
- 必须先让 `https://mind-api.uni-robot.cn` 可用（TLS 就绪），SPA 才能成功携带凭证调用 API。
  推荐顺序：API + 其 TLS 就绪 → 构建 web → web TLS（见 `tls.md`）。

## 步骤

`deploy/.env.prod` 里应已含：

```
VITE_API_BASE_URL=https://mind-api.uni-robot.cn
```

构建并启动 web 容器：

```bash
cd ~/mind-imprint
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build web
```

校验（容器内直连）：

```bash
curl -sS -I http://127.0.0.1:8091/            # 期望 200，Content-Type text/html
curl -sS http://127.0.0.1:8091/ | grep -o '<title>[^<]*'   # 看到应用标题
```

确认 bundle 里打进了正确的 API origin：

```bash
docker compose -f deploy/docker-compose.prod.yml exec web \
  sh -c "grep -ro 'mind-api.uni-robot.cn' /usr/share/nginx/html/assets | head -1"
```

## 内存注意

vite 构建峰值可能到 1–2 GB。ECS 已加 4 GB swap 兜底。若构建 OOM（`Killed` / exit 137）：

1. 单独构建、避免与其它构建并行；
2. 仍不行则把 ECS 升到 8 GB 后重试 `--build web`。

## 更新前端

```bash
git pull
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build web
```

`index.html` 设了 `no-store`，`/assets/*`（内容哈希）长缓存，客户端不会被钉在旧 bundle。
