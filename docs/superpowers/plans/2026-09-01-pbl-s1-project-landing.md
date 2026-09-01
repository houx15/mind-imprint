# PBL S1 — project landing, type detection, kanban Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A lite student writes a free-text idea into a big input box on a new 项目 tab, 印记 classifies it into a project type, she names it and picks a cover, and it appears on a kanban board columned by status — with a dark theme available to both editions.

**Architecture:** A PBL project is an **atom** (`atom.kind = 'project'`), the same way `reading` and `writing` are, so it inherits lite's existing ownership check, activity tracking, heartbeat, message thread and LLM metering rather than growing a parallel substrate beside them. `pbl_project` is the detail table keyed by `atom_id`. Type detection is one fast model call behind the existing `gateway.Collect` seam, with the parse as a pure function so it can be tested without a provider.

**Tech Stack:** Go 1.26 (`net/http`, `pgx/v5`, `sqlc`, `goose`), PostgreSQL, React + Vite + TypeScript + Tailwind.

**Spec:** `docs/superpowers/specs/2026-09-01-pbl-project-room-design.md`

## Global Constraints

- **The client never calls a model.** All LLM calls are server-side; keys live only in `apps/api`.
- **Every model call records an `llm_call` row** with tier, tokens and cost. Lite's path is `a.recordLiteLLMCall(ctx, userID, atomID, purpose, resolved, usage)` (`api/reading_turn.go:539`), which writes `RecordAtomLLMCall`. Do **not** use `agent.RecordLLMCall` — it calls `GetProject` and requires a pro `project` row.
- **Lite must never break pro.** `atom` and everything keyed off it are lite-only (migration 0092). Do not touch `apps/web` behaviour. Adding CSS tokens under a new `[data-theme="dark"]` selector is additive and inert until the attribute is set.
- **Only logic tests.** Pure functions, validation, reducers, handlers, permission checks. No assertion-per-rendered-element tests. Verify UI by taking a Playwright screenshot and looking at it.
- **`mk-*` tokens are bare CSS variables.** Every Tailwind alpha modifier on them (`bg-mk-accent/40`) emits **no CSS at all**. Use `color-mix()` or `linear-gradient`.
- **No 不是…而是 antithesis** in any Chinese or English UI copy. Positive declaratives only.
- **AI failures surface.** Never return a plausible canned sentence when a model call or parse fails.
- Go tests: `CGO_ENABLED=0 go test ./... -timeout 1800s` (testcontainers are slow).
- sqlc is pinned: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`.
- Project types: `website | research | design | making | investigation`.
- Project statuses: `talking | running | review | keeping | archived`.

---

### Task 1: Schema — project as an atom

**Files:**
- Create: `apps/api/internal/store/migrations/0108_pbl_project.sql`
- Create: `apps/api/internal/store/queries/pbl_project.sql`
- Modify: `apps/api/internal/store/sqlc/` (regenerated, do not hand-edit)
- Test: `apps/api/internal/api/pbl_store_test.go`

**Interfaces:**
- Consumes: the `atom` table and `CreateAtom` from migration 0092.
- Produces: sqlc methods `CreatePblProject(ctx, CreatePblProjectParams{AtomID, Idea, Kind}) (PblProject, error)`, `GetPblProject(ctx, atomID) (GetPblProjectRow, error)`, `ListPblProjectsByUser(ctx, userID) ([]ListPblProjectsByUserRow, error)`, `UpdatePblProjectMeta(ctx, UpdatePblProjectMetaParams{AtomID, Name, CoverGround, CoverGlyph}) (PblProject, error)`, `SetPblProjectStatus(ctx, SetPblProjectStatusParams{AtomID, Status}) (PblProject, error)`, `CountPblProjectsByUser(ctx, userID) (int64, error)`.

- [ ] **Step 1: Confirm the existing CHECK constraint name**

Run:
```bash
grep -n "kind" apps/api/internal/store/migrations/0092_atom_substrate.sql
```
Expected: `kind text NOT NULL CHECK (kind IN ('reading','writing')),` declared inline on the column, so PostgreSQL names it `atom_kind_check`. If your local database disagrees, get the real name with `\d atom` and use that in Step 2 instead.

- [ ] **Step 2: Write the migration**

Create `apps/api/internal/store/migrations/0108_pbl_project.sql`:

```sql
-- +goose Up
-- PBL 项目接进 atom 底座：一个项目和一篇阅读、一篇写作一样，是一颗原子。
--
-- 为什么不是新起一张根表：atom 已经带着轻量版全部的共享机制——归属校验
-- （loadOwnedAtom）、最近活跃（last_activity_at）、专注时长（active_seconds）
-- 与心跳、对话（atom_message）、报告（atom_report）、以及计费用的
-- llm_call.atom_id。项目自己再长一套，等于把同一件事实现两遍，而且第二遍
-- 没有被测试过。reading / writing 都是 atom_id 主键的细节表，项目照做。
ALTER TABLE atom DROP CONSTRAINT atom_kind_check;
ALTER TABLE atom ADD CONSTRAINT atom_kind_check
  CHECK (kind IN ('reading','writing','project'));

