# 参考 · Go 后端最佳实践（小到中型生产服务）

> 参考文档，为指导「思维印记」Go 后端搭建而生成 ｜ 2026-06-24
>
> 范围：单服务 Go HTTP 后端，REST/JSON，Postgres，JWT 或 cookie 鉴权，邮箱验证，LLM 流式代理网关（密钥仅在服务端），异步评估 worker。前端是独立的 React+Vite SPA。
> 写作约定：中文叙述 + 英文技术名词/库名。每个主题给出**理由 + 选型 + 主要替代**。结论已用 2025/2026 的现状（Go 1.22+ 路由、river v0.x、sqlc/pgx、testcontainers-go 等）核对，不只凭记忆。
>
> 一句话原则：**标准库优先，能不加依赖就不加；该用成熟库时直接用，别自己造。** 这是一个 demo→小型生产的服务，刻意保持「无 Redis、单二进制、单 Postgres」。

---

## 1. 项目布局（Project Layout）

**理由：** `golang-standards/project-layout` 是社区约定俗成的参考，但它对单服务后端是**过度工程**——`pkg/` / `api/` / `scripts/` 那一套是给大型多模块仓库的。对本项目只取三个真正有价值的约定：`cmd/`（入口）、`internal/`（编译器强制的私有边界，外部模块 import 不到）、`migrations/`（SQL 迁移）。其余按「领域 + 层」放在 `internal/` 下。

**选型：** `cmd/ + internal/`，**不**照搬完整 layout。主要替代：完全扁平（适合 <10 文件的玩具）或完整 layout（适合多服务 monorepo）——本项目都不需要。

本项目建议目录树（后端独立目录，与现有 `apps/web` 平行）：

```
apps/api/
├── cmd/
│   └── api/
│       └── main.go              # 唯一入口：装配依赖、启动 HTTP server + river worker
├── internal/
│   ├── config/                  # env 加载 → Config struct（见 §3）
│   ├── http/                    # 路由、middleware、handler、JSON 信封（见 §2/§9/§12）
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── handlers/            # task.go / auth.go / llm.go / eval.go ...
│   ├── store/                   # 数据访问层（见 §4）
│   │   ├── db.go                # pgxpool 初始化
│   │   ├── queries/             # *.sql —— sqlc 的输入
│   │   └── sqlc/                # sqlc 生成的 Go（不手改）
│   ├── auth/                    # 密码哈希、token、session、CSRF（见 §5）
│   ├── email/                   # Mailer 接口 + smtp/resend/noop 实现（见 §6）
│   ├── llm/                     # LLM 网关：provider 适配、SSE 代理、计量（见 §8）
│   ├── eval/                    # 评估 worker：river job 定义 + 执行（见 §7）
│   └── domain/                  # 纯业务类型：Task / CardInstance / Evaluation（无依赖）
├── migrations/                  # 0001_init.sql ...（goose / golang-migrate）
├── sqlc.yaml
├── Dockerfile
├── go.mod
└── go.sum
```

要点：`internal/domain` 是纯类型（标准信封等），不 import 框架；`store` / `http` / `llm` 依赖它，而非反过来。`cmd/api/main.go` 是唯一把它们「接线」起来的地方（手写依赖注入，不要引 DI 框架）。

---

## 2. HTTP 路由（Routing）

**理由：** Go 1.22 给 `net/http.ServeMux` 加了方法匹配和路径通配（`GET /tasks/{id}`），社区共识是：**对绝大多数服务，标准库已经够用**，第三方 router 的核心卖点（method routing、path params）被吸收进了标准库。chi 仍有优势的地方是：`r.Route` 子路由分组、`r.Use` 中间件链的人机工程、以及一大批现成 middleware——但这些都能用标准库 + 几十行自写 middleware 链补上。

**选型：标准库 `net/http` ServeMux（Go 1.22+）。** 用一个极薄的 middleware 链辅助（`func(http.Handler) http.Handler` 组合）。主要替代：**chi**（如果嫌手写分组/中间件啰嗦，chi 是最贴近标准库、最不「框架味」的选择，可无痛升级）。**不要**用 gin / echo——它们引入自有 `Context` 类型，把整个 handler 生态绑死，对这个规模是负担。

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /api/tasks", h.CreateTask)
mux.HandleFunc("GET /api/tasks/{id}", h.GetTask)
mux.HandleFunc("POST /api/llm/chat", h.LLMChat) // SSE
// id := r.PathValue("id")

