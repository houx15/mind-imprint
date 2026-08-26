# 轻量版 P1（地基 + 阅读原子）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一个 `edition = 'lite'` 的学生在独立的轻量站上新建一次「阅读」，贴进一篇文章，用**与现有产品完全相同的阅读室**（透镜、工具卡、批注、AI 引导）读完，并写下「我的收获」。

**Architecture:** 阅读是一个一等原子（`reading` 表），它静默持有一行 `project`（`kind='container'`）作为存储锚点。房间端点不重写：`apps/api/internal/api/projects.go:220` 的 `loadOwnedProjectRow` 是 107 个调用点唯一把 URL 变成 project id 的地方，中间件把容器 id 放进 request context，该函数优先读它——房间路由因此可以在 `/readings/{id}/room/*` 前缀下用**同一批 handler 函数**再注册一次。前端新建 `apps/lite-web`，跨 workspace 从 `apps/web` 引入房间组件，不复制。

**Tech Stack:** Go 1.22+（`net/http` 路由、`pgx`、`sqlc`、`goose`）、PostgreSQL、React + Vite + TypeScript + Tailwind、pnpm workspace、Playwright。

**Spec:** `docs/superpowers/specs/2026-08-26-lite-edition-writings-readings-design.md`

## Global Constraints

- **迁移编号从 `0092` 起**（当前最高为 `0091_demo_reading_notes.sql`）。goose 格式：`-- +goose Up` / `-- +goose Down` 两段，Down 必须真正可回滚。
- **sqlc 重新生成必须用 `CGO_ENABLED=0`**：`cd apps/api && make sqlc`（Makefile 已内置该变量；macOS 上原生 pg_query C 库会构建失败）。
- **Go 测试**：`cd apps/api && go test ./internal/api/ -timeout 1800s`。集成测试用 testcontainers，第一次会拉 Postgres 镜像。
- **绝不 `git add -A`**：每次提交只 stage 本任务明确列出的文件。
- **密钥绝不进 git / 日志 / 错误体**。
- **归属失败一律 404**（`httpx.ErrNotFound("资源不存在")`），绝不用 403 —— 既有约定，不泄漏资源存在性。
- **`mk-*` 是裸 CSS 变量**：`bg-mk-x/NN` 这类 Tailwind alpha 语法**不产出任何 CSS**。需要透明度时用 `linear-gradient` / `color-mix`，并在**真实浏览器**里验证。
- **铁律②**：不做连胜、排行榜、徽章、推送。
- 本期**不做** `writing` 表、`reading_check`、`atom_report` —— 它们随各自的 handler 在 P2/P3 落地（YAGNI：P1 没有任何代码读它们）。
- 本期**不实现** spec §4.1 的 `POST /readings/{id}/text`：`GET /readings/{id}` 已回传 `referenceId`，前端直接走 `/room/references/{rid}/paste-content` 透传，与既有实现完全一致，不再包一层。

---

### Task 1: `project.kind` 与容器泄漏封堵

隐藏容器是本设计最高风险项。这个任务先立起「容器行绝不出现在任何项目列表/聚合里」的护栏，**再**让任何东西去创建容器。

**Files:**
- Create: `apps/api/internal/store/migrations/0092_project_kind.sql`
- Modify: `apps/api/internal/store/queries/project.sql`
- Modify: `apps/api/internal/store/queries/teacher.sql`
- Modify: `apps/api/internal/store/queries/org.sql`
- Regenerate: `apps/api/internal/store/sqlc/*.sql.go`（由 `make sqlc` 产出，不手改）
- Test: `apps/api/internal/api/project_kind_test.go`

**Interfaces:**
- Consumes: 无（本计划的第一个任务）
- Produces: `project.kind text NOT NULL DEFAULT 'project' CHECK (kind IN ('project','container'))`。后续任务用 `kind = 'container'` 建容器。sqlc 生成的 `sqlc.Project` 结构体新增字段 `Kind string`。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/project_kind_test.go`：

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// insertContainerProject inserts a raw container-kind project row for the seed
// user, bypassing the API (nothing creates containers yet — that lands in
// Task 4). Returns its id.
func insertContainerProject(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) interface{ Scan(...any) error }
}) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO project (user_id, qualification, title, board_cfg_ver, kind)
		 VALUES ($1, '0457', '容器', 1, 'container') RETURNING id::text`,
		SeedUserID).Scan(&id)
	if err != nil {
		t.Fatalf("insert container project: %v", err)
	}
	return id
}

// TestListProjects_ExcludesContainers is the guard rail for the hidden-container
// design: a container row is storage for a lite atom, never a project. If this
// ever fails, containers are leaking into the student's project list — and by
// the same query shape, into teacher dashboards and cost aggregates.
func TestListProjects_ExcludesContainers(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	containerID := insertContainerProject(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /projects = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	for _, p := range out.Projects {
		if p.ID == containerID {
			t.Fatalf("container project %s leaked into GET /projects", containerID)
		}
	}
}
```

> 若 `GET /projects` 的响应键不是 `projects`，先读 `apps/api/internal/api/projects.go` 的 `listProjects` 末尾的 `httpx.WriteJSON` 确认真实键名，并改这里的结构体标签。

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestListProjects_ExcludesContainers -timeout 1800s
```

预期：FAIL —— 编译期就会挂在 `kind` 列不存在（`INSERT ... kind` 报 `column "kind" of relation "project" does not exist`）。

- [ ] **Step 3: 写迁移**

新建 `apps/api/internal/store/migrations/0092_project_kind.sql`：

```sql
-- +goose Up
-- 轻量版（lite edition）的写作/阅读原子各自静默持有一行 project 作为存储锚点
-- （reference / material / card_instances / outline_node / snippet /
-- draft_snapshot 等全部外键到 project）。kind 把这些容器行与真正的项目分开，
-- 好让每一处项目列表与聚合都能把它们排除掉。
ALTER TABLE project ADD COLUMN kind text NOT NULL DEFAULT 'project'
  CHECK (kind IN ('project','container'));
CREATE INDEX project_kind_idx ON project (kind);

-- +goose Down
DROP INDEX IF EXISTS project_kind_idx;
ALTER TABLE project DROP COLUMN kind;
```

- [ ] **Step 4: 给每一处读 `project` 的查询加过滤**

先把范围列全：

```bash
cd apps/api && grep -rn "FROM project\b\|JOIN project\b" internal/store/queries/
```

对上面列出的**每一条**查询，判断它是否面向「学生的项目 / 教师看板 / 组织聚合」；是则加 `kind = 'project'`。已知至少这两条必须改（`internal/store/queries/project.sql`）：

```sql
-- name: ListProjectsByUser :many
SELECT * FROM project
WHERE user_id = $1 AND kind = 'project'
ORDER BY last_active_at DESC;

-- name: ListDemoProjects :many
-- The world-readable demo project(s) — shown in EVERY authenticated user's list,
-- pinned last and marked isDemo (guided-tour P5). Same columns as
-- ListProjectsByUser so the handler folds both into one projectListItem shape.
SELECT * FROM project
WHERE is_demo = true AND kind = 'project'
ORDER BY last_active_at DESC;
```

`GetProject`（按主键取单行）**不加**过滤 —— 容器行必须能被 Task 5 的透传取到；直达 `/projects/{id}` 的拦截在 handler 层做（Task 5 Step 5）。

`CountLLMCallsByUserProject` 与 `CountActivityLogByUserProject` 是按 project 分组的计数：前者从 `llm_call` 出发、不 JOIN project，容器的调用量会落在一个不在列表里的 id 上，无害；后者 `JOIN project p` 且只按 `p.user_id` 过滤，**必须**补 `AND p.kind = 'project'`：

```sql
-- name: CountActivityLogByUserProject :many
-- Per-project activity-log totals for the caller's whole project list, in ONE
-- grouped pass. activity_log_entry has no user_id, so join project to scope to
-- the caller; activity_log_entry is indexed on project_id. Containers (lite
-- atoms' storage rows) are excluded — they are not projects.
SELECT a.project_id, COUNT(*)::int AS n
FROM activity_log_entry a
JOIN project p ON p.id = a.project_id
WHERE p.user_id = $1 AND p.kind = 'project'
GROUP BY a.project_id;
```

`queries/teacher.sql` 与 `queries/org.sql` 里每一条 `JOIN project` / `FROM project` 的聚合，同样补 `kind = 'project'`。

- [ ] **Step 5: 重新生成 sqlc**

```bash
cd apps/api && make sqlc
```

预期：`internal/store/sqlc/*.sql.go` 变更，`sqlc.Project` 多出 `Kind string` 字段。

- [ ] **Step 6: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run TestListProjects_ExcludesContainers -timeout 1800s
```

预期：PASS。

- [ ] **Step 7: 跑全量 API 测试，确认没打坏别的**

```bash
cd apps/api && go test ./internal/api/ ./internal/store/ ./internal/teacher/ -timeout 1800s
```

预期：全部 PASS。任何因新增 `Kind` 字段而挂掉的结构体字面量（缺字段的 `sqlc.Project{...}`）就地补上。

- [ ] **Step 8: 提交**

```bash
git add apps/api/internal/store/migrations/0092_project_kind.sql \
        apps/api/internal/store/queries/ \
        apps/api/internal/store/sqlc/ \
        apps/api/internal/api/project_kind_test.go
git commit -m "feat(lite): add project.kind and exclude containers from every project read"
```

---

### Task 2: `users.edition` 与站点分流

**Files:**
- Create: `apps/api/internal/store/migrations/0093_users_edition.sql`
- Modify: `apps/api/internal/api/dto.go`（`meUserDTO` + `buildMeUser`）
- Test: `apps/api/internal/api/edition_test.go`

**Interfaces:**
- Consumes: Task 1 的迁移编号序列
- Produces: `users.edition text NOT NULL DEFAULT 'pro' CHECK (edition IN ('pro','lite'))`；`GET /api/v1/auth/me` 的 `user` 对象新增 `"edition"` 字段（字符串）。前端据此判断是否走错了站。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/edition_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestMe_ReportsEdition — /auth/me carries the account's edition so each site
// can tell a student they've landed on the wrong one. Default is 'pro'.
func TestMe_ReportsEdition(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/auth/me", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /auth/me = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		User struct {
			Edition string `json:"edition"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.User.Edition != "pro" {
		t.Fatalf("edition = %q, want \"pro\"", out.User.Edition)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestMe_ReportsEdition -timeout 1800s
```

预期：FAIL —— `edition = "", want "pro"`。

- [ ] **Step 3: 写迁移**

新建 `apps/api/internal/store/migrations/0093_users_edition.sql`：

```sql
-- +goose Up
-- edition 只决定账号进哪个站（现有的 pro 站 / 轻量站），不改变组织归属：
-- 每个账号仍必属某学校 + ≥1 班级，注册仍需 join code。
ALTER TABLE users ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));

