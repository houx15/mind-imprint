# 部署分册 · API（Go）

后端是唯一持有密钥、访问数据库和模型的单元。容器监听 `:8080`，通过 compose 只发布到
`127.0.0.1:8090`（对外由宿主机 nginx 反代，见 `tls.md`）。

## 环境变量（`apps/api/internal/config/config.go` 为准）

| 变量 | 必填 | 说明 |
|---|---|---|
| `DATABASE_URL` | ✅ | Postgres DSN（compose 里由 `POSTGRES_PASSWORD` 拼出，指向 `db:5432`）|
| `CORS_ORIGINS` | ✅ | 允许携带凭证的 SPA origin，逗号分隔 → `https://mind-web.uni-robot.cn` |
| `DEEPSEEK_API_KEY` | ✅ | LLM 密钥（服务端）。与 `apps/api/.env.local` 同值 |
| `COOKIE_SECURE` | — | 生产恒 `true`（compose 已硬编码）|
| `ANTHROPIC_API_KEY` | — | 可选备用 provider |
| `VOICE_APP_ID` / `VOICE_ACCESS_KEY` / `VOICE_TTS_VOICE` | — | 语音（Volcano）。留空则语音路由 503，平台照常启动 |
| `PORT` | — | 默认 `8080` |

> `SESSION_SECRET`、`SMTP_*` 出现在旧的 `.env.local` 里，但 **当前 config 并不读取**（会话是 DB
> 内不透明 token），无需在生产提供。

## 步骤

在 **repo 根目录** `/home/deploy/mind-imprint` 执行。

### 1. 拉取最新代码

```bash
cd ~/mind-imprint
git checkout main
git pull
```

### 2. 写入密钥文件（不进 git）

```bash
cp deploy/env.prod.example deploy/.env.prod
# 编辑 deploy/.env.prod，至少填写：
#   POSTGRES_PASSWORD=<强口令>
#   CORS_ORIGINS=https://mind-web.uni-robot.cn
#   VITE_API_BASE_URL=https://mind-api.uni-robot.cn
#   DEEPSEEK_API_KEY=<与 apps/api/.env.local 同值>
#   VOICE_APP_ID / VOICE_ACCESS_KEY / VOICE_TTS_VOICE=<同 .env.local，可选>
```

`deploy/.env.prod` 命中 `.gitignore` 的 `.env.*`，不会被提交。

### 3. 启动数据库

```bash
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d db
# 等待 healthy
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml ps
```

### 4. 迁移 + 种子（同一条命令）

迁移与种子都嵌在二进制里（goose + `*_seed_*.sql`）。用一次性容器执行：

```bash
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml \
  run --rm --build api -migrate-up
```

期望输出 `migrations applied successfully`。这一步会建表并写入种子（示例学校/班级/学生 Phoebe、
教师 wu.teacher、管理员、示例课程与周报数据）。**幂等**：重复执行不会重复种子。

### 5. 启动 API

```bash
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build api
```

### 6. 校验（容器内直连，尚未过 nginx/TLS）

```bash
curl -sS http://127.0.0.1:8090/healthz    # 期望 200
curl -sS http://127.0.0.1:8090/readyz     # 期望 200（含 DB ping）
# 登录冒烟（种子学生）：
curl -sS -i -X POST http://127.0.0.1:8090/api/v1/auth/signin \
  -H 'Content-Type: application/json' \
  -d '{"email":"phoebe@demo.mindimprint.local","password":"phoebe-dev-pass"}'
# 期望 200 + Set-Cookie: mk_session=...
```

## 运维

```bash
# 日志
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml logs -f api
# 重启 / 重建
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build api
# 备份数据库卷
docker exec -t $(docker compose -f deploy/docker-compose.prod.yml ps -q db) \
  pg_dump -U mindimprint mindimprint > backup-$(date +%F).sql
```

数据持久化在命名卷 `mindimprint_pgdata`；`docker compose down` **不带 `-v`** 不会删数据。