CREATE TABLE pbl_project (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  -- 她原样写进大输入框的那段话。永远保留：项目改过名之后，这仍然是她当初
  -- 自己的说法，也是过程评估唯一能读到的"起点"。
  idea         text NOT NULL,
  -- 印记读完 idea 判的类型。website 由 spec §4 的规则强制，不由分类器决定。
  kind         text NOT NULL
               CHECK (kind IN ('website','research','design','making','investigation')),
  -- 名字和封面在弹窗里由她给。建号那一刻还没有，所以默认空串而不是 NOT NULL
  -- 无默认——项目要先存在，她才能给它起名。
  name         text NOT NULL DEFAULT '',
  cover_ground text NOT NULL DEFAULT '',
  cover_glyph  text NOT NULL DEFAULT '',
  status       text NOT NULL DEFAULT 'talking'
               CHECK (status IN ('talking','running','review','keeping','archived')),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- 看板按状态分列，列内按最近活跃排序；活跃在 atom 上，所以这里只给 status。
CREATE INDEX pbl_project_status_idx ON pbl_project (status);

-- +goose Down
DROP TABLE pbl_project;
ALTER TABLE atom DROP CONSTRAINT atom_kind_check;
ALTER TABLE atom ADD CONSTRAINT atom_kind_check CHECK (kind IN ('reading','writing'));
```

- [ ] **Step 3: Write the queries**

Create `apps/api/internal/store/queries/pbl_project.sql`:

```sql
-- PBL 项目。atom 是身份，这里是细节——和 reading.sql / writing_atom.sql 同构。

-- name: CreatePblProject :one
INSERT INTO pbl_project (atom_id, idea, kind) VALUES ($1, $2, $3) RETURNING *;

-- name: GetPblProject :one
SELECT p.*, a.user_id, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE p.atom_id = $1;

-- name: ListPblProjectsByUser :many
-- 看板一次读全部：一个学生的项目是十几个量级，不分页。按最近活跃降序，
-- 前端再按 status 分列，于是"看板"和"时间线"是同一份数据的两种画法。
SELECT p.*, a.created_at AS atom_created_at, a.last_activity_at
FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project'
ORDER BY a.last_activity_at DESC;

-- name: CountPblProjectsByUser :one
-- spec §4：她还没有项目时，第一个项目必须是个人主页。
SELECT count(*) FROM pbl_project p JOIN atom a ON a.id = p.atom_id
WHERE a.user_id = $1 AND a.kind = 'project';

-- name: UpdatePblProjectMeta :one
UPDATE pbl_project SET name = $2, cover_ground = $3, cover_glyph = $4, updated_at = now()
WHERE atom_id = $1 RETURNING *;

-- name: SetPblProjectStatus :one
UPDATE pbl_project SET status = $2, updated_at = now()
WHERE atom_id = $1 RETURNING *;
```

- [ ] **Step 4: Regenerate sqlc**

Run:
```bash
cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate
```
Expected: new `internal/store/sqlc/pbl_project.sql.go`, no errors. If sqlc reports an unknown table, the migration file name or `-- +goose Up` marker is wrong.

- [ ] **Step 5: Write the failing store test**

Create `apps/api/internal/api/pbl_store_test.go`:

```go
package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// A project is an atom. This test exists to catch the two things that are
// silently wrong if the migration drifts: an atom kind that no longer accepts
// 'project', and a status/kind CHECK that lets a typo through.
func TestPblProjectRoundTrip(t *testing.T) {
	d := newTestDeps(t)
	u := seedTestStudent(t, d)

	at, err := d.Queries.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: u.ID})
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	p, err := d.Queries.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "我想给学校food waste做点什么", Kind: "investigation",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if p.Status != "talking" {
		t.Fatalf("status = %q, want talking (a new project has no plan yet)", p.Status)
	}
	if p.Name != "" {
		t.Fatalf("name = %q, want empty — she names it in the modal, after it exists", p.Name)
	}

	n, err := d.Queries.CountPblProjectsByUser(t.Context(), u.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestPblProjectRejectsUnknownKind(t *testing.T) {
	d := newTestDeps(t)
	u := seedTestStudent(t, d)
	at, err := d.Queries.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: u.ID})
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	if _, err := d.Queries.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "x", Kind: "podcast",
	}); err == nil {
		t.Fatal("expected the CHECK constraint to reject kind=podcast")
	}
}
```

- [ ] **Step 6: Find the real test helper names before running**

Run:
```bash
grep -rn "func newTestDeps\|func seedTestStudent\|func seedStudent" apps/api/internal/api/*_test.go | head
```
Expected: the package's existing helpers for a test database and a seeded student. **Replace `newTestDeps` / `seedTestStudent` in Step 5 with whatever this prints** — do not add new helpers when the package already has them.

- [ ] **Step 7: Run the test to verify it fails**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestPblProject -timeout 1800s -v
```
Expected: FAIL — `CreatePblProject` undefined, until Step 4's generated code is in place; then a migration error if 0108 was not applied.

- [ ] **Step 8: Run the test to verify it passes**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestPblProject -timeout 1800s -v
```
Expected: PASS, both tests.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/store/migrations/0108_pbl_project.sql \
        apps/api/internal/store/queries/pbl_project.sql \
        apps/api/internal/store/sqlc/ \
        apps/api/internal/api/pbl_store_test.go
git commit -m "feat(pbl): 项目接进 atom 底座——0108 + pbl_project 查询"
```

---

### Task 2: Type detection

**Files:**
- Create: `apps/api/internal/pbl/classify.go`
- Test: `apps/api/internal/pbl/classify_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `gateway.ChatRequest/ChatMessage/ChatUsage` — the same shapes `agent.ProposeSearchKeywords` uses (`agent/search_guidance.go`).
- Produces: `pbl.ProjectKinds []string`, `pbl.IsProjectKind(string) bool`, `pbl.parseKind(string) (string, error)`, and `pbl.DetectKind(ctx, prov gateway.Provider, resolved gateway.Resolved, idea string) (string, gateway.ChatUsage, error)`.

- [ ] **Step 1: Write the failing parse test**

Create `apps/api/internal/pbl/classify_test.go`:

```go
package pbl

import "testing"

func TestParseKind(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		bad  bool
	}{
		{"plain json", `{"kind":"research"}`, "research", false},
		{"fenced", "```json\n{\"kind\":\"design\"}\n```", "design", false},
		{"prose around it", "好的。{\"kind\":\"making\"} 就这样", "making", false},
		{"unknown kind", `{"kind":"podcast"}`, "", true},
		{"empty", ``, "", true},
		{"not json", `research`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseKind(c.in)
			if c.bad {
				if err == nil {
					t.Fatalf("parseKind(%q) = %q, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKind(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("parseKind(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/pbl/ -run TestParseKind -v
```
Expected: FAIL — the package does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `apps/api/internal/pbl/classify.go`:

```go
// Package pbl is the project room's server side: classification now, the step
// loop and artifacts in later slices. It calls the gateway; it never holds a
// key and never talks to a provider directly.
package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// ProjectKinds is the closed set, mirroring pbl_project's CHECK constraint.
// Adding one is a migration plus this slice — the two must not drift.
var ProjectKinds = []string{"website", "research", "design", "making", "investigation"}

func IsProjectKind(s string) bool {
	for _, k := range ProjectKinds {
		if k == s {
			return true
		}
	}
	return false
}

const classifySystem = `你要判断一个中学生描述的项目属于哪一类。

只返回一个 JSON 对象：{"kind": "..."}

kind 只能是下面五个之一：
- website：做一个网站、主页、展示页
- research：想弄明白一个问题，需要查资料、读文献、分析
- design：做一个设计、方案、作品、活动策划
- making：动手做出一个实物或一个能用的东西
- investigation：到真实世界里去看、去问、去记录（走访、观察、问卷）

只回 JSON，不要解释，不要代码块以外的话。`

var errNoKind = errors.New("pbl: no usable kind in model output")

// parseKind pulls {"kind": "..."} out of the model's text. It accepts a fenced
// block or prose around the object, and it REFUSES anything outside the closed
// set — a kind the database would reject must fail here, loudly, rather than
// reach a CHECK constraint as a 500.
func parseKind(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errNoKind
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return "", errNoKind
	}
	var out struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return "", fmt.Errorf("pbl: %w", err)
	}
	k := strings.TrimSpace(strings.ToLower(out.Kind))
	if !IsProjectKind(k) {
		return "", fmt.Errorf("pbl: kind %q is not one of %v", k, ProjectKinds)
	}
	return k, nil
}

const maxClassifyAttempts = 2

// DetectKind classifies the idea she typed. Usage is returned so the caller
// meters even when the parse fails — a call that yielded nothing still cost
// money.
//
// It returns an error rather than a default when the model output cannot be
// used. A silent fallback to "research" would put a wrong, invisible label on
// her project, and the caller (api/pbl_projects.go) is the right place to
// decide what the student sees.
func DetectKind(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, idea string) (string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: classifySystem},
			{Role: gateway.RoleUser, Content: strings.TrimSpace(idea)},
		},
		MaxTokens: 200,
	}
	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxClassifyAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		kind, perr := parseKind(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return kind, lastUsage, nil
	}
	return "", lastUsage, lastErr
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/pbl/ -run TestParseKind -v
```
Expected: PASS, all six subtests.

- [ ] **Step 5: Verify the gateway signatures match reality**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go build ./internal/pbl/
```
Expected: builds clean. If `gateway.Collect` or `gateway.ChatRequest` disagrees, read `internal/agent/search_guidance.go:85-95` and copy that call shape exactly.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/pbl/
git commit -m "feat(pbl): 印记读一段话判项目类型，判不出就报错不兜底"
```

---

### Task 3: The first project is her website

**Files:**
- Modify: `apps/api/internal/pbl/classify.go`
- Test: `apps/api/internal/pbl/classify_test.go`

**Interfaces:**
- Produces: `pbl.ResolveKind(existingProjects int64, detected string) string`.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/pbl/classify_test.go`:

```go
// spec §4 — 她还没有项目时，第一个项目就是做自己的主页。不是推荐，是它就是。
func TestResolveKindForcesWebsiteOnFirstProject(t *testing.T) {
	if got := ResolveKind(0, "research"); got != "website" {
		t.Fatalf("first project = %q, want website", got)
	}
	if got := ResolveKind(0, "website"); got != "website" {
		t.Fatalf("first project = %q, want website", got)
	}
	if got := ResolveKind(3, "research"); got != "research" {
		t.Fatalf("fourth project = %q, want the detected kind", got)
	}
	if got := ResolveKind(1, "making"); got != "making" {
		t.Fatalf("second project = %q, want the detected kind", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/pbl/ -run TestResolveKind -v
```
Expected: FAIL — `ResolveKind` undefined.

- [ ] **Step 3: Write the implementation**

Append to `apps/api/internal/pbl/classify.go`:

```go
// ResolveKind applies spec §4 over the classifier's answer.
//
// A student with no projects yet gets the website project, whatever she wrote
// in the box. The website is the one artifact we host, it is where her
// readings, writings and later projects land, and it is a real build with a
// real audience — so it is what the first project IS, rather than something
// offered alongside others.
//
// The count is "how many projects does she have", used as the stand-in for
// "does she have a site" until pbl_site exists in S5. Swap the input, not the
// rule, when that lands.
func ResolveKind(existingProjects int64, detected string) string {
	if existingProjects == 0 {
		return "website"
	}
	return detected
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/pbl/ -v
```
Expected: PASS, every test in the package.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/pbl/
git commit -m "feat(pbl): 第一个项目就是做自己的主页"
```

---

### Task 4: Handlers and routes

**Files:**
- Create: `apps/api/internal/api/pbl_projects.go`
- Modify: `apps/api/internal/api/api.go` (route table, beside the `writings` block that ends around line 259)
- Test: `apps/api/internal/api/pbl_projects_test.go`

**Interfaces:**
- Consumes: Task 1's sqlc methods; Task 2/3's `pbl.DetectKind` and `pbl.ResolveKind`; `a.resolveEval(ctx) (gateway.Resolved, bool)` (`api/proposal_track.go:120`); `a.recordLiteLLMCall(ctx, userID, atomID, purpose, resolved, usage)` (`api/reading_turn.go:539`); `HasEntitlement`; `httpx.WriteError`.
- Produces: `POST /api/v1/projects`, `GET /api/v1/projects`, `PATCH /api/v1/projects/{id}` and the `projectDTO` JSON shape the frontend reads in Task 5.

- [ ] **Step 1: Write the failing handler tests**

Create `apps/api/internal/api/pbl_projects_test.go`:

```go
package api

import (
	"net/http"
	"strings"
	"testing"
)

// An idea is required. An empty box is a mis-click, not a project.
func TestCreateProjectRejectsEmptyIdea(t *testing.T) {
	env := newLiteTestEnv(t)
	res := env.do(t, http.MethodPost, "/api/v1/projects", `{"idea":"   "}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}

// Her first project is the website whatever she typed (spec §4).
func TestCreateProjectFirstIsWebsite(t *testing.T) {
	env := newLiteTestEnv(t)
	res := env.do(t, http.MethodPost, "/api/v1/projects",
		`{"idea":"我想研究我们学校的剩饭到底去哪了"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s, want 201", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"kind":"website"`) {
		t.Fatalf("body = %s, want kind=website for the first project", res.Body.String())
	}
}

// Another student's project is not hers to rename.
func TestPatchProjectRejectsOtherStudent(t *testing.T) {
	env := newLiteTestEnv(t)
	id := env.createProject(t, "做一个记录校园植物的网站")
	other := env.newStudent(t)
	res := other.do(t, http.MethodPatch, "/api/v1/projects/"+id, `{"name":"偷来的"}`)
	if res.Code != http.StatusNotFound && res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 404 or 403", res.Code)
	}
}

// A status outside the closed set never reaches the CHECK constraint.
func TestPatchProjectRejectsUnknownStatus(t *testing.T) {
	env := newLiteTestEnv(t)
	id := env.createProject(t, "做一个记录校园植物的网站")
	res := env.do(t, http.MethodPatch, "/api/v1/projects/"+id, `{"status":"done"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
}
```

- [ ] **Step 2: Find the real test-env helpers before running**

Run:
```bash
grep -rn "func newLiteTestEnv\|func (e \*liteTestEnv)\|func newTestEnv" apps/api/internal/api/*_test.go | head -20
```
Expected: the package's existing lite HTTP test harness. **Rename the helpers in Step 1 to match what exists**, and add only `createProject` / `newStudent` if the harness genuinely lacks an equivalent. Reuse beats inventing a second harness.

- [ ] **Step 3: Run the tests to verify they fail**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestCreateProject -timeout 1800s -v
```
Expected: FAIL — 404, because the routes are not registered.

- [ ] **Step 4: Write the handlers**

Create `apps/api/internal/api/pbl_projects.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// projectDTO is what the 项目 tab reads. `idea` rides along because the kanban
// card shows her own opening sentence until she has named the project — and
// after, it is still the only record of how she first put it.
type projectDTO struct {
	ID             string `json:"id"`
	Idea           string `json:"idea"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	CoverGround    string `json:"coverGround"`
	CoverGlyph     string `json:"coverGlyph"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
	LastActivityAt string `json:"lastActivityAt"`
}

var projectStatuses = map[string]bool{
	"talking": true, "running": true, "review": true, "keeping": true, "archived": true,
}

const maxIdeaRunes = 4000

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
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
		Idea string `json:"idea"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	idea := strings.TrimSpace(req.Idea)
	if idea == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_idea", "先写一句你想做什么", nil))
		return
	}
	if len([]rune(idea)) > maxIdeaRunes {
		idea = string([]rune(idea)[:maxIdeaRunes])
	}

	// How many she already has decides whether this is the website project.
	// Counted before the insert, so this one is not counted as its own
	// predecessor.
	existing, err := a.d.Queries.CountPblProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Classify. A first project is the website regardless, so skip the call
	// entirely and spend nothing — the answer cannot change the outcome.
	detected := "website"
	if existing > 0 {
		resolved, ok := a.resolveEval(r.Context())
		if !ok {
			httpx.WriteError(w, r, httpx.ErrBadGateway("model_unavailable", "现在联系不上模型，待会儿再试一次", nil))
			return
		}
		kind, usage, derr := pbl.DetectKind(r.Context(), a.d.Provider, resolved, idea)
		// Meter before any bail: a call that yielded nothing still cost money.
		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			a.recordLiteLLMCall(r.Context(), u.ID, uuid.Nil, "pbl_classify", resolved, usage)
		}
		if derr != nil {
			// Surface it. A silent fallback would put a wrong label on her
			// project and hide a broken classifier behind a plausible answer.
			slog.Warn("pbl: classify failed", "err", derr, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrBadGateway("classify_failed", "没能判断这个项目的类型，再说一遍试试", nil))
			return
		}
		detected = kind
	}
	kind := pbl.ResolveKind(existing, detected)

	// atom + pbl_project in ONE transaction: an atom with no project row is an
	// identity nothing can render.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := qtx.CreatePblProject(r.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: idea, Kind: kind,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, projectDTO{
		ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
		CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
		CreatedAt: at.CreatedAt.Format(timeLayout), LastActivityAt: at.CreatedAt.Format(timeLayout),
	})
}

func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListPblProjectsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]projectDTO, 0, len(rows))
	for _, p := range rows {
		out = append(out, projectDTO{
			ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
			CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
			CreatedAt:      p.AtomCreatedAt.Format(timeLayout),
			LastActivityAt: p.LastActivityAt.Format(timeLayout),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// patchProject carries the name-and-cover modal and the kanban's status moves.
func (a *API) patchProject(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_id", "项目不存在", nil))
		return
	}
	row, err := a.d.Queries.GetPblProject(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("no_project", "项目不存在", nil))
		return
	}
	if row.UserID != u.ID {
		// Not "forbidden" — a project she does not own should not be
		// distinguishable from one that does not exist.
		httpx.WriteError(w, r, httpx.ErrNotFound("no_project", "项目不存在", nil))
		return
	}

	var req struct {
		Name        *string `json:"name"`
		CoverGround *string `json:"coverGround"`
		CoverGlyph  *string `json:"coverGlyph"`
		Status      *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}

	if req.Status != nil {
		s := strings.TrimSpace(*req.Status)
		if !projectStatuses[s] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个状态", nil))
			return
		}
		if _, err := a.d.Queries.SetPblProjectStatus(r.Context(),
			sqlc.SetPblProjectStatusParams{AtomID: id, Status: s}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.Status = s
	}

	if req.Name != nil || req.CoverGround != nil || req.CoverGlyph != nil {
		name, ground, glyph := row.Name, row.CoverGround, row.CoverGlyph
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
			if len([]rune(name)) > 60 {
				name = string([]rune(name)[:60])
			}
		}
		if req.CoverGround != nil {
			ground = strings.TrimSpace(*req.CoverGround)
		}
		if req.CoverGlyph != nil {
			glyph = strings.TrimSpace(*req.CoverGlyph)
		}
		p, err := a.d.Queries.UpdatePblProjectMeta(r.Context(),
			sqlc.UpdatePblProjectMetaParams{AtomID: id, Name: name, CoverGround: ground, CoverGlyph: glyph})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		row.Name, row.CoverGround, row.CoverGlyph = p.Name, p.CoverGround, p.CoverGlyph
	}

	httpx.WriteJSON(w, http.StatusOK, projectDTO{
		ID: row.AtomID.String(), Idea: row.Idea, Kind: row.Kind, Name: row.Name,
		CoverGround: row.CoverGround, CoverGlyph: row.CoverGlyph, Status: row.Status,
		CreatedAt:      row.AtomCreatedAt.Format(timeLayout),
		LastActivityAt: row.LastActivityAt.Format(timeLayout),
	})
}
```

- [ ] **Step 5: Reconcile the helper names this file assumes**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go build ./internal/api/
```
Expected: compiler errors naming any helper that differs — likely `timeLayout`, `httpx.WriteJSON`, `httpx.ErrNotFound`, `httpx.ErrBadGateway`. Fix each by grepping for the real one, for example:
```bash
grep -rn "func WriteJSON\|func ErrNotFound\|func ErrBadGateway" apps/api/internal/httpx/
grep -rn "timeLayout\s*=" apps/api/internal/api/
```
Use what exists. Do not add a second time format or a second JSON writer.

- [ ] **Step 6: Register the routes**

In `apps/api/internal/api/api.go`, immediately after the `writings` block (around line 259), add:

```go
	// 项目（PBL，S1）。和 readings / writings 一样只对轻量版开放。
	mux.Handle("POST /api/v1/projects", liteOnly(a.createProject))
	mux.Handle("GET /api/v1/projects", liteOnly(a.listProjects))
	mux.Handle("PATCH /api/v1/projects/{id}", liteOnly(a.patchProject))
```

- [ ] **Step 7: Run the tests to verify they pass**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run "TestCreateProject|TestPatchProject" -timeout 1800s -v
```
Expected: PASS, all four.

- [ ] **Step 8: Run the whole API suite for regressions**

Run:
```bash
cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s
```
Expected: PASS. The `atom_kind_check` change is the one thing that could break an existing test; if something fails, it is telling you a real constraint assumption elsewhere.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/api/pbl_projects.go apps/api/internal/api/pbl_projects_test.go apps/api/internal/api/api.go
git commit -m "feat(pbl): 建项目 / 列项目 / 改名改封面改状态三个端点"
```

---

### Task 5: The lite API client and route

**Files:**
- Create: `apps/lite-web/src/api/projects.ts`
- Modify: `apps/lite-web/src/routing.ts`
- Test: `apps/lite-web/src/api/projects.test.ts`

**Interfaces:**
- Consumes: `apiFetch` from `./client`; Task 4's `projectDTO`.
- Produces: `Project`, `ProjectStatus`, `PROJECT_STATUSES`, `listProjects()`, `createProject(idea)`, `updateProject(id, patch)`, and `groupByStatus(projects)` for the kanban.

- [ ] **Step 1: Write the failing test**

Create `apps/lite-web/src/api/projects.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { groupByStatus, PROJECT_STATUSES, type Project } from "./projects";

function p(id: string, status: Project["status"]): Project {
  return {
    id, idea: "x", kind: "research", name: "", coverGround: "", coverGlyph: "",
    status, createdAt: "2026-09-01T00:00:00Z", lastActivityAt: "2026-09-01T00:00:00Z",
  };
}

describe("groupByStatus", () => {
  it("returns every column even when empty, so the board keeps its shape", () => {
    const got = groupByStatus([]);
    expect(Object.keys(got)).toEqual([...PROJECT_STATUSES]);
    for (const s of PROJECT_STATUSES) expect(got[s]).toEqual([]);
  });

  it("puts each project in its own column and preserves input order", () => {
    const got = groupByStatus([p("a", "running"), p("b", "talking"), p("c", "running")]);
    expect(got.running.map((x) => x.id)).toEqual(["a", "c"]);
    expect(got.talking.map((x) => x.id)).toEqual(["b"]);
    expect(got.archived).toEqual([]);
  });

  it("drops a status the server does not know rather than throwing", () => {
    const rogue = { ...p("z", "running"), status: "done" } as unknown as Project;
    expect(() => groupByStatus([rogue])).not.toThrow();
    expect(Object.values(groupByStatus([rogue])).flat()).toEqual([]);
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run:
```bash
cd apps/lite-web && npx vitest run src/api/projects.test.ts
```
Expected: FAIL — module not found.

- [ ] **Step 3: Write the client**

Create `apps/lite-web/src/api/projects.ts`:

```ts
import { apiFetch } from "./client";

// api/projects.ts — the 项目 tab's client. Shapes read straight off
// apps/api/internal/api/pbl_projects.go's projectDTO, not guessed.

/** Mirrors pbl_project's status CHECK. Order is the kanban's column order. */
export const PROJECT_STATUSES = ["talking", "running", "review", "keeping", "archived"] as const;
export type ProjectStatus = (typeof PROJECT_STATUSES)[number];