-- +goose Down
ALTER TABLE users DROP COLUMN edition;
```

- [ ] **Step 4: 重新生成 sqlc 并把字段接进 DTO**

```bash
cd apps/api && make sqlc
```

在 `apps/api/internal/api/dto.go` 的 `meUserDTO` 结构体里，`Role` 之后加一行：

```go
	Edition        string       `json:"edition"`
```

在同文件 `buildMeUser` 的返回字面量里，`Role: full.Role,` 之后加一行：

```go
		Edition:        full.Edition,
```

- [ ] **Step 5: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run TestMe_ReportsEdition -timeout 1800s
```

预期：PASS。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/store/migrations/0093_users_edition.sql \
        apps/api/internal/store/queries/ apps/api/internal/store/sqlc/ \
        apps/api/internal/api/dto.go apps/api/internal/api/edition_test.go
git commit -m "feat(lite): add users.edition and surface it on /auth/me"
```

---

### Task 3: `reading` 表与查询

**Files:**
- Create: `apps/api/internal/store/migrations/0094_reading_atom.sql`
- Create: `apps/api/internal/store/queries/reading.sql`
- Regenerate: `apps/api/internal/store/sqlc/reading.sql.go`
- Test: `apps/api/internal/store/reading_store_test.go`

**Interfaces:**
- Consumes: Task 1 的 `project.kind`
- Produces: sqlc 方法 `CreateReading(ctx, CreateReadingParams{UserID, Title, Lang, ContainerID}) (Reading, error)`、`GetReading(ctx, id) (Reading, error)`、`ListReadingsByUser(ctx, userID) ([]Reading, error)`、`RenameReading(ctx, RenameReadingParams{ID, Title})`、`SetReadingReference(ctx, SetReadingReferenceParams{ID, ReferenceID})`、`SetReadingFinished(ctx, id)`。`sqlc.Reading` 字段：`ID`、`UserID`、`Title`、`Lang`、`Status`、`ContainerID`（均 uuid.UUID / string）、`ReferenceID`（可空）、`CreatedAt`、`UpdatedAt`、`FinishedAt`。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/store/reading_store_test.go`。先读 `apps/api/internal/store/sqlc_test.go` 顶部，照抄它的 pool bootstrap helper 名（本包已有，不要新建）：

```go
package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// TestCreateReading_RoundTrips — a reading atom persists and reads back with
// its container, and defaults to status 'active'.
func TestCreateReading_RoundTrips(t *testing.T) {
	pool := newTestPool(t) // ← 用本包既有的 helper 名，见 sqlc_test.go
	q := sqlc.New(pool)
	ctx := context.Background()

	var userID, containerID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO project (user_id, qualification, title, board_cfg_ver, kind)
		 VALUES ($1, '0457', '容器', 1, 'container') RETURNING id::text`,
		userID).Scan(&containerID); err != nil {
		t.Fatalf("insert container: %v", err)
	}

	rd, err := q.CreateReading(ctx, sqlc.CreateReadingParams{
		UserID:      mustParseUUID(t, userID),
		Title:       "一篇文章",
		Lang:        "zh",
		ContainerID: mustParseUUID(t, containerID),
	})
	if err != nil {
		t.Fatalf("CreateReading: %v", err)
	}
	if rd.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", rd.Status)
	}

	got, err := q.GetReading(ctx, rd.ID)
	if err != nil {
		t.Fatalf("GetReading: %v", err)
	}
	if got.Title != "一篇文章" || got.Lang != "zh" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	list, err := q.ListReadingsByUser(ctx, mustParseUUID(t, userID))
	if err != nil {
		t.Fatalf("ListReadingsByUser: %v", err)
	}
	if len(list) != 1 || list[0].ID != rd.ID {
		t.Fatalf("list = %+v, want exactly the created reading", list)
	}
}
```

若本包没有 `mustParseUUID`，在测试文件底部加：

```go
func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}
```

并 import `"github.com/google/uuid"`。

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/store/ -run TestCreateReading_RoundTrips -timeout 1800s
```

预期：FAIL —— 编译不过，`sqlc.CreateReadingParams` 未定义。

- [ ] **Step 3: 写迁移**

新建 `apps/api/internal/store/migrations/0094_reading_atom.sql`：

```sql
-- +goose Up
-- 阅读是轻量版的一等原子：学生的一次阅读练习。它静默持有一行 kind='container'
-- 的 project 作为存储锚点，好让阅读室的既有端点一行都不用改。writing 是它的
-- 兄弟表（P3 落地），chat / project 之后同样以兄弟身份加入——所以这里不设
-- kind 判别列。
CREATE TABLE reading (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title        text NOT NULL DEFAULT '',
  lang         text NOT NULL CHECK (lang IN ('zh','en')),
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  container_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  reference_id uuid REFERENCES reference(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz
);
CREATE INDEX reading_user_created_idx ON reading (user_id, created_at DESC);
CREATE UNIQUE INDEX reading_container_idx ON reading (container_id);

-- +goose Down
DROP TABLE reading;
```

- [ ] **Step 4: 写查询**

新建 `apps/api/internal/store/queries/reading.sql`：

```sql
-- name: CreateReading :one
INSERT INTO reading (user_id, title, lang, container_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetReading :one
SELECT * FROM reading WHERE id = $1;

-- name: ListReadingsByUser :many
SELECT * FROM reading
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: RenameReading :exec
UPDATE reading SET title = $2, updated_at = now() WHERE id = $1;

-- name: SetReadingReference :exec
-- The atom's single reference row, minted alongside the container at create
-- time. The reading room addresses itself by project + reference, so the DTO
-- hands this id to the client.
UPDATE reading SET reference_id = $2, updated_at = now() WHERE id = $1;

-- name: SetReadingFinished :exec
UPDATE reading SET status = 'finished', finished_at = now(), updated_at = now()
WHERE id = $1;
```

- [ ] **Step 5: 重新生成 sqlc**

```bash
cd apps/api && make sqlc
```

- [ ] **Step 6: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/store/ -run TestCreateReading_RoundTrips -timeout 1800s
```

预期：PASS。

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/store/migrations/0094_reading_atom.sql \
        apps/api/internal/store/queries/reading.sql \
        apps/api/internal/store/sqlc/ \
        apps/api/internal/store/reading_store_test.go
git commit -m "feat(lite): add the reading atom table and its queries"
```

---

### Task 4: 阅读原子 CRUD 端点

**Files:**
- Create: `apps/api/internal/api/readings.go`
- Modify: `apps/api/internal/api/api.go`（注册 4 条路由）
- Test: `apps/api/internal/api/readings_test.go`

**Interfaces:**
- Consumes: Task 3 的 sqlc 方法；Task 1 的 `kind='container'`
- Produces:
  - `POST /api/v1/readings` → `201 {"id","referenceId"}`
  - `GET /api/v1/readings` → `200 {"readings":[readingDTO]}`
  - `GET /api/v1/readings/{id}` → `200 readingDTO`
  - `PATCH /api/v1/readings/{id}` → `200 readingDTO`（改标题）
  - `readingDTO` = `{id, title, lang, status, referenceId, createdAt, updatedAt, finishedAt}`，时间用 RFC3339，`referenceId`/`finishedAt` 可为 `null`
  - Go 侧导出的构造器 `func (a *API) readingDTOOf(rd sqlc.Reading) readingDTO`，Task 5/6 复用

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/api/internal/api/readings_test.go`：

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func liteHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries) {
	t.Helper()
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), q
}

// TestCreateReading_MintsContainerAndReference — creating a reading atom also
// creates its hidden container project and the single reference the reading
// room addresses itself by. All three in one transaction.
func TestCreateReading_MintsContainerAndReference(t *testing.T) {
	h, cookie, q := liteHandler(t)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"气候变化与农业","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /readings = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID          string `json:"id"`
		ReferenceID string `json:"referenceId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.ID == "" || out.ReferenceID == "" {
		t.Fatalf("want both id and referenceId, got %+v", out)
	}

	rd, err := q.GetReading(context.Background(), mustUUID(out.ID))
	if err != nil {
		t.Fatalf("GetReading: %v", err)
	}
	proj, err := q.GetProject(context.Background(), rd.ContainerID)
	if err != nil {
		t.Fatalf("GetProject(container): %v", err)
	}
	if proj.Kind != "container" {
		t.Fatalf("container project kind = %q, want \"container\"", proj.Kind)
	}
}

// TestListReadings_OnlyMine — a reading is private to its owner; another
// account's atom is invisible, and a direct GET on it is 404 (never 403).
func TestListReadings_OnlyMine(t *testing.T) {
	h, cookie, _ := liteHandler(t)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"我的","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readings = %d, want 200", rec.Code)
	}
	var list struct {
		Readings []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(list.Readings) != 1 || list.Readings[0].Title != "我的" {
		t.Fatalf("list = %+v, want exactly my one reading", list.Readings)
	}
}

