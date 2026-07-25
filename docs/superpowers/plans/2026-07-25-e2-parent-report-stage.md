# 家长报告（阶段模式）· E2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the teacher console's inert 导出家长版·阶段报告 stub into a real,
printable per-(student, week) parent projection: 4 usage-stat cards + a
cross-session growth narrative + shared E1 front-matter.

**Architecture:** Same "one producer, one more projection" + D2 compose-and-store
as E1. A stage report reuses D2's event usage pipeline + week-window math and
Spec C's `ability.Aggregate`; one flagship call gentles them into a stored
`ParentStageProse` bundle. Parallel stage handlers (week_start date scope,
`llm_call` ProjectID NULL) leave E1's UUID-typed project path byte-unchanged;
storage reuses migration 0035's `parent_report_prose` table.

**Tech Stack:** Go (`net/http` + `pgx`/`sqlc`), Postgres, Zod contracts, React.

Spec: `docs/superpowers/specs/2026-07-25-e2-parent-report-stage-design.md`.

## Global Constraints

- 客户端绝不直连模型；所有 LLM 调用走后端网关；密钥只在 `apps/api` 服务端。
- **评估/家长措辞走旗舰绝不降级**；家长 compose 用 `llm_call` `Purpose:"parent_report"`,
  `Surface:"teacher"`, **`ProjectID` NULL**（阶段不属于任何 project）; **meters even on rejection**.
- **两轴永不合成总分** (RL-5)；阶段面 **无任何 A 轴数字，无任何能力等级数字/字母代码**。
- **机会供给先于判定 / 敢于空白**：thin 使用 → composer 走 起步 register，`stageHighlight` 留空，不硬凑亮点。
- **说人话**：compose 输出正则拦截内部代码 / 术语，含 `L1–L4` 字母等级。
- **家长端与教师端隔离**：无预警 / 标签 / 👍👎 / 提示词透镜 / 官方 AP / 交互证据 / 作品与过程。
- **GET 永不 spend；POST 是唯一 spend**，失败返回 200 + `prose:null`，绝不成墙；first-open-wins (`ON CONFLICT DO NOTHING`).
- Single-source deterministic parts (stat labels, week label) server-side (D1 anti-drift).
- **No new migration** — reuse 0035 `parent_report_prose` (surface='stage', scope_id=week_start `YYYY-MM-DD`).
- `make sqlc` from `apps/api`; **never hand-edit** `apps/api/internal/store/sqlc/*`;
  the only dir-level `git add` allowed is the generated `apps/api/internal/store/sqlc`.
- **NEVER `git add` a whole directory otherwise** — name each file explicitly
  (`M package.json` + untracked docs/PNGs at repo root are NOT ours).
