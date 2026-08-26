# 轻量版 P1（原子底座 + 阅读跑通）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一个 lite 学校的学生在独立的轻量站上新建一次「阅读」，贴进一篇文章，用与现有产品同样的 AI 引导与工具卡读完，并写下「我的收获」。

**Architecture:** 阅读是一等原子，**自持存储**——不借用 `project` 行。薄薄一层 `atom` 身份表承载共享机制（消息流 / 工具卡 / 批注 / 报告），阅读专属字段各自成表。AI 层**原样复用**：`agent.RouteReading` 等全是纯函数，输入 `ReadingRouteInput` 里没有任何 project 引用，handler 只需把值从新表装配出来。前端新建 `apps/lite-web`，跨 workspace 从 `apps/web` 引入房间组件与设计 token。

**Tech Stack:** Go 1.22+（`net/http`、`pgx`、`sqlc`、`goose`）、PostgreSQL、React + Vite + TypeScript + Tailwind、pnpm workspace、Playwright。

**Spec:** `docs/superpowers/specs/2026-08-26-lite-edition-writings-readings-design.md`

## Global Constraints

- **迁移编号从 `0092` 起**（当前最高 `0091_demo_reading_notes.sql`）。goose 两段式 `-- +goose Up` / `-- +goose Down`，Down 必须真正可回滚。
- **sqlc 重新生成：`cd apps/api && make sqlc`**（Makefile 已内置必需的 `CGO_ENABLED=0`）。**绝不手改 `internal/store/sqlc/` 下的文件。**
- 🚨 **实现者只跑定向测试**（`go test ./internal/... -run TestX -timeout 1800s`）。**绝不运行整包 Go 测试**：本仓库每个集成测试都会启动一个独立 Postgres 容器，整包要 10 分钟以上，**超过前台命令 10 分钟上限**——实现者一旦把它放后台就会丢失它、然后开始写等待脚本。整包验证由 controller 统一在后台跑。
- **绝不 `git add -A`**：每次提交只 stage 本任务明确列出的文件。
- **归属失败一律 404**（`httpx.ErrNotFound("资源不存在")`），绝不用 403 —— 既有约定，不泄漏资源存在性。
- **不碰 `/projects/*` 的任何 handler，不碰 project 的任何表。** 本期对 pro 的唯一改动是 Task 2 的路由分流包装与 Task 10 的房间组件能力对象（后者默认值保证 pro 行为不变）。
- **密钥绝不进 git / 日志 / 错误体。**
- **标准信封结构与 pro 一致**：Go 只做边界校验（`status` 枚举、id、`field_values` 为对象、`event_trace` 为数组），内层深结构真相归 `packages/contracts` 的 Zod 契约。
- **`mk-*` 是裸 CSS 变量**：`bg-mk-x/NN` 这类 Tailwind alpha 语法**不产出任何 CSS**。需要透明度用 `linear-gradient` / `color-mix`，并在**真实浏览器**里验证。
- **铁律②**：不做连胜、排行榜、徽章、推送。
- **铁律④**：跳过工具卡必须留痕（改状态，绝不删行）。

---

### Task 1: 原子底座建表

一次迁移立起整个底座：身份、阅读、以及四种形态共用的机制表。放在一个任务里，是因为它们是一个不可分割的结构决定——评审要么接受这个底座，要么不接受。

**Files:**
- Create: `apps/api/internal/store/migrations/0092_atom_substrate.sql`
- Create: `apps/api/internal/store/queries/atom.sql`
- Create: `apps/api/internal/store/queries/reading.sql`
- Regenerate: `apps/api/internal/store/sqlc/*.sql.go`
- Test: `apps/api/internal/store/atom_store_test.go`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces: 表 `atom` / `reading` / `atom_message` / `atom_card` / `atom_annotation` / `reading_source` / `reading_brief` / `reading_takeaway`，以及 sqlc 方法 `CreateAtom` / `GetAtom` / `CreateReading` / `GetReading` / `ListReadingsByUser` / `RenameReading` / `SetReadingFinished` / `UpsertReadingSource` / `GetReadingSource` / `UpsertReadingBrief` / `GetReadingBrief` / `UpsertReadingTakeaway` / `GetReadingTakeaway` / `AppendAtomMessage` / `ListAtomMessages` / `NextAtomMessageSeq` / `CreateAtomCard` / `GetAtomCard` / `ListAtomCards` / `UpdateAtomCardStatus` / `SubmitAtomCard` / `CreateAtomAnnotation` / `ListAtomAnnotations`。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/store/atom_store_test.go`。先读 `apps/api/internal/store/sqlc_test.go` 顶部，**照抄本包既有的 pool bootstrap helper 名**（不要新建）：

```go
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// TestAtomSubstrate_ReadingRoundTrips — an atom carries identity, the reading
// row carries the reading's own fields, and the two are created together.
func TestAtomSubstrate_ReadingRoundTrips(t *testing.T) {
	pool := newTestPool(t) // ← 用本包既有的 helper 名，见 sqlc_test.go
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	rd, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: a.ID, Title: "气候与农业", Lang: "zh"})
	if err != nil {
		t.Fatalf("CreateReading: %v", err)
	}
	if rd.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", rd.Status)
	}

	list, err := q.ListReadingsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("ListReadingsByUser: %v", err)
	}
	if len(list) != 1 || list[0].Title != "气候与农业" {
		t.Fatalf("list = %+v, want exactly my one reading", list)
	}
}

// TestAtomSubstrate_SharedMachineryIsPerAtom — messages, cards and annotations
// hang off the atom id, so every future kind (writing, chat, project) reuses
// them without a second implementation.
func TestAtomSubstrate_SharedMachineryIsPerAtom(t *testing.T) {
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}

	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 1, Role: "student", Content: "这段在讲什么？",
	}); err != nil {
		t.Fatalf("AppendAtomMessage: %v", err)
	}
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 2, Role: "ai", Content: "它在比较两种口径。",
	}); err != nil {
		t.Fatalf("AppendAtomMessage 2: %v", err)
	}
	msgs, err := q.ListAtomMessages(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Seq != 1 || msgs[1].Role != "ai" {
		t.Fatalf("messages = %+v, want the two in seq order", msgs)
	}

	// (atom_id, seq) is unique — a duplicated seq must be rejected, so a
	// concurrent double-append can never silently reorder the transcript.
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 2, Role: "student", Content: "重复",
	}); err == nil {
		t.Fatal("duplicate seq accepted, want a unique-violation")
	}
}