// TestGetReading_UnknownIs404 — existence is never leaked.
func TestGetReading_UnknownIs404(t *testing.T) {
	h, cookie, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unknown reading = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateReading_MintsContainerAndReference|TestListReadings_OnlyMine|TestGetReading_UnknownIs404' -timeout 1800s
```

预期：全部 FAIL —— 404，路由尚不存在。

- [ ] **Step 3: 确认 `CreateReference` 的真实参数**

阅读原子建容器时要顺带建一行 reference。**先读真实签名**，不要凭记忆写：

```bash
cd apps/api && sed -n '366,430p' internal/api/workspace_library.go
grep -n "name: CreateReference" -A 12 internal/store/queries/*.sql
```

把 `sqlc.CreateReferenceParams` 的必填字段抄进下一步的代码里（下面的实现按「`ProjectID` + `Title` + 其余留零值」写，若真实结构体有额外 NOT NULL 字段，照抄 `createReference` handler 的填法）。

- [ ] **Step 4: 写实现**

新建 `apps/api/internal/api/readings.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readings.go — the lite edition's 阅读 atom. A reading is a student's single
// reading exercise. It quietly owns one kind='container' project row plus one
// reference row: every reading-room endpoint addresses itself by project +
// reference, so giving the atom both lets those endpoints be reused verbatim
// through the /readings/{id}/room/* passthrough (atom_container.go).

type readingDTO struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Lang        string  `json:"lang"`
	Status      string  `json:"status"`
	ReferenceID *string `json:"referenceId"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	FinishedAt  *string `json:"finishedAt"`
}

func (a *API) readingDTOOf(rd sqlc.Reading) readingDTO {
	out := readingDTO{
		ID:        rd.ID.String(),
		Title:     rd.Title,
		Lang:      rd.Lang,
		Status:    rd.Status,
		CreatedAt: rd.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt: rd.UpdatedAt.Time.Format(time.RFC3339),
	}
	if rd.ReferenceID.Valid {
		s := uuid.UUID(rd.ReferenceID.Bytes).String()
		out.ReferenceID = &s
	}
	if rd.FinishedAt.Valid {
		s := rd.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// loadOwnedReading parses {id} and confirms the caller owns it. Ownership
// failure is 404, never 403 — existence is never leaked (same convention as
// loadOwnedProjectRow).
func (a *API) loadOwnedReading(w http.ResponseWriter, r *http.Request) (sqlc.Reading, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Reading{}, false
	}
	rd, err := a.d.Queries.GetReading(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Reading{}, false
	}
	if rd.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Reading{}, false
	}
	return rd, true
}

// createReading mints, in ONE transaction: the container project, the single
// reference inside it, and the reading atom pointing at both. Any failure rolls
// all three back — a half-built atom would strand the student in a room with no
// reference to read.
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

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	container, err := qtx.CreateProject(r.Context(), sqlc.CreateProjectParams{
		UserID: u.ID, Qualification: "lite", Title: title,
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1, Cover: nil,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// CreateProject does not set kind (it is the pro funnel's query); flip this
	// row to a container so every project list and aggregate skips it.
	if _, err := tx.Exec(r.Context(), `UPDATE project SET kind = 'container' WHERE id = $1`, container.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ref, err := qtx.CreateReference(r.Context(), sqlc.CreateReferenceParams{
		ProjectID: container.ID, Title: title,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := qtx.CreateReading(r.Context(), sqlc.CreateReadingParams{
		UserID: u.ID, Title: title, Lang: lang, ContainerID: container.ID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.SetReadingReference(r.Context(), sqlc.SetReadingReferenceParams{
		ID: rd.ID, ReferenceID: pgtype.UUID{Bytes: ref.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": rd.ID.String(), "referenceId": ref.ID.String(),
	})
}

func (a *API) listReadings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]readingDTO, 0, len(rows))
	for _, rd := range rows {
		out = append(out, a.readingDTOOf(rd))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"readings": out})
}

func (a *API) getReading(w http.ResponseWriter, r *http.Request) {
	rd, ok := a.loadOwnedReading(w, r)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd))
}

func (a *API) renameReading(w http.ResponseWriter, r *http.Request) {
	rd, ok := a.loadOwnedReading(w, r)
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
	if err := a.d.Queries.RenameReading(r.Context(), sqlc.RenameReadingParams{ID: rd.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	fresh, err := a.d.Queries.GetReading(r.Context(), rd.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(fresh))
}
```

> 若 `sqlc.Reading.ReferenceID` 生成为 `*uuid.UUID` 而非 `pgtype.UUID`，按生成结果调整 `readingDTOOf` 与 `SetReadingReference` 的取值；`pgtype` 与 `google/uuid` 不可混用。

- [ ] **Step 5: 注册路由**

在 `apps/api/internal/api/api.go` 的 `mux.Handle("POST /api/v1/projects", protected(a.createProject))` 那一行**之后**插入：

```go
	// 轻量版（lite edition）· 阅读原子。房间端点在 atom_container.go 里以
	// /readings/{id}/room/* 前缀复用同一批 handler，不在这里重复声明。
	mux.Handle("GET /api/v1/readings", protected(a.listReadings))
	mux.Handle("POST /api/v1/readings", protected(a.createReading))
	mux.Handle("GET /api/v1/readings/{id}", protected(a.getReading))
	mux.Handle("PATCH /api/v1/readings/{id}", protected(a.renameReading))
```

- [ ] **Step 6: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateReading_MintsContainerAndReference|TestListReadings_OnlyMine|TestGetReading_UnknownIs404' -timeout 1800s
```

预期：全部 PASS。

- [ ] **Step 7: 确认容器护栏仍然成立**

```bash
cd apps/api && go test ./internal/api/ -run TestListProjects_ExcludesContainers -timeout 1800s
```

预期：PASS —— 现在有真实的容器行在被创建，这条护栏才第一次有真正的意义。

- [ ] **Step 8: 提交**

```bash
git add apps/api/internal/api/readings.go apps/api/internal/api/readings_test.go apps/api/internal/api/api.go
git commit -m "feat(lite): reading atom CRUD, minting container + reference in one tx"
```

---

### Task 5: 容器透传中间件与 `loadOwnedProjectRow` 接缝

这是整个设计的支点：一个函数改动，换来整个阅读室零改动复用。

**Files:**
- Create: `apps/api/internal/api/atom_container.go`
- Modify: `apps/api/internal/api/projects.go:220-241`（`loadOwnedProjectRow`）
- Modify: `apps/api/internal/api/api.go`（调用 `a.mountReadingRoom(mux)`）
- Test: `apps/api/internal/api/atom_container_test.go`

**Interfaces:**
- Consumes: Task 4 的 `loadOwnedReading`；Task 1 的 `project.kind`
- Produces: `func (a *API) withReadingContainer(h http.HandlerFunc) http.Handler`；`func (a *API) mountReadingRoom(mux *http.ServeMux)`；context 读写对 `withContainerID(ctx, uuid.UUID) context.Context` / `containerIDFrom(ctx) (uuid.UUID, bool)`

- [ ] **Step 1: 先确认没有 handler 绕开接缝**

`loadOwnedProjectRow` 是唯一的 project-id 入口——但要亲手证实，别信注释：

```bash
cd apps/api && grep -rn 'PathValue("id")' internal/api/ | grep -v _test
```

预期：只有 `projects.go` 里那一处（`loadOwnedProjectRow` 内）。若还有其他命中，**逐一确认它不属于阅读室路由表**（Step 5 的列表）。属于的话，把它也改成走 `containerIDFrom`，并在提交信息里点名。

- [ ] **Step 2: 写下会失败的测试**

新建 `apps/api/internal/api/atom_container_test.go`：

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createReadingAtom returns (readingID, referenceID).
func createReadingAtom(t *testing.T, h http.Handler, cookie *http.Cookie) (string, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"一篇文章","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reading = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID          string `json:"id"`
		ReferenceID string `json:"referenceId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out.ID, out.ReferenceID
}

// TestRoomPassthrough_ReachesTheContainer — the reading room's own endpoints,
// mounted under the atom prefix, resolve to the atom's container project. This
// is the whole reuse story: same handler, different owner.
func TestRoomPassthrough_ReachesTheContainer(t *testing.T) {
	h, cookie, _ := liteHandler(t)
	rid, refID := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"text":"太阳能装机在过去十年增长了十倍。"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+rid+"/room/references/"+refID+"/paste-content", body), cookie))
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("paste-content through atom prefix = %d, want 2xx; body=%s", rec.Code, rec.Body)
	}
}

// TestRoomPassthrough_ForeignAtomIs404 — another account's atom is unreachable,
// and the failure is 404 (never 403).
func TestRoomPassthrough_ForeignAtomIs404(t *testing.T) {
	h, cookie, _ := liteHandler(t)
	_, refID := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"text":"x"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/00000000-0000-0000-0000-0000000009ff/room/references/"+refID+"/paste-content",
		body), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown atom = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestContainerNotReachableAsProject — a container is storage for an atom, not
// a project. Hitting it directly on /projects/{id} is 404.
func TestContainerNotReachableAsProject(t *testing.T) {
	h, cookie, q := liteHandler(t)
	rid, _ := createReadingAtom(t, h, cookie)

	rd, err := q.GetReading(t.Context(), mustUUID(rid))
	if err != nil {
		t.Fatalf("GetReading: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/projects/"+rd.ContainerID.String(), nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /projects/{container} = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
```

> `t.Context()` 需要 Go 1.24+。若本仓库的 Go 版本更低（看 `apps/api/go.mod` 的 `go` 行），换成 `context.Background()` 并 import `"context"`。

- [ ] **Step 3: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run 'TestRoomPassthrough|TestContainerNotReachableAsProject' -timeout 1800s
```

预期：前两条 404（路由不存在），第三条 200（容器目前还能当项目直接打开）—— 三条全 FAIL。

- [ ] **Step 4: 写中间件与路由表**

新建 `apps/api/internal/api/atom_container.go`：

```go
package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// atom_container.go — the one seam that lets the lite edition reuse the rooms
// instead of forking them.
//
// Every room endpoint addresses itself by project id, and every one of them
// funnels through loadOwnedProjectRow. So rather than re-declaring ~20 handlers
// under a second prefix, we re-REGISTER the same handler funcs under
// /readings/{id}/room/* behind a middleware that resolves the atom to its
// container project and puts that id in the request context.
// loadOwnedProjectRow prefers the context value over PathValue("id").

type containerCtxKey struct{}

// withContainerID scopes an atom's container project id into ctx. Set only by
// the atom middlewares below; read only by loadOwnedProjectRow.
func withContainerID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, containerCtxKey{}, id)
}

func containerIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(containerCtxKey{}).(uuid.UUID)
	return id, ok
}

// withReadingContainer resolves {id} as a reading atom owned by the caller and
// hands the wrapped handler its container. Ownership failure is 404.
func (a *API) withReadingContainer(h http.HandlerFunc) http.Handler {
	return RequireUser(func(w http.ResponseWriter, r *http.Request) {
		rd, ok := a.loadOwnedReading(w, r)
		if !ok {
			return
		}
		h(w, r.WithContext(withContainerID(r.Context(), rd.ContainerID)))
	})
}

// mountReadingRoom registers the reading room's surface a second time under the
// atom prefix. Same handler funcs as /projects/{id}/… — the ONLY difference is
// which project id they resolve to. Keep this list in sync when a reading-room
// endpoint is added on the pro side.
func (a *API) mountReadingRoom(mux *http.ServeMux) {
	const p = "/api/v1/readings/{id}/room"
	for _, rt := range []struct {
		pattern string
		h       http.HandlerFunc
	}{
		{"GET " + p + "/library", a.getLibrary},
		{"PATCH " + p + "/references/{rid}", a.patchReference},
		{"POST " + p + "/references/{rid}/enter-reading", a.enterReading},
		{"POST " + p + "/references/{rid}/paste-content", a.pasteContent},
		{"POST " + p + "/references/{rid}/ingest-file", a.ingestReferenceFile},
		{"PUT " + p + "/references/{rid}/reading-brief", a.putReadingBrief},
		{"GET " + p + "/references/{rid}/takeaway-draft", a.getTakeawayDraft},
		{"POST " + p + "/references/{rid}/finalize-reading", a.postFinalizeReading},
		{"GET " + p + "/materials/{mid}/source", a.getMaterialSource},
		{"POST " + p + "/materials/{mid}/read-turn", a.postReadTurn},
		{"POST " + p + "/materials/{mid}/summon-card", a.postSummonCard},
		{"GET " + p + "/materials/{mid}/open-card", a.getOpenCard},
		{"POST " + p + "/cards/{cid}/activate", a.activateCard},
		{"POST " + p + "/cards/{cid}/skip", a.skipCard},
		{"POST " + p + "/cards/{cid}/submit", a.submitCard},
		{"POST " + p + "/cards/persist", a.persistCard},
		{"POST " + p + "/cards/reflect", a.reflectCard},
		{"GET " + p + "/annotations", a.listAnnotations},
		{"POST " + p + "/annotations/open", a.openAnnotations},
		{"POST " + p + "/citations", a.postCitation},
	} {
		mux.Handle(rt.pattern, a.withReadingContainer(rt.h))
	}
}
```

> **方法名必须核对。** 上表的 handler 方法名取自 `api.go` 的既有注册行。逐条与 `api.go` 里对应的 `/projects/{id}/…` 注册行比对，名字不一致就以 `api.go` 为准改这里 —— 编译器会替你抓出大部分，但 `getTakeawayDraft` / `getMaterialSource` 这类要亲眼确认。

- [ ] **Step 5: 改接缝**

把 `apps/api/internal/api/projects.go` 的 `loadOwnedProjectRow` 整体替换为：

```go
// loadOwnedProjectRow is loadOwnedProject's row-returning sibling: same
// existence/ownership/demo-visibility semantics, but it does NOT apply the
// non-GET demo_readonly 403 itself — it returns the row so a caller can
// decide (e.g. a future canned-fixture short-circuit for demo POSTs instead
// of a flat 403). loadOwnedProject is a thin wrapper around this for the
// common case.
//
// It is also the lite edition's seam (atom_container.go): when the request
// arrived through an atom prefix (/readings/{id}/room/…), the middleware has
// already resolved the atom to its container project and put that id in the
// context — prefer it over {id}, which is the ATOM's id there, not a project's.
func (a *API) loadOwnedProjectRow(w http.ResponseWriter, r *http.Request) (sqlc.Project, bool) {
	u, _ := UserFromContext(r.Context())
	id, viaAtom := containerIDFrom(r.Context())
	if !viaAtom {
		parsed, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return sqlc.Project{}, false
		}
		id = parsed
	}
	p, err := a.d.Queries.GetProject(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Project{}, false
	}
	// A container row is a lite atom's storage, reachable ONLY through its atom
	// prefix. Hit directly on /projects/{id} it is simply not a project.
	if p.Kind == "container" && !viaAtom {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Project{}, false
	}
	if p.IsDemo {
		// World-readable to any authenticated user, regardless of ownership.
		return p, true
	}
	if p.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Project{}, false
	}
	return p, true
}
```

- [ ] **Step 6: 挂上路由**

在 `apps/api/internal/api/api.go` 中，Task 4 加的四条 `/readings` 路由**之后**加一行：

```go
	a.mountReadingRoom(mux)
```

- [ ] **Step 7: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run 'TestRoomPassthrough|TestContainerNotReachableAsProject' -timeout 1800s
```

预期：全部 PASS。

- [ ] **Step 8: 跑全量，确认 pro 一点没坏**

```bash
cd apps/api && go test ./... -timeout 1800s
```

预期：全部 PASS。这一步是本任务真正的验收 —— 接缝改的是 107 个调用点共用的函数。

- [ ] **Step 9: 提交**

```bash
git add apps/api/internal/api/atom_container.go apps/api/internal/api/atom_container_test.go \
        apps/api/internal/api/projects.go apps/api/internal/api/api.go
git commit -m "feat(lite): reuse the reading room under /readings/{id}/room via a container seam"
```

---

### Task 6: edition 分流闸

**Files:**
- Modify: `apps/api/internal/api/authz.go`（新增 `requireEdition`）
- Modify: `apps/api/internal/api/api.go`（包住两组路由）
- Test: `apps/api/internal/api/edition_test.go`（追加）

**Interfaces:**
- Consumes: Task 2 的 `users.edition`
- Produces: `func requireEdition(want string, h http.Handler) http.Handler`

- [ ] **Step 1: 写下会失败的测试**

追加到 `apps/api/internal/api/edition_test.go`：

```go
// TestEditionGate_LiteAccountCannotReachProjects — a lite student has no
// projects; the pro surface is simply not there for them. 404, not 403.
func TestEditionGate_LiteAccountCannotReachProjects(t *testing.T) {
	h, cookie, _ := liteHandler(t)
	pool := poolOf(t, h) // see note below

	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET edition = 'lite' WHERE id = $1`, SeedUserID); err != nil {
		t.Fatalf("set edition: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("lite account GET /projects = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestEditionGate_ProAccountCannotReachReadings — the mirror image.
func TestEditionGate_ProAccountCannotReachReadings(t *testing.T) {
	h, cookie, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("pro account GET /readings = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
```

把 Task 4 的 `liteHandler` 改成同时回传 pool，好让上面的 `UPDATE users` 能跑（并把 Task 4/5 里的调用点一起改）：

```go
func liteHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries, *pgxpool.Pool) { … }
```

**同时**：Task 4/5 的既有测试目前用的是默认 `edition='pro'` 的种子账号，而它们打的是 `/readings/*` —— 加闸后会全部变 404。所以在 `liteHandler` 里建完 handler 后立刻把种子账号切成 lite：

```go
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET edition = 'lite' WHERE id = $1`, SeedUserID); err != nil {
		t.Fatalf("set edition lite: %v", err)
	}
```

并把 `TestEditionGate_ProAccountCannotReachReadings` 改用一个**新的** pro handler（直接用 `newAPITestPool` + `signInSeed`，不经 `liteHandler`）。

- [ ] **Step 2: 跑测试，确认它失败**

```bash
cd apps/api && go test ./internal/api/ -run TestEditionGate -timeout 1800s
```

预期：FAIL —— 两条都拿到 200。

- [ ] **Step 3: 写闸**

在 `apps/api/internal/api/authz.go` 末尾追加：

```go
// requireEdition gates a route group on the caller's account edition. A
// mismatch is 404, not 403 — from a lite student's point of view the pro
// surface does not exist, and vice versa. Same non-leaking convention as
// ownership failures.
func requireEdition(want string, h http.Handler) http.Handler {
	return RequireUser(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFromContext(r.Context())
		if u.Edition != want {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		h.ServeHTTP(w, r)
	})
}
```

若 `User`（session principal）上没有 `Edition` 字段，在它的构造处（`SessionAuth` / `UserFromContext` 的来源，见 `apps/api/internal/api/auth.go`）把 `edition` 一并带上；session 已经查了 users 行，加一个字段即可，不增加查询。

- [ ] **Step 4: 包住两组路由**

在 `api.go` 里，把 `protected` 之外再定义两个包装器，并把 `/projects` 与 `/readings` 两组分别换掉：

```go
	// 站点分流：pro 账号看不见 /readings，lite 账号看不见 /projects——
	// 双向都是 404，不泄漏另一侧的存在。
	proOnly := func(h http.HandlerFunc) http.Handler { return requireEdition("pro", http.HandlerFunc(h)) }
	liteOnly := func(h http.HandlerFunc) http.Handler { return requireEdition("lite", http.HandlerFunc(h)) }
```

把 `/api/v1/projects` 及其全部子路由的 `protected(...)` 改为 `proOnly(...)`；把 Task 4 的四条 `/readings` 改为 `liteOnly(...)`；`mountReadingRoom` 里的 `a.withReadingContainer(rt.h)` 外面再包一层 `requireEdition("lite", ...)`。

> `/auth/*`、`/courses/*`、`/cards/*`、`/users/me/*`、`/oss/*` 等**不加闸** —— 两个站都要用。

- [ ] **Step 5: 跑测试，确认通过**

```bash
cd apps/api && go test ./internal/api/ -run TestEditionGate -timeout 1800s
```

预期：PASS。

- [ ] **Step 6: 跑全量**

```bash
cd apps/api && go test ./... -timeout 1800s
```

预期：全部 PASS。**大量既有 pro 测试会打 `/projects/*`** —— 种子账号默认 `edition='pro'`，所以它们应当原样通过。若有失败，是某个测试用了非种子账号，给那个账号显式设 `edition='pro'`。

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/api/authz.go apps/api/internal/api/api.go \
        apps/api/internal/api/auth.go apps/api/internal/api/edition_test.go \
        apps/api/internal/api/readings_test.go apps/api/internal/api/atom_container_test.go
git commit -m "feat(lite): gate the pro and lite surfaces on users.edition"
```

---

### Task 7: `apps/lite-web` 脚手架

**Files:**
- Modify: `apps/web/package.json`（取包名 + 暴露源码）
- Create: `apps/lite-web/package.json`
- Create: `apps/lite-web/vite.config.ts`
- Create: `apps/lite-web/tailwind.config.ts`
- Create: `apps/lite-web/postcss.config.js`
- Create: `apps/lite-web/tsconfig.json`
- Create: `apps/lite-web/index.html`
- Create: `apps/lite-web/src/main.tsx`
- Create: `apps/lite-web/src/index.css`
- Create: `apps/lite-web/src/LiteApp.tsx`（本任务只放一个占位骨架，Task 9 填内容）

**Interfaces:**
- Consumes: 无
- Produces: 一个能 `pnpm --filter lite-web build` 成功的应用；`@mind-imprint/web` 这个包名可被跨包引入；`@/…` 别名在 lite 侧解析到 `apps/web/src`

- [ ] **Step 1: 给 `apps/web` 取包名并暴露源码**

编辑 `apps/web/package.json`，把 `"name": "web"` 改为：

```json
  "name": "@mind-imprint/web",
  "exports": {
    "./src/*": "./src/*"
  },
```

> 改名会影响既有的 filter 命令：原先的 `pnpm --filter web …` 要改成 `pnpm --filter @mind-imprint/web …`。改完立刻全仓 grep 一遍并同步：
> ```bash
> grep -rn -- "--filter web" --include=*.json --include=*.sh --include=*.yml --include=Dockerfile . | grep -v node_modules
> ```
> 逐条改掉（至少检查 `apps/web/Dockerfile`、`.deploy-local/` 下的部署脚本、根 `package.json`）。

- [ ] **Step 2: 建 lite 应用骨架**

`apps/lite-web/package.json`：

```json
{
  "name": "@mind-imprint/lite-web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run --passWithNoTests",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@mind-imprint/contracts": "workspace:*",
    "@mind-imprint/web": "workspace:*",
    "lucide-react": "^1.28.0",
    "marked": "^18.0.7",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "react-markdown": "^10.1.0",
    "remark-cjk-friendly": "2.3.1",
    "remark-gfm": "^4.0.1",
    "zod": "^3.23.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.0",
    "jsdom": "^24.1.0",
    "postcss": "^8.4.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.4.0",
    "vite": "^5.3.0",
    "vitest": "^1.6.0"
  }
}
```

`apps/lite-web/vite.config.ts` —— **`@/` 必须指向 `apps/web/src`**，因为被引入的房间组件内部就是这么写的：

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
      // 房间组件是从 apps/web 的源码里直接引入的，它们内部用 "@/…" 自引用 —— 这个
      // 别名必须解析到 web 的 src，否则一进阅读室就是一片解析失败。
      { find: /^@\//, replacement: webSrc + "/" },
      { find: "@lite", replacement: path.resolve(__dirname, "src") },
    ],
  },
  // web 是 workspace 链接的源码包，不能被预打包成 CJS。
  optimizeDeps: { exclude: ["@mind-imprint/web", "@mind-imprint/contracts"] },
  server: { proxy: { "/api": { target: "http://localhost:8080", changeOrigin: true } } },
});
```

`apps/lite-web/tailwind.config.ts` —— **content glob 必须覆盖 web 的源码**，否则房间用到的 class 会被 purge 掉：

```ts
import type { Config } from "tailwindcss";
import base from "../web/tailwind.config";

export default {
  ...base,
  // 房间组件的 class 写在 apps/web 里。漏掉这一条，阅读室会渲染成没有样式的裸 DOM。
  content: [
    "./index.html",
    "./src/**/*.{ts,tsx}",
    "../web/src/**/*.{ts,tsx}",
  ],
} satisfies Config;
```

`apps/lite-web/postcss.config.js`：照抄 `apps/web/postcss.config.js`。

`apps/lite-web/tsconfig.json`：

```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "jsx": "react-jsx",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "baseUrl": ".",
    "paths": {
      "@/*": ["../web/src/*"],
      "@lite/*": ["./src/*"]
    },
    "types": ["vite/client"]
  },
  "include": ["src", "vite.config.ts", "tailwind.config.ts"]
}
```

`apps/lite-web/index.html`：照抄 `apps/web/index.html`，把 `<title>` 改为 `思维印记 · 轻量版`，脚本入口指向 `/src/main.tsx`。

`apps/lite-web/src/index.css`：第一行引入 web 的全局样式（token 变量都在里面），再放 lite 自己的：

```css
@import "@/index.css";
```

`apps/lite-web/src/main.tsx`：

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { LiteApp } from "./LiteApp";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <LiteApp />
  </StrictMode>,
);
```