export const PROJECT_STATUS_LABELS: Record<ProjectStatus, string> = {
  talking: "在聊",
  running: "在做",
  review: "复盘",
  keeping: "在养着",
  archived: "收起来了",
};

export type ProjectKind = "website" | "research" | "design" | "making" | "investigation";

export interface Project {
  id: string;
  /** Her own opening sentence. Shown on the card until she names it, and kept
   *  afterwards — it is the only record of how she first put it. */
  idea: string;
  kind: ProjectKind;
  name: string;
  coverGround: string;
  coverGlyph: string;
  status: ProjectStatus;
  createdAt: string;
  lastActivityAt: string;
}

export function listProjects(): Promise<Project[]> {
  return apiFetch<Project[]>("/api/v1/projects");
}

export function createProject(idea: string): Promise<Project> {
  return apiFetch<Project>("/api/v1/projects", {
    method: "POST",
    body: JSON.stringify({ idea }),
  });
}

export function updateProject(
  id: string,
  patch: Partial<Pick<Project, "name" | "coverGround" | "coverGlyph" | "status">>,
): Promise<Project> {
  return apiFetch<Project>(`/api/v1/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

/**
 * Bucket projects into kanban columns.
 *
 * Every column is present even when empty, so the board does not change shape
 * as projects move — a column that vanishes when it empties makes the board
 * jump under her hand. A status the server does not know is dropped rather
 * than thrown on: a future status must not blank the whole page.
 */
export function groupByStatus(projects: Project[]): Record<ProjectStatus, Project[]> {
  const out = Object.fromEntries(PROJECT_STATUSES.map((s) => [s, [] as Project[]])) as Record<
    ProjectStatus,
    Project[]
  >;
  for (const p of projects) {
    if (p.status in out) out[p.status].push(p);
  }
  return out;
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
cd apps/lite-web && npx vitest run src/api/projects.test.ts
```
Expected: PASS, three tests.

- [ ] **Step 5: Add the route**

Read `apps/lite-web/src/routing.ts`, then extend `LiteRoute` with a `projects` tab and `parseLiteRoute` / `liteRoutePath` to handle `/projects` and `/projects/:id`, following exactly how `readings` is handled in that file.

- [ ] **Step 6: Typecheck**

Run:
```bash
cd apps/lite-web && npx tsc --noEmit
```
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add apps/lite-web/src/api/projects.ts apps/lite-web/src/api/projects.test.ts apps/lite-web/src/routing.ts
git commit -m "feat(pbl): 项目的前端客户端 + 路由"
```

---

### Task 6: The landing page — big input box and kanban

**Files:**
- Create: `apps/lite-web/src/projects/ProjectsLanding.tsx`
- Create: `apps/lite-web/src/projects/ProjectCard.tsx`
- Modify: `apps/lite-web/src/LiteApp.tsx` (the `TABS` array and the route switch)

**Interfaces:**
- Consumes: Task 5's `listProjects`, `createProject`, `groupByStatus`, `PROJECT_STATUSES`, `PROJECT_STATUS_LABELS`.
- Produces: `<ProjectsLanding />`, mounted at the `projects` tab.

- [ ] **Step 1: Read the page this one is a sibling of**

Run:
```bash
sed -n '1,120p' apps/lite-web/src/readings/ReadingsLanding.tsx
```
Match its structure: a greeting, one hero input, quiet secondary objects below. Reuse `PromptTile` and `@/ui`'s `Button` / `Icon` rather than new primitives. Do not copy the 圈点 ink-stroke flourish — that gesture belongs to 阅读, and repeating it makes both pages generic.

- [ ] **Step 2: Build `ProjectCard.tsx`**

One card: the cover (ground + glyph), the name or — when she has not named it yet — her own `idea` sentence, and the kind as a quiet label. No progress bars, no streaks, no counts.

- [ ] **Step 3: Build `ProjectsLanding.tsx`**

Two parts, in this order:

1. The big input box. One textarea, one submit. On submit call `createProject(idea)`, then hand the returned project to the modal from Task 7. While the request is in flight, disable submit and show that 印记 is reading it — this call includes a model round-trip, so it is not instant.
2. The kanban: `PROJECT_STATUSES.map(...)` into columns, each headed by `PROJECT_STATUS_LABELS[s]` and a count, each holding `ProjectCard`s. Columns scroll independently; the page itself never scrolls sideways.

Empty state: when she has no projects, show only the input box and one line saying the first project will be her own homepage. Do not render five empty columns at someone who has never been here.

- [ ] **Step 4: Mount the tab**

In `apps/lite-web/src/LiteApp.tsx`, add to `TABS`:

```tsx
{ key: "projects", label: "项目", icon: Hammer },
```

importing `Hammer` from `lucide-react`, and extend `LiteTab`, `tabPath` and the route switch to mount `<ProjectsLanding />`. Follow the existing `readings` / `writings` branches exactly.

- [ ] **Step 5: Look at it in a real browser**

Run the dev server, sign in as the seeded trial student, open `/projects`, and take a Playwright screenshot. **Open the PNG and look at it.** Check: the input box is the hero, the columns keep their shape, nothing scrolls sideways, and the empty state is not five empty columns. A screenshot you did not look at proves nothing — in 2026-08-30 lite had 344 green tests over a completely blank exported PNG.

- [ ] **Step 6: Commit**

```bash
git add apps/lite-web/src/projects/ apps/lite-web/src/LiteApp.tsx
git commit -m "feat(pbl): 项目首页——一个大输入框，一块按状态分列的看板"
```

---

### Task 7: The name-and-cover modal

**Files:**
- Create: `apps/lite-web/src/projects/NameAndCover.tsx`
- Create: `apps/lite-web/src/projects/covers.ts`
- Modify: `apps/lite-web/src/projects/ProjectsLanding.tsx`
- Test: `apps/lite-web/src/projects/covers.test.ts`

**Interfaces:**
- Consumes: Task 5's `updateProject`; the prototype's `eco/projects/CoverPicker.tsx` and `eco/data/projects.ts` as source material.
- Produces: `COVER_GROUNDS: string[]`, `COVER_GLYPHS: string[]`, `defaultCover(kind)`, and `<NameAndCover project onDone />`.

- [ ] **Step 1: Lift the cover vocabulary out of the prototype**

Run:
```bash
sed -n '1,144p' apps/lite-web/src/eco/projects/CoverPicker.tsx
grep -n "cover\|COVER" apps/lite-web/src/eco/data/projects.ts | head -30
```
Copy the 8 grounds and 16 glyphs into `covers.ts` as plain exported arrays. Take the values, leave the prototype's store wiring behind.

- [ ] **Step 2: Write the failing test**

Create `apps/lite-web/src/projects/covers.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { COVER_GLYPHS, COVER_GROUNDS, defaultCover } from "./covers";

describe("covers", () => {
  it("offers the full 8 × 16 vocabulary", () => {
    expect(COVER_GROUNDS).toHaveLength(8);
    expect(COVER_GLYPHS).toHaveLength(16);
    expect(new Set(COVER_GROUNDS).size).toBe(8);
    expect(new Set(COVER_GLYPHS).size).toBe(16);
  });

  it("always proposes a cover that exists in the vocabulary", () => {
    for (const kind of ["website", "research", "design", "making", "investigation"] as const) {
      const c = defaultCover(kind);
      expect(COVER_GROUNDS).toContain(c.ground);
      expect(COVER_GLYPHS).toContain(c.glyph);
    }
  });
});
```

- [ ] **Step 3: Run it to verify it fails**

Run:
```bash
cd apps/lite-web && npx vitest run src/projects/covers.test.ts
```
Expected: FAIL — module not found.

- [ ] **Step 4: Write `covers.ts`**

Export the two arrays and:

```ts
/** A starting suggestion so the modal is never blank. She overrides it in one
 *  tap; the point is that she is choosing, and choosing is easier against
 *  something than against nothing. */
export function defaultCover(kind: ProjectKind): { ground: string; glyph: string } { /* ... */ }
```

- [ ] **Step 5: Run the test to verify it passes**

Run:
```bash
cd apps/lite-web && npx vitest run src/projects/covers.test.ts
```
Expected: PASS, two tests.

- [ ] **Step 6: Build the modal**

`<NameAndCover project onDone />` shows, in this order: what 印记 judged this project to be, in one plain sentence; a name field; the 8 × 16 cover picker. Confirm calls `updateProject(id, { name, coverGround, coverGlyph })`, then `onDone(updated)`.

Two rules: **the name field starts empty** — a pre-filled name is a name she will accept rather than choose — and the modal cannot be dismissed into nothing, because the project already exists on the server by this point. Closing it leaves the project unnamed on the board, where her `idea` sentence stands in for the name.

- [ ] **Step 7: Look at it in a real browser**

Create a project, screenshot the modal, open the PNG and look at it. Check the 16 glyphs are all reachable without scrolling the page sideways and the picker works at phone width.

- [ ] **Step 8: Commit**

```bash
git add apps/lite-web/src/projects/
git commit -m "feat(pbl): 起个名字，挑张封面"
```

---

### Task 8: Dark theme tokens

**Files:**
- Modify: `apps/web/src/index.css` (the shared `:root` token block, lines 7–35)
- Test: `apps/lite-web/src/projects/theme.test.ts`

**Interfaces:**
- Produces: a `:root[data-theme="dark"]` token block, and `applyTheme(theme)` / `readStoredTheme()` in `apps/lite-web/src/shared/theme.ts`.

- [ ] **Step 1: Read the light tokens**

Run:
```bash
sed -n '1,40p' apps/web/src/index.css
```
Note every `--mk-*` name. The dark block redefines **only colour tokens** — radii, shadows, easings and durations stay as they are.

- [ ] **Step 2: Add the dark block**

In `apps/web/src/index.css`, immediately after the `:root` block, add a `:root[data-theme="dark"]` block redefining `--mk-paper`, `--mk-surface`, `--mk-border`, `--mk-muted`, `--mk-secondary`, `--mk-faint`, `--mk-ink`, `--mk-input-border`, the accent ramp, the seven macaron triples and the four semantic pairs.

Two constraints:
- **Additive and inert.** Without the attribute nothing changes, so pro renders exactly as it does today.
- **Keep the warmth.** This product's light theme is warm paper, so its dark theme is warm dark — a near-black with red-brown in it, never a blue-grey slate. The accent stays 朱砂; darken the ramp's light end rather than shifting its hue.

- [ ] **Step 3: Write the failing test**

Create `apps/lite-web/src/projects/theme.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { applyTheme, readStoredTheme } from "../shared/theme";

describe("theme", () => {
  it("sets the attribute for dark and removes it for light", () => {
    applyTheme("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    applyTheme("light");
    expect(document.documentElement.getAttribute("data-theme")).toBeNull();
  });

  it("falls back to light when storage holds something unknown", () => {
    localStorage.setItem("mk-theme", "midnight");
    expect(readStoredTheme()).toBe("light");
  });

  it("survives storage being unavailable", () => {
    const original = Object.getOwnPropertyDescriptor(window, "localStorage");
    Object.defineProperty(window, "localStorage", {
      get() { throw new Error("blocked"); }, configurable: true,
    });
    expect(() => readStoredTheme()).not.toThrow();
    expect(readStoredTheme()).toBe("light");
    if (original) Object.defineProperty(window, "localStorage", original);
  });
});
```

- [ ] **Step 4: Run it to verify it fails**

Run:
```bash
cd apps/lite-web && npx vitest run src/projects/theme.test.ts
```
Expected: FAIL — `../shared/theme` not found.

- [ ] **Step 5: Write `apps/lite-web/src/shared/theme.ts`**

```ts
export type Theme = "light" | "dark";

const KEY = "mk-theme";

/** Light removes the attribute rather than setting `data-theme="light"`, so
 *  the light palette has exactly one definition — the bare `:root` block — and
 *  cannot drift from the dark block's assumptions about it. */
export function applyTheme(theme: Theme): void {
  if (theme === "dark") document.documentElement.setAttribute("data-theme", "dark");
  else document.documentElement.removeAttribute("data-theme");
}

/** Reading storage throws outright in some contexts (a browser set to block
 *  site data), so this never lets a theme preference break the app boot. */
export function readStoredTheme(): Theme {
  try {
    return localStorage.getItem(KEY) === "dark" ? "dark" : "light";
  } catch {
    return "light";
  }
}

export function storeTheme(theme: Theme): void {
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    // A preference we cannot persist is not worth failing a render over.
  }
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run:
```bash
cd apps/lite-web && npx vitest run src/projects/theme.test.ts
```
Expected: PASS, three tests.

- [ ] **Step 7: Apply it at boot and look at both themes**

Call `applyTheme(readStoredTheme())` in `apps/lite-web/src/main.tsx` before render. Then screenshot `/projects` in **both** themes and open both PNGs. Check contrast on the kanban column headers and the cover glyphs — glyph art tuned for warm paper is where a dark theme most often goes muddy.

- [ ] **Step 8: Confirm pro is untouched**

Run:
```bash
cd apps/web && npx vitest run
```
Expected: PASS. Then screenshot one pro screen and confirm it looks exactly as before — the block is inert without the attribute, and this is the check that proves it.

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/index.css apps/lite-web/src/shared/theme.ts apps/lite-web/src/projects/theme.test.ts apps/lite-web/src/main.tsx
git commit -m "feat(theme): 暖色的暗色主题，两个版本共用"
```

---

## Self-review notes

**Spec coverage.** S1's line in spec §20 is "tables · the kanban landing · big input box · type detection · name-and-cover modal · dark-theme tokens". Tasks 1, 6, 6, 2, 7, 8 cover them in order. §4 (first project is the website) is Task 3, which the spec listed under S5 — it is here because the create endpoint cannot pick a kind without it.

**Deliberate deviation from the spec, needs a §16 amendment.** The spec has `pbl_project` as a root table with its own `pbl_thread_item`. This plan makes a project an **atom**, so it inherits ownership, activity, heartbeat, `atom_message` and `llm_call.atom_id` metering rather than growing a second substrate beside an identical one. **Checked on 2026-09-01, answer recorded in spec §10.1–10.4:** `atom_message` does carry it. `pbl_thread_item` and `pbl_branch_turn` are both dropped — a branch is `atom_message.branch_id` (mirroring 0102's `block_id`), and non-prose items ride in `payload` (0106). 🚨 S2 must also fix the `NextAtomMessageSeq` race described in spec §10.2 **in the same slice as branches**: there is no `FOR UPDATE` in `queries/`, and branches make concurrent appends to one atom normal rather than rare.

**Only `pbl_project` is created.** The spec lists ten tables. The other nine describe the plan, branches and artifacts, whose shape depends on the method layer the product owner is still designing (§6). Empty tables built against a pending design are tables that get rewritten. Each later slice brings its own.

**Type consistency.** `ProjectKind` and `ProjectStatus` are declared once each on both sides — `pbl.ProjectKinds` / `projectStatuses` in Go, `PROJECT_STATUSES` in TS — and both mirror the CHECK constraints in 0108. `projectDTO`'s field names match `Project`'s exactly.

**Known unknowns, flagged as verification steps rather than guesses.** Tasks 1, 4 and 5 each open with a `grep` for the real helper names (`newTestDeps`, the lite HTTP harness, `timeLayout`, `httpx.WriteJSON`) because those were not read while writing this plan. Reconcile against what exists; do not add a second helper.

---

## What actually happened (2026-09-01, executed inline)

All eight tasks are done and pushed. Where execution departed from the plan is
the useful part.

**1 · The plan's routes and handler names were WRONG and would have broken pro.**
`/api/v1/projects` is already a pro route (`api.go:88`), `a.createProject` and
`a.listProjects` already exist as methods, and `edition_test.go` asserts a lite
student gets **404** on that path. Everything moved to `/api/v1/pbl/projects`
with `createPblProject` / `listPblProjects` / `patchPblProject`. Caught by the
grep-first steps, which is what they were for.

**2 · The plan's test helpers do not exist.** The real harness is
`liteHandler(t) → (handler, cookie, *sqlc.Queries, *pgxpool.Pool)` plus
`SeedUserID`, `SeedSchoolID`, `createStudent`, `signInAs(t, pool, userID)` and
`withCookie`. Tests live in package `api_test` and need the dot-import
`. "mindimprint/api/internal/api"` to see the exported seed ids.

**3 · `httpx` has no `ErrBadGateway`.** The codebase uses
`httpx.ErrAIDialogueFailed(reason)` (502) when a model reply cannot be
understood. `httpx.ErrNotFound` takes only a message. There is no `timeLayout`
constant — DTOs format with `time.RFC3339`.

**4 · `recordLiteLLMCall` needed a one-line fix.** It hardcoded
`AtomID: pgtype.UUID{…, Valid: true}`, but classification happens BEFORE the
project exists. `llm_call.atom_id` has been nullable since 0094, so `uuid.Nil`
now records as NULL; otherwise the row fails its foreign key and vanishes into
a swallowed warning, losing the cost of a call that was really made.

**5 · Covers use string ids, not the prototype's array index.** An index in a
database column is a promise never to reorder the array.

**6 · Two defects found by LOOKING at screenshots, invisible to any unit test.**
The cover fallback existed in two places, so an uncovered project rendered
coral on the board and blue in the modal and changed colour the moment she
saved (now one `resolveCover`, with a regression test). And on a phone the
开始 button wrapped into two stacked characters.

**7 · Dark theme hit two real traps.** `!important` is required on `--mk-paper`
and the accent ramp, because `BackgroundProvider` / `AccentProvider` write
those vars as inline styles and inline beats any stylesheet rule — without it
dark mode is light-on-light. And the theme must be applied before first paint;
flipping `data-theme` at runtime left cards painting light colours while the
vars already read dark. Both documented at the block itself.

**8 · A false test failure worth remembering.** `go test ./...` run beside a
live e2e stack reported 7 packages FAILED at exactly **1980s** — that is
`-timeout 1800s` firing under Docker contention, not an assertion. All passed
in seconds when run alone. Read the duration before believing a failure.

Verified at the end: 23/23 Go packages (`internal/api` re-run uncached, 44.9s),
390 lite unit tests, and a live Playwright walk against a real API and Postgres.