handler := chain(mux, RequestID, RealIP, Recover, Logger, CORS) // 见 §9
srv := &http.Server{Addr: ":8080", Handler: handler,
    ReadHeaderTimeout: 5 * time.Second}
```

---

## 3. 配置（Config）

**理由：** 严格遵守 12-factor「config 存在环境变量里」。`.env` 文件只是**开发便利**，绝不进 git，也不在生产用。Viper 功能强但对「一把 env → 一个 typed struct」是杀鸡用牛刀，还会鼓励文件优先的反模式。

**选型：`caarlos0/env`（v11+）** 把环境变量解析进一个带 `env:` tag 的 struct，配合 **`joho/godotenv`** 仅在本地加载 `.env.local`。主要替代：`kelseyhightower/envconfig`（老牌、稳定，等价选择）或纯 `os.Getenv`（变量一多就乏味、易漏）。

**密钥处理：** `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` / `JWT_SECRET` / `DATABASE_URL` / SMTP 凭据全部从环境读。`.gitignore` 里加 `.env*`（保留 `.env.example` 作为文档）。生产用平台的 secret 管理（Fly secrets / Railway vars / Docker secrets）。启动时**fail-fast**：缺关键变量直接退出，别让服务半残运行。

```go
type Config struct {
    Port          string `env:"PORT" envDefault:"8080"`
    DatabaseURL   string `env:"DATABASE_URL,required"`
    JWTSecret     string `env:"JWT_SECRET,required"`     // 至少 32 字节随机
    AnthropicKey  string `env:"ANTHROPIC_API_KEY,required"`
    DeepSeekKey   string `env:"DEEPSEEK_API_KEY"`
    SMTPHost      string `env:"SMTP_HOST"`
    CORSOrigins   []string `env:"CORS_ORIGINS" envSeparator:","`
}
func Load() (Config, error) {
    _ = godotenv.Load(".env.local") // 本地有则加载，没有不报错
    return env.ParseAs[Config]()
}
```

---

## 4. 数据库访问（Database）

**理由：** `pgx` 是 Postgres 的事实标准驱动，原生支持 PG 类型（jsonb、arrays、`COPY`、`LISTEN/NOTIFY`），比 `database/sql` 少一层抽象损耗，基准上比 GORM/sqlx 快 30–50%。查询层选 **sqlc**：你写原生 SQL，它在编译期生成 type-safe 的 Go 函数和 struct——schema/query 一变，代码就编不过。这正好契合「正确性 > 便利」的服务，也避开 ORM 在 CTE/窗口函数/jsonb 上的笨拙（本项目过程树、标准信封都是 jsonb）。

**选型：`pgx/v5`（用 `pgxpool` 连接池）+ `sqlc`。** 主要替代：`Bun`（轻量 ORM，比 GORM 干净，需要动态查询时考虑）；GORM 仅适合快速原型，不推荐用于这里。

**迁移：`goose`。** 单文件 up/down SQL，可嵌入二进制，CLI 简单。主要替代：`golang-migrate`（等价、生态更大）；`Atlas`（声明式 + 自动 diff，团队大或 schema 复杂时强，但对本项目偏重）。

**连接池：** 用 `pgxpool.New(ctx, dsn)`，设 `MaxConns`（小服务 10–25 足够），所有查询带 `context`（超时见 §9）。river（§7）可与应用**共用同一个 pgxpool**。

```go
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
// sqlc 生成的用法：
q := sqlc.New(pool)
task, err := q.GetTask(ctx, taskID)
```

---

## 5. 鉴权（Auth）

**理由：** 前端是同源/独立 SPA，且本服务要求「即时可撤销」（封号、登出立即失效）——这是 **session cookie 优于无状态 JWT** 的典型场景。无状态 JWT 的撤销要靠黑名单，等于又退回 DB 查询，得不偿失。密码哈希按 OWASP / NIST SP 800-63B（2025）首选 **argon2id**（memory-hard，抗 GPU/ASIC）。

**选型：**
- **密码哈希：`argon2id`**（`golang.org/x/crypto/argon2`，或封装好的 `alexedwards/argon2id`）。主要替代：`bcrypt`（`x/crypto/bcrypt`，cost≥12）——更简单、依然安全，是稳妥的保底。
- **Session：服务端 session + httpOnly cookie。** session 记录存 Postgres（`session` 表：id、user_id、expires_at），cookie 存不可猜的随机 session id。Cookie flags：`HttpOnly; Secure; SameSite=Lax`（Lax 足以挡多数 CSRF 且不破常规导航；纯 API 跨站表单时考虑 Strict）。主要替代：JWT access(短 15min)+refresh(httpOnly cookie) 双 token——只有当你确实要无状态横向扩展时才值得这套复杂度。
- **邮箱验证 token：DB 里存随机 token + TTL。** 生成 32 字节 `crypto/rand` → base64url，存 `email_verification`（token_hash、user_id、expires_at，24h 过期），**存哈希不存明文**（防 DB 泄露即冒用）。点链接 → 查未过期 → 标记 verified → 删 token。主要替代：HMAC 签名的无状态 token（省一张表，但无法主动作废、无法做一次性）——DB 方案更可控，推荐。
- **CSRF：** 用 cookie 鉴权就必须防 CSRF。`SameSite=Lax` + 对所有改状态请求校验 **double-submit cookie**（或 `gorilla/csrf` / Go 1.25 起标准库的 `http.CrossOriginProtection`）。SPA 从 cookie 读 CSRF token 放进自定义 header。
- **限流：** 登录、注册、发验证邮件、忘记密码这些端点要按 IP + 账号限流（`golang.org/x/time/rate` 或 `ulule/limiter`），抗暴力破解和邮件轰炸。

---

## 6. 发送邮件（Email）

**理由：** 自建 SMTP 投递率惨（40–60% 进收件箱 vs 服务商 90–99%），且要操心 SPF/DKIM/反垃圾。关键是**把发信抽象成接口**，这样测试用假实现、本地用打印实现、生产换 provider 都不动业务代码。

**选型：定义 `Mailer` 接口**，提供三种实现：
- 开发：`LogMailer`（把邮件打到 stdout / 写文件）或 `Mailtrap`（捕获不真发）。
- 生产：**Resend**（DX 最好，TypeScript/Go SDK 干净，免费 3000 封/月，HTTP API 无需管 SMTP）。主要替代：**Postmark**（投递最快、事务/营销隔离，适合「验证邮件绝不能丢」的硬需求）；**AWS SES**（量大最便宜，但 sandbox 转生产审批麻烦）。
- 兜底：`net/smtp` 实现一份，便于接任何 SMTP 服务，零供应商锁定。

```go
type Mailer interface {
    Send(ctx context.Context, to, subject, htmlBody string) error
}
// 业务里只依赖接口；main.go 按 env 选实现；测试注入 fake
```

发信本身走异步（§7）：注册时入队一个 `SendVerificationEmail` job，HTTP 请求不被 SMTP 延迟阻塞。

---

## 7. 异步 / 后台任务（Async Jobs）

**理由：** 评估 worker 是典型「提交后慢慢算」的场景，需要重试、可观测、崩溃不丢任务。既然已经有 Postgres，**最大化收益是别引入 Redis**。`river` 用 Postgres 做队列，可与业务**在同一事务里入队**——「事务提交了，job 一定入队；事务回滚了，job 一定不入队」，从根上消除「写了库没发任务 / 发了任务库没写」的不一致。这正是评估这种「跟着某条记录走」的任务最需要的保证。

**选型：`river`（riverqueue，Postgres-backed）。** 内建重试（指数退避）、唯一任务、定时/cron、Web UI、与 pgx 同池。主要替代：`asynq`（Redis-backed，UI/CLI 成熟，但要多养一个 Redis；本项目刻意避开）；最简方案「goroutine + DB 轮询表」（够用但重试/并发/可观测都得自己写，river 已经替你写好了，不建议重造）。

```go
// 定义 job
type EvalArgs struct{ TaskID string `json:"task_id"` }
func (EvalArgs) Kind() string { return "evaluate_task" }