`apps/lite-web/src/LiteApp.tsx`（占位，Task 9 填内容）：

```tsx
export function LiteApp() {
  return <div className="p-8 text-mk-ink">轻量版</div>;
}
```

- [ ] **Step 3: 装依赖并构建**

```bash
pnpm install
pnpm --filter @mind-imprint/lite-web build
```

预期：构建成功，产出 `apps/lite-web/dist`。

- [ ] **Step 4: 验证跨包别名真的通了**

临时在 `LiteApp.tsx` 里引一个 web 的叶子组件，证明别名与 Tailwind 都成立：

```tsx
import { Segmented } from "@/ui";

export function LiteApp() {
  return (
    <div className="p-8 text-mk-ink bg-mk-paper min-h-screen">
      <Segmented options={[{ value: "a", label: "阅读" }, { value: "b", label: "写作" }]} value="a" onChange={() => {}} />
    </div>
  );
}
```

> `@/ui` 的真实导出名以 `apps/web/src/ui/index.ts` 为准；`Segmented` 在 `WritingBlock.tsx` 里被引用过，签名以那里为准。

```bash
pnpm --filter @mind-imprint/lite-web build
```

预期：构建成功。**然后在真实浏览器里看一眼**（`pnpm --filter @mind-imprint/lite-web dev`），确认它有样式而不是裸 DOM —— 这是 Tailwind content glob 是否生效的唯一可靠验证。确认后把这段临时代码还原成占位骨架。