// TestAtomSubstrate_CascadesFromAtom — deleting the atom takes its whole world
// with it; no orphaned messages, cards or annotations.
func TestAtomSubstrate_CascadesFromAtom(t *testing.T) {
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateReading: %v", err)
	}
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 1, Role: "student", Content: "x",
	}); err != nil {
		t.Fatalf("AppendAtomMessage: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM atom WHERE id = $1`, a.ID); err != nil {
		t.Fatalf("delete atom: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM atom_message WHERE atom_id = $1`, a.ID).Scan(&n); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphaned %d messages after atom delete", n)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/store/ -run TestAtomSubstrate -timeout 1800s
```

预期：FAIL —— 编译不过，`sqlc.CreateAtomParams` 未定义。

- [ ] **Step 3: 写迁移**

新建 `apps/api/internal/store/migrations/0092_atom_substrate.sql`：

```sql
-- +goose Up
-- 轻量版的原子底座。阅读 / 写作（以及之后的 AI 聊天、AI 项目）四者不同的是各自
-- 的专属字段，相同的是四件事：一条消息流、一批工具卡、一层批注、一份小报告。
-- 所以：薄薄一层 atom 身份 + 共享机制表，专属字段各自成表。加一种新形态 =
-- 加一张专属表 + 复用底座。
--
-- 这些表与 pro 的 project 及其子表完全无关，互不引用。

CREATE TABLE atom (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind       text NOT NULL CHECK (kind IN ('reading','writing')),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_user_kind_idx ON atom (user_id, kind, created_at DESC);

CREATE TABLE reading (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

-- 共享机制 ---------------------------------------------------------------

-- seq 由服务端分配；(atom_id, seq) 唯一，保证并发下不会静默乱序。
CREATE TABLE atom_message (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  seq        integer NOT NULL,
  role       text NOT NULL CHECK (role IN ('student','ai','system')),
  content    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX atom_message_seq_idx ON atom_message (atom_id, seq);

-- 标准信封。结构与 pro 的 card_instances 保持一致——它是过程数据与评估的
-- 共同地基。Go 只做边界校验，内层深结构真相归 packages/contracts 的 Zod 契约。
CREATE TABLE atom_card (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  card_id      text NOT NULL,
  block_id     text,
  status       text NOT NULL CHECK (status IN ('proposed','active','submitted','skipped')),
  field_values jsonb NOT NULL DEFAULT '{}'::jsonb,
  event_trace  jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  submitted_at timestamptz
);
CREATE INDEX atom_card_atom_idx ON atom_card (atom_id, created_at);

CREATE TABLE atom_annotation (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  block_id   text NOT NULL,
  span       jsonb NOT NULL,
  quote      text NOT NULL DEFAULT '',
  note       text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_annotation_atom_idx ON atom_annotation (atom_id, created_at);

-- 阅读专属 ---------------------------------------------------------------

CREATE TABLE reading_source (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  body        text NOT NULL,
  source_url  text,
  bib         jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reading_brief (
  atom_id        uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  phase_tag      text,
  reading_reason text NOT NULL DEFAULT '',
  reading_focus  text NOT NULL DEFAULT '',
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reading_takeaway (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE reading_takeaway;
DROP TABLE reading_brief;
DROP TABLE reading_source;
DROP TABLE atom_annotation;
DROP TABLE atom_card;
DROP TABLE atom_message;
DROP TABLE reading;
DROP TABLE atom;
```

- [ ] **Step 4: 写共享机制的查询**

新建 `apps/api/internal/store/queries/atom.sql`：

```sql
-- name: CreateAtom :one
INSERT INTO atom (kind, user_id) VALUES ($1, $2) RETURNING *;

-- name: GetAtom :one
SELECT * FROM atom WHERE id = $1;

-- name: AppendAtomMessage :one
INSERT INTO atom_message (atom_id, seq, role, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAtomMessages :many
SELECT * FROM atom_message WHERE atom_id = $1 ORDER BY seq;

-- name: NextAtomMessageSeq :one
-- The next free seq for this atom. Callers append inside the same transaction
-- as this read, so the (atom_id, seq) unique index — not this read — is the
-- real guard against a concurrent double-append.
SELECT COALESCE(MAX(seq), 0)::int + 1 AS next FROM atom_message WHERE atom_id = $1;

-- name: CreateAtomCard :one
INSERT INTO atom_card (atom_id, card_id, block_id, status, field_values, event_trace)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAtomCard :one
SELECT * FROM atom_card WHERE id = $1;

-- name: ListAtomCards :many
SELECT * FROM atom_card WHERE atom_id = $1 ORDER BY created_at;

-- name: UpdateAtomCardStatus :one
UPDATE atom_card SET status = $2 WHERE id = $1 RETURNING *;

-- name: SubmitAtomCard :one
UPDATE atom_card
SET status = 'submitted', field_values = $2, event_trace = $3, submitted_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateAtomAnnotation :one
INSERT INTO atom_annotation (atom_id, block_id, span, quote, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAtomAnnotations :many
SELECT * FROM atom_annotation WHERE atom_id = $1 ORDER BY created_at;
```

- [ ] **Step 5: 写阅读的查询**

新建 `apps/api/internal/store/queries/reading.sql`：

```sql
-- name: CreateReading :one
INSERT INTO reading (atom_id, title, lang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetReading :one
SELECT * FROM reading WHERE atom_id = $1;

-- name: ListReadingsByUser :many
-- The list the 阅读 tab shows. Joins atom for ownership + creation order.
SELECT r.*, a.created_at AS atom_created_at
FROM reading r
JOIN atom a ON a.id = r.atom_id
WHERE a.user_id = $1 AND a.kind = 'reading'
ORDER BY a.created_at DESC;

-- name: RenameReading :exec
UPDATE reading SET title = $2, updated_at = now() WHERE atom_id = $1;

-- name: SetReadingFinished :exec
UPDATE reading SET status = 'finished', finished_at = now(), updated_at = now()
WHERE atom_id = $1;

-- name: UpsertReadingSource :one
INSERT INTO reading_source (atom_id, title, body, source_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET title = EXCLUDED.title, body = EXCLUDED.body,
      source_url = EXCLUDED.source_url, ingested_at = now()
RETURNING *;

-- name: GetReadingSource :one
SELECT * FROM reading_source WHERE atom_id = $1;

-- name: UpsertReadingBrief :one
INSERT INTO reading_brief (atom_id, phase_tag, reading_reason, reading_focus)
VALUES ($1, $2, $3, $4)
ON CONFLICT (atom_id) DO UPDATE
  SET phase_tag = EXCLUDED.phase_tag,
      reading_reason = EXCLUDED.reading_reason,
      reading_focus = EXCLUDED.reading_focus,
      updated_at = now()
RETURNING *;

-- name: GetReadingBrief :one
SELECT * FROM reading_brief WHERE atom_id = $1;

-- name: UpsertReadingTakeaway :one
INSERT INTO reading_takeaway (atom_id, text)
VALUES ($1, $2)
ON CONFLICT (atom_id) DO UPDATE SET text = EXCLUDED.text, updated_at = now()
RETURNING *;

-- name: GetReadingTakeaway :one
SELECT * FROM reading_takeaway WHERE atom_id = $1;
```

- [ ] **Step 6: 重新生成 sqlc**

```bash
cd apps/api && make sqlc
```

- [ ] **Step 7: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/store/ -run TestAtomSubstrate -timeout 1800s
```

预期：三条全部 PASS。**不要跑整包。**

- [ ] **Step 8: 提交**

```bash
git add apps/api/internal/store/migrations/0092_atom_substrate.sql \
        apps/api/internal/store/queries/atom.sql \
        apps/api/internal/store/queries/reading.sql \
        apps/api/internal/store/sqlc/ \
        apps/api/internal/store/atom_store_test.go
git commit -m "feat(lite): atom substrate — shared identity, messages, cards, annotations"
```

---

### Task 2: `schools.edition` 与站点分流闸

edition 是**学校**的属性：一所学校买的是轻量版还是现有版本，校内账号在注册（凭 join code 进班级 → 班级属于学校）那一刻随之确定。**不动 `users` 表，不改 `/auth/me`** —— 前端不需要知道，将来每校各自的子域名本身即已分流。

**Files:**
- Create: `apps/api/internal/store/migrations/0093_schools_edition.sql`
- Modify: `apps/api/internal/api/authz.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/edition_test.go`

**Interfaces:**
- Consumes: Task 1
- Produces: `schools.edition text NOT NULL DEFAULT 'pro' CHECK (edition IN ('pro','lite'))`；`func (a *API) requireEdition(want string, h http.Handler) http.Handler`（方法而非自由函数——它要查库）；测试 helper `liteHandler(t) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool)`，Task 3-8 全部复用。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/edition_test.go`：

```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// liteHandler builds an API whose seed SCHOOL is on the lite edition, plus a
// signed-in cookie. Shared by every lite test in this package.
func liteHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatalf("set school edition lite: %v", err)
	}
	return h, signInSeed(t, pool), q, pool
}

// TestEditionGate_LiteSchoolCannotReachProjects — a lite student has no
// projects; the pro surface simply is not there for them. 404, never 403.
func TestEditionGate_LiteSchoolCannotReachProjects(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("lite school GET /projects = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestEditionGate_ProSchoolKeepsProjects — the default edition is 'pro', so
// every existing school and every existing test keeps working untouched.
func TestEditionGate_ProSchoolKeepsProjects(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("pro school GET /projects = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestEditionGate -timeout 1800s
```

预期：`LiteSchoolCannotReachProjects` FAIL（拿到 200，闸还不存在）；另一条 PASS。

- [ ] **Step 3: 写迁移**

新建 `apps/api/internal/store/migrations/0093_schools_edition.sql`：

```sql
-- +goose Up
-- edition 属于学校，不属于账号：一所学校买的是轻量版还是现有版本，校内账号在
-- 注册（凭 join code 进班级 → 班级属于学校）那一刻随之确定。组织不变式不变。
-- 默认 'pro'，存量学校行为完全不变。
ALTER TABLE schools ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));

-- +goose Down
ALTER TABLE schools DROP COLUMN edition;
```

```bash
cd apps/api && make sqlc
```

- [ ] **Step 4: 写闸**

在 `apps/api/internal/api/authz.go` 末尾追加：

```go
// requireEdition gates a route group on the edition of the caller's SCHOOL.
// edition is an organisation fact, not an account one: a school buys the lite
// edition or the pro one and every account under it follows — so this reads
// schools.edition rather than any per-user field.
//
// A mismatch is 404, not 403: from a lite student's point of view the pro
// surface does not exist, and vice versa. Same non-leaking convention as
// ownership failures.
func (a *API) requireEdition(want string, h http.Handler) http.Handler {
	return RequireUser(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFromContext(r.Context())
		school, err := a.d.Queries.GetSchool(r.Context(), u.SchoolID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if school.Edition != want {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		h.ServeHTTP(w, r)
	})
}
```

`api.User` 已经携带 `SchoolID`（见 `apps/api/internal/api/auth.go` 的 `type User struct`），**不需要给 principal 加字段**。每请求多一次 `schools` 主键查询——表极小，代价可忽略。

- [ ] **Step 5: 把 `/projects` 组包成 pro-only**

在 `apps/api/internal/api/api.go` 的 `protected := …` 之后加：

```go
	// 站点分流：pro 学校的账号看不见 /readings，lite 学校的账号看不见
	// /projects —— 双向都是 404，不泄漏另一侧的存在。
	proOnly := func(h http.HandlerFunc) http.Handler { return a.requireEdition("pro", http.HandlerFunc(h)) }
```

然后把**所有** `/api/v1/projects` 及其子路由的 `protected(...)` 改成 `proOnly(...)`。先数一下有多少行，改完再数一次核对：

```bash
grep -c '"\(GET\|POST\|PUT\|PATCH\|DELETE\) /api/v1/projects' apps/api/internal/api/api.go
```

`/auth/*`、`/courses/*`、`/cards/*`、`/users/me/*`、`/oss/*`、`/voice/*`、`/chat/*`、`/classes/*`、`/admin/*` **不加闸**（两个站或教师端都要用）。

- [ ] **Step 6: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run TestEditionGate -timeout 1800s
```

预期：两条都 PASS。**不要跑整包**——controller 会统一验证。

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/store/migrations/0093_schools_edition.sql \
        apps/api/internal/store/sqlc/ \
        apps/api/internal/api/authz.go apps/api/internal/api/api.go \
        apps/api/internal/api/edition_test.go
git commit -m "feat(lite): edition rides on the school and gates the pro surface"
```

---

### Task 3: 阅读原子 CRUD 端点

**Files:**
- Create: `apps/api/internal/api/readings.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/readings_test.go`

**Interfaces:**
- Consumes: Task 1 的 sqlc 方法；Task 2 的 `requireEdition` 与 `liteHandler`
- Produces:
  - `POST /api/v1/readings` → `201 {"id"}`（id = **atom id**）
  - `GET /api/v1/readings` → `200 {"readings":[readingDTO]}`
  - `GET|PATCH /api/v1/readings/{id}` → `200 readingDTO`
  - `readingDTO` = `{id,title,lang,status,hasSource,createdAt,updatedAt,finishedAt}`，RFC3339 时间，`finishedAt` 可为 `null`
  - 供 Task 4-8 复用：`func (a *API) loadOwnedReadingAtom(w, r) (sqlc.Atom, bool)`、`func (a *API) hasSource(r, atomID) (bool, error)`、测试 helper `createReadingAtom(t, h, cookie) string`
  - `liteOnly` 包装器（注册在 api.go）

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/readings_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createReadingAtom returns a new reading's atom id. Shared by Tasks 3-8.
func createReadingAtom(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"一篇文章","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reading = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

func TestCreateReading_CreatesAtomAndReading(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	a, err := q.GetAtom(t.Context(), mustUUID(id))
	if err != nil {
		t.Fatalf("GetAtom: %v", err)
	}
	if a.Kind != "reading" {
		t.Fatalf("atom kind = %q, want \"reading\"", a.Kind)
	}
	if _, err := q.GetReading(t.Context(), a.ID); err != nil {
		t.Fatalf("GetReading: %v", err)
	}
}

func TestListReadings_OnlyMineNewestFirst(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	createReadingAtom(t, h, cookie)
	createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readings = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Readings []struct {
			ID        string `json:"id"`
			HasSource bool   `json:"hasSource"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Readings) != 2 {
		t.Fatalf("got %d readings, want 2", len(out.Readings))
	}
	if out.Readings[0].HasSource {
		t.Fatal("a brand-new reading reports hasSource=true; nothing has been pasted yet")
	}
}

func TestGetReading_UnknownIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown reading = %d, want 404", rec.Code)
	}
}

func TestRenameReading(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"新标题"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/readings/"+id, body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Title != "新标题" {
		t.Fatalf("title = %q, want 新标题 (err=%v)", out.Title, err)
	}
}
```

> `t.Context()` 需要 Go 1.24+。看 `apps/api/go.mod` 的 `go` 行；更低就换 `context.Background()` 并 import `"context"`。

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateReading|TestListReadings|TestGetReading|TestRenameReading' -timeout 1800s
```

预期：全 FAIL —— 404，路由不存在。

- [ ] **Step 3: 写实现**

新建 `apps/api/internal/api/readings.go`：

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readings.go — the lite edition's 阅读 atom: one student's single reading
// exercise. It owns its storage outright (atom + reading + the shared atom_*
// machinery); it does NOT borrow a project row and never touches any
// project-scoped table.

type readingDTO struct {
	ID         string  `json:"id"` // the ATOM id — every reading endpoint is keyed by it
	Title      string  `json:"title"`
	Lang       string  `json:"lang"`
	Status     string  `json:"status"`
	HasSource  bool    `json:"hasSource"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	FinishedAt *string `json:"finishedAt"`
}

func (a *API) readingDTOOf(rd sqlc.Reading, hasSource bool, createdAt time.Time) readingDTO {
	out := readingDTO{
		ID: rd.AtomID.String(), Title: rd.Title, Lang: rd.Lang, Status: rd.Status,
		HasSource: hasSource,
		CreatedAt: createdAt.Format(time.RFC3339),
		UpdatedAt: rd.UpdatedAt.Time.Format(time.RFC3339),
	}
	if rd.FinishedAt.Valid {
		s := rd.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// loadOwnedReadingAtom parses {id} as an atom of kind 'reading' owned by the
// caller. Every failure — malformed id, missing row, wrong kind, wrong owner —
// is a flat 404, so atom existence is never leaked. Shared by Tasks 3-8.
func (a *API) loadOwnedReadingAtom(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	at, err := a.d.Queries.GetAtom(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Atom{}, false
	}
	if at.Kind != "reading" || at.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	return at, true
}

// hasSource reports whether the article body has been pasted yet. A missing
// row is a legitimate state (a brand-new reading), not an error.
func (a *API) hasSource(r *http.Request, atomID uuid.UUID) (bool, error) {
	if _, err := a.d.Queries.GetReadingSource(r.Context(), atomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *API) createReading(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req struct {
		Title string `json:"title"`
		Lang  string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名阅读"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}

	// atom + reading in ONE transaction: an atom with no reading row would be
	// an identity nothing can render.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "reading", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateReading(r.Context(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: title, Lang: lang,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": at.ID.String()})
}

func (a *API) listReadings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]readingDTO, 0, len(rows))
	for _, row := range rows {
		hasSrc, err := a.hasSource(r, row.AtomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, a.readingDTOOf(sqlc.Reading{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang,
			Status: row.Status, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
		}, hasSrc, row.AtomCreatedAt.Time))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"readings": out})
}

func (a *API) getReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt.Time))
}

func (a *API) renameReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "标题不能为空。", nil))
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if err := a.d.Queries.RenameReading(r.Context(), sqlc.RenameReadingParams{AtomID: at.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt.Time))
}
```

> `ListReadingsByUser` 生成的 row 类型字段名以 `make sqlc` 的产出为准（`AtomCreatedAt` 等），按生成结果调整装配。`pgtype` 与 `google/uuid` 不可混用——以生成的类型为准。

- [ ] **Step 4: 注册路由**

在 `apps/api/internal/api/api.go` 里，Task 2 的 `proOnly` 旁边加 `liteOnly`：

```go
	liteOnly := func(h http.HandlerFunc) http.Handler { return a.requireEdition("lite", http.HandlerFunc(h)) }

	// 轻量版（lite edition）· 阅读原子。{id} 一律是 atom id。
	mux.Handle("GET /api/v1/readings", liteOnly(a.listReadings))
	mux.Handle("POST /api/v1/readings", liteOnly(a.createReading))
	mux.Handle("GET /api/v1/readings/{id}", liteOnly(a.getReading))
	mux.Handle("PATCH /api/v1/readings/{id}", liteOnly(a.renameReading))
```

- [ ] **Step 5: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateReading|TestListReadings|TestGetReading|TestRenameReading|TestEditionGate' -timeout 1800s
```

预期：全部 PASS。**不要跑整包。**

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/api/readings.go apps/api/internal/api/readings_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): reading atom CRUD endpoints"
```

---

### Task 4: 文章入库与分段

阅读室要把正文按段渲染，工具卡要悬挂在**某一段**上，AI 的 `ExampleBlockID` 也指向段 id。所以分段规则必须确定、可单测。

**Files:**
- Create: `apps/api/internal/api/reading_blocks.go`
- Create: `apps/api/internal/api/reading_source.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/reading_blocks_test.go`
- Test: `apps/api/internal/api/reading_source_test.go`

**Interfaces:**
- Consumes: Task 3 的 `loadOwnedReadingAtom`、`liteOnly`
- Produces:
  - `PUT /api/v1/readings/{id}/source` — body `{"title"?,"text"?,"url"?}` → `200 sourceDTO`
  - `GET /api/v1/readings/{id}/source` → `200 sourceDTO`（未贴过 → 404）
  - `sourceDTO` = `{title, sourceUrl, blocks:[{id,text}]}`
  - `type Block struct { ID, Text string }`（JSON `id`/`text`）与 `func SplitBlocks(body string) []Block` —— 段 id 形如 `b1`、`b2`，Task 7 装配与 Task 12 渲染都依赖它

- [ ] **Step 1: 先看 pro 的锚点类型**

```bash
grep -n "type MaterialBlock" -A 8 apps/api/internal/agent/*.go
grep -n "func ResolveExampleAnchor" -A 10 apps/api/internal/agent/reading_gate.go
```

`agent.MaterialBlock` 是 `ResolveExampleAnchor` 的输入——**你的 `Block` 必须能转成它**。在报告里写明字段对应关系。

- [ ] **Step 2: 写下会失败的分段测试**

新建 `apps/api/internal/api/reading_blocks_test.go`：

```go
package api_test

import (
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestSplitBlocks_ParagraphsGetStableIDs(t *testing.T) {
	blocks := SplitBlocks("第一段。\n\n第二段，稍长一些。\n\n\n第三段。")
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3: %+v", len(blocks), blocks)
	}
	if blocks[0].ID != "b1" || blocks[1].ID != "b2" || blocks[2].ID != "b3" {
		t.Fatalf("ids = %s/%s/%s, want b1/b2/b3", blocks[0].ID, blocks[1].ID, blocks[2].ID)
	}
	if blocks[1].Text != "第二段，稍长一些。" {
		t.Fatalf("block 2 text = %q", blocks[1].Text)
	}
}

func TestSplitBlocks_IgnoresBlankAndTrims(t *testing.T) {
	blocks := SplitBlocks("  \n\n  正文  \n\n   \n\n 尾段 \n")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2: %+v", len(blocks), blocks)
	}
	if blocks[0].Text != "正文" || blocks[1].Text != "尾段" {
		t.Fatalf("blocks = %+v, want trimmed text", blocks)
	}
}

func TestSplitBlocks_EmptyBodyYieldsNone(t *testing.T) {
	if got := SplitBlocks("   \n\n  "); len(got) != 0 {
		t.Fatalf("got %d blocks, want 0", len(got))
	}
}

// Block ids must not shift when the body is re-split — a hanging card stores
// its block id, so renumbering would move every card to the wrong paragraph.
func TestSplitBlocks_IsDeterministic(t *testing.T) {
	body := strings.Repeat("一段。\n\n", 5)
	a, b := SplitBlocks(body), SplitBlocks(body)
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Text != b[i].Text {
			t.Fatalf("block %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}
```

- [ ] **Step 3: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestSplitBlocks -timeout 1800s
```

预期：FAIL —— `SplitBlocks` 未定义。

- [ ] **Step 4: 写分段**

新建 `apps/api/internal/api/reading_blocks.go`：

```go
package api

import (
	"strconv"
	"strings"
)

// reading_blocks.go — 正文分段。工具卡悬挂在某一段上、AI 的 ExampleBlockID 也
// 指向段 id，所以这个函数必须是纯的、确定的：同样的正文永远得到同样的 id，
// 否则已经悬挂的卡片会集体移位到错误的段落。

type Block struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// SplitBlocks splits an article body into paragraphs on blank lines, trims
// each, drops the empties, and numbers what SURVIVES b1, b2, … — so the
// numbering stays stable for as long as the body does.
func SplitBlocks(body string) []Block {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	raw := strings.Split(body, "\n\n")
	out := make([]Block, 0, len(raw))
	for _, p := range raw {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, Block{ID: "b" + strconv.Itoa(len(out)+1), Text: t})
	}
	return out
}
```

- [ ] **Step 5: 写 source 端点的测试**

新建 `apps/api/internal/api/reading_source_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func putSource(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"text": text})
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(string(body))), cookie))
	return rec
}

func TestPutSource_StoresAndSegments(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := putSource(t, h, cookie, id, "太阳能装机十年增长十倍。\n\n但储能仍是瓶颈。")
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT source = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Blocks []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Blocks) != 2 || out.Blocks[0].ID != "b1" {
		t.Fatalf("blocks = %+v, want two starting at b1", out.Blocks)
	}
}

func TestPutSource_RejectsEmptyBody(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := putSource(t, h, cookie, id, "   \n\n  ")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func TestPutSource_ReplacesOnSecondPut(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	putSource(t, h, cookie, id, "第一版。")
	putSource(t, h, cookie, id, "第二版。\n\n多了一段。")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET source = %d, want 200", rec.Code)
	}
	var out struct {
		Blocks []struct {
			Text string `json:"text"`
		} `json:"blocks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Blocks) != 2 || out.Blocks[0].Text != "第二版。" {
		t.Fatalf("blocks = %+v, want the second version", out.Blocks)
	}
}

func TestGetSource_BeforePasteIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET source before paste = %d, want 404", rec.Code)
	}
}

func TestPutSource_ForeignAtomIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := putSource(t, h, cookie, "00000000-0000-0000-0000-0000000009ff", "正文")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 6: 写 source 端点**

新建 `apps/api/internal/api/reading_source.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_source.go — the article the student is reading. One reading, one
// article: PUT replaces it wholesale (a student who pastes twice meant the
// second one).

type sourceDTO struct {
	Title     string  `json:"title"`
	SourceURL string  `json:"sourceUrl"`
	Blocks    []Block `json:"blocks"`
}

func (a *API) putReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Text)
	srcURL := strings.TrimSpace(req.URL)

	// A URL is fetched server-side through the same guarded fetcher the pro
	// side uses; a pasted body is taken as-is.
	if body == "" && srcURL != "" {
		if a.d.Fetcher == nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_unavailable", "暂时无法抓取链接，请直接粘贴正文。", nil))
			return
		}
		fetchedTitle, text, _, err := a.d.Fetcher.FetchReadable(r.Context(), srcURL)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed", "这个链接抓不到正文，请直接粘贴。", nil))
			return
		}
		body = strings.TrimSpace(text)
		if title == "" {
			title = fetchedTitle
		}
	}

	if len(SplitBlocks(body)) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先把文章正文放进来。", nil))
		return
	}
	if title == "" {
		title = "未命名文章"
	}

	row, err := a.d.Queries.UpsertReadingSource(r.Context(), sqlc.UpsertReadingSourceParams{
		AtomID: at.ID, Title: title, Body: body, SourceUrl: nullableText(srcURL),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: srcURL, Blocks: SplitBlocks(row.Body),
	})
}

func (a *API) getReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404: nothing pasted yet
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: derefOr(row.SourceUrl, ""), Blocks: SplitBlocks(row.Body),
	})
}
```

⚠️ **命名冲突自查**：`apps/api/internal/api/` 是同一个 package，pro 已有 `getMaterialSource` 等名字。动手前 grep 确认你要用的每个方法名都空着：

```bash
grep -rn "func (a \*API) \(putReadingSourceLite\|getReadingSourceLite\|nullableText\|derefOr\)" apps/api/internal/api/
grep -rn "func derefOr\|func nullableText" apps/api/internal/api/
```

`derefOr` 本包很可能已有（`projects.go` 在用）——先看签名再用，**不要重复定义**。`nullableText`（空串存 NULL）若没有就加一个。

文件上传（PDF/DOCX 走 `internal/docextract`）与 `bib` **本期不接**：落地页只提供粘贴与链接。这是有意的范围收窄。

- [ ] **Step 7: 注册路由**

```go
	mux.Handle("PUT /api/v1/readings/{id}/source", liteOnly(a.putReadingSourceLite))
	mux.Handle("GET /api/v1/readings/{id}/source", liteOnly(a.getReadingSourceLite))
```

- [ ] **Step 8: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run 'TestSplitBlocks|TestPutSource|TestGetSource' -timeout 1800s
```

- [ ] **Step 9: 提交**

```bash
git add apps/api/internal/api/reading_blocks.go apps/api/internal/api/reading_blocks_test.go \
        apps/api/internal/api/reading_source.go apps/api/internal/api/reading_source_test.go \
        apps/api/internal/api/api.go
git commit -m "feat(lite): article ingestion with deterministic paragraph blocks"
```

---

### Task 5: brief、takeaway、annotations

三组小端点，形状相同（upsert 一张表 / 追加一行），一并评审。

**Files:**
- Create: `apps/api/internal/api/reading_notes.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/reading_notes_test.go`

**Interfaces:**
- Consumes: Task 3 的 `loadOwnedReadingAtom`、`liteOnly`
- Produces:
  - `GET|PUT /api/v1/readings/{id}/brief` → `{phaseTag, readingReason, readingFocus}`
  - `GET|PUT /api/v1/readings/{id}/takeaway` → `{text, updatedAt}`
  - `GET|POST /api/v1/readings/{id}/annotations` → `{annotations:[{id,blockId,span,quote,note,createdAt}]}`
  - 方法名：`liteGetBrief` / `litePutBrief` / `liteGetTakeaway` / `litePutTakeaway` / `liteListAnnotations` / `liteCreateAnnotation`

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/reading_notes_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrief_EmptyBeforeSetThenRoundTrips(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	// A reading with no brief yet is a legitimate state, not a 404 — the brief
	// is optional context and the room asks for it the moment it opens, so a
	// 404 here would make the frontend treat "normal" as "broken".
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/brief", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET brief before set = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"phaseTag":null,"readingReason":"想弄清储能瓶颈","readingFocus":"看数据口径"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/brief", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT brief = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/brief", nil), cookie))
	var out struct {
		ReadingReason string `json:"readingReason"`
		ReadingFocus  string `json:"readingFocus"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.ReadingReason != "想弄清储能瓶颈" || out.ReadingFocus != "看数据口径" {
		t.Fatalf("brief = %+v, want what we just PUT", out)
	}
}

func TestTakeaway_EmptyBeforeSetThenRoundTrips(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/takeaway", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET takeaway before set = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	body := strings.NewReader(`{"text":"作者其实没证明因果，只给了相关。"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT takeaway = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/takeaway", nil), cookie))
	var out struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !strings.Contains(out.Text, "没证明因果") {
		t.Fatalf("takeaway = %q, want what we PUT", out.Text)
	}
}

func TestAnnotations_AppendAndList(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"b1","span":{"start":0,"end":6},"quote":"太阳能装机","note":"这个数字要查来源"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST annotation = %d, want 201; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/annotations", nil), cookie))
	var out struct {
		Annotations []struct {
			BlockID string `json:"blockId"`
			Note    string `json:"note"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Annotations) != 1 || out.Annotations[0].BlockID != "b1" {
		t.Fatalf("annotations = %+v, want the one we appended", out.Annotations)
	}
}

func TestAnnotations_RequiresBlockID(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"","span":{"start":0,"end":1},"quote":"x","note":"y"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/annotations", body), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank blockId = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func TestAnnotations_ForeignAtomIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"blockId":"b1","span":{"start":0,"end":1},"quote":"x","note":"y"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/00000000-0000-0000-0000-0000000009ff/annotations", body), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run 'TestBrief|TestTakeaway|TestAnnotations' -timeout 1800s
```

- [ ] **Step 3: 写实现**

新建 `apps/api/internal/api/reading_notes.go`，六个 handler。**动手前 grep 确认名字没被 pro 占用**（`putReadingBrief` 在 pro 的 `reading_brief.go` 里已存在，所以本组统一加 `lite` 前缀）：

```bash
grep -rn "func (a \*API) lite" apps/api/internal/api/
```

要点：

- **GET brief / takeaway 在没有行时回 200 + 零值**，不是 404（理由写在测试注释里）。用 `errors.Is(err, pgx.ErrNoRows)` 分支返回零值 DTO。
- **PUT 是全量替换**（upsert），与 pro 的 brief 语义一致：每次保存重发全部字段，否则没动的字段会被静默清空。
- **span 原样存 jsonb**：Go 只校验「是一个 JSON 对象」，形状真相归前端契约。
- annotation 的 `blockId` 必须非空（400 `missing_block_id`）；`note` 与 `quote` 可空。

- [ ] **Step 4: 注册路由**

```go
	mux.Handle("GET /api/v1/readings/{id}/brief", liteOnly(a.liteGetBrief))
	mux.Handle("PUT /api/v1/readings/{id}/brief", liteOnly(a.litePutBrief))
	mux.Handle("GET /api/v1/readings/{id}/takeaway", liteOnly(a.liteGetTakeaway))
	mux.Handle("PUT /api/v1/readings/{id}/takeaway", liteOnly(a.litePutTakeaway))
	mux.Handle("GET /api/v1/readings/{id}/annotations", liteOnly(a.liteListAnnotations))
	mux.Handle("POST /api/v1/readings/{id}/annotations", liteOnly(a.liteCreateAnnotation))
```

- [ ] **Step 5: 跑测试，确认通过；Step 6: 提交**

```bash
cd apps/api && go test ./internal/api/ -run 'TestBrief|TestTakeaway|TestAnnotations' -timeout 1800s
git add apps/api/internal/api/reading_notes.go apps/api/internal/api/reading_notes_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): reading brief, takeaway and annotations"
```

---

### Task 6: 工具卡生命周期

**Files:**
- Create: `apps/api/internal/api/reading_cards.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/reading_cards_test.go`

**Interfaces:**
- Consumes: Task 1 的 `atom_card` 查询；Task 3 的 `loadOwnedReadingAtom`、`liteOnly`
- Produces:
  - `GET /api/v1/readings/{id}/cards` → `{"cards":[cardDTO]}`
  - `POST /api/v1/readings/{id}/cards/{cid}/activate|skip` → `200 cardDTO`
  - `POST /api/v1/readings/{id}/cards/{cid}/submit` — body `{fieldValues, eventTrace}` → `200 cardDTO`
  - `cardDTO` = `{id, cardId, blockId, status, fieldValues, eventTrace, createdAt, submittedAt}`
  - `func (a *API) loadOwnedAtomCard(w, r, atomID uuid.UUID) (sqlc.AtomCard, bool)`（Task 7 复用）
  - `func validateEnvelope(fieldValues, eventTrace json.RawMessage) error`

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/reading_cards_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedCard inserts a proposed card directly — the AI turn that would normally
// propose one lands in Task 7.
func seedCard(t *testing.T, pool *pgxpool.Pool, atomID, blockID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status)
		 VALUES ($1, 'craap', $2, 'proposed') RETURNING id::text`,
		mustUUID(atomID), blockID).Scan(&id); err != nil {
		t.Fatalf("seed card: %v", err)
	}
	return id
}

func TestCardSubmit_StoresEnvelopeAndStamps(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"fieldValues":{"currency":"2024"},"eventTrace":[{"t":"open"}]}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Status      string `json:"status"`
		SubmittedAt string `json:"submittedAt"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "submitted" || out.SubmittedAt == "" {
		t.Fatalf("card = %+v, want submitted with a timestamp", out)
	}
}

func TestCardSubmit_RejectsMalformedEnvelope(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	for _, tc := range []struct{ name, body string }{
		{"fieldValues is an array", `{"fieldValues":[],"eventTrace":[]}`},
		{"eventTrace is an object", `{"fieldValues":{},"eventTrace":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
				"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", strings.NewReader(tc.body)), cookie))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s = %d, want 400; body=%s", tc.name, rec.Code, rec.Body)
			}
		})
	}
}

// 铁律④ 过程即数据：跳过是信号，必须留痕，不能删行。
func TestCardSkip_IsRecordedNotDeleted(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/skip", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("skip = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var status string
	if err := pool.QueryRow(t.Context(),
		`SELECT status FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "skipped" {
		t.Fatalf("status = %q, want \"skipped\" (the row must survive)", status)
	}
}

func TestCardActivate_MovesProposedToActive(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/activate", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("activate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", out.Status)
	}
}

// A card belonging to another atom must not be reachable through this one.
func TestCard_CrossAtomIs404(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomA := createReadingAtom(t, h, cookie)
	atomB := createReadingAtom(t, h, cookie)
	cardOfB := seedCard(t, pool, atomB, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomA+"/cards/"+cardOfB+"/activate", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-atom card = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestCard -timeout 1800s
```

- [ ] **Step 3: 写实现**

新建 `apps/api/internal/api/reading_cards.go`。边界校验单独成函数：

```go
// validateEnvelope is the boundary check the spec mandates: Go verifies only
// the envelope's OUTER shape — field_values is a JSON object, event_trace is a
// JSON array — and stores the inner structure verbatim. The deep truth lives
// in packages/contracts' Zod schemas; duplicating it here would guarantee the
// two drift apart.
func validateEnvelope(fieldValues, eventTrace json.RawMessage) error {
	var obj map[string]any
	if err := json.Unmarshal(fieldValues, &obj); err != nil {
		return httpx.ErrBadRequest("bad_field_values", "字段值格式不对", nil)
	}
	var arr []any
	if err := json.Unmarshal(eventTrace, &arr); err != nil {
		return httpx.ErrBadRequest("bad_event_trace", "事件轨迹格式不对", nil)
	}
	return nil
}
```

`loadOwnedAtomCard` **必须校验卡片属于这个 atom**（`card.AtomID == atomID`），否则跨原子越权；不匹配一律 404。

- [ ] **Step 4: 注册路由；Step 5: 跑测试；Step 6: 提交**

```go
	mux.Handle("GET /api/v1/readings/{id}/cards", liteOnly(a.liteListCards))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/activate", liteOnly(a.liteActivateCard))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/skip", liteOnly(a.liteSkipCard))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/submit", liteOnly(a.liteSubmitCard))
```

```bash
cd apps/api && go test ./internal/api/ -run TestCard -timeout 1800s
git add apps/api/internal/api/reading_cards.go apps/api/internal/api/reading_cards_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): tool-card lifecycle over the atom substrate"
```

---

### Task 7: 陪练一轮 —— 复用 AI 大脑

本期最有价值的任务：证明 AI 层可以原样复用。

**Files:**
- Create: `apps/api/internal/store/migrations/0094_llm_call_atom.sql`
- Create: `apps/api/internal/api/reading_turn.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/reading_turn_test.go`

**Interfaces:**
- Consumes: Task 1、3、4、6
- Produces:
  - `POST /api/v1/readings/{id}/turn` — body `{"text":"…","focusedSpans":[…]}` → `200 {"reply","decision","card":cardDTO|null,"nudge"}`
  - `GET /api/v1/readings/{id}/messages` → `{"messages":[{seq,role,content,createdAt}]}`
  - `llm_call.atom_id` 列
  - `func (a *API) buildReadingRouteInput(ctx context.Context, atomID uuid.UUID, studentText string, spans []agent.FocusSpan) (agent.ReadingRouteInput, error)`

- [ ] **Step 1: 先读清楚 AI 层的真实签名**

**不要凭本计划的描述写代码**，打开这些确认类型与字段：

```bash
sed -n '1,70p' apps/api/internal/agent/reading_router.go
grep -n "type FocusSpan\|type PacingState\|type ReadingBrief\|type ReadingCard\|type MaterialBlock\|type OrderingGuard" -A 8 apps/api/internal/agent/*.go
grep -n "func ReadingDeck\|func ApplyReadingGate\|func ResolveExampleAnchor" -A 8 apps/api/internal/agent/reading_deck.go apps/api/internal/agent/reading_gate.go
grep -rn "ai_dialogue_failed" apps/api/internal/ | head
grep -rn "func .*RecordLLMCall" -A 12 apps/api/internal/agent/agentstore.go | head -20
```

在报告里写明每个类型的真实字段、以及你如何从新表装配它们。

- [ ] **Step 2: 写迁移**

新建 `apps/api/internal/store/migrations/0094_llm_call_atom.sql`：

```sql
-- +goose Up
-- 轻量版的模型调用记在 llm_call：project_id 为 NULL（与 course/chat 调用一致），
-- surface='lite'，并用 atom_id 指回具体的阅读/写作。llm_usage 视图按 user_id /
-- tier / tokens / cost 聚合、不读 project_id，所以学校维度的成本汇总无需改动。
ALTER TABLE llm_call ADD COLUMN atom_id uuid REFERENCES atom(id) ON DELETE SET NULL;
CREATE INDEX llm_call_atom_created_idx ON llm_call (atom_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS llm_call_atom_created_idx;
ALTER TABLE llm_call DROP COLUMN atom_id;
```

```bash
cd apps/api && make sqlc
```

- [ ] **Step 3: 写下会失败的测试**

`RouteReading` 走 `gateway.Provider`，测试用 `gateway.NewStubProvider` 脚本化模型回复（`maintest_test.go` 的 `assessStubProvider` 与 `project_create_test.go` 的 `composeJourneyStubProvider` 都是现成写法，照抄那个形状；把 stub 通过 `Deps{Provider: …}` 注入）。

必须覆盖三件事：

1. **respond**：模型回 `{"decision":"respond","reply":"…"}` → 端点返回 reply，且**学生与 AI 两条消息都进了 `atom_message`，seq 连续**（`GET /messages` 断言）。
2. **summon**：模型回 `{"decision":"summon","card_id":"craap","example_block_id":"b1",…}` → 建出一行 `atom_card`（`status='proposed'`、`block_id='b1'`），响应带 `card` 与 `nudge`。
3. **模型失败必须如实报错**：provider 返回错误 → **502 且 body 含 `ai_dialogue_failed`**，**绝不返回罐头回复**。这是记录在案的用户规则：AI 对话错误必须被暴露，不能被掩盖。

```go
func TestReadingTurn_ModelFailureSurfacesAsError(t *testing.T) {
	// …用一个 Stream 立刻返回 error 的 provider stub 构造 Deps…
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
}
```

- [ ] **Step 4: 写实现**

`reading_turn.go` 的骨架（类型以 Step 1 读到的为准）：

```go
// postReadingTurn — one coach turn. This is the whole reuse thesis in a single
// handler: the reading brain (agent.RouteReading + agent.ApplyReadingGate) is
// a set of PURE functions over plain values, so it runs UNCHANGED over the
// lite tables. All this handler does is assemble the input, persist the
// output, and meter the call.
func (a *API) postReadingTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	// … HasEntitlement gate …

	// 1. 装配 —— 全部来自 lite 自己的表
	in, err := a.buildReadingRouteInput(r.Context(), at.ID, studentText, spans)

	// 2. 复用 AI 大脑，一行没改
	decision, resolved, usage, err := agent.RouteReading(r.Context(), a.d.Provider, a.d.ChatResolver, in)
	if err != nil {
		// 用户规则：AI 对话失败必须如实暴露，绝不用罐头回复掩盖。
		httpx.WriteError(w, r, aiDialogueFailed())
		return
	}
	decision = agent.ApplyReadingGate(decision, in.Pacing, orderingGuard)

	// 3. 落库：学生一条、AI 一条，seq 连续；summon 则建一行 proposed 卡
	// 4. 计量：surface="lite", purpose="reading_turn", atom_id=at.ID
}
```

`aiDialogueFailed()` 用 pro 侧既有的 502 构造方式（Step 1 已 grep 出它在哪）。**seq 分配与两条消息的写入必须在同一事务里**，靠 `(atom_id, seq)` 唯一索引兜住并发。

`buildReadingRouteInput` 单独成函数，方便单测：`Article` 来自 `reading_source.body`，`RecentTurns` 来自 `ListAtomMessages` 的尾部，`Brief` 来自 `reading_brief`，`Catalog` 来自 `agent.ReadingDeck()`，`Pacing` 由已提交/跳过的 `atom_card` 计数得出。

- [ ] **Step 5: 注册路由；Step 6: 跑测试；Step 7: 提交**

```go
	mux.Handle("POST /api/v1/readings/{id}/turn", liteOnly(a.postReadingTurn))
	mux.Handle("GET /api/v1/readings/{id}/messages", liteOnly(a.liteListMessages))
```

```bash
cd apps/api && go test ./internal/api/ -run TestReadingTurn -timeout 1800s
git add apps/api/internal/store/migrations/0094_llm_call_atom.sql apps/api/internal/store/sqlc/ \
        apps/api/internal/api/reading_turn.go apps/api/internal/api/reading_turn_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): reading coach turn reusing the agent brain verbatim"
```

---

### Task 8: 完成阅读

**Files:**
- Modify: `apps/api/internal/api/readings.go`、`apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/readings_test.go`（追加）

**Interfaces:**
- Produces: `POST /api/v1/readings/{id}/finish` → `200 readingDTO`（`status='finished'`）

- [ ] **Step 1: 写下会失败的测试**

```go
// 「我的收获」是这次阅读的产出。空着就完成，等于没读——所以 finish 以它为门槛。
func TestFinishReading_RequiresTakeaway(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/finish", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("finish without takeaway = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "missing_takeaway") {
		t.Fatalf("want missing_takeaway, got %s", rec.Body)
	}
}

func TestFinishReading_IsIdempotent(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	putTakeaway := httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"我的收获。"}`))
	h.ServeHTTP(httptest.NewRecorder(), withCookie(putTakeaway, cookie))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
			"/api/v1/readings/"+id+"/finish", strings.NewReader("{}")), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("finish #%d = %d, want 200; body=%s", i+1, rec.Code, rec.Body)
		}
	}
}
```

- [ ] **Step 2-4: 跑失败 → 实现 → 跑通过**

```bash
cd apps/api && go test ./internal/api/ -run TestFinishReading -timeout 1800s
```

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/readings.go apps/api/internal/api/readings_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): finish a reading, gated on the takeaway"
```

---

### Task 9: `apps/lite-web` 脚手架

**Files:** `apps/web/package.json`（改名）+ `apps/lite-web/` 全套配置

**Interfaces:**
- Produces: 可 `pnpm --filter @mind-imprint/lite-web build` 成功的应用；`@/` 别名解析到 `apps/web/src`；Tailwind content 覆盖 web 源码

- [ ] **Step 1: 给 `apps/web` 取包名并暴露源码**

`apps/web/package.json`：`"name": "web"` → `"name": "@mind-imprint/web"`，并加 `"exports": { "./src/*": "./src/*" }`。

**改名会波及既有命令**，全仓 grep 并同步（至少 `apps/web/Dockerfile`、`.deploy-local/` 部署脚本、根 `package.json`）：

```bash
grep -rn -- "--filter web" --include=*.json --include=*.sh --include=*.yml --include=Dockerfile . | grep -v node_modules
```

- [ ] **Step 2: 建 lite 骨架**

`apps/lite-web/package.json`（依赖版本对齐 `apps/web`；scripts: dev/build/test/typecheck；deps: `@mind-imprint/contracts`、`@mind-imprint/web`、react、react-dom、react-markdown、remark-gfm、remark-cjk-friendly、marked、lucide-react、zod；devDeps: vite、@vitejs/plugin-react、tailwindcss、postcss、autoprefixer、typescript、vitest、jsdom、@testing-library/react、@testing-library/jest-dom、@types/*）。

`apps/lite-web/vite.config.ts`：

```ts
import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const webSrc = path.resolve(__dirname, "../web/src");

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // 房间组件从 apps/web 源码引入，它们内部用 "@/…" 自引用 —— 这个别名必须
      // 解析到 web 的 src，否则一进阅读室就是一片解析失败。
      { find: /^@\//, replacement: webSrc + "/" },
      { find: "@lite", replacement: path.resolve(__dirname, "src") },
    ],
  },
  optimizeDeps: { exclude: ["@mind-imprint/web", "@mind-imprint/contracts"] },
  server: { proxy: { "/api": { target: "http://localhost:8080", changeOrigin: true } } },
});
```

`apps/lite-web/tailwind.config.ts`：

```ts
import type { Config } from "tailwindcss";
import base from "../web/tailwind.config";

export default {
  ...base,
  // 房间组件的 class 写在 apps/web 里。漏掉这一条，阅读室会渲染成没有样式的裸 DOM。
  content: ["./index.html", "./src/**/*.{ts,tsx}", "../web/src/**/*.{ts,tsx}"],
} satisfies Config;
```

`tsconfig.json`（`paths`: `@/*` → `../web/src/*`，`@lite/*` → `./src/*`）、`postcss.config.js`、`index.html`（title「思维印记 · 轻量版」）、`src/index.css`（首行 `@import "@/index.css";`）、`src/main.tsx`、`src/LiteApp.tsx`（占位）、`vitest.config.ts`（别名与 vite 一致）——均照 `apps/web` 对应文件改写。

- [ ] **Step 3: 构建，并在真实浏览器里验证样式**

```bash
pnpm install
pnpm --filter @mind-imprint/lite-web build
pnpm --filter @mind-imprint/web build && pnpm --filter @mind-imprint/web typecheck
```

在 `LiteApp.tsx` 里临时引一个 web 的叶子组件（如 `@/ui` 的 `Segmented`），`pnpm --filter @mind-imprint/lite-web dev` 后**在真实浏览器里确认它有样式而不是裸 DOM** —— 这是 Tailwind content glob 是否生效的唯一可靠验证。确认后还原占位。

- [ ] **Step 4: 提交**

```bash
git add apps/web/package.json apps/lite-web pnpm-lock.yaml
git commit -m "feat(lite): scaffold apps/lite-web importing the rooms from apps/web"
```

---

### Task 10: `RoomCapabilities`

**Files:**
- Create: `apps/web/src/rooms/capabilities.ts`
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx`
- Test: `apps/web/test/roomCapabilities.test.tsx`

**Interfaces:**
- Produces: `RoomCapabilities` + `PRO_CAPABILITIES` / `DEMO_CAPABILITIES` / `LITE_READING_CAPABILITIES`；`ReadingRoomProps` 新增可选 `capabilities?: RoomCapabilities`，**缺省即 `PRO_CAPABILITIES`**，保证既有调用点行为一字不变

- [ ] **Step 1: 写下会失败的测试**

```tsx
import { describe, expect, it } from "vitest";
import { PRO_CAPABILITIES, DEMO_CAPABILITIES, LITE_READING_CAPABILITIES } from "@/rooms/capabilities";

describe("RoomCapabilities", () => {
  it("pro keeps the whole project lifecycle", () => {
    expect(PRO_CAPABILITIES.mode).toBe("pro");
    expect(PRO_CAPABILITIES.evidenceMap).toBe(true);
    expect(PRO_CAPABILITIES.proposalImpact).toBe(true);
  });
  it("lite reading drops every project-lifecycle surface", () => {
    expect(LITE_READING_CAPABILITIES.mode).toBe("lite");
    for (const k of ["plan", "evidenceMap", "explorationLeads", "proposalImpact", "essayTrack"] as const) {
      expect(LITE_READING_CAPABILITIES[k]).toBe(false);
    }
  });
  it("demo is read-only pro, not a third lifecycle", () => {
    expect(DEMO_CAPABILITIES.mode).toBe("demo");
    expect(DEMO_CAPABILITIES.evidenceMap).toBe(PRO_CAPABILITIES.evidenceMap);
  });
});
```

- [ ] **Step 2-4: 跑失败 → 实现 → 跑通过**

`capabilities.ts`：

```ts
// capabilities.ts — what a room is allowed to show.
//
// The rooms used to reach for project-lifecycle facts directly (and carried a
// separate `demoMode` boolean). The lite edition runs the SAME room components
// with no project around them, so a room now reads one object instead of
// asking what stage a project is in. Adding an edition means adding a preset
// here, not threading another boolean through the tree.
export type RoomCapabilities = {
  mode: "pro" | "lite" | "demo";
  plan: boolean;
  evidenceMap: boolean;
  explorationLeads: boolean;
  proposalImpact: boolean;
  essayTrack: boolean;
  comprehensionCheck: boolean;
  exemplars: boolean;
};

export const PRO_CAPABILITIES: RoomCapabilities = {
  mode: "pro", plan: true, evidenceMap: true, explorationLeads: true,
  proposalImpact: true, essayTrack: true, comprehensionCheck: false, exemplars: false,
};

// Demo is read-only pro, not a third lifecycle — same surfaces.
export const DEMO_CAPABILITIES: RoomCapabilities = { ...PRO_CAPABILITIES, mode: "demo" };

export const LITE_READING_CAPABILITIES: RoomCapabilities = {
  mode: "lite", plan: false, evidenceMap: false, explorationLeads: false,
  proposalImpact: false, essayTrack: false, comprehensionCheck: true, exemplars: false,
};
```

接进 `ReadingRoom.tsx`：`const caps = props.capabilities ?? PRO_CAPABILITIES;`，并把**证据笔记**与**追踪来源 / 线索**两处 JSX 分别包上 `{caps.evidenceMap && (…)}`、`{caps.explorationLeads && (…)}`。用 grep 定位：

```bash
grep -n "EvidenceNote\|TraceSourcePanel\|onTraceCitation\|onTraceSearch\|onAdoptSource" apps/web/src/studio/reading/ReadingRoom.tsx
```

`demoMode` 这一轮**保持不动**（收敛进 `caps.mode` 是独立清理，不在 P1 关键路径上）。

```bash
pnpm --filter @mind-imprint/web test && pnpm --filter @mind-imprint/web typecheck
```

预期：全部 PASS —— 默认值是 `PRO_CAPABILITIES`，pro 行为一字未变。

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/rooms/capabilities.ts apps/web/src/studio/reading/ReadingRoom.tsx apps/web/test/roomCapabilities.test.tsx
git commit -m "feat(lite): give the rooms a RoomCapabilities object, defaulting to pro"
```

---

### Task 11: 轻量版 shell 与阅读落地页

**Files:**
- Create: `apps/lite-web/src/routing.ts`、`src/LiteApp.tsx`、`src/api/client.ts`、`src/api/readings.ts`、`src/readings/ReadingsLanding.tsx`
- Test: `apps/lite-web/test/routing.test.ts`

**Interfaces:**
- Produces: `parseLiteRoute(pathname)`、`readingPath(id)`、`navigate(path)`；`listReadings()`、`createReading()`、`getReading()`、`putReadingSource()`

- [ ] **Step 1: 写下会失败的路由测试**

```ts
import { describe, expect, it } from "vitest";
import { parseLiteRoute, readingPath } from "@lite/routing";

describe("parseLiteRoute", () => {
  it("defaults to the readings tab", () => expect(parseLiteRoute("/")).toEqual({ tab: "readings" }));
  it("reads a reading id", () =>
    expect(parseLiteRoute("/readings/abc-123")).toEqual({ tab: "readings", readingId: "abc-123" }));
  it("knows the writings tab", () => expect(parseLiteRoute("/writings")).toEqual({ tab: "writings" }));
  it("round-trips", () => expect(parseLiteRoute(readingPath("xyz"))).toEqual({ tab: "readings", readingId: "xyz" }));
});
```

- [ ] **Step 2-4: 跑失败 → 实现 → 跑通过**

`routing.ts`：不引 router 库；`parseLiteRoute` 解析 pathname；`navigate` 用 `history.pushState` + 派发 `popstate`。

`api/client.ts`：`fetch` 包装，`credentials: "include"`。**错误体键名先读 `apps/api/internal/httpx/errors.go` 的 `WriteError` 确认**再写。

`api/readings.ts`：`listReadings` / `createReading` / `getReading` / `putReadingSource`。

`ReadingsLanding.tsx`：标题输入（placeholder「给这次阅读起个名字（可留空）」）+ 正文 textarea（placeholder「把文章正文粘贴到这里…」）+「开始阅读」按钮 → `createReading` → `putReadingSource` → `navigate(readingPath(id))`；下方「过往的阅读」列表。**这三个字符串 Task 14 的 e2e 会按字面匹配，改动请同步。**

`LiteApp.tsx`：**左侧边栏 + tab 切换，可自动折叠为纯图标**（与既有前端同形），`popstate` 监听，按路由渲染；写作 tab 本期显示「写作即将上线」。`ReadingRoomHost` 先放一个临时占位（Task 12 实现），好让构建通过。

> **为什么是 tab 侧边栏而不是 Cowork 式的会话列表**（2026-08-26 用户定夺）：Cowork 那类「顶部切换 + 下方历史会话」是为**并行工作**设计的——同时活着许多线程，列表本身就是工作区。而**读一篇文章、写一篇东西是聚焦任务**：同一时刻只有一件事在手上，侧边栏是导航而非工作区。因此保留 tab 形态，并让它**自动折叠成图标**，把宽度还给阅读室（正文 + 陪练 + 悬挂卡片三者都吃横向空间）。历史仍留在各自 tab 的落地页上。

```bash
pnpm --filter @mind-imprint/lite-web test && pnpm --filter @mind-imprint/lite-web typecheck && pnpm --filter @mind-imprint/lite-web build
```

- [ ] **Step 5: 提交**

```bash
git add apps/lite-web/src apps/lite-web/test apps/lite-web/vitest.config.ts
git commit -m "feat(lite): lite shell with the readings landing and history"
```

---

### Task 12: 把阅读室接进轻量站

**Files:**
- Create: `apps/lite-web/src/api/readingRoom.ts`、`src/readings/ReadingRoomHost.tsx`
- Test: `apps/lite-web/test/readingRoomHost.test.tsx`

- [ ] **Step 1: 照抄 pro 的调用点，摸清 `ReadingRoom` 到底要什么**

**不要凭 props 类型猜。** 打开真实调用点，把每个 prop 与 `api` 对象的每个方法列出来：

```bash
sed -n '1540,1615p' apps/web/src/workspace/WorkspaceContainer.tsx
sed -n '52,150p' apps/web/src/studio/reading/ReadingRoom.tsx
grep -n "export type ReadingLoopApi" -A 40 apps/web/src/studio/reading/readingLoop.ts
```

lite 的 `api` 对象要实现**同一组方法名与签名**，少一个就会在运行时炸在 `undefined is not a function`。在报告里列出你对齐后的完整方法表。

- [ ] **Step 2-4: 测试 → 实现 → 通过**

`api/readingRoom.ts`：每个方法映射到 Task 3-7 的 lite 端点。签名保留 pro 的形参位置（lite 不需要 projectId，忽略即可）。

`ReadingRoomHost.tsx`：加载 reading + source → 装配 props → 渲染 `<ReadingRoom capabilities={LITE_READING_CAPABILITIES} api={readingRoomApi(id)} … />`。加载中与错误各有朴素文案。

```bash
pnpm --filter @mind-imprint/lite-web test && pnpm --filter @mind-imprint/lite-web typecheck
```

- [ ] **Step 5: 在真实浏览器里走一遍**

起 `cd apps/api && make run` 与 lite dev server，把种子学校设为 lite（`UPDATE schools SET edition='lite'`），贴一篇文章 → 开始阅读 → 确认**透镜库、段落上的悬挂工具卡、批注、AI 引导都在**，证据笔记 / 追踪来源**不在**。jsdom 测不出这些，必须真看。

- [ ] **Step 6: 提交**

```bash
git add apps/lite-web/src/readings apps/lite-web/src/api/readingRoom.ts apps/lite-web/test
git commit -m "feat(lite): mount the real reading room on the lite site"
```

---

### Task 13: 部署

**Files:** `apps/lite-web/Dockerfile`、`apps/lite-web/nginx.conf`、`.deploy-local/deploy-lite.sh`

照 `apps/web/Dockerfile` / `nginx.conf` / `.deploy-local/deploy-site.sh` 改写：`--filter` 换成 `@mind-imprint/lite-web`，产物目录 `apps/lite-web/dist`，新容器名与新端口（如 `8093`），新域名。

- ⚠️ 构建上下文必须是**仓库根目录**（lite 依赖 `apps/web` 与 `packages/contracts` 源码）。
- ⚠️ 既有教训：根 `.dockerignore` 的 `*.png` 会排除图片。
- nginx 保留 SPA 的 `try_files $uri /index.html;`（`/readings/:id` 深链需要）。
- 🚨 **绝不在 ECS 上跑 `docker prune -a`。**

```bash
docker build -f apps/lite-web/Dockerfile -t mind-lite-web:dev .
docker run --rm -p 8093:80 mind-lite-web:dev
```

打开 `http://localhost:8093`，确认有样式、深链不 404。

```bash
git add apps/lite-web/Dockerfile apps/lite-web/nginx.conf .deploy-local/deploy-lite.sh
git commit -m "chore(lite): dockerfile, nginx and deploy script for the lite site"
```

---

### Task 14: 端到端走查

**Files:** `apps/lite-web/e2e/playwright.config.ts`、`apps/lite-web/e2e/reading-walk.spec.ts`

照 `apps/web/e2e/` 配置改写（`baseURL` 指 lite dev server，`webServer` 用 lite dev 命令；测试账号所属学校须为 lite，在 `globalSetup` 里种）。

```ts
import { expect, test } from "@playwright/test";

// A lite student's whole P1 journey: paste an article, read it in the real
// room, write a takeaway. Anything that dead-ends here is a shipping blocker.
test("lite reading walk: paste → read → takeaway", async ({ page }) => {
  await page.goto("/readings");
  await page.getByPlaceholder("给这次阅读起个名字（可留空）").fill("太阳能的十年");
  await page.getByPlaceholder("把文章正文粘贴到这里…").fill(
    "过去十年，全球太阳能装机容量增长了约十倍。成本下降是主要驱动力，" +
      "但并网能力与储能仍是瓶颈。若不解决储能，装机增长的边际收益会递减。",
  );
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(/\/readings\/[0-9a-f-]+$/);
  await expect(page.getByText("全球太阳能装机容量")).toBeVisible({ timeout: 30_000 });
  // 项目专属面板不在轻量版里
  await expect(page.getByText("对立题的影响")).toHaveCount(0);
});
```

> 断言文案必须与真实 DOM 对齐——先手动跑一遍 dev server，用真实文案改选择器，不要留猜的。

```bash
pnpm --filter @mind-imprint/lite-web exec playwright install --with-deps chromium
pnpm --filter @mind-imprint/lite-web exec playwright test -c e2e/playwright.config.ts
git add apps/lite-web/e2e
git commit -m "test(lite): end-to-end walk of the lite reading journey"
```

---

### Task 15: 文章文件上传（DOCX / PDF）

Task 4 有意只接了粘贴与链接（Ruling R11）。用户看过真实页面后明确要求落地页也能上传 DOCX / PDF，所以补上——`internal/docextract` 已经存在且是 pro 侧的真实解析器，直接复用。

**Files:**
- Create: `apps/api/internal/api/reading_source_file.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/reading_source_file_test.go`

**Interfaces:**
- Consumes: Task 3 的 `loadOwnedReadingAtom` / `liteOnly`；Task 4 的 `SplitBlocks` 与 `UpsertReadingSource`
- Produces: `POST /api/v1/readings/{id}/source/file`（multipart），→ `200 sourceDTO`（与 `PUT .../source` 同形，前端一条渲染路径）

- [ ] **Step 1: 先读 pro 是怎么做的**

```bash
grep -rn "ingestReferenceFile" -A 40 apps/api/internal/api/workspace_library.go | head -60
grep -rn "func " apps/api/internal/docextract/*.go | head
```

照它的**大小上限、类型允许列表、错误文案**来，不要另发明一套。在报告里写明你抄到的三项值。

- [ ] **Step 2: 写下会失败的测试**

覆盖：一个真实的小 .docx 上传成功并被分段（断言 `blocks` 非空）；不在允许列表内的类型 → 400；超过大小上限 → 400；抽取结果为空正文 → 400 `missing_text`（与 `PUT .../source` 同一语义）；跨账号 atom → 404。测试用的 fixture 文件放在 `apps/api/internal/api/testdata/`。

- [ ] **Step 3-5: 跑失败 → 实现 → 跑通过**

```bash
cd apps/api && go test ./internal/api/ -run TestReadingSourceFile -timeout 1800s
```

实现要点：抽取后走**与 `PUT .../source` 完全相同**的落库路径（`UpsertReadingSource` + `SplitBlocks`），这样「粘贴 / 链接 / 上传」三条入口产生的数据完全一致，前端不需要分支。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/api/reading_source_file.go apps/api/internal/api/reading_source_file_test.go apps/api/internal/api/testdata/ apps/api/internal/api/api.go
git commit -m "feat(lite): accept DOCX/PDF uploads for a reading's article"
```

---

### Task 16: 阅读落地页重设计（AI 产品的门面）

Task 11 交付的是**能跑的骨架**，不是设计过的门面——一个输入框、一个文本域、一个按钮。用户看过真实页面后的原话：「it is not cool, not like an AI-product」。这一任务把它变成产品。

> **实现者必须先 invoke `frontend-design` skill。** 这是一个「有辨识度的视觉设计」任务（手绘书本 / 圈词 / 手写体、边框辉光动效），不是布局任务。默认的 AI 味排版正是要避免的东西。

**Files:**
- Modify: `apps/lite-web/src/readings/ReadingsLanding.tsx`
- Create: `apps/lite-web/src/readings/ReadingHistoryPanel.tsx`
- Create: `apps/lite-web/src/readings/recommendations.ts`（种子数据）
- Modify: `apps/lite-web/src/api/readings.ts`（上传）
- Test: `apps/lite-web/test/readingsLanding.test.tsx`

**要做成什么样**

1. **居中的一句招呼**：AI 向学生打招呼——「Hi，今天要读点什么」。**「读」字要有设计**：手绘书本、圈出来的笔触、手写体三选一或组合。这是整页的视觉锚点。
2. **粘贴框**：边框有**轻微的辉光动效**（呼吸感，不是闪烁）。提示文案说明：可以贴链接、贴整篇正文，**也可以上传 DOCX / PDF**（走 Task 15 的端点）。
3. **不知道读什么？**：下方给出**今日推荐**。本期用**种子数据**（`recommendations.ts` 里 3-5 条，真实的标题 + 一句话理由 + 正文或链接），点一条即以它开始阅读。将来接样本库与教师布置的任务。
4. **右上角入口 → 我的阅读面板**：**未完成的排在最上面**，点击**继续**；已完成的在下面，点击**看报告**（报告本身是 P2）。
5. **落地页提示条**：有未完成时显示「你有 N 篇还没读完」；教师任务的位置**留出来但不接线**（P4）。

**约束**

- `mk-*` 是裸 CSS 变量：**任何 Tailwind alpha 语法（`bg-mk-x/50`）都不产出 CSS**。辉光用 `linear-gradient` / `color-mix` / `box-shadow` 做。
- 辉光与任何动效都必须尊重 `prefers-reduced-motion`。
- 复用 `apps/web` 的设计 token 与组件，不要新造一套色板。
- **动到任何 e2e 相关文案，必须在报告里逐条列出新值**——Task 14 的走查按字面匹配，它在本任务之后才写。
- 铁律②：推荐是「不知道读什么」时的帮助，**不是**信息流、不做无限滚动、不做「继续读」的成瘾式钩子。

- [ ] **Step 1: invoke frontend-design skill，先定方向**
- [ ] **Step 2: 实现落地页三块（招呼 / 辉光框 / 推荐）**
- [ ] **Step 3: 实现我的阅读面板与提示条**
- [ ] **Step 4: 接上传**
- [ ] **Step 5: `pnpm --filter @mind-imprint/lite-web test && typecheck && build`**
- [ ] **Step 6: 在真实浏览器里看，并截图**——这是唯一能验收「像不像 AI 产品」的方式
- [ ] **Step 7: 提交**

---

## 自检（写完计划后对照 spec）

| Spec 条目 | 落在哪个任务 |
|---|---|
| §4.2 `atom` / `reading` | Task 1 |
| §4.3 共享机制建表 | Task 1 |
| §4.3 message 端点 | Task 7 |
| §4.3 card 端点 | Task 6 |
| §4.3 annotation 端点 | Task 5 |
| §4.4 `reading_source` / `reading_brief` / `reading_takeaway` | Task 1（建表）、4（source）、5（brief/takeaway） |
| §4.5 `llm_call.atom_id` 成本归属 | Task 7 |
| §4.6 `schools.edition` | Task 2 |
| §5 API 全表 | Task 3、4、5、6、7、8 |
| §5.1 鉴权（本人 404 / 跨 edition 404 / 跨 atom 404） | Task 2、3、6 |
| §3 复用边界（AI 大脑原样复用） | Task 7 |
| §7.1 lite-web 与两处构建陷阱 | Task 9 |
| §7.3 `RoomCapabilities` | Task 10 |
| §7.2 shell 与落地页 | Task 11、12 |
| §6.1 阅读主动线至「我的收获」 | Task 12、14 |
| §9 风险 1（信封边界校验） | Task 6 |
| §9 风险 2（`ReadingRouteInput` 装配） | Task 4（分段单测）、7（装配函数） |
| §9 风险 3（共享组件回归 pro） | Task 10 |
| §4.4 `reading_check` / §4.3 `atom_report` / §8 简版报告 | **本期不做** —— P2 |
| §6.2 写作 | **本期不做** —— P3 |
| 文件上传（PDF/DOCX）与 `bib` | **本期不做** —— Task 4 只接粘贴与链接 |

**类型一致性：** `readingDTO.id` 全程是 **atom id**（Task 3 定义，Task 11 的 `Reading` 类型消费）；`loadOwnedReadingAtom` Task 3 定义，Task 4/5/6/7/8 消费；`Block`/`SplitBlocks` Task 4 定义，Task 7 装配与 Task 12 渲染消费；`liteOnly` Task 3 定义（依赖 Task 2 的 `requireEdition`），Task 4-8 复用；`liteHandler` / `createReadingAtom` 测试 helper 分别在 Task 2 / Task 3 定义，之后各任务复用。

**必须在实现时亲手核实的外部名字**（各任务内已标出核实步骤，不是占位符）：`agent.FocusSpan` / `PacingState` / `ReadingBrief` / `ReadingCard` / `MaterialBlock` / `OrderingGuard` 的真实字段与 `RecordLLMCall` 的签名（Task 7 Step 1）、`ReadingRoomProps` 与 `ReadingLoopApi` 的完整方法集（Task 12 Step 1）、`httpx` 的 502 构造器与错误体键名（Task 7 Step 1、Task 11）、sqlc 为 `ListReadingsByUser` 生成的 row 字段名（Task 3 Step 3）、`derefOr` / `nullableText` 是否已存在（Task 4 Step 6）。