// 与业务同事务入队（关键）
tx, _ := pool.Begin(ctx)
// ... 写 task / 落标准信封 ...
_, err = riverClient.InsertTx(ctx, tx, EvalArgs{TaskID: id}, nil)
tx.Commit(ctx) // job 与数据一起提交，原子

// worker（在 cmd/api 里和 HTTP server 一起启动，或单独进程）
type EvalWorker struct{ river.WorkerDefaults[EvalArgs] }
func (w *EvalWorker) Work(ctx context.Context, job *river.Job[EvalArgs]) error {
    // 调旗舰模型跑 rubric；返回 error 会自动重试
}
```

- **重试：** river 默认指数退避重试；`Work` 返回 error 即重试，可自定义 `MaxAttempts`。
- **幂等：** worker 必须可重入——评估前先查「该 task 是否已有 evaluation」，有则跳过/覆盖；用唯一任务（`InsertOpts{UniqueOpts}`）防同一 task 重复入队。
- **前端查状态：** 评估结果写 `evaluation` 表并带 `status`（pending/running/done/failed）。前端**轮询** `GET /api/tasks/{id}/evaluation` 直到 done（简单、无状态、对 demo 足够）；若想要实时，可让该端点用 SSE（§8）推进度。优先轮询，别为这个上 WebSocket。

---

## 8. LLM 代理 / 流式（LLM Proxy & Streaming）

**理由：** 客户端**绝不**直连模型——key 只能在服务端，且要记录档位/token/成本（这是 PRD 的硬约束）。陪练回复要流式（SSE）以降低首字延迟。Go 做 SSE 代理的关键三点：及时 `Flush`、跟随客户端断开取消上游、设对超时。

**选型：标准库 `net/http` 手写 SSE 代理**（不需要框架）。对 Anthropic/DeepSeek 用各自 HTTP API（或官方 Go SDK），后端按 provider 配 `baseURL + model + key`。

实现要点：
- 响应头：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`、`X-Accel-Buffering: no`（告诉 Nginx 等别缓冲）。
- 每写一个 event 后 `flusher.Flush()`，否则被 Go 的 buffer 攒住，失去实时性。
- **上下文取消：** 用 `r.Context()`，客户端断开时 `Done()` 触发；把它传给上游请求，客户端一走就掐断对模型的调用（省钱）。
- **超时：** SSE 端点不能套全局 `WriteTimeout`（会掐断长流）——对流式 handler 单独处理，靠 `context.WithTimeout` 控整体上限，靠心跳（每 ~20s 发注释行 `:\n\n`）防中间代理掐空闲连接。
- **计量：** 流结束时（或读 usage event）记录 `model_tier`、`prompt_tokens`、`completion_tokens`、估算 `cost`，连同 task_id/message_id 落库。陪练走中档可降级，评估走旗舰不降级——档位在请求层决定并记录。