- [ ] **Step 5: 确认没打坏 pro 的构建**

```bash
pnpm --filter @mind-imprint/web build && pnpm --filter @mind-imprint/web typecheck
```

预期：成功。改包名后 Dockerfile / 部署脚本里的 filter 若漏改，这一步不会报错——所以 Step 1 的 grep 必须做完。

- [ ] **Step 6: 提交**

```bash
git add apps/web/package.json apps/lite-web pnpm-lock.yaml
git commit -m "feat(lite): scaffold apps/lite-web importing the rooms from apps/web"
```

---

### Task 8: `RoomCapabilities`

**Files:**
- Create: `apps/web/src/rooms/capabilities.ts`
- Modify: `apps/web/src/studio/reading/ReadingRoom.tsx`
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx`（传入 pro 能力集）
- Test: `apps/web/test/roomCapabilities.test.tsx`

**Interfaces:**
- Consumes: 无
- Produces:
  ```ts
  export type RoomCapabilities = {
    mode: "pro" | "lite" | "demo";
    plan: boolean; evidenceMap: boolean; explorationLeads: boolean;
    proposalImpact: boolean; essayTrack: boolean;
    comprehensionCheck: boolean; exemplars: boolean;
  };
  export const PRO_CAPABILITIES: RoomCapabilities;
  export const DEMO_CAPABILITIES: RoomCapabilities;
  export const LITE_READING_CAPABILITIES: RoomCapabilities;
  ```
  `ReadingRoomProps` 新增可选字段 `capabilities?: RoomCapabilities`（缺省即 `PRO_CAPABILITIES`，保证既有调用点零改动仍是今天的行为）。

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/web/test/roomCapabilities.test.tsx`：