- Test discipline: card/gate/projection/creation/config/migration/**sqlc-query** changes
  run **FULL** Go packages, never `-run` subsets. `internal/api` needs ~210s →
  every backend dispatch allows **≥600s** (`timeout: 600000`) and runs foreground.
  Backend: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`.
  Web: `cd apps/web && npm test` + `npx tsc --noEmit`.
  Contracts: `cd packages/contracts && npm test` + `npx tsc --noEmit`.
- Import-cycle guard: `internal/ability` imports `internal/agent`, so **`internal/agent`
  must NOT import `internal/ability`** — the composer takes a plain `agent.AbilitySummary`
  that the `internal/api` handler builds from `ability.Model`.

**Task deps:** T1, T2, T3 independent · T4 needs T1+T3 · T5 independent (web
refactor) · T6 needs T2+T5.

---

### Task 1: Stage composer — `internal/agent/compose_parent_stage.go`

**Files:**
- Create: `apps/api/internal/agent/compose_parent_stage.go`
- Test: `apps/api/internal/agent/compose_parent_stage_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `gateway.Chat*`,
  and E1's rune caps (`parentWarmMax`, `parentOppMax`, `parentAdviceMax`) + `ParentAdvice`
  from `compose_parent.go` (same package).
- Produces (later tasks rely on these exact names):
  - `type ParentStageProse struct{ WarmLine, StageGrowth, StageHighlight, StageForward string; Advice []ParentAdvice }`
  - `type AbilityDepthFact struct{ Name, LevelLabel string; EvidenceCount int }`
  - `type AbilitySummary struct{ TotalSessions, BoundarySettings, AdversaryInvites, OpportunitiesTaken, OpportunitiesMissed int; Depth []AbilityDepthFact }`
  - `type ParentStageFacts struct{ Name, Subject, Klass string; ActiveDays, Turns, Reports, CourseSteps int; Ability AbilitySummary }`
  - `func ComposeParentStage(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f ParentStageFacts) (ParentStageProse, gateway.ChatUsage, error)`

- [ ] **Step 1: Write the failing test** — `compose_parent_stage_test.go`

```go
package agent

import (
	"strings"
	"testing"
)

func validStageProse() ParentStageProse {
	return ParentStageProse{
		WarmLine:       "这一阶段，孩子在自己拿主意上有明显的进步。",
		StageGrowth:    "这段时间他更愿意先自己想清楚，再去请 AI 帮忙检查，而不是一上来就要答案。",
		StageHighlight: "本周他主动请 AI 扮演反方，来挑自己论证里的问题，这是很成熟的学习方式。",
		StageForward:   "可以给他更高一点的目标，鼓励他把研究的意义讲得更具体。",
		Advice: []ParentAdvice{
			{Title: "请他讲给你听", Text: "让他用一句话说清这份研究不能说明什么。"},
			{Title: "保护他的自主", Text: "鼓励他先自己判断，再去问 AI。"},
			{Title: "给一点挑战", Text: "问他如果要再进一步，还差哪一步。"},
		},
	}
}

func TestValidateParentStageProse_OK(t *testing.T) {
	if err := validateParentStageProse(validStageProse()); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}
}

func TestValidateParentStageProse_EmptyHighlightAllowed(t *testing.T) {
	p := validStageProse()
	p.StageHighlight = "" // 敢于空白: a thin window has no highlight
	if err := validateParentStageProse(p); err != nil {
		t.Fatalf("empty highlight must be allowed: %v", err)
	}
}

func TestValidateParentStageProse_RejectsEmptyGrowth(t *testing.T) {
	p := validStageProse()
	p.StageGrowth = "   "
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("empty stageGrowth must be rejected")
	}
}

func TestValidateParentStageProse_RejectsLevelCodeLeak(t *testing.T) {
	p := validStageProse()
	p.StageGrowth = "他从 L2 迈向 L3，进步明显。" // bare level codes must never reach parents
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("L-code leak must be rejected")
	}
}

func TestValidateParentStageProse_RejectsAxisCodeLeak(t *testing.T) {
	p := validStageProse()
	p.Advice[0].Text = "在 A4 上多鼓励他。"
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("A-code leak must be rejected")
	}
}

func TestValidateParentStageProse_RejectsWrongAdviceCount(t *testing.T) {
	p := validStageProse()
	p.Advice = p.Advice[:2] // must be exactly 3
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("advice count != 3 must be rejected")
	}
}

func TestValidateParentStageProse_RejectsOverCap(t *testing.T) {
	p := validStageProse()
	p.WarmLine = strings.Repeat("字", parentWarmMax+1)
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("over-cap warmLine must be rejected")
	}
}
```

- [ ] **Step 2: Run it, expect FAIL** (undefined `validateParentStageProse`)

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestValidateParentStageProse`
Expected: compile error / FAIL.

- [ ] **Step 3: Write `compose_parent_stage.go`**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
)

// ParentStageProse is the ONLY thing the model contributes to a stage report:
// gentled family-facing wording over deterministic usage stats + the
// cross-session ability standing. It cannot change any number — those are
// computed and rendered deterministically. Stored as parent_report_prose.prose
// under surface='stage'.
type ParentStageProse struct {
	WarmLine       string         `json:"warmLine"`
	StageGrowth    string         `json:"stageGrowth"`
	StageHighlight string         `json:"stageHighlight"` // may be "" (敢于空白)
	StageForward   string         `json:"stageForward"`
	Advice         []ParentAdvice `json:"advice"`
}

// AbilityDepthFact / AbilitySummary are a plain, import-cycle-safe projection of
// ability.Model (which imports agent). The internal/api handler builds this from
// ability.Aggregate and hands it to the composer as reference substance — its
// numbers/labels are input only and must never surface in the OUTPUT.
type AbilityDepthFact struct {
	Name          string
	LevelLabel    string // "" ⇒ 证据不足·需更多任务
	EvidenceCount int
}

type AbilitySummary struct {
	TotalSessions       int
	BoundarySettings    int
	AdversaryInvites    int
	OpportunitiesTaken  int
	OpportunitiesMissed int
	Depth               []AbilityDepthFact
}

// ParentStageFacts is the composer input for one (student, week).
type ParentStageFacts struct {
	Name    string
	Subject string // week label, e.g. 第 30 周（7.20–7.26）
	Klass   string

	ActiveDays  int
	Turns       int
	Reports     int
	CourseSteps int

	Ability AbilitySummary
}

// parentStageBareCode extends E1's leak guard with bare LEVEL codes (L1–L4):
// the composer is fed level labels, but the parent surface shows only the 台阶
// words 起步/发展/熟练/优秀, never L3. Case-insensitive, space-tolerant.
var parentStageBareCode = regexp.MustCompile(`(?i)(\b[da]\s*[1-6]\b|\bl\s*[1-4]\b|given_taken|given_not_taken|not_supplied|solo|\bp\s*[0-3]\b)`)

func parentStageSystemPrompt() string {
	return strings.Join([]string{
		"你在为一位学生的家长写一份「阶段成长报告」的措辞。读者是家长，不是老师，也不是学生本人。",
		"你只负责措辞。所有数字（使用天数、对话轮次等）都已算好并会另行展示，你不得改动，也不得编造未给出的数字或结论。",
		"规则：",
		"1. 只使用给你的事实（本阶段使用数据 + 跨会话能力概况）。不得引入任何未给出的行为、数字或结论。",
		"2. 说人话、温和。绝不出现 D1–D6 / A1–A6 / L1–L4 这类内部代码或字母数字等级，也不出现 given_taken / SOLO / P0–P3 之类术语。",
		"3. 描述「这段时间的变化」时，只用台阶词（起步/发展/熟练/优秀）或大白话，绝不写出等级数字/字母。",
		"4. 「机会供给先于判定」+「敢于空白」：使用很少或刚起步时，如实说「刚起步、暂未见到明显提升」，不硬凑亮点；本阶段没有明显亮点时，stageHighlight 留空字符串。",
		"5. 不贴标签、不排名、不预测考分。只描述这一阶段观察到的行为与变化。",
		"6. 只输出 JSON：{\"warmLine\":\"\",\"stageGrowth\":\"\",\"stageHighlight\":\"\",\"stageForward\":\"\",\"advice\":[{\"title\":\"\",\"text\":\"\"}]}",
		"advice 写恰好 3 条家长在家可以怎么帮；stageHighlight 可为空字符串。",
	}, "\n")
}

// ParentStageFactsPrompt renders the reference substance the composer may see.
// Deliberately excludes any project-scoped canonical readings — a stage report
// is not scoped to one project.
func ParentStageFactsPrompt(f ParentStageFacts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n班级：%s\n时间范围：%s\n", f.Name, f.Klass, f.Subject)
	fmt.Fprintf(&b, "本阶段使用：活跃 %d 天，AI 对话 %d 轮，生成能力报告 %d 份，完成课程 %d 节。\n",
		f.ActiveDays, f.Turns, f.Reports, f.CourseSteps)
	b.WriteString("跨会话能力当前概况（仅供你参考，措辞里不得写出等级代码或字母数字等级）：\n")
	for _, d := range f.Ability.Depth {
		label := d.LevelLabel
		if label == "" {
			label = "证据不足·需更多任务"
		}
		fmt.Fprintf(&b, "- %s：%s（累计证据 %d 次）\n", d.Name, label, d.EvidenceCount)
	}
	fmt.Fprintf(&b, "自主观察：累计参与 %d 次会话；主动设界 %d 次，主动请对手检验 %d 次；机会已给并接住 %d 次，机会已给但未接住 %d 次。\n",
		f.Ability.TotalSessions, f.Ability.BoundarySettings, f.Ability.AdversaryInvites,
		f.Ability.OpportunitiesTaken, f.Ability.OpportunitiesMissed)
	return b.String()
}

// ComposeParentStage makes ONE flagship call. Usage is returned even when the
// output is rejected, so the caller records the spend either way.
func ComposeParentStage(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f ParentStageFacts) (ParentStageProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: parentStageSystemPrompt()},
			{Role: gateway.RoleUser, Content: ParentStageFactsPrompt(f)},
		},
	})
	if err != nil {
		return ParentStageProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage
	var out ParentStageProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return ParentStageProse{}, usage, fmt.Errorf("agent: parent stage prose not JSON: %w", err)
	}
	if err := validateParentStageProse(out); err != nil {
		return ParentStageProse{}, usage, err
	}
	return out, usage, nil
}

// validateParentStageProse enforces wording-only: warmLine/stageGrowth/
// stageForward non-empty + capped; stageHighlight optional but capped; exactly
// 3 advice items each non-empty + capped; no internal register leaks anywhere.
func validateParentStageProse(p ParentStageProse) error {
	req := []struct {
		name, val string
		max       int
	}{
		{"warmLine", p.WarmLine, parentWarmMax},
		{"stageGrowth", p.StageGrowth, parentOppMax},
		{"stageForward", p.StageForward, parentWarmMax},
	}
	for _, s := range req {
		if strings.TrimSpace(s.val) == "" {
			return fmt.Errorf("agent: parent stage prose leaves %s empty", s.name)
		}
		if utf8.RuneCountInString(s.val) > s.max {
			return fmt.Errorf("agent: parent stage prose %s too long", s.name)
		}
	}
	if utf8.RuneCountInString(p.StageHighlight) > parentOppMax {
		return fmt.Errorf("agent: parent stage highlight too long")
	}
	if len(p.Advice) != 3 {
		return fmt.Errorf("agent: parent stage prose needs exactly 3 advice items, got %d", len(p.Advice))
	}
	for _, ad := range p.Advice {
		if strings.TrimSpace(ad.Title) == "" || strings.TrimSpace(ad.Text) == "" {
			return fmt.Errorf("agent: parent stage advice item empty")
		}
		if utf8.RuneCountInString(ad.Title) > parentAdviceMax || utf8.RuneCountInString(ad.Text) > parentAdviceMax {
			return fmt.Errorf("agent: parent stage advice too long")
		}
	}
	texts := []string{p.WarmLine, p.StageGrowth, p.StageHighlight, p.StageForward}
	for _, ad := range p.Advice {
		texts = append(texts, ad.Title, ad.Text)
	}
	for _, t := range texts {
		if parentStageBareCode.MatchString(t) {
			return fmt.Errorf("agent: parent stage prose contains an internal code/term")
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the FULL agent package**

Run (allow ≥600s): `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/compose_parent_stage.go apps/api/internal/agent/compose_parent_stage_test.go
git commit -m "feat(e2): stage-report parent composer + validation"
```

---

### Task 2: Contract — `packages/contracts/src/parentReport.ts`

**Files:**
- Modify: `packages/contracts/src/parentReport.ts` (append; do not touch E1's shapes)
- Modify: `packages/contracts/src/index.ts` (already `export * from "./parentReport"` — verify; no change needed if so)
- Test: `packages/contracts/src/parentReport.test.ts` (append cases; create file only if absent)

**Interfaces:**
- Produces: `ParentStageProse`, `ParentStageStat`, `ParentStageReport` Zod schemas + inferred types.

- [ ] **Step 1: Write failing test** — append to `parentReport.test.ts` (mirror the existing E1 test file's style; if no test file exists, create it with these cases):

```ts
import { describe, it, expect } from "vitest";
import { ParentStageReport, ParentStageProse } from "./parentReport";

describe("ParentStageReport", () => {
  const base = {
    cover: { name: "林知远", subject: "第 30 周（7.20–7.26）", klass: "IBDP 一年级 · 研究组", typeLabel: "阶段报告", dateStr: "2026年7月25日", warmLine: "" },
    stats: [
      { value: "6 天", label: "本周活跃" },
      { value: "78", label: "对话轮次" },
      { value: "3 份", label: "生成报告" },
      { value: "5 节", label: "完成课程" },
    ],
    stageGrowth: "", stageHighlight: "", stageForward: "",
    advice: [],
    prose: null as null | "present",
  };

  it("accepts a deterministic (pre-prose) stage report", () => {
    expect(ParentStageReport.parse(base)).toBeTruthy();
  });

  it("accepts a composed stage report", () => {
    const composed = { ...base, stageGrowth: "有变化", stageHighlight: "亮点", stageForward: "往前看",
      advice: [{ title: "t", text: "x" }], prose: "present" as const };
    expect(ParentStageReport.parse(composed)).toBeTruthy();
  });

  it("rejects a non-sentinel prose value", () => {
    expect(() => ParentStageReport.parse({ ...base, prose: "yes" })).toThrow();
  });

  it("ParentStageProse round-trips the composer bundle", () => {
    expect(ParentStageProse.parse({
      warmLine: "w", stageGrowth: "g", stageHighlight: "", stageForward: "f",
      advice: [{ title: "t", text: "x" }, { title: "t2", text: "y" }, { title: "t3", text: "z" }],
    })).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run it, expect FAIL** — `cd packages/contracts && npx vitest run src/parentReport.test.ts` (undefined exports).

- [ ] **Step 3: Append to `parentReport.ts`** (below E1's `ParentReport` export; reuse the file's existing `ParentAdvice`):

```ts
// ── E2: stage mode ──────────────────────────────────────────────────────────
// The stage report is a per-(student, week) parent projection: usage stats +
// a cross-session growth narrative. No per-dim D/A rows, no A number (RL-5),
// no ability level number.

export const ParentStageProse = z.object({
  warmLine: z.string(),
  stageGrowth: z.string(),
  stageHighlight: z.string(), // may be "" (敢于空白)
  stageForward: z.string(),
  advice: z.array(ParentAdvice), // exactly 3 when composed
});
export type ParentStageProse = z.infer<typeof ParentStageProse>;

export const ParentStageStat = z.object({
  value: z.string(), // "6 天" / "78" / "3 份" / "5 节" (unit folded in server-side)
  label: z.string(), // 本周活跃 / 对话轮次 / 生成报告 / 完成课程
});
export type ParentStageStat = z.infer<typeof ParentStageStat>;

export const ParentStageReport = z.object({
  cover: z.object({
    name: z.string(),
    subject: z.string(), // 第 N 周（M.D–M.D）
    klass: z.string(),
    typeLabel: z.string(), // 阶段报告
    dateStr: z.string(),
    warmLine: z.string(),
  }),
  stats: z.array(ParentStageStat), // 4
  stageGrowth: z.string(),
  stageHighlight: z.string(), // "" ⇒ section hidden
  stageForward: z.string(),
  advice: z.array(ParentAdvice), // 3, or [] pre-prose
  prose: z.union([z.literal("present"), z.null()]),
});
export type ParentStageReport = z.infer<typeof ParentStageReport>;
```

- [ ] **Step 4: Verify export** — confirm `packages/contracts/src/index.ts` re-exports this module (E1 added `export * from "./parentReport"`). No change if present.

- [ ] **Step 5: Run contracts suite + tsc**

Run: `cd packages/contracts && npm test && npx tsc --noEmit`
Expected: PASS, no type errors.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/parentReport.ts packages/contracts/src/parentReport.test.ts
git commit -m "feat(e2): ParentStageReport + ParentStageProse contract"
```

---

### Task 3: sqlc queries + GET stage handler + DTO + route

**Files:**
- Modify: `apps/api/internal/store/queries/teacher.sql` (append two queries)
- Regenerate: `apps/api/internal/store/sqlc/*` (via `make sqlc`)
- Create: `apps/api/internal/api/parent_stage_report.go`
- Create: `apps/api/internal/api/parent_stage_report_test.go`
- Modify: `apps/api/internal/api/api.go` (register GET route)

**Interfaces:**
- Consumes: `teacher.WeekWindow`, `teacher.WeekLabel`; `a.authTeacherStudent`;
  `a.d.Queries.{GetStudentWeekStats, ListStudentEvaluationsForTeacher, GetUserByID, GetClassByID, GetParentReportProse}`;
  E1's `ParentCoverDTO`, `ParentAdviceDTO`, `parentDateStr` (same package).
- Produces (T4 relies on): `type ParentStageReportDTO`, `func (a *API) loadParentStage(...)`,
  `func (a *API) getParentStageProse(...)`, `func resolveWeekStart(...)`,
  `func parentStageDTO(...)`, `func buildStageStats(...)`.

- [ ] **Step 1: Append the two sqlc queries to `teacher.sql`**

```sql
-- name: GetStudentWeekStats :one
-- One student's four stage-card counts for a half-open window. Same口径 as
-- GetClassWeekStats: active days bucketed via AT TIME ZONE 'UTC'; turns =
-- prompt_sent + course_message; reports = student_evaluation rows; course_steps
-- = DISTINCT (course, ordinal) step_viewed. Tenancy is the handler's
-- (authTeacherStudent has proven this student is in the teacher's class).
SELECT
  COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date)::int AS active_days,
  COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(*) FROM student_evaluation se
     WHERE se.user_id = @user_id
       AND se.created_at >= @week_start AND se.created_at < @week_end)::int AS reports,
  COUNT(DISTINCT (ev.course_id, ev.payload->>'ordinal'))
    FILTER (WHERE ev.type = 'step_viewed')::int AS course_steps
FROM event ev
WHERE ev.user_id = @user_id
  AND ev.created_at >= @week_start AND ev.created_at < @week_end;

-- name: ListStudentEvaluationsForTeacher :many
-- Every report scores payload one class member owns, across all scopes, oldest
-- first (ability.Aggregate re-sorts defensively anyway). Teacher variant of
-- ListEvaluationsByUser: owner filter replaced by the handler's class-membership
-- proof. Feeds the cross-session 能力素养 merge behind the stage growth prose.
SELECT se.scores, se.created_at
FROM student_evaluation se
WHERE se.user_id = @user_id
ORDER BY se.created_at;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: new `GetStudentWeekStats`, `ListStudentEvaluationsForTeacher` in `internal/store/sqlc/teacher.sql.go`. (Row fields: `ActiveDays, Turns, Reports, CourseSteps int32`.)

- [ ] **Step 3: Write the GET handler file `parent_stage_report.go`**

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// ParentStageStatDTO is one of the four usage cards (value已含单位: "6 天").
type ParentStageStatDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ParentStageReportDTO is the whole stage page. Stats/cover(week label) are
// deterministic; growth/highlight/forward/warmLine/advice come from stored
// prose (nil until composed).
type ParentStageReportDTO struct {
	Cover          ParentCoverDTO       `json:"cover"`
	Stats          []ParentStageStatDTO `json:"stats"`
	StageGrowth    string               `json:"stageGrowth"`
	StageHighlight string               `json:"stageHighlight"`
	StageForward   string               `json:"stageForward"`
	Advice         []ParentAdviceDTO    `json:"advice"`
	Prose          *string              `json:"prose"`
}

// parentStageData is everything both handlers need: the live stats + identity +
// the resolved week. GET renders it; POST composes prose over it.
type parentStageData struct {
	Name        string
	Klass       string
	WeekStart   time.Time
	WeekLabel   string
	ScopeID     string // week_start "2006-01-02" — the storage/scope key
	StudentID   uuid.UUID
	ActiveDays  int
	Turns       int
	Reports     int
	CourseSteps int
}

// resolveWeekStart maps the {weekStart} path value to a Monday-00:00-UTC start.
// "" or "current" → the week enclosing now; else strict YYYY-MM-DD (UTC).
func resolveWeekStart(weekStart string, now time.Time) (time.Time, error) {
	if weekStart == "" || weekStart == "current" {
		start, _ := teacher.WeekWindow(now)
		return start, nil
	}
	d, err := time.ParseInLocation("2006-01-02", weekStart, time.UTC)
	if err != nil {
		return time.Time{}, httpx.ErrNotFound("资源不存在")
	}
	return d, nil
}

// loadParentStage runs the deterministic stage layer for one (student, week).
// No model call.
func (a *API) loadParentStage(ctx context.Context, classID, userID uuid.UUID, start time.Time) (parentStageData, error) {
	end := start.AddDate(0, 0, 7)
	stats, err := a.d.Queries.GetStudentWeekStats(ctx, sqlc.GetStudentWeekStatsParams{
		UserID: userID, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		return parentStageData{}, err
	}
	user, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return parentStageData{}, err
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return parentStageData{}, err
	}
	return parentStageData{
		Name: user.DisplayName, Klass: cls.Name,
		WeekStart: start, WeekLabel: teacher.WeekLabel(start), ScopeID: start.Format("2006-01-02"),
		StudentID: userID,
		ActiveDays: int(stats.ActiveDays), Turns: int(stats.Turns),
		Reports: int(stats.Reports), CourseSteps: int(stats.CourseSteps),
	}, nil
}

// buildStageStats renders the four cards' value strings server-side (D1 anti-drift).
func buildStageStats(d parentStageData) []ParentStageStatDTO {
	return []ParentStageStatDTO{
		{Value: strconv.Itoa(d.ActiveDays) + " 天", Label: "本周活跃"},
		{Value: strconv.Itoa(d.Turns), Label: "对话轮次"},
		{Value: strconv.Itoa(d.Reports) + " 份", Label: "生成报告"},
		{Value: strconv.Itoa(d.CourseSteps) + " 节", Label: "完成课程"},
	}
}

// parentStageDTO merges the deterministic stage layer with whatever prose exists.
func parentStageDTO(d parentStageData, prose *agent.ParentStageProse) ParentStageReportDTO {
	warm := ""
	if prose != nil {
		warm = prose.WarmLine
	}
	dto := ParentStageReportDTO{
		Cover: ParentCoverDTO{
			Name: d.Name, Subject: d.WeekLabel, Klass: d.Klass,
			TypeLabel: "阶段报告", DateStr: parentDateStr(time.Now()), WarmLine: warm,
		},
		Stats: buildStageStats(d),
	}
	if prose != nil {
		dto.StageGrowth, dto.StageHighlight, dto.StageForward = prose.StageGrowth, prose.StageHighlight, prose.StageForward
		for _, ad := range prose.Advice {
			dto.Advice = append(dto.Advice, ParentAdviceDTO{Title: ad.Title, Text: ad.Text})
		}
		present := "present"
		dto.Prose = &present
	}
	return dto
}

// getParentStageProse reads and decodes the stored stage bundle, or pgx.ErrNoRows.
func (a *API) getParentStageProse(ctx context.Context, userID uuid.UUID, scopeID string) (*agent.ParentStageProse, error) {
	raw, err := a.d.Queries.GetParentReportProse(ctx, sqlc.GetParentReportProseParams{
		StudentUserID: userID, Surface: "stage", ScopeID: scopeID,
	})
	if err != nil {
		return nil, err
	}
	var p agent.ParentStageProse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// getParentStageReport handles GET .../parent-stage-report/{weekStart}. Computes
// usage stats live, merges stored prose when present. NEVER calls a model.
func (a *API) getParentStageReport(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	start, err := resolveWeekStart(r.PathValue("weekStart"), time.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, err := a.loadParentStage(r.Context(), classID, userID, start)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.getParentStageProse(r.Context(), userID, data.ScopeID)
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}

// (POST handler added in Task 4 — same file.)
var _ = pgtype.UUID{} // POST (T4) uses pgtype; keep the import stable across tasks.
```

> **Implementer note:** drop the `var _ = pgtype.UUID{}` placeholder line once
> Task 4 uses pgtype (it exists only so this GET-only file compiles before T4
> adds the POST handler that consumes pgtype). If your Go build complains that
> `pgtype` is imported-and-unused in T3 alone, keep the placeholder until T4.

- [ ] **Step 4: Register the GET route in `api.go`** (immediately after E1's two parent-report routes, ~line 135):

```go
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/parent-stage-report/{weekStart}", teacherOrAdmin(a.getParentStageReport))
```

- [ ] **Step 5: Write `parent_stage_report_test.go`** (mirrors `parent_report_test.go` helpers)

```go
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

type parentStageForTest struct {
	Cover struct {
		Name, Subject, Klass, TypeLabel, DateStr, WarmLine string
	} `json:"cover"`
	Stats []struct {
		Value, Label string
	} `json:"stats"`
	StageGrowth, StageHighlight, StageForward string
	Advice                                    []struct{ Title, Text string } `json:"advice"`
	Prose                                     *string                        `json:"prose"`
}

// seedStageStudent creates a teacher-owned class + enrolled student, one project
// report (surfaces in student_evaluation → reports=1 and feeds ability), and
// this-week usage events. Returns the class id, student id, and teacher cookie.
func seedStageStudent(t *testing.T, pool *pgxpool.Pool, h http.Handler, teacherEmail, studentEmail string) (classID, studentID string, teacherCookie *http.Cookie) {
	t.Helper()
	q := mustNewQueries(pool)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, teacherEmail))
	cls := createClassViaAPI(t, h, teacher, "Parent Stage Class")
	student := createStudent(t, pool, SeedSchoolID, studentEmail)
	enrollStudent(t, pool, student, cls)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: student, Qualification: "0457", Title: "嵌入式体育博彩广告与博彩正常化",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L3", Evidence: "e1"},
			{Code: "D2", Name: "证据与信源", Level: "L3", Evidence: "e2"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A3", Name: "边界主权", Level: 3, Opportunity: "given_taken", Evidence: "a3"},
			{Code: "A4", Name: "对抗与检验", Level: 2, Opportunity: "given_not_taken", Evidence: "a4"},
		},
	}
	scores, _ := json.Marshal(report)
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	// This-week usage: 3 prompt_sent (turns=3) + 2 course step_viewed
	// (course_steps=2), all today (active_days=1).
	for i := 0; i < 3; i++ {
		if _, err := q.AppendEvent(context.Background(), sqlc.AppendEventParams{
			ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
			UserID:    student, // event.user_id is a non-null uuid.UUID
			Surface:   "project", Type: "prompt_sent", Payload: []byte(`{}`),
		}); err != nil {
			t.Fatalf("append prompt_sent: %v", err)
		}
	}
	courseID := pgtype.UUID{Bytes: proj.ID, Valid: true} // any uuid; distinctness is by (course,ordinal)
	for _, ord := range []string{"1", "2"} {
		if _, err := q.AppendEvent(context.Background(), sqlc.AppendEventParams{
			UserID:   student,
			CourseID: courseID, Surface: "course", Type: "step_viewed",
			Payload: []byte(fmt.Sprintf(`{"ordinal":%q}`, ord)),
		}); err != nil {
			t.Fatalf("append step_viewed: %v", err)
		}
	}
	return cls, student.String(), teacher
}

func stageURL(classID, studentID, week string) string {
	return fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-stage-report/%s", classID, studentID, week)
}

func TestGetParentStageReport_DeterministicStats(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	classID, studentID, teacher := seedStageStudent(t, pool, h, "ps-teacher@demo.local", "ps-student@demo.local")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", stageURL(classID, studentID, "current"), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentStageForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if dto.Prose != nil {
		t.Errorf("prose should be nil before composition, got %v", *dto.Prose)
	}
	if dto.Cover.TypeLabel != "阶段报告" {
		t.Errorf("typeLabel = %q", dto.Cover.TypeLabel)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("want 4 stat cards, got %d", len(dto.Stats))
	}
	want := map[string]string{"本周活跃": "1 天", "对话轮次": "3", "生成报告": "1 份", "完成课程": "2 节"}
	for _, s := range dto.Stats {
		if w, ok := want[s.Label]; !ok || s.Value != w {
			t.Errorf("stat %q = %q, want %q", s.Label, s.Value, w)
		}
	}
	// Week label shape: 第 N 周（M.D–M.D）
	if len(dto.Cover.Subject) == 0 || dto.Cover.Subject[0:3] != "第 " {
		t.Errorf("cover.subject not a week label: %q", dto.Cover.Subject)
	}
}

// TestGetParentStageReport_ForeignTeacher404 — a teacher who does not own the
// student's class gets 404 (existence-hidden).
func TestGetParentStageReport_ForeignTeacher404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	classID, studentID, _ := seedStageStudent(t, pool, h, "ps-owner@demo.local", "ps-student2@demo.local")
	outsider := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ps-outsider@demo.local"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", stageURL(classID, studentID, "current"), nil), outsider))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign teacher got %d, want 404: body=%s", rec.Code, rec.Body.String())
	}
	_ = time.Now
}
```

> **Implementer note:** confirm `sqlc.AppendEventParams` field names after `make
> sqlc` (they derive from `event.sql`'s `AppendEvent`). If a field is a
> `pgtype.UUID` that must stay NULL (e.g. `SessionID`/`ThreadID`), leave it zero
> (`Valid:false`). Adjust the struct literal to the generated names — do not
> invent fields.

- [ ] **Step 6: Run the FULL api package** (allow ≥600s, foreground)

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS (incl. the two new stage tests + all E1 tests unchanged).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/store/queries/teacher.sql apps/api/internal/store/sqlc apps/api/internal/api/parent_stage_report.go apps/api/internal/api/parent_stage_report_test.go apps/api/internal/api/api.go
git commit -m "feat(e2): GetStudentWeekStats + stage GET handler, DTO, route"
```

---

### Task 4: POST stage compose handler + metering + route

**Files:**
- Modify: `apps/api/internal/api/parent_stage_report.go` (add POST handler + compose + `toAbilitySummary`)
- Modify: `apps/api/internal/api/parent_stage_report_test.go` (add compose tests)
- Modify: `apps/api/internal/api/api.go` (register POST route)

**Interfaces:**
- Consumes: `agent.ComposeParentStage`, `agent.ParentStageFacts`, `agent.AbilitySummary`;
  `ability.Aggregate`, `ability.Sample`; `a.d.EvalResolver`, `a.d.Provider`,
  `gateway.EstimateCost`, `gateway.CostNumeric`, `a.d.Queries.{ListStudentEvaluationsForTeacher, RecordLLMCall, InsertParentReportProse}`;
  `UserFromContext`. Mirrors E1's `composeParentProse` exactly, with ProjectID NULL.

- [ ] **Step 1: Add compose tests** to `parent_stage_report_test.go`

```go
// stageProseStubReply is a valid agent.ParentStageProse JSON — no bare codes,
// no A numbers, no level codes; exactly 3 advice items; highlight non-empty.
const stageProseStubReply = `{
  "warmLine":"这一阶段，孩子在自己拿主意上表现突出。",
  "stageGrowth":"这段时间他更愿意先自己想清楚，再请 AI 帮忙检查，而不是一上来就要答案。",
  "stageHighlight":"本周他主动请 AI 扮演反方，来挑自己论证里的问题。",
  "stageForward":"可以给他更高一点的目标，鼓励他把研究的意义讲得更具体。",
  "advice":[
    {"title":"请他讲给你听","text":"让他用一句话说清这份研究不能说明什么。"},
    {"title":"保护他的自主","text":"鼓励他先自己判断，再去问 AI。"},
    {"title":"给一点挑战","text":"问他如果要再进一步，还差哪一步。"}
  ]
}`

func TestPostParentStageProse_ComposesOnceThenCostFree(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(stageProseStubReply)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, teacher := seedStageStudent(t, pool, h, "pss-teacher@demo.local", "pss-student@demo.local")
	url := stageURL(classID, studentID, "current") + "/prose"

	post := func() parentStageForTest {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var dto parentStageForTest
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v — body=%s", err, rec.Body)
		}
		return dto
	}

	first := post()
	if first.Prose == nil {
		t.Fatalf("first POST should compose: %+v", first)
	}
	if len(first.Advice) != 3 || first.StageGrowth == "" {
		t.Errorf("composed prose incomplete: %+v", first)
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("want 1 parent_report llm_call, got %d", n)
	}
	second := post()
	if second.Prose == nil {
		t.Fatal("second POST should return stored prose")
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("second POST must not spend; llm_calls=%d", n)
	}
}

func TestPostParentStageProse_RejectionNeverWalls(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(`not json`)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, teacher := seedStageStudent(t, pool, h, "pssf-teacher@demo.local", "pssf-student@demo.local")
	url := stageURL(classID, studentID, "current") + "/prose"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("rejected compose must never wall: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentStageForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Prose != nil {
		t.Errorf("prose must be nil after rejection, got %v", *dto.Prose)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("deterministic stats must still render: %d", len(dto.Stats))
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("cost recorded even on rejection; llm_calls=%d, want 1", n)
	}
}
```

- [ ] **Step 2: Run, expect FAIL** (undefined POST route → 404/405).

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestPostParentStage`
Expected: FAIL.

- [ ] **Step 3: Add the POST handler + helpers to `parent_stage_report.go`**

Add imports `log/slog`, `mindimprint/api/internal/ability`, `mindimprint/api/internal/gateway`; remove the `var _ = pgtype.UUID{}` placeholder (pgtype now used).

```go
// toAbilitySummary flattens the cross-session ability model into the composer's
// import-cycle-safe reference struct.
func toAbilitySummary(m ability.Model) agent.AbilitySummary {
	s := agent.AbilitySummary{
		TotalSessions:       m.TotalSessions,
		BoundarySettings:    m.Autonomy.BoundarySettings,
		AdversaryInvites:    m.Autonomy.AdversaryInvites,
		OpportunitiesTaken:  m.Autonomy.OpportunitiesTaken,
		OpportunitiesMissed: m.Autonomy.OpportunitiesMissed,
	}
	for _, d := range m.Depth {
		s.Depth = append(s.Depth, agent.AbilityDepthFact{Name: d.Name, LevelLabel: d.LevelLabel, EvidenceCount: d.EvidenceCount})
	}
	return s
}

// studentAbility fetches the student's whole cross-session report history
// (teacher-scoped) and merges it. A malformed row must not sink the aggregate.
func (a *API) studentAbility(ctx context.Context, userID uuid.UUID) (agent.AbilitySummary, error) {
	rows, err := a.d.Queries.ListStudentEvaluationsForTeacher(ctx, userID)
	if err != nil {
		return agent.AbilitySummary{}, err
	}
	samples := make([]ability.Sample, 0, len(rows))
	for _, row := range rows {
		var rep agent.Report
		if json.Unmarshal(row.Scores, &rep) != nil {
			continue
		}
		samples = append(samples, ability.Sample{Report: rep, CreatedAt: row.CreatedAt})
	}
	return toAbilitySummary(ability.Aggregate(samples)), nil
}

// postParentStageProse handles POST .../parent-stage-report/{weekStart}/prose —
// the ONLY stage endpoint that spends. Compose-once (first-open-wins); a failed
// composition never walls (the deterministic stats still render).
func (a *API) postParentStageProse(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	start, err := resolveWeekStart(r.PathValue("weekStart"), time.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	data, err := a.loadParentStage(ctx, classID, userID, start)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Already composed → return it, no spend.
	if existing, gerr := a.getParentStageProse(ctx, userID, data.ScopeID); gerr == nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, existing))
		return
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	prose, spent := a.composeParentStageProse(ctx, r, data)
	if !spent {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
		return
	}
	raw, merr := json.Marshal(prose)
	if merr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
		return
	}
	if ierr := a.d.Queries.InsertParentReportProse(ctx, sqlc.InsertParentReportProseParams{
		StudentUserID: userID, Surface: "stage", ScopeID: data.ScopeID, Prose: raw,
	}); ierr != nil {
		httpx.WriteError(w, r, ierr)
		return
	}
	stored, serr := a.getParentStageProse(ctx, userID, data.ScopeID)
	if serr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, &prose))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, stored))
}

// composeParentStageProse makes the flagship call and records its cost —
// including on rejection. ProjectID is NULL (a stage report is not
// project-scoped). spent=false means "no prose this time", never a client error.
func (a *API) composeParentStageProse(ctx context.Context, r *http.Request, data parentStageData) (agent.ParentStageProse, bool) {
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("parent stage prose: no provider", "err", rerr)
		return agent.ParentStageProse{}, false
	}
	summary, aerr := a.studentAbility(ctx, data.StudentID)
	if aerr != nil {
		slog.Warn("parent stage prose: ability", "err", aerr)
		return agent.ParentStageProse{}, false
	}
	facts := agent.ParentStageFacts{
		Name: data.Name, Subject: data.WeekLabel, Klass: data.Klass,
		ActiveDays: data.ActiveDays, Turns: data.Turns, Reports: data.Reports, CourseSteps: data.CourseSteps,
		Ability: summary,
	}
	prose, usage, cerr := agent.ComposeParentStage(ctx, a.d.Provider, resolved, facts)
	if u, ok := UserFromContext(ctx); ok && resolved.Provider != "" {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("parent stage llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Valid: false}, // stage is not project-scoped
			Surface: "teacher", Purpose: "parent_report",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("parent stage prose: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("parent stage prose: rejected", "err", cerr)
		return agent.ParentStageProse{}, false
	}
	return prose, true
}
```

- [ ] **Step 4: Register the POST route in `api.go`** (right after the GET stage route):

```go
	mux.Handle("POST /api/v1/classes/{id}/students/{userId}/parent-stage-report/{weekStart}/prose", teacherOrAdmin(a.postParentStageProse))
```

- [ ] **Step 5: Run the FULL api package** (allow ≥600s, foreground)

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS (both compose tests + Task 3 tests + all E1 tests).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/parent_stage_report.go apps/api/internal/api/parent_stage_report_test.go apps/api/internal/api/api.go
git commit -m "feat(e2): stage prose POST compose, meter (ProjectID NULL), first-open-wins"
```

---

### Task 5: Web — extract shared chrome, refactor `ParentReport.tsx`

**Files:**
- Create: `apps/web/src/console/ParentReportChrome.tsx`
- Modify: `apps/web/src/console/ParentReport.tsx` (use the chrome; render project middle as children)
- Test: `apps/web/test/console/ParentReport.test.tsx` (existing E1 test must stay green; no assertion weakening)

**Goal:** Factor the mode-independent presentation (control bar + cover + 这份报告
怎么读 + 两条轴 + 判断怎么来 + 生成 button + 下一步/advice + supply + 名词解释 +
footer) into `ParentReportChrome`, so both project (E1) and stage (T6) render only
their middle section. **All rendered text stays identical** — E1's test queries
by text and must still pass.

**Interfaces:**
- Produces: `ParentReportChrome` React component:
  ```ts
  ParentReportChrome({
    studentName?: string;
    cover: { name; subject; klass; typeLabel; dateStr; warmLine };
    advice: { title: string; text: string }[];
    proseAbsent: boolean;         // renders 生成 button when true
    generating: boolean;
    generateLabel: string;        // "生成家长版正文"
    onGenerate: () => void;
    onClose: () => void;
    children: React.ReactNode;    // the mode-specific middle
  })
  ```

- [ ] **Step 1: Create `ParentReportChrome.tsx`** — move E1's shared JSX here verbatim.

Extract from the current `ParentReport.tsx`: the outer overlay `<div>` + `<style>`,
the control bar (title uses `studentName`), the `{error}` slot **stays in the
caller** (data-specific), the cover block (from `data.cover`), 这份报告怎么读,
两条轴 (both cards), 判断是怎么得出来的, then `{children}`, then the 生成 button
(`proseAbsent` gate + `generateLabel`), then 下一步 (advice from prop) + 平台接下来
会做 (SUPPLY), 名词解释 (GLOSSARY), footer. Keep `CARD`/`H2`/`DBadge`/`AState`
where each is used (DBadge/AState stay in `ParentReport.tsx` — they're
project-only). Import the content constants the chrome uses
(`PRINCIPLES, D_LEVELS, D_DIMS, A_STATES, A_SIGNALS, HOW_LIST, SUPPLY, GLOSSARY, FOOTER`).

```tsx
import { PRINCIPLES, D_LEVELS, D_DIMS, A_STATES, A_SIGNALS, HOW_LIST, SUPPLY, GLOSSARY, FOOTER } from "./parentReportContent";

const CARD: React.CSSProperties = { border: "1px solid #E4E7EF", borderRadius: 14, padding: "16px 18px" };
const H2: React.CSSProperties = { fontSize: 19, fontWeight: 800, color: "#1C2333", margin: "30px 0 10px" };

export function ParentReportChrome({
  studentName, cover, advice, proseAbsent, generating, generateLabel, onGenerate, onClose, children,
}: {
  studentName?: string;
  cover: { name: string; subject: string; klass: string; typeLabel: string; dateStr: string; warmLine: string };
  advice: { title: string; text: string }[];
  proseAbsent: boolean;
  generating: boolean;
  generateLabel: string;
  onGenerate: () => void;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <>
      {/* control bar */}
      <div className="no-print" style={{ position: "sticky", top: 0, zIndex: 10, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, background: "#fff", borderBottom: "1px solid #E4E7EF", padding: "12px 24px" }}>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>家长版报告{studentName ? ` · ${studentName}` : ""}</div>
        <div style={{ display: "flex", gap: 10 }}>
          <button onClick={() => window.print()} style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#2A3B7A", color: "#fff", fontSize: 13, fontWeight: 700, padding: "9px 16px", borderRadius: 10, border: "none", cursor: "pointer", fontFamily: "inherit" }}>下载 PDF</button>
          <button onClick={onClose} style={{ background: "#F1F2F6", color: "#4A5060", fontSize: 13, fontWeight: 700, padding: "9px 16px", borderRadius: 10, border: "none", cursor: "pointer", fontFamily: "inherit" }}>关闭</button>
        </div>
      </div>

      <div style={{ maxWidth: 780, margin: "0 auto", padding: "26px 30px 80px" }}>
        {/* cover */}
        <div style={{ border: "1px solid #E4E7EF", borderRadius: 16, padding: "26px 28px", background: "linear-gradient(180deg,#F7F8FC 0%,#FFFFFF 70%)" }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#2A3B7A", letterSpacing: ".02em" }}>思维印记 · 学生能力成长报告</div>
          <div style={{ marginTop: 20, fontSize: 30, fontWeight: 900, color: "#1C2333", lineHeight: 1.15 }}>{cover.name} 的能力成长报告</div>
          {cover.warmLine ? <div style={{ marginTop: 10, fontSize: 14, color: "#5A6377", lineHeight: 1.7 }}>{cover.warmLine}</div> : null}
          <div style={{ marginTop: 18, display: "flex", flexWrap: "wrap", gap: 8 }}>
            <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, background: "#EDEFF9", color: "#2A3B7A", fontSize: 12.5, fontWeight: 700 }}>{cover.typeLabel}</span>
            <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>{cover.subject}</span>
            <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>{cover.klass}</span>
            <span style={{ display: "inline-flex", alignItems: "center", padding: "5px 12px", borderRadius: 20, border: "1px solid #E4E7EF", color: "#5A6377", fontSize: 12.5 }}>生成日期 {cover.dateStr}</span>
          </div>
        </div>

        {/* 这份报告怎么读 */}
        <h2 style={H2}>这份报告怎么读</h2>
        <p style={{ fontSize: 15, lineHeight: 1.9, color: "#3A4256", margin: "0 0 14px" }}>思维印记陪伴孩子做研究和思考，记录的是过程，不是一张成绩单。在看具体内容之前，有四条我们始终遵守的原则，也希望和您达成共识。</p>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          {PRINCIPLES.map((p) => (
            <div key={p.title} style={CARD}>
              <div style={{ fontSize: 14.5, fontWeight: 800, color: "#2A3B7A" }}>{p.title}</div>
              <div style={{ marginTop: 8, fontSize: 13.5, lineHeight: 1.75, color: "#3A4256" }}>{p.text}</div>
            </div>
          ))}
        </div>

        {/* 两条轴 — copy verbatim from current ParentReport.tsx lines 135–186 */}
        {/* ...D 轴 card (D_LEVELS + D_DIMS) and A 轴 card (A_STATES + A_SIGNALS)... */}

        {/* 判断是怎么得出来的 — copy verbatim (HOW_LIST) */}

        {children}

        {proseAbsent && (
          <div className="no-print" style={{ marginTop: 16 }}>
            <button onClick={onGenerate} disabled={generating} style={{ background: "#2A3B7A", color: "#fff", fontSize: 13.5, fontWeight: 700, padding: "11px 18px", borderRadius: 11, border: "none", cursor: generating ? "default" : "pointer", opacity: generating ? 0.7 : 1, fontFamily: "inherit" }}>
              {generating ? "生成中…" : generateLabel}
            </button>
          </div>
        )}

        {/* 下一步 (advice prop) + 平台接下来会做 (SUPPLY) + 名词解释 (GLOSSARY) + footer
            — copy verbatim from current ParentReport.tsx lines 257–296, using the
            `advice` prop instead of data.advice. */}
      </div>
    </>
  );
}
```

> **Implementer note:** the elided blocks (`两条轴`, `判断怎么来`, `下一步`/supply/
> glossary/footer) are moved **verbatim** from the current `ParentReport.tsx` — do
> not paraphrase; the E1 test asserts this exact text. Reference the file's lines
> 135–199 and 257–296. `A_STATES`/`D_LEVELS` labels keep the trailing `：`
> disambiguation exactly as in E1.

- [ ] **Step 2: Refactor `ParentReport.tsx`** to wrap its project middle in the chrome:

```tsx
// keep: imports of ParentReport DTO, api, ApiError, DBadge/AState helpers,
// D_BADGE_COLOR/A_STATE_COLOR (project-only). Add: import { ParentReportChrome }.
// The outer overlay <div> + <style> + {error} stay here; chrome renders inside.