```go
func (h *Handler) LLMChat(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("X-Accel-Buffering", "no")
    flusher, ok := w.(http.Flusher)
    if !ok { http.Error(w, "stream unsupported", 500); return }

    ctx := r.Context() // 客户端断开即取消上游
    stream, err := h.llm.Chat(ctx, req) // key 在 h.llm 内，绝不出服务端
    // ...
    for chunk := range stream {
        fmt.Fprintf(w, "data: %s\n\n", chunk.JSON())
        flusher.Flush()
    }
    h.store.RecordUsage(ctx, usage) // 档位 + token + 成本
}
```

---

## 9. 中间件与横切关注点（Middleware & Cross-cutting）

**理由：** 这些是每个生产服务的「卫生标准」，标准库 + slog 基本全包，无需框架。

**选型（全用标准库）：**
- **结构化日志：`log/slog`**（Go 标准库，1.21+）。生产用 `slog.NewJSONHandler`。主要替代：`zerolog` / `zap`（更快，但 slog 对这个量级足够且零依赖）。
- **Request ID：** 每请求生成（`google/uuid` 或 `crypto/rand`），放进 `context` 和响应头 `X-Request-ID`，日志全程带上。
- **Panic recovery：** middleware `defer recover()`，记 stack，返 500 JSON 信封（§12），别让一个 panic 拖垮 server。
- **CORS：** SPA 是独立 origin，需要 CORS。用 `rs/cors` 或手写：`Access-Control-Allow-Origin` 精确到前端域名（**不要 `*`** + cookie 鉴权下必须 `Allow-Credentials: true` 且 origin 不能是 `*`），允许 `Content-Type, X-CSRF-Token`。
- **请求超时：** 普通端点用 `http.TimeoutHandler` 或 per-handler `context.WithTimeout`；SSE 端点豁免（见 §8）。Server 设 `ReadHeaderTimeout` 防慢连接攻击。
- **优雅关闭：** 监听 `SIGINT/SIGTERM` → `srv.Shutdown(ctx)`（停收新请求、等在途完成）+ `riverClient.Stop(ctx)`（等在途 job）+ `pool.Close()`。