```tsx
import { describe, expect, it } from "vitest";
import {
  PRO_CAPABILITIES,
  DEMO_CAPABILITIES,
  LITE_READING_CAPABILITIES,
} from "@/rooms/capabilities";

describe("RoomCapabilities", () => {
  it("pro keeps the whole project lifecycle", () => {
    expect(PRO_CAPABILITIES.mode).toBe("pro");
    expect(PRO_CAPABILITIES.evidenceMap).toBe(true);
    expect(PRO_CAPABILITIES.proposalImpact).toBe(true);
  });

  it("lite reading drops every project-lifecycle surface", () => {
    expect(LITE_READING_CAPABILITIES.mode).toBe("lite");
    expect(LITE_READING_CAPABILITIES.plan).toBe(false);
    expect(LITE_READING_CAPABILITIES.evidenceMap).toBe(false);
    expect(LITE_READING_CAPABILITIES.explorationLeads).toBe(false);
    expect(LITE_READING_CAPABILITIES.proposalImpact).toBe(false);
    expect(LITE_READING_CAPABILITIES.essayTrack).toBe(false);
  });

  it("demo is read-only pro, not a third lifecycle", () => {
    expect(DEMO_CAPABILITIES.mode).toBe("demo");
    expect(DEMO_CAPABILITIES.evidenceMap).toBe(PRO_CAPABILITIES.evidenceMap);
  });
});
```

- [ ] **Step 2: 跑测试，确认它失败**

```bash
pnpm --filter @mind-imprint/web test -- roomCapabilities
```

预期：FAIL —— 模块不存在。

- [ ] **Step 3: 写实现**

新建 `apps/web/src/rooms/capabilities.ts`：

```ts
// capabilities.ts — what a room is allowed to show.
//
// The rooms used to reach for project-lifecycle facts directly (and carried a
// separate `demoMode` boolean). The lite edition runs the SAME rooms without a
// project around them, so the rooms now read one object instead of asking what
// stage the project is in. Adding an edition means adding a preset here, not
// threading another boolean through the tree.
export type RoomCapabilities = {
  mode: "pro" | "lite" | "demo";
  /** 计划 room + plan items. */
  plan: boolean;
  /** 证据图 / evidence map sidebar and its review actions. */
  evidenceMap: boolean;
  /** 探索 leads, dig, adopt, edges. */
  explorationLeads: boolean;
  /** The finalize step's 「对立题的影响」 field — meaningless without a 立题. */
  proposalImpact: boolean;
  /** essay-track stages: statement guide, submission guide, stage advance. */
  essayTrack: boolean;
  /** Post-reading comprehension check (lite only; lands in P2). */
  comprehensionCheck: boolean;
  /** English-writing 示范 paragraphs (lite only; lands in P3). */
  exemplars: boolean;
};

export const PRO_CAPABILITIES: RoomCapabilities = {
  mode: "pro",
  plan: true,
  evidenceMap: true,
  explorationLeads: true,
  proposalImpact: true,
  essayTrack: true,
  comprehensionCheck: false,
  exemplars: false,
};

// Demo is read-only pro, not a third lifecycle — it shows the same surfaces.
export const DEMO_CAPABILITIES: RoomCapabilities = { ...PRO_CAPABILITIES, mode: "demo" };

export const LITE_READING_CAPABILITIES: RoomCapabilities = {
  mode: "lite",
  plan: false,
  evidenceMap: false,
  explorationLeads: false,
  proposalImpact: false,
  essayTrack: false,
  comprehensionCheck: true,
  exemplars: false,
};
```

- [ ] **Step 4: 跑测试，确认通过**

```bash
pnpm --filter @mind-imprint/web test -- roomCapabilities
```

预期：PASS。

- [ ] **Step 5: 把能力对象接进 `ReadingRoom`**

在 `apps/web/src/studio/reading/ReadingRoom.tsx` 的 `ReadingRoomProps` 里加：

```ts
  /**
   * What this room may show. Defaults to PRO_CAPABILITIES so every existing
   * call site keeps today's behaviour untouched; the lite host passes
   * LITE_READING_CAPABILITIES.
   */
  capabilities?: RoomCapabilities;
```

在组件体的开头解构默认值：

```ts
  const caps = props.capabilities ?? PRO_CAPABILITIES;
```

然后把**证据笔记 / 线索 / 追踪来源**三处渲染分别包上条件。用 `grep -n "EvidenceNote\|TraceSourcePanel\|onTraceCitation\|onTraceSearch\|onAdoptSource" apps/web/src/studio/reading/ReadingRoom.tsx` 找到它们的 JSX 位置，把每一处包成：

```tsx
{caps.evidenceMap && ( /* …既有 JSX 原样… */ )}
```
```tsx
{caps.explorationLeads && ( /* …既有 TraceSourcePanel JSX 原样… */ )}
```

`demoMode` 这一轮**保持不动**（它已在多处使用，收敛进 `caps.mode === "demo"` 是独立的清理，不在 P1 的关键路径上）。

- [ ] **Step 6: 跑 web 全量测试与类型检查**

```bash
pnpm --filter @mind-imprint/web test && pnpm --filter @mind-imprint/web typecheck
```

预期：全部 PASS —— 因为默认值是 `PRO_CAPABILITIES`，pro 的行为一字未变。

- [ ] **Step 7: 提交**

```bash
git add apps/web/src/rooms/capabilities.ts apps/web/src/studio/reading/ReadingRoom.tsx \
        apps/web/test/roomCapabilities.test.tsx
git commit -m "feat(lite): give the rooms a RoomCapabilities object, defaulting to pro"
```

---

### Task 9: 轻量版 shell 与阅读落地页

**Files:**
- Create: `apps/lite-web/src/api/client.ts`
- Create: `apps/lite-web/src/api/readings.ts`
- Create: `apps/lite-web/src/routing.ts`
- Modify: `apps/lite-web/src/LiteApp.tsx`
- Create: `apps/lite-web/src/readings/ReadingsLanding.tsx`
- Test: `apps/lite-web/test/routing.test.ts`

**Interfaces:**
- Consumes: Task 4 的 `/api/v1/readings` 端点；Task 7 的脚手架
- Produces: `parseLiteRoute(pathname): LiteRoute`，其中 `type LiteRoute = {tab:"readings"|"writings"} | {tab:"readings", readingId:string}`；`listReadings()`、`createReading(input)`、`getReading(id)` 三个客户端函数

- [ ] **Step 1: 写下会失败的测试**

新建 `apps/lite-web/test/routing.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import { parseLiteRoute, readingPath } from "@lite/routing";

describe("parseLiteRoute", () => {
  it("defaults to the readings tab", () => {
    expect(parseLiteRoute("/")).toEqual({ tab: "readings" });
  });

  it("reads a reading id out of the path", () => {
    expect(parseLiteRoute("/readings/abc-123")).toEqual({ tab: "readings", readingId: "abc-123" });
  });

  it("knows the writings tab", () => {
    expect(parseLiteRoute("/writings")).toEqual({ tab: "writings" });
  });

  it("round-trips a reading path", () => {
    expect(parseLiteRoute(readingPath("xyz"))).toEqual({ tab: "readings", readingId: "xyz" });
  });
});
```

新建 `apps/lite-web/vitest.config.ts`，照抄 `apps/web/vitest.config.ts` 并把别名改成 lite 的（`@` → `../web/src`，`@lite` → `./src`）。

- [ ] **Step 2: 跑测试，确认它失败**

```bash
pnpm --filter @mind-imprint/lite-web test
```

预期：FAIL —— 模块不存在。

- [ ] **Step 3: 写路由**

新建 `apps/lite-web/src/routing.ts`：

```ts
// routing.ts — no router library, same convention as the pro shell: parse the
// pathname, push with history.pushState, listen for popstate.
export type LiteRoute =
  | { tab: "readings"; readingId?: string }
  | { tab: "writings" };

export function parseLiteRoute(pathname: string): LiteRoute {
  const parts = pathname.split("/").filter(Boolean);
  if (parts[0] === "writings") return { tab: "writings" };
  if (parts[0] === "readings" && parts[1]) return { tab: "readings", readingId: parts[1] };
  return { tab: "readings" };
}

export function readingPath(id: string): string {
  return `/readings/${id}`;
}

export function navigate(path: string): void {
  window.history.pushState({}, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}
```

- [ ] **Step 4: 跑测试，确认通过**

```bash
pnpm --filter @mind-imprint/lite-web test
```

预期：PASS。

- [ ] **Step 5: 写 API 客户端**

新建 `apps/lite-web/src/api/client.ts`：

```ts
// client.ts — the lite site's fetch wrapper. Same cookie-session contract as
// the pro client: credentials are sent, errors carry the server's envelope.
export class LiteApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!res.ok) {
    let code = "unknown";
    let message = `请求失败（${res.status}）`;
    try {
      const body = await res.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? message;
    } catch {
      // 非 JSON 错误体（网关层的 502/504）——保留兜底文案
    }
    throw new LiteApiError(res.status, code, message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}
```

> 错误体的真实形状以 `apps/api/internal/httpx/errors.go` 的 `WriteError` 为准 —— 打开它确认 `error.code` / `error.message` 的键名，不一致就照改。

新建 `apps/lite-web/src/api/readings.ts`：

```ts
import { api } from "./client";

export type Reading = {
  id: string;
  title: string;
  lang: "zh" | "en";
  status: "active" | "finished";
  referenceId: string | null;
  createdAt: string;
  updatedAt: string;
  finishedAt: string | null;
};

export function listReadings(): Promise<{ readings: Reading[] }> {
  return api("/readings");
}

export function createReading(input: { title: string; lang: "zh" | "en" }): Promise<{ id: string; referenceId: string }> {
  return api("/readings", { method: "POST", body: JSON.stringify(input) });
}

export function getReading(id: string): Promise<Reading> {
  return api(`/readings/${id}`);
}

/** Paste the article body into the atom's single reference, through the room passthrough. */
export function pasteReadingText(readingId: string, referenceId: string, text: string): Promise<unknown> {
  return api(`/readings/${readingId}/room/references/${referenceId}/paste-content`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
}
```