return (
  <div style={{ position: "fixed", inset: 0, zIndex: 100, background: "#F3F4F8", overflowY: "auto" }}>
    <style>{"@media print { .no-print { display:none !important; } }"}</style>
    {error && <div style={{ margin: "20px 24px", color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error}</div>}
    {data && (
      <ParentReportChrome
        studentName={studentName}
        cover={data.cover}
        advice={data.advice}
        proseAbsent={data.prose === null}
        generating={generating}
        generateLabel="生成家长版正文"
        onGenerate={handleGenerate}
        onClose={onClose}
      >
        {/* {name} 这次的表现 — the project middle: glance + D rows + A rows + opportunity.
           Moved verbatim from current lines 201–243. */}
      </ParentReportChrome>
    )}
  </div>
);
```

- [ ] **Step 3: Run the web suite + tsc**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS — the existing `ParentReport.test.tsx` still green (text unchanged).

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/console/ParentReportChrome.tsx apps/web/src/console/ParentReport.tsx
git commit -m "refactor(e2): extract ParentReportChrome shared shell (E1 output unchanged)"
```

---

### Task 6: Web — `ParentStageReport.tsx` + API methods + wire stub

**Files:**
- Create: `apps/web/src/console/ParentStageReport.tsx`
- Create: `apps/web/test/console/ParentStageReport.test.tsx`
- Modify: `apps/web/src/api/index.ts` + the teacher API module (add two methods)
- Modify: `apps/web/src/console/StudentDetailView.tsx` (wire 阶段报告 stub)

**Interfaces:**
- Consumes: `ParentReportChrome` (T5), `ParentStageReport` contract type (T2),
  `api.getParentStageReport` / `api.generateParentStageProse`,
  `parentReportContent.ts` (via chrome).

- [ ] **Step 1: Add the two API methods** — mirror E1's `getParentReport` /
  `generateParentReportProse` (find them in `src/api/index.ts` + the teacher
  module; match their exact style + the singleton-`api` spy shape):

```ts
// GET .../parent-stage-report/{weekStart} (cost-free)
getParentStageReport(classId: string, studentId: string, weekStart = "current"): Promise<ParentStageReport> {
  return this.get(`/classes/${classId}/students/${studentId}/parent-stage-report/${weekStart}`);
},
// POST .../parent-stage-report/{weekStart}/prose (the only spend)
generateParentStageProse(classId: string, studentId: string, weekStart = "current"): Promise<ParentStageReport> {
  return this.post(`/classes/${classId}/students/${studentId}/parent-stage-report/${weekStart}/prose`, {});
},
```
> Match the actual client's method signatures/verbs (`this.get`/`this.post` or
> whatever E1's methods use). Import the `ParentStageReport` type from
> `@mind-imprint/contracts`.

- [ ] **Step 2: Create `ParentStageReport.tsx`**

```tsx
import { useEffect, useState } from "react";
import type { ParentStageReport as ParentStageReportDTO } from "@mind-imprint/contracts";
import { api, ApiError } from "../api";
import { ParentReportChrome } from "./ParentReportChrome";

const CARD: React.CSSProperties = { border: "1px solid #E4E7EF", borderRadius: 14, padding: "16px 18px" };
const H2: React.CSSProperties = { fontSize: 19, fontWeight: 800, color: "#1C2333", margin: "30px 0 10px" };

export function ParentStageReport({
  classId, studentId, studentName, onClose,
}: {
  classId: string;
  studentId: string;
  studentName?: string;
  onClose: () => void;
}) {
  const [data, setData] = useState<ParentStageReportDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);

  useEffect(() => {
    setError(null);
    api.getParentStageReport(classId, studentId).then(setData).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }, [classId, studentId]);

  async function handleGenerate() {
    setGenerating(true);
    try {
      setData(await api.generateParentStageProse(classId, studentId));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "生成失败");
    } finally {
      setGenerating(false);
    }
  }

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 100, background: "#F3F4F8", overflowY: "auto" }}>
      <style>{"@media print { .no-print { display:none !important; } }"}</style>
      {error && <div style={{ margin: "20px 24px", color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error}</div>}
      {data && (
        <ParentReportChrome
          studentName={studentName}
          cover={data.cover}
          advice={data.advice}
          proseAbsent={data.prose === null}
          generating={generating}
          generateLabel="生成家长版正文"
          onGenerate={handleGenerate}
          onClose={onClose}
        >
          {/* 这一阶段的使用与成长 */}
          <h2 style={H2}>这一阶段的使用与成长</h2>
          <p style={{ fontSize: 14, lineHeight: 1.8, color: "#5A6377", margin: "0 0 14px" }}>这段时间，孩子在思维印记上的使用情况，以及思考维度的变化。</p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 10 }}>
            {data.stats.map((s) => (
              <div key={s.label} style={{ border: "1px solid #E4E7EF", borderRadius: 12, padding: "14px 12px", textAlign: "center" }}>
                <div style={{ fontSize: 24, fontWeight: 900, color: "#2A3B7A" }}>{s.value}</div>
                <div style={{ fontSize: 12, color: "#6C7488", marginTop: 4 }}>{s.label}</div>
              </div>
            ))}
          </div>
          {data.stageGrowth ? (
            <div style={{ ...CARD, marginTop: 16 }}>
              <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>这段时间的变化</div>
              <div style={{ marginTop: 9, fontSize: 14, lineHeight: 1.8, color: "#3A4256" }}>{data.stageGrowth}</div>
            </div>
          ) : null}
          {data.stageHighlight ? (
            <div style={{ marginTop: 12, border: "1px solid #DFEDE7", background: "linear-gradient(180deg,#F3F9F6,#fff)", borderRadius: 14, padding: "16px 18px" }}>
              <div style={{ fontSize: 14, fontWeight: 800, color: "#3E8A6E" }}>本阶段亮点</div>
              <div style={{ marginTop: 8, fontSize: 14, lineHeight: 1.8, color: "#2A3040" }}>{data.stageHighlight}</div>
            </div>
          ) : null}
          {data.stageForward ? (
            <div style={{ marginTop: 12, fontSize: 14, lineHeight: 1.85, color: "#3A4256" }}>
              <b style={{ color: "#20263A" }}>往前看：</b>{data.stageForward}
            </div>
          ) : null}
        </ParentReportChrome>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Write `ParentStageReport.test.tsx`** (mirror `ParentReport.test.tsx`'s api-spy style)

```tsx
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ParentStageReport } from "@/console/ParentStageReport";
import { api } from "@/api";