```go
func GracefulShutdown(srv *http.Server, river *river.Client[pgx.Tx], pool *pgxpool.Pool) {
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    _ = srv.Shutdown(ctx)
    _ = river.Stop(ctx)
    pool.Close()
}
```

---

## 10. 请求校验（Validation）

**理由：** REST/JSON 入参必须校验，否则脏数据进库或触发上游报错。`go-playground/validator` 用 struct tag 声明规则，覆盖 90% 场景且少样板。

**选型：`go-playground/validator/v10`**（tag 声明：`validate:"required,email"`）。主要替代：手写校验函数——规则简单或要返回高度定制的逐字段错误信息时更直接，可与 validator 混用。无论哪种，**校验失败统一映射到 §12 的错误信封**（400 + 字段级 detail）。

```go
type CreateTaskReq struct {
    Title string `json:"title" validate:"required,max=200"`
    URL   string `json:"url"   validate:"omitempty,url"`
}
```

---

## 11. 测试（Testing）

**理由：** Go 的 table-driven + `httptest` 是 handler 测试的标准姿势，无需第三方框架。DB 层不能靠 mock（会和真 SQL 漂移），要打真 Postgres。

**选型：**
- **单元/handler：** 标准库 `testing` + **table-driven** + `net/http/httptest`（`httptest.NewRecorder` + `httptest.NewServer`）。断言可选 `stretchr/testify`（`assert`/`require`，纯糖，团队习惯即用）。
- **DB / 集成：`testcontainers-go`**（其 `postgres` 模块：`WithImage/WithDatabase/WithUsername`，自动起容器、迁移、用完即焚）。它比 dockertest API 更干净、有技术专属模块，是 2025 的主流。主要替代：`dockertest`（回调式、等价可用）；或 CI 里起一个共享 Postgres service（更快但隔离差）。
- **建议：** 对 sqlc 生成的查询写集成测试（真 PG 跑真 SQL）；对 handler 用 `httptest` + 注入 fake store / fake mailer（§6 的接口在这里回本）；对 LLM 网关注入 fake provider 验 SSE 拼帧与计量落库。

```go
func TestCreateTask(t *testing.T) {
    tests := []struct{ name, body string; want int }{
        {"ok", `{"title":"x"}`, 201},
        {"empty title", `{"title":""}`, 400},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(tt.body))
            rec := httptest.NewRecorder()
            h.ServeHTTP(rec, req)
            if rec.Code != tt.want { t.Fatalf("got %d want %d", rec.Code, tt.want) }
        })
    }
}
```

---

## 12. 错误处理与 API 响应（Errors & Responses）

**理由：** 一致的 JSON 错误信封让前端只写一套错误处理；内部错误**绝不直接吐给客户端**（泄露栈/SQL/路径是安全风险）。

**选型：统一错误信封 + 内部错误→状态码的集中映射。**

```jsonc
// 成功：直接返回数据对象或 { "data": ... }
// 错误：统一信封
{
  "error": {
    "code": "validation_failed",      // 机器可读、稳定
    "message": "标题不能为空",          // 人可读、可直接展示
    "details": [ { "field": "title", "issue": "required" } ] // 可选，字段级
  }
}
```