> `paste-content` 的真实请求体键名以 `apps/api/internal/api/workspace_library.go:987` 的 `pasteContent` 为准 —— 打开确认是 `text` 还是别的键。

- [ ] **Step 6: 写 shell 与阅读落地页**

新建 `apps/lite-web/src/readings/ReadingsLanding.tsx`：

```tsx
import { useEffect, useState } from "react";
import { createReading, listReadings, pasteReadingText, type Reading } from "@lite/api/readings";
import { navigate, readingPath } from "@lite/routing";

// ReadingsLanding — 阅读 tab 的落地面：贴一篇文章开始，下面是过往的阅读。
export function ReadingsLanding() {
  const [history, setHistory] = useState<Reading[]>([]);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listReadings()
      .then((r) => setHistory(r.readings))
      .catch((e) => setError(e.message));
  }, []);

  async function start() {
    const body = text.trim();
    if (!body) {
      setError("先把文章正文贴进来。");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const created = await createReading({ title: title.trim() || "未命名阅读", lang: "zh" });
      await pasteReadingText(created.id, created.referenceId, body);
      navigate(readingPath(created.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "出了点问题，再试一次。");
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-3xl px-6 py-10">
      <h1 className="text-2xl text-mk-ink">阅读</h1>
      <p className="mt-1 text-sm text-mk-secondary">把一篇文章贴进来，我们一起读。</p>

      <input
        className="mt-6 w-full rounded-lg border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-ink"
        placeholder="给这次阅读起个名字（可留空）"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <textarea
        className="mt-3 h-64 w-full resize-y rounded-lg border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-ink"
        placeholder="把文章正文粘贴到这里…"
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      {error && <p className="mt-2 text-sm text-mk-danger">{error}</p>}
      <button
        className="mt-3 rounded-lg bg-mk-accent px-4 py-2 text-white disabled:opacity-50"
        disabled={busy}
        onClick={start}
      >
        {busy ? "正在准备…" : "开始阅读"}
      </button>

      {history.length > 0 && (
        <div className="mt-12">
          <h2 className="text-sm text-mk-secondary">过往的阅读</h2>
          <ul className="mt-3 space-y-2">
            {history.map((r) => (
              <li key={r.id}>
                <button
                  className="w-full rounded-lg border border-mk-border bg-mk-surface px-4 py-3 text-left text-mk-ink"
                  onClick={() => navigate(readingPath(r.id))}
                >
                  <span>{r.title}</span>
                  <span className="ml-2 text-xs text-mk-muted">
                    {r.status === "finished" ? "已完成" : "进行中"}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
```

替换 `apps/lite-web/src/LiteApp.tsx`：

```tsx
import { useEffect, useState } from "react";
import { parseLiteRoute, navigate, type LiteRoute } from "@lite/routing";
import { ReadingsLanding } from "@lite/readings/ReadingsLanding";
import { ReadingRoomHost } from "@lite/readings/ReadingRoomHost";

// LiteApp — 轻量版的 shell。左栏只有两项：写作、阅读。没有图鉴，没有课程，
// 没有项目。
export function LiteApp() {
  const [route, setRoute] = useState<LiteRoute>(() => parseLiteRoute(window.location.pathname));

  useEffect(() => {
    const onPop = () => setRoute(parseLiteRoute(window.location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  return (
    <div className="flex min-h-screen bg-mk-paper">
      <nav className="w-40 shrink-0 border-r border-mk-border px-3 py-6">
        <NavItem label="写作" active={route.tab === "writings"} onClick={() => navigate("/writings")} />
        <NavItem label="阅读" active={route.tab === "readings"} onClick={() => navigate("/readings")} />
      </nav>
      <main className="flex-1">
        {route.tab === "writings" && (
          <div className="mx-auto w-full max-w-3xl px-6 py-10 text-mk-secondary">写作即将上线。</div>
        )}
        {route.tab === "readings" && !route.readingId && <ReadingsLanding />}
        {route.tab === "readings" && route.readingId && <ReadingRoomHost readingId={route.readingId} />}
      </main>
    </div>
  );
}

function NavItem({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      className={`mb-1 w-full rounded-lg px-3 py-2 text-left ${active ? "bg-mk-surface text-mk-ink" : "text-mk-secondary"}`}
      onClick={onClick}
    >
      {label}
    </button>
  );
}
```

> `ReadingRoomHost` 在 Task 10 建。本任务先建一个只渲染 `<div>加载中…</div>` 的临时版本，好让构建通过。

- [ ] **Step 7: 构建 + 类型检查**

```bash
pnpm --filter @mind-imprint/lite-web build && pnpm --filter @mind-imprint/lite-web typecheck
```

预期：成功。

- [ ] **Step 8: 提交**

```bash
git add apps/lite-web/src apps/lite-web/test apps/lite-web/vitest.config.ts
git commit -m "feat(lite): lite shell with the readings landing and history"
```

---

### Task 10: 把阅读室接进轻量站

**Files:**
- Create: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Create: `apps/lite-web/src/api/readingRoom.ts`
- Test: `apps/lite-web/test/readingRoomHost.test.tsx`

**Interfaces:**
- Consumes: Task 5 的 `/readings/{id}/room/*`；Task 8 的 `LITE_READING_CAPABILITIES`；Task 9 的路由与客户端
- Produces: `ReadingRoomHost({ readingId }: { readingId: string })`

- [ ] **Step 1: 照抄 pro 的调用点，摸清 `ReadingRoom` 到底要什么**

**不要凭 props 类型猜。** 打开真实调用点，把它需要的每一个 prop 与 `api` 对象的每一个方法列出来：

```bash
sed -n '1540,1610p' apps/web/src/workspace/WorkspaceContainer.tsx
sed -n '52,140p' apps/web/src/studio/reading/ReadingRoom.tsx
grep -n "export type ReadingLoopApi" -A 40 apps/web/src/studio/reading/readingLoop.ts
```

`ReadingRoomApi` = `ReadingLoopApi` + 三个 brief/takeaway 方法。lite 的 `api` 对象要实现同一组方法，只是每个 URL 都带 `/readings/{id}/room` 前缀。

- [ ] **Step 2: 写下会失败的测试**

新建 `apps/lite-web/test/readingRoomHost.test.tsx`：

```tsx
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ReadingRoomHost } from "@lite/readings/ReadingRoomHost";

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("ReadingRoomHost", () => {
  it("shows a plain error when the reading cannot be loaded", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "not_found", message: "资源不存在" } }), { status: 404 }),
    ));
    render(<ReadingRoomHost readingId="missing" />);
    await waitFor(() => expect(screen.getByText(/资源不存在/)).toBeInTheDocument());
  });

  it("waits for the reading before mounting the room", async () => {
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => new Promise(() => {})));
    render(<ReadingRoomHost readingId="pending" />);
    expect(screen.getByText(/加载中/)).toBeInTheDocument();
  });
});
```

需要 `@testing-library/react`、`@testing-library/jest-dom`、`jsdom` —— 加进 `apps/lite-web/package.json` 的 devDependencies（版本对齐 `apps/web`），并照抄 `apps/web` 的 vitest setup 文件配置。

- [ ] **Step 3: 跑测试，确认它失败**

```bash
pnpm --filter @mind-imprint/lite-web test -- readingRoomHost
```

预期：FAIL —— 模块不存在。

- [ ] **Step 4: 写 room API 客户端**

新建 `apps/lite-web/src/api/readingRoom.ts`。为每一个 `ReadingLoopApi` 方法实现一个带前缀的版本：

```ts
import { api } from "./client";

// readingRoom.ts — the reading room's own API surface, addressed through the
// atom passthrough. Every path is the pro path with /readings/{id}/room in
// front of it; the server resolves the atom to its container project, so the
// request shapes and responses are byte-identical to the pro side.
export function readingRoomApi(readingId: string) {
  const base = `/readings/${readingId}/room`;
  return {
    enterReading: (_projectId: string, rid: string) =>
      api(`${base}/references/${rid}/enter-reading`, { method: "POST", body: "{}" }),
    getMaterialSource: (_projectId: string, mid: string) =>
      api(`${base}/materials/${mid}/source`),
    readTurn: (_projectId: string, mid: string, body: unknown) =>
      api(`${base}/materials/${mid}/read-turn`, { method: "POST", body: JSON.stringify(body) }),
    summonCard: (_projectId: string, mid: string, body: unknown) =>
      api(`${base}/materials/${mid}/summon-card`, { method: "POST", body: JSON.stringify(body) }),
    openCard: (_projectId: string, mid: string) =>
      api(`${base}/materials/${mid}/open-card`),
    activateCard: (_projectId: string, cid: string) =>
      api(`${base}/cards/${cid}/activate`, { method: "POST", body: "{}" }),
    skipCard: (_projectId: string, cid: string) =>
      api(`${base}/cards/${cid}/skip`, { method: "POST", body: "{}" }),
    submitCard: (_projectId: string, cid: string, body: unknown) =>
      api(`${base}/cards/${cid}/submit`, { method: "POST", body: JSON.stringify(body) }),
    putReadingBrief: (_projectId: string, rid: string, brief: unknown) =>
      api(`${base}/references/${rid}/reading-brief`, { method: "PUT", body: JSON.stringify(brief) }),
    getTakeawayDraft: (_projectId: string, rid: string) =>
      api(`${base}/references/${rid}/takeaway-draft`),
    postFinalizeReading: (_projectId: string, rid: string, body: unknown) =>
      api(`${base}/references/${rid}/finalize-reading`, { method: "POST", body: JSON.stringify(body) }),
  };
}
```