const deterministic = {
  cover: { name: "林知远", subject: "第 30 周（7.20–7.26）", klass: "IBDP 一年级 · 研究组", typeLabel: "阶段报告", dateStr: "2026年7月25日", warmLine: "" },
  stats: [
    { value: "6 天", label: "本周活跃" }, { value: "78", label: "对话轮次" },
    { value: "3 份", label: "生成报告" }, { value: "5 节", label: "完成课程" },
  ],
  stageGrowth: "", stageHighlight: "", stageForward: "", advice: [], prose: null,
};
const composed = { ...deterministic, warmLineFilled: true,
  cover: { ...deterministic.cover, warmLine: "这一阶段，孩子表现突出。" },
  stageGrowth: "这段时间他更愿意先自己想。", stageHighlight: "本周主动请 AI 当反方。",
  stageForward: "给他更高的目标。", advice: [{ title: "请他讲给你听", text: "让他讲清楚。" }], prose: "present" };

describe("ParentStageReport", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("renders deterministic stats + 生成 button pre-prose", async () => {
    vi.spyOn(api, "getParentStageReport").mockResolvedValue(deterministic as any);
    render(<ParentStageReport classId="c1" studentId="s1" studentName="林知远" onClose={() => {}} />);
    expect(await screen.findByText("6 天")).toBeInTheDocument();
    expect(screen.getByText("本周活跃")).toBeInTheDocument();
    expect(screen.getByText("阶段报告")).toBeInTheDocument();
    expect(screen.getByText("生成家长版正文")).toBeInTheDocument();
  });

  it("composes on click and fills the growth prose", async () => {
    vi.spyOn(api, "getParentStageReport").mockResolvedValue(deterministic as any);
    const gen = vi.spyOn(api, "generateParentStageProse").mockResolvedValue(composed as any);
    render(<ParentStageReport classId="c1" studentId="s1" onClose={() => {}} />);
    fireEvent.click(await screen.findByText("生成家长版正文"));
    await waitFor(() => expect(gen).toHaveBeenCalledWith("c1", "s1"));
    expect(await screen.findByText("这段时间他更愿意先自己想。")).toBeInTheDocument();
    expect(screen.getByText("本周主动请 AI 当反方。")).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Wire the 阶段报告 stub in `StudentDetailView.tsx`**

The inert 导出家长版·阶段报告 button (currently ~line 142) gets an `onClick` that
opens `ParentStageReport` (mirror how 项目报告 opens `ParentReport` via a
`useState` flag). Import `ParentStageReport`; add `const [stageOpen, setStageOpen]
= useState(false)`; set the button `onClick={() => setStageOpen(true)}`; render
`{stageOpen && <ParentStageReport classId={...} studentId={student.userId} studentName={student.displayName} onClose={() => setStageOpen(false)} />}` next to the
existing `<ParentReport .../>`. Use the same classId/studentId props E1's
`<ParentReport>` receives.

> If an existing `StudentDetailView.test.tsx` asserts the 阶段报告 button is inert,
> update it to expect the now-wired behavior (preserve, don't weaken, the
> assertion — as E1's T6 did for the 项目报告 button).

- [ ] **Step 5: Run the web suite + tsc**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS (new stage test + all E1/console tests).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/console/ParentStageReport.tsx apps/web/test/console/ParentStageReport.test.tsx apps/web/src/api/index.ts apps/web/src/console/StudentDetailView.tsx
git commit -m "feat(e2): ParentStageReport view + API methods + wire 阶段报告 stub"
```
> Add the teacher API module path to the `git add` list if the two methods live
> there (e.g. `apps/web/src/api/teacher.ts`) — name it explicitly, never `git add .`.

---

## Whole-branch review + finish

After Task 6, dispatch the whole-branch review (opus) over `main..HEAD`, triage
findings (fix Critical/Important before merge, record Minors), then use
superpowers:finishing-a-development-branch → **direct-merge to main + push**
(program workflow: no PR, fast-forward).

**Acceptance re-check (spec §10):** teacher opens a student → 导出家长版·阶段报告 →
current-week stats + 生成 button → click → one `parent_report` llm_call
(ProjectID NULL) → growth/highlight/forward/advice fill → re-open identical, no
2nd call → prose has no A numbers / bare D-A / L codes / teacher constructs →
thin-usage student shows zero-stats + 起步 register + no 亮点 → 下载 PDF prints
report only → E1 项目模式 unchanged → all three suites green.
```