做法：定义 `APIError{ Status int; Code, Message string }`（实现 `error`）。handler 返回 `error`，由一层 wrapper / middleware 集中转译：已知 `APIError` 按其 status；`pgx.ErrNoRows` → 404；validator 错误 → 400 + details；其余一律 500 + 通用文案，**真实 error 只写进 slog（带 request id），不进响应体**。状态码：200/201、400（校验）、401（未登录）、403（无权）、404、409（冲突，如邮箱已注册）、429（限流）、500（内部）。

---

## 13. 可观测性与运维基础（Observability & Ops）

**理由：** 容器编排和负载均衡靠健康检查路由流量；多阶段构建让镜像小且无构建链残留。

**选型：**
- **健康检查：** `GET /healthz`（liveness，进程活着即 200）+ `GET /readyz`（readiness，ping Postgres、检查 river 可用，依赖没就绪返 503）。两者分开，别合并。
- **优雅关闭：** 见 §9（SIGTERM → drain HTTP + river + pool）。
- **Dockerfile：多阶段构建。** builder 阶段编译静态二进制，runtime 用 `gcr.io/distroless/static` 或 `alpine`，镜像 ~10–20MB、无 shell、攻击面小。

```dockerfile
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

- **部署：** 单二进制 + 单 Postgres，适合 **Fly.io / Railway / Render**（托管 Postgres、env secrets、零运维）这类 PaaS；也可裸 VM + systemd 或任意容器平台。无 Redis、无额外 worker 进程（river worker 可与 HTTP server 同进程起，量大了再拆）。
- **指标/追踪（可选，后期）：** 需要时加 OpenTelemetry + Prometheus `/metrics`；demo 阶段 slog 结构化日志 + request id 已足够排障。

---

## 推荐技术栈一览（Recommended Stack at a Glance）

| # | 主题 | 选型 | 主要替代 |
|---|---|---|---|
| 1 | 项目布局 | `cmd/` + `internal/`（务实裁剪，非完整 layout） | 完整 golang-standards layout（过重）/ 扁平（过简） |
| 2 | HTTP 路由 | 标准库 `net/http` ServeMux（Go 1.22+） | chi（贴近标准库）；不用 gin/echo |
| 3 | 配置 | `caarlos0/env` + `joho/godotenv`（仅本地） | `kelseyhightower/envconfig` / 纯 os.Getenv；不用 Viper |
| 4 | 数据库 | `pgx/v5` + `pgxpool` + `sqlc`；迁移 `goose` | Bun（轻 ORM）；`golang-migrate`/`Atlas`；不用 GORM |
| 5 | 鉴权 | argon2id + 服务端 session + httpOnly cookie + DB 邮箱 token | bcrypt；JWT access/refresh；签名 token |
| 6 | 邮件 | `Mailer` 接口；dev=Log/Mailtrap，prod=Resend | Postmark（投递优先）/ SES（成本优先）/ net/smtp 兜底 |
| 7 | 异步任务 | `river`（Postgres-backed，与业务同事务入队） | asynq（需 Redis）/ 自写 DB 轮询 |
| 8 | LLM 流式代理 | 标准库 SSE 手写代理 + 上下文取消 + 计量落库 | 框架封装（不必要） |
| 9 | 中间件/横切 | `log/slog` + request id + recover + `rs/cors` + 优雅关闭 | zerolog/zap |
| 10 | 校验 | `go-playground/validator/v10` | 手写校验函数 |
| 11 | 测试 | `testing` table-driven + `httptest` + `testcontainers-go` | dockertest / 共享 PG service；testify（可选糖） |
| 12 | 错误/响应 | 统一 JSON 错误信封 + 集中 error→status 映射 | — |
| 13 | 运维 | `/healthz`+`/readyz` + 多阶段 distroless Dockerfile + PaaS | OpenTelemetry/Prometheus（后期） |

> 一致主线：**Postgres 是唯一有状态依赖**（业务数据 + 队列 + session 全在它身上），**标准库做骨架**，第三方只在真正省事处引入（pgx/sqlc、river、validator、env）。这样 demo 能跑、小型生产能扛、心智负担最低。