> 方法名与签名**必须**与 Step 1 里读到的 `ReadingLoopApi` / `ReadingRoomApi` 完全一致 —— 少一个方法，阅读室会在运行时炸在一个 `undefined is not a function` 上。`_projectId` 参数保留是为了签名对齐（lite 不需要它，容器由服务端解析）。

- [ ] **Step 5: 写 host**

新建 `apps/lite-web/src/readings/ReadingRoomHost.tsx`：

```tsx
import { useEffect, useState } from "react";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";
import { LITE_READING_CAPABILITIES } from "@/rooms/capabilities";
import { getReading, type Reading } from "@lite/api/readings";
import { readingRoomApi } from "@lite/api/readingRoom";
import { navigate } from "@lite/routing";

// ReadingRoomHost — mounts the SAME ReadingRoom the pro product uses. The only
// differences: the api object routes through the atom passthrough, and the
// capabilities object hides the surfaces that need a project around them.
export function ReadingRoomHost({ readingId }: { readingId: string }) {
  const [reading, setReading] = useState<Reading | null>(null);
  const [source, setSource] = useState<unknown | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const rd = await getReading(readingId);
        if (cancelled) return;
        setReading(rd);
        if (!rd.referenceId) throw new Error("这次阅读还没有文章。");
        const api = readingRoomApi(readingId);
        const entered = await api.enterReading("", rd.referenceId);
        if (!cancelled) setSource(entered);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "打不开这次阅读。");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [readingId]);

  if (error) return <div className="px-6 py-10 text-mk-danger">{error}</div>;
  if (!reading || !source) return <div className="px-6 py-10 text-mk-secondary">加载中…</div>;

  return (
    <ReadingRoom
      projectId=""
      referenceId={reading.referenceId!}
      source={extractSource(source)}
      capabilities={LITE_READING_CAPABILITIES}
      api={readingRoomApi(readingId)}
      onBack={() => navigate("/readings")}
    />
  );
}
```

`extractSource` 与其余必填 prop，照 Step 1 读到的 pro 调用点补齐 —— **以 `WorkspaceContainer.tsx` 的那一段为准**，逐个 prop 对照，缺一个就补一个。`projectId` 传空串：lite 不需要它，容器由服务端从 atom 解析；若 `ReadingRoom` 内部真的用它拼了 URL，改成让那处走 `api` 对象而不是自己拼 —— 那本来就是一个应该收进 api 层的漏洞。

- [ ] **Step 6: 跑测试，确认通过**

```bash
pnpm --filter @mind-imprint/lite-web test && pnpm --filter @mind-imprint/lite-web typecheck
```

预期：PASS。

- [ ] **Step 7: 在真实浏览器里走一遍**

```bash
# 终端 1
cd apps/api && make run
# 终端 2
pnpm --filter @mind-imprint/lite-web dev
```

把种子账号设为 lite（`UPDATE users SET edition='lite' WHERE id = …`），登录后：贴一篇文章 → 开始阅读 → 确认**透镜库、段落上的悬挂工具卡、批注、AI 引导都在**，且证据笔记 / 追踪来源**不在**。jsdom 测不出这些，必须真看。

- [ ] **Step 8: 提交**

```bash
git add apps/lite-web/src/readings apps/lite-web/src/api/readingRoom.ts apps/lite-web/test
git commit -m "feat(lite): mount the real reading room on the lite site"
```

---

### Task 11: 部署

**Files:**
- Create: `apps/lite-web/Dockerfile`
- Create: `apps/lite-web/nginx.conf`
- Create: `.deploy-local/deploy-lite.sh`

**Interfaces:**
- Consumes: Task 7 的构建产物
- Produces: 一个可部署到独立域名的静态站

- [ ] **Step 1: 照抄 pro 的部署形状**

```bash
cat apps/web/Dockerfile
cat apps/web/nginx.conf
ls .deploy-local/
```

- [ ] **Step 2: 写 Dockerfile 与 nginx.conf**

`apps/lite-web/Dockerfile` 完全照 `apps/web/Dockerfile` 改写，只换两处：`--filter` 的包名改为 `@mind-imprint/lite-web`，产物目录改为 `apps/lite-web/dist`。

> ⚠️ 构建上下文必须是**仓库根目录**（lite 依赖 `apps/web` 与 `packages/contracts` 的源码）。
> ⚠️ 既有教训：根 `.dockerignore` 里的 `*.png` 会把图片排除掉 —— 若 lite 用到图片，确认这一条。

`apps/lite-web/nginx.conf` 照 `apps/web/nginx.conf`，保留 SPA 的 `try_files $uri /index.html;`（`/readings/:id` 深链需要它）。

- [ ] **Step 3: 写部署脚本**

`.deploy-local/deploy-lite.sh` 照 `.deploy-local/deploy-site.sh` 改写：新的容器名、新的端口（取一个未被占用的，例如 `8093`）、新的域名。域名与证书按既有 certbot 流程办。

- [ ] **Step 4: 本地验证镜像能构建并跑起来**

```bash
docker build -f apps/lite-web/Dockerfile -t mind-lite-web:dev .
docker run --rm -p 8093:80 mind-lite-web:dev
```

打开 `http://localhost:8093`，确认页面有样式、`/readings/xxx` 深链不 404。

> 🚨 **绝不在 ECS 上跑 `docker prune -a`。**

- [ ] **Step 5: 提交**

```bash
git add apps/lite-web/Dockerfile apps/lite-web/nginx.conf .deploy-local/deploy-lite.sh
git commit -m "chore(lite): dockerfile, nginx and deploy script for the lite site"
```

---

### Task 12: 端到端走查

**Files:**
- Create: `apps/lite-web/e2e/reading-walk.spec.ts`
- Create: `apps/lite-web/e2e/playwright.config.ts`

**Interfaces:**
- Consumes: 前面所有任务
- Produces: 一条覆盖「新建 → 贴文 → 精读 → 我的收获」的 Playwright 走查

- [ ] **Step 1: 照抄 pro 的 e2e 配置**

```bash
ls apps/web/e2e/
cat apps/web/e2e/playwright.config.ts
```

`apps/lite-web/e2e/playwright.config.ts` 照抄，`baseURL` 指向 lite 的 dev server，`webServer` 命令改为 `pnpm --filter @mind-imprint/lite-web dev`。

- [ ] **Step 2: 写走查**

新建 `apps/lite-web/e2e/reading-walk.spec.ts`：

```ts
import { expect, test } from "@playwright/test";

// A lite student's whole P1 journey: paste an article, read it with the real
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

  // 阅读室真的起来了：文章正文在页面上。
  await expect(page.getByText("全球太阳能装机容量")).toBeVisible({ timeout: 30_000 });

  // 项目专属的面板不在轻量版里。
  await expect(page.getByText("对立题的影响")).toHaveCount(0);
});
```

> 断言用的可见文案必须与真实 DOM 对齐 —— 先手动跑一遍 dev server，用真实文案改这些选择器，不要留猜的。

- [ ] **Step 3: 跑走查**

```bash
pnpm --filter @mind-imprint/lite-web exec playwright install --with-deps chromium
pnpm --filter @mind-imprint/lite-web exec playwright test -c e2e/playwright.config.ts
```

预期：PASS。测试账号需要 `edition='lite'`，在 config 的 `globalSetup` 里种，或复用 pro e2e 的登录方式。

- [ ] **Step 4: 跑全仓验证**

```bash
cd apps/api && go test ./... -timeout 1800s
cd ../.. && pnpm -r typecheck && pnpm -r test
```

预期：全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add apps/lite-web/e2e
git commit -m "test(lite): end-to-end walk of the lite reading journey"
```

---

## 自检（写完计划后对照 spec）

**Spec 覆盖：**

| Spec 条目 | 落在哪个任务 |
|---|---|
| §3.2 `project.kind` + 容器泄漏封堵 | Task 1 |
| §3.5 `users.edition` | Task 2 |
| §3.1 `reading` 表 | Task 3 |
| §4.1 阅读原子端点 | Task 4 |
| §4.2 房间透传接缝 | Task 5 |
| §4.3 鉴权（归属 404、容器不可直达、edition 闸） | Task 4 Step 1 / Task 5 Step 5 / Task 6 |
| §6.1 `apps/lite-web` 与两处构建陷阱 | Task 7 |
| §6.3 `RoomCapabilities` | Task 8 |
| §6.2 shell 与阅读落地页 | Task 9 |
| §5.1 阅读主动线（至「我的收获」） | Task 10 + Task 12 |
| §8.4 测试 | 各任务内 + Task 12 |
| §3.1 `writing` 表 | **本期不做** —— 见 Global Constraints，随 P3 的 handler 落地 |
| §3.3 `reading_check` / §3.4 `atom_report` / §7 简版报告 | **本期不做** —— P2/P3 |
| §4.1 `POST /readings/{id}/text` | **本期不做** —— 走 `/room/…/paste-content` 透传，见 Global Constraints |

**类型一致性：** `readingDTO` 的字段（`referenceId` / `createdAt` / `finishedAt`）在 Task 4 定义、Task 9 的 `Reading` 类型消费，键名一致；`readingRoomApi` 的方法名在 Task 10 Step 1 被要求对照 `ReadingLoopApi` 逐一核实；`withContainerID` / `containerIDFrom` 在 Task 5 一处定义、一处消费。

**已知需要在实现时亲手核实的外部名字**（计划里已各自标出核实步骤，不是占位符）：`sqlc.CreateReferenceParams` 的字段（Task 4 Step 3）、`mountReadingRoom` 表里的 handler 方法名（Task 5 Step 4）、`ReadingRoomProps` 的完整 prop 集与 `ReadingLoopApi` 的方法集（Task 10 Step 1）、`httpx.WriteError` 的错误体键名（Task 9 Step 5）、`pasteContent` 的请求体键名（Task 9 Step 5）。
