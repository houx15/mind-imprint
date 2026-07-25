# Spec E1 · 家长报告（项目模式）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the teacher console's inert 导出家长版·项目报告 stub into a real, printable parent projection of one student's canonical project `DualAxisReport` — deterministic D 四台阶 badges + numberless A 三态, one flagship compose call for gentled prose (stored first-open-wins), browser-print PDF.

**Architecture:** The D2 class-weekly pattern applied per-(student, project): deterministic parts (badges/states/static教育 front-matter) computed live and never model-touched; ONE flagship LLM call gentles the finished canonical object into warm parent prose, validated against internal-register leaks, stored first-open-wins (`ON CONFLICT DO NOTHING`). Two teacher-tenancy-guarded endpoints — GET is cost-free, POST is the only spend and never walls.

**Tech Stack:** Go (`net/http` + `pgx`/`sqlc` + `goose`), Postgres, React + Vite + TypeScript (inline styles), Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-25-e1-parent-report-project-design.md`

## Global Constraints

- 客户端绝不直连模型；所有 LLM 调用走后端网关；密钥只在 `apps/api` 服务端。
- 评估走旗舰**绝不降级**；parent compose 用 `EvalResolver` (flagship) + `llm_call` `Purpose:"parent_report"`, `Surface:"teacher"`; meters **even on rejection**.
- **两轴永不合成总分** (RL-5)；parent 面 **A 轴不出现任何数字**。
- **机会供给先于判定**：平台没给机会 = 平台欠账，不算学生短板；carried in the composed 机会与真实性 paragraph.
- **敢于空白**：a depth dim at `NA`/no-evidence renders 暂无 badge + the literal 「暂无可计入的证据」 (deterministic, shown regardless of prose); the composer never invents a highlight.
- **说人话**：parent prose must not contain internal codes/术语 — output regex rejects `(?i)\b[DA]\s*[1-6]\b`, `given_taken|given_not_taken|not_supplied`, `SOLO`, `P[0-3]`.
- **家长端与教师端隔离**：the projection carries only D badges, A states, gentled readings, opportunity, advice — never promptLens / interactionEvidence / officialProjection / workAndProcess / watch-tags / 👍👎.
- **GET never spends; POST is the only spend**, failure returns `200 + prose:null`, never walls.
- Single-source badges/states server-side (D1 anti-drift): the wire carries the badge/state **label**; colors are client presentation constants.
- Migration number is **0035** (last is `0034_seed_teacher_week.sql`).
- `make sqlc` from `apps/api`; **never** hand-edit `apps/api/internal/store/sqlc/*`.
- Never `git add` a whole directory (pre-existing `M package.json` + untracked docs/PNGs are not ours) — name files explicitly; the one exception is generated `apps/api/internal/store/sqlc`.
- Go tests: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — FULL packages, never `-run` subsets for card/gate/projection/migration/config changes. `internal/api` needs ~210s → any backend test run must allow ≥600s.
- Web: `cd apps/web && npm test` + `npx tsc --noEmit`. Contracts: `cd packages/contracts && npm test` + `npx tsc --noEmit`.
- Route convention is **class-scoped**, mirroring `getStudentReport`:
  `GET /api/v1/classes/{id}/students/{userId}/parent-report/{surface}/{scopeId}` and `POST …/prose`, both behind `teacherOrAdmin`.
- Reuse guards verbatim: `authTeacherStudent(w, r) (classID, userID uuid.UUID, ok bool)` (does `assertTeacherOwnsClass` + enrollment `role_in_class='student'`, all failures existence-hidden 404).

---

## File Structure

**New**
- `apps/api/internal/parent/parent.go` (+ `parent_test.go`) — pure projection: `DBadge(level string) string`, `AState(level int) string`.
- `apps/api/internal/agent/compose_parent.go` (+ `compose_parent_test.go`) — `ParentProse`, `ComposeParent`, `validateParentProse`, `ParentFactsPrompt`.
- `apps/api/internal/store/migrations/0035_parent_report_prose.sql`.
- `apps/api/internal/store/queries/parent.sql` — `GetParentReportProse`, `InsertParentReportProse`.
- `apps/api/internal/api/parent_report.go` (+ `parent_report_test.go`) — DTOs + GET/POST handlers.
- `packages/contracts/src/parentReport.ts` (+ `packages/contracts/test/parentReport.test.ts`).
- `apps/web/src/console/parentReportContent.ts` — the static教育 front-matter constants (verbatim from the binding dc-html).
- `apps/web/src/console/ParentReport.tsx` (+ `apps/web/test/console/ParentReport.test.tsx`).

**Modified**
- `apps/api/internal/api/api.go` — register the two routes.
- `packages/contracts/src/index.ts` — export the new contract.
- `apps/web/src/console/StudentDetailView.tsx` — wire 导出家长版·项目报告 stub.
- `apps/web/src/console/TeacherReportView.tsx` — wire 导出家长版 PDF stub.

**Task dependency order:** 1, 2, 3 are independent. Task 4 depends on 1. Task 5 depends on 2 + 4. Task 6 depends on 3.

---

## Task 1: `internal/parent` — deterministic projection

**Files:**
- Create: `apps/api/internal/parent/parent.go`
- Test: `apps/api/internal/parent/parent_test.go`

**Interfaces:**
- Consumes: nothing (pure).
- Produces:
  - `func DBadge(level string) string` — `"L1"→"起步"`, `"L2"→"发展"`, `"L3"→"熟练"`, `"L4"→"优秀"`, anything else (incl. `"NA"`, `""`) → `"暂无"`.
  - `func AState(level int) string` — `0,1→"暂未观察到"`, `2,3→"偶有·多在引导后"`, `4,5→"观察到主动信号"` (clamped: `<0→"暂未观察到"`, `>5→"观察到主动信号"`).

- [ ] **Step 1: Write the failing test**

```go
package parent

import "testing"

func TestDBadge(t *testing.T) {
	cases := map[string]string{"L1": "起步", "L2": "发展", "L3": "熟练", "L4": "优秀", "NA": "暂无", "": "暂无", "L9": "暂无"}
	for in, want := range cases {
		if got := DBadge(in); got != want {
			t.Errorf("DBadge(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAState(t *testing.T) {
	cases := map[int]string{0: "暂未观察到", 1: "暂未观察到", 2: "偶有·多在引导后", 3: "偶有·多在引导后", 4: "观察到主动信号", 5: "观察到主动信号", -1: "暂未观察到", 7: "观察到主动信号"}
	for in, want := range cases {
		if got := AState(in); got != want {
			t.Errorf("AState(%d)=%q want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/parent/`
Expected: FAIL — `undefined: DBadge` / `undefined: AState`.

- [ ] **Step 3: Write the implementation**

```go
// Package parent holds pure parent-facing projections of the canonical
// assessment object. The parent surface is gentled and numberless: D levels
// become a four-step ladder, A levels a three-state signal read. Never a
// composite score (RL-5); recomputed on every read, never persisted.
package parent

// DBadge maps a canonical depth level (L1–L4, or NA/"") to the parent 四台阶
// label. Unknown/NA/empty → 暂无 (敢于空白): no real level, no reading.
func DBadge(level string) string {
	switch level {
	case "L1":
		return "起步"
	case "L2":
		return "发展"
	case "L3":
		return "熟练"
	case "L4":
		return "优秀"
	default:
		return "暂无"
	}
}

// AState maps an autonomy level (0..5) to the numberless three-state parent
// read. NO number ever reaches the parent surface (design §2「A 轴不出现任何
// 数字」). not_supplied signals arrive here as level 0 → 暂未观察到.
func AState(level int) string {
	switch {
	case level <= 1:
		return "暂未观察到"
	case level <= 3:
		return "偶有·多在引导后"
	default:
		return "观察到主动信号"
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/parent/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/parent/parent.go apps/api/internal/parent/parent_test.go
git commit -m "feat(e1): parent projection package (D 四台阶 + A numberless 三态)"
```

---

## Task 2: `compose_parent.go` — the flagship gentling composer

**Files:**
- Create: `apps/api/internal/agent/compose_parent.go`
- Test: `apps/api/internal/agent/compose_parent_test.go`

**Interfaces:**
- Consumes: `agent.Report` (`.DepthAxis []DepthDim{Code,Name,Level string,Evidence,PromptEvidence}`, `.AutonomyAxis []AutonomySignal{Code,Name,Level int,Opportunity string,Evidence,PromptEvidence}`); `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `gateway.ChatRequest/ChatMessage/RoleSystem/RoleUser` (see `compose_weekly.go` for the exact call shape).
- Produces:
  - `type ParentProse struct` with json tags `glance,dOverview,aOverview,opportunity,warmLine,dReadings(map[string]string),aReadings(map[string]string),advice([]ParentAdvice)`.
  - `type ParentAdvice struct { Title string \`json:"title"\`; Text string \`json:"text"\` }`.
  - `func ComposeParent(ctx, prov gateway.Provider, r gateway.Resolved, rep Report, name, subject string) (ParentProse, gateway.ChatUsage, error)` — returns usage even on rejection (caller records spend either way).
  - `func ParentFactsPrompt(rep Report, name, subject string) string` (exported for a test to assert what it carries).

- [ ] **Step 1: Write the failing test**

```go
package agent

import "testing"

func sampleParentReport() Report {
	return Report{
		DepthAxis: []DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L4", Evidence: "把宽泛影响收窄为可研究关系"},
			{Code: "D2", Name: "证据与信源", Level: "L3", Evidence: "比较多篇文献推出 gap"},
			{Code: "D3", Name: "论证结构", Level: "L3", Evidence: "gap-method-evidence 链条稳定"},
			{Code: "D4", Name: "视角与偏见", Level: "L3", Evidence: "识别自陈与便利样本限制"},
			{Code: "D5", Name: "反馈处理与修订", Level: "L3", Evidence: "按反馈重排图表收窄结论"},
			{Code: "D6", Name: "反思与元认知", Level: "NA", Evidence: ""},
		},
		AutonomyAxis: []AutonomySignal{
			{Code: "A1", Name: "方向自主", Level: 4, Opportunity: "given_taken", Evidence: "主动改题"},
			{Code: "A2", Name: "发起自主", Level: 4, Opportunity: "given_taken", Evidence: "自发补证据"},
			{Code: "A3", Name: "边界主权", Level: 4, Opportunity: "given_taken", Evidence: "拒绝代写"},
			{Code: "A4", Name: "对抗与检验", Level: 3, Opportunity: "given_not_taken", Evidence: "部分引导后强化"},
			{Code: "A5", Name: "判断署名", Level: 5, Opportunity: "given_taken", Evidence: "自评 Paper 4"},
			{Code: "A6", Name: "求真优先", Level: 4, Opportunity: "given_taken", Evidence: "因证据收窄结论"},
		},
	}
}

func TestParentFactsPromptCarriesEvidenceNotLensOrOfficial(t *testing.T) {
	rep := sampleParentReport()
	rep.PromptLens.Note = "透镜秘密"
	p := ParentFactsPrompt(rep, "林知远", "嵌入式广告与正常化")
	if !contains(p, "主动改题") || !contains(p, "任务理解与问题表述") {
		t.Fatal("facts prompt must carry the canonical evidence + dim names")
	}
	if contains(p, "透镜秘密") {
		t.Fatal("facts prompt must NOT carry prompt-lens / teacher-only material")
	}
	// D6 is NA → not offered to the composer for a reading.
	if contains(p, "D6") && contains(p, "反思与元认知：") {
		t.Fatal("NA dim must not be sent for a reading")
	}
}

func validParentProse() ParentProse {
	return ParentProse{
		Glance: "方法对齐、证据充分，自主性强", DOverview: "多数维度稳定在熟练", AOverview: "六个方面都观察到主动信号",
		Opportunity: "这次机会多由孩子自己创造，边界守得好，真实性高。", WarmLine: "把 AI 当审稿人而不是代笔。",
		DReadings: map[string]string{"D1": "能把宽泛话题收窄成可研究的问题。", "D2": "会比较资料、说清每份能支持什么。", "D3": "用证据一步步推出结论。", "D4": "会承认研究的局限与偏差。", "D5": "收到质疑真的改进论证。"},
		AReadings: map[string]string{"A1": "自己决定方向。", "A2": "主动查证补证据。", "A3": "用 AI 时设了边界。", "A4": "主动请人挑刺。", "A5": "为自己的结论负责。", "A6": "证据不足时愿意把结论改小。"},
		Advice:    []ParentAdvice{{Title: "请他讲给你听", Text: "用一句话说清这份研究不能说明什么。"}},
	}
}

func TestValidateParentProse(t *testing.T) {
	rep := sampleParentReport()

	if err := validateParentProse(validParentProse(), rep); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}

	// Missing a wanted (real-level) D reading.
	p := validParentProse()
	delete(p.DReadings, "D3")
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for missing D3 reading")
	}

	// A reading for NA dim D6 is not wanted → unknown key rejected.
	p = validParentProse()
	p.DReadings["D6"] = "不该有的反思读数"
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for NA-dim reading D6")
	}

	// Missing an A reading (all six wanted).
	p = validParentProse()
	delete(p.AReadings, "A4")
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for missing A4 reading")
	}

	// Empty wording.
	p = validParentProse()
	p.Glance = "  "
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for empty glance")
	}

	// Bare internal code leaks (说人话).
	for _, bad := range []string{"D3 很稳", "d 4 不足", "given_not_taken 出现", "SOLO 分布", "P2 水平"} {
		p = validParentProse()
		p.Opportunity = bad
		if err := validateParentProse(p, rep); err == nil {
			t.Errorf("expected rejection for leaked code %q", bad)
		}
	}
}
```

(`contains` is a tiny helper — add `func contains(h, n string) bool { return strings.Contains(h, n) }` if not already present in the test package; otherwise use `strings.Contains` inline.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestParent -run TestValidateParentProse`
Expected: FAIL — undefined `ParentProse` / `ParentFactsPrompt` / `validateParentProse`.

- [ ] **Step 3: Write the implementation**

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

// ParentAdvice is one 在家可以怎么帮 item.
type ParentAdvice struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ParentProse is the ONLY thing the model contributes to the parent report:
// gentled wording over the finished canonical object. It cannot change any
// level, state, badge, or which dims/signals appear — those are deterministic.
// dReadings is keyed by depth code (only dims with a real level); aReadings by
// autonomy code (all six). Stored as the parent_report_prose.prose bundle.
type ParentProse struct {
	Glance      string            `json:"glance"`
	DOverview   string            `json:"dOverview"`
	AOverview   string            `json:"aOverview"`
	Opportunity string            `json:"opportunity"`
	WarmLine    string            `json:"warmLine"`
	DReadings   map[string]string `json:"dReadings"`
	AReadings   map[string]string `json:"aReadings"`
	Advice      []ParentAdvice    `json:"advice"`
}

// Rune-count caps (Chinese-first; a byte cap would let CJK blow past it).
const (
	parentGlanceMax   = 120
	parentOverviewMax = 120
	parentReadingMax  = 220
	parentOppMax      = 300
	parentWarmMax     = 160
	parentAdviceMax   = 220
)

// parentBareCode rejects internal register leaking into parent prose: bare axis
// codes (D3 / a 4), the opportunity enum, SOLO, and P0–P3. Case-insensitive and
// space-tolerant — the fact sheet is Chinese-only, so a real alphanumeric
// collision is rare and a false reject only fails this one compose call.
var parentBareCode = regexp.MustCompile(`(?i)(\b[da]\s*[1-6]\b|given_taken|given_not_taken|not_supplied|solo|\bp\s*[0-3]\b)`)

func parentSystemPrompt() string {
	return strings.Join([]string{
		"你在为一位学生的家长写一份「能力成长报告」的措辞。读者是家长，不是老师，也不是学生本人。",
		"你只负责措辞。每一维的等级、每个信号的状态，都已经由系统判定完毕，你不得改动，也不得新增或删减维度/信号。",
		"规则：",
		"1. 只使用给你的事实（每一维/信号的证据）。不得引入任何未给出的行为、数字或结论。",
		"2. 说人话、温和。绝不出现 D1–D6 / A1–A6 这类内部代码，也不出现 given_taken / SOLO / P0–P3 之类术语。",
		"3. A 轴（智识自主）只描述信号，绝不写任何数字或等级。",
		"4. 「机会供给先于判定」：平台没给到锻炼机会 = 平台欠账，不算孩子短板；只有「机会已给、没接住」才算孩子信号。opportunity 段要体现这一点，并如实描述真实性（哪些由 AI 代写需当面核对）。",
		"5. 不贴标签、不排名、不预测考分。只描述这一次作品里的行为。",
		"6. 只输出 JSON：{\"glance\":\"\",\"dOverview\":\"\",\"aOverview\":\"\",\"opportunity\":\"\",\"warmLine\":\"\",\"dReadings\":{\"D1\":\"\"},\"aReadings\":{\"A1\":\"\"},\"advice\":[{\"title\":\"\",\"text\":\"\"}]}",
		"dReadings 只为「给了读数的维度」写；aReadings 为全部六个信号写；advice 写 2–3 条家长在家可以怎么帮。",
	}, "\n")
}

// wantedDepth returns the depth codes that carry a real level — the only ones
// offered to the composer. NA/"" dims render 敢于空白 deterministically.
func wantedDepth(rep Report) []DepthDim {
	out := make([]DepthDim, 0, len(rep.DepthAxis))
	for _, d := range rep.DepthAxis {
		if d.Level == "L1" || d.Level == "L2" || d.Level == "L3" || d.Level == "L4" {
			out = append(out, d)
		}
	}
	return out
}

// ParentFactsPrompt renders the canonical evidence the composer may see —
// deliberately excludes promptLens, interactionEvidence, officialProjection and
// workAndProcess (家长端与教师端隔离).
func ParentFactsPrompt(rep Report, name, subject string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n研究主题：%s\n", name, subject)
	b.WriteString("认知深度（给了读数的维度，需写 dReadings）：\n")
	for _, d := range wantedDepth(rep) {
		fmt.Fprintf(&b, "- %s %s：等级 %s，证据=%s\n", d.Code, d.Name, d.Level, d.Evidence)
	}
	b.WriteString("智识自主（全部六个信号，需写 aReadings，措辞里不得出现数字/等级）：\n")
	for _, a := range rep.AutonomyAxis {
		opp := map[string]string{"given_taken": "机会已给·已接住", "given_not_taken": "机会已给·没接住", "not_supplied": "平台还没提供机会"}[a.Opportunity]
		fmt.Fprintf(&b, "- %s %s：内部级别 %d（不得写出），机会=%s，证据=%s\n", a.Code, a.Name, a.Level, opp, a.Evidence)
	}
	return b.String()
}

// ComposeParent makes ONE flagship call gentling the canonical object into
// parent wording. Usage is returned even when the output is rejected, so the
// caller records the spend either way (mirrors ComposeWeekly / AssessReport).
func ComposeParent(ctx context.Context, prov gateway.Provider, r gateway.Resolved, rep Report, name, subject string) (ParentProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: parentSystemPrompt()},
			{Role: gateway.RoleUser, Content: ParentFactsPrompt(rep, name, subject)},
		},
	})
	if err != nil {
		return ParentProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage
	var out ParentProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return ParentProse{}, usage, fmt.Errorf("agent: parent prose not JSON: %w", err)
	}
	if err := validateParentProse(out, rep); err != nil {
		return ParentProse{}, usage, err
	}
	return out, usage, nil
}

// validateParentProse enforces wording-only: every wanted depth dim and all six
// autonomy signals get exactly one non-empty reading keyed to a real code,
// overviews/glance/opportunity/warmLine/advice are non-empty and within caps,
// and no internal code/term leaks (说人话).
func validateParentProse(p ParentProse, rep Report) error {
	// Depth: readings for exactly the real-level dims.
	wantD := map[string]bool{}
	for _, d := range wantedDepth(rep) {
		wantD[d.Code] = false
	}
	for code, txt := range p.DReadings {
		seen, known := wantD[code]
		if !known {
			return fmt.Errorf("agent: parent prose has an unwanted depth reading %q", code)
		}
		if seen {
			return fmt.Errorf("agent: parent prose repeats depth reading %q", code)
		}
		wantD[code] = true
		if strings.TrimSpace(txt) == "" {
			return fmt.Errorf("agent: parent prose leaves depth %q empty", code)
		}
		if utf8.RuneCountInString(txt) > parentReadingMax {
			return fmt.Errorf("agent: parent depth reading %q too long", code)
		}
	}
	for code, seen := range wantD {
		if !seen {
			return fmt.Errorf("agent: parent prose missing depth reading %q", code)
		}
	}
	// Autonomy: all six signals.
	wantA := map[string]bool{}
	for _, a := range rep.AutonomyAxis {
		wantA[a.Code] = false
	}
	for code, txt := range p.AReadings {
		seen, known := wantA[code]
		if !known {
			return fmt.Errorf("agent: parent prose has an unknown autonomy reading %q", code)
		}
		if seen {
			return fmt.Errorf("agent: parent prose repeats autonomy reading %q", code)
		}
		wantA[code] = true
		if strings.TrimSpace(txt) == "" {
			return fmt.Errorf("agent: parent prose leaves autonomy %q empty", code)
		}
		if utf8.RuneCountInString(txt) > parentReadingMax {
			return fmt.Errorf("agent: parent autonomy reading %q too long", code)
		}
	}
	for code, seen := range wantA {
		if !seen {
			return fmt.Errorf("agent: parent prose missing autonomy reading %q", code)
		}
	}
	// Scalars non-empty + capped.
	scalars := []struct {
		name, val string
		max       int
	}{
		{"glance", p.Glance, parentGlanceMax}, {"dOverview", p.DOverview, parentOverviewMax},
		{"aOverview", p.AOverview, parentOverviewMax}, {"opportunity", p.Opportunity, parentOppMax},
		{"warmLine", p.WarmLine, parentWarmMax},
	}
	for _, s := range scalars {
		if strings.TrimSpace(s.val) == "" {
			return fmt.Errorf("agent: parent prose leaves %s empty", s.name)
		}
		if utf8.RuneCountInString(s.val) > s.max {
			return fmt.Errorf("agent: parent prose %s too long", s.name)
		}
	}
	if len(p.Advice) == 0 {
		return fmt.Errorf("agent: parent prose has no advice")
	}
	for _, ad := range p.Advice {
		if strings.TrimSpace(ad.Title) == "" || strings.TrimSpace(ad.Text) == "" {
			return fmt.Errorf("agent: parent advice item empty")
		}
		if utf8.RuneCountInString(ad.Text) > parentAdviceMax {
			return fmt.Errorf("agent: parent advice too long")
		}
	}
	// 说人话: no internal register anywhere.
	texts := []string{p.Glance, p.DOverview, p.AOverview, p.Opportunity, p.WarmLine}
	for _, t := range p.DReadings {
		texts = append(texts, t)
	}
	for _, t := range p.AReadings {
		texts = append(texts, t)
	}
	for _, ad := range p.Advice {
		texts = append(texts, ad.Title, ad.Text)
	}
	for _, t := range texts {
		if parentBareCode.MatchString(t) {
			return fmt.Errorf("agent: parent prose contains an internal code/term")
		}
	}
	return nil
}
```

> **Note on the facts prompt:** it prints the autonomy internal level as "内部级别 %d（不得写出）" so the model can calibrate the *reading's tone* without echoing the number; the validator + the numberless projection guarantee no number reaches the parent surface regardless of what the model writes.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS (full package — the composer touches package-level names).

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/compose_parent.go apps/api/internal/agent/compose_parent_test.go
git commit -m "feat(e1): flagship parent-prose composer + validation"
```

---

## Task 3: contract `parentReport.ts`

**Files:**
- Create: `packages/contracts/src/parentReport.ts`
- Modify: `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/parentReport.test.ts`

**Interfaces:**
- Consumes: `zod`.
- Produces (exported): `ParentReport` (zod schema + inferred type) = the GET wire object; `ParentReportProse` (the composer output / stored bundle). Field names mirror the Go `ParentReportDTO` / `ParentProse` exactly.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest";
import { ParentReport, ParentReportProse } from "../src/parentReport";

const wire = {
  cover: { name: "林知远", subject: "嵌入式广告与正常化", klass: "IBDP 一年级 · 研究组", typeLabel: "项目报告", dateStr: "2026年7月25日", warmLine: "把 AI 当审稿人。" },
  glance: "方法对齐、证据充分", dOverview: "多数维度稳定在熟练", aOverview: "六个方面都观察到主动信号",
  dRows: [
    { code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "能把宽泛话题收窄。" },
    { code: "D6", name: "反思与元认知", badge: "暂无", reading: "暂无可计入的证据" },
  ],
  aRows: [{ code: "A1", name: "方向自主", state: "观察到主动信号", reading: "自己决定方向。" }],
  opportunity: "机会多由孩子自己创造。",
  advice: [{ title: "请他讲给你听", text: "说清这份研究不能说明什么。" }],
  prose: "present" as const,
};

describe("ParentReport", () => {
  it("accepts a full wire object", () => {
    expect(ParentReport.parse(wire)).toBeTruthy();
  });
  it("accepts prose:null (deterministic-only)", () => {
    expect(ParentReport.parse({ ...wire, prose: null }).prose).toBeNull();
  });
  it("rejects an unknown prose sentinel", () => {
    expect(() => ParentReport.parse({ ...wire, prose: "maybe" })).toThrow();
  });
  it("validates the composer bundle", () => {
    const prose = {
      glance: "g", dOverview: "d", aOverview: "a", opportunity: "o", warmLine: "w",
      dReadings: { D1: "r" }, aReadings: { A1: "r", A2: "r", A3: "r", A4: "r", A5: "r", A6: "r" },
      advice: [{ title: "t", text: "x" }],
    };
    expect(ParentReportProse.parse(prose)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/contracts && npm test -- parentReport`
Expected: FAIL — cannot resolve `../src/parentReport`.

- [ ] **Step 3: Write the implementation**

`packages/contracts/src/parentReport.ts`:

```ts
import { z } from "zod";

// ParentReport: the parent-facing projection of the canonical assessment
// object (project mode). Deterministic badges/states + gentled prose. RL-5:
// no total; the A axis carries no number anywhere.
const ParentDRow = z.object({
  code: z.string(),
  name: z.string(),
  badge: z.string(), // 起步/发展/熟练/优秀/暂无 (server-single-sourced)
  reading: z.string(),
});

const ParentARow = z.object({
  code: z.string(),
  name: z.string(),
  state: z.string(), // 观察到主动信号 / 偶有·多在引导后 / 暂未观察到
  reading: z.string(),
});

const ParentAdvice = z.object({ title: z.string(), text: z.string() });

// ParentReportProse = the composer output / stored bundle.
export const ParentReportProse = z.object({
  glance: z.string(),
  dOverview: z.string(),
  aOverview: z.string(),
  opportunity: z.string(),
  warmLine: z.string(),
  dReadings: z.record(z.string()),
  aReadings: z.record(z.string()),
  advice: z.array(ParentAdvice),
});
export type ParentReportProse = z.infer<typeof ParentReportProse>;

export const ParentReport = z.object({
  cover: z.object({
    name: z.string(),
    subject: z.string(),
    klass: z.string(),
    typeLabel: z.string(),
    dateStr: z.string(),
    warmLine: z.string(),
  }),
  glance: z.string(),
  dOverview: z.string(),
  aOverview: z.string(),
  dRows: z.array(ParentDRow),
  aRows: z.array(ParentARow),
  opportunity: z.string(),
  advice: z.array(ParentAdvice),
  prose: z.union([z.literal("present"), z.null()]),
});
export type ParentReport = z.infer<typeof ParentReport>;
```

- [ ] **Step 4: Wire the export + run tests**

Add to `packages/contracts/src/index.ts` (follow the existing `export * from "./..."` style):

```ts
export * from "./parentReport";
```

Run: `cd packages/contracts && npm test -- parentReport && npx tsc --noEmit`
Expected: PASS + no type errors.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add packages/contracts/src/parentReport.ts packages/contracts/src/index.ts packages/contracts/test/parentReport.test.ts
git commit -m "feat(e1): parentReport contract (wire object + composer bundle)"
```

---

## Task 4: migration 0035 + sqlc + GET handler (cost-free read)

**Files:**
- Create: `apps/api/internal/store/migrations/0035_parent_report_prose.sql`
- Create: `apps/api/internal/store/queries/parent.sql`
- Create: `apps/api/internal/api/parent_report.go`
- Modify: `apps/api/internal/api/api.go` (register both routes now — POST handler lands in Task 5)
- Test: `apps/api/internal/api/parent_report_test.go`
- Generated: `apps/api/internal/store/sqlc/*` (via `make sqlc`)

**Interfaces:**
- Consumes: `authTeacherStudent`, `assertTeacherOwnsClass` (returns `sqlc.Class`), `a.d.Queries.GetClassByID`, `GetUserByID`, `GetStudentProjectEvaluationForTeacher` (params `{ScopeID pgtype.UUID, UserID uuid.UUID}` → `{Scores []byte, CreatedAt time.Time, ProjectTitle string, RqBody ...}`), `parent.DBadge`, `parent.AState`, `agent.Report`, `agent.ParentProse`.
- Produces: `ParentCoverDTO`, `ParentDRowDTO`, `ParentARowDTO`, `ParentAdviceDTO`, `ParentReportDTO`, `parentReportData` (shared struct), `func (a *API) loadParentReport(ctx, classID, userID, surface string, scopeID uuid.UUID) (parentReportData, error)`, `func parentReportDTO(d parentReportData, prose *agent.ParentProse) ParentReportDTO`, `func (a *API) getParentReport(w,r)`. Task 5 adds `postParentReportProse`.

- [ ] **Step 1: Write the migration**

`apps/api/internal/store/migrations/0035_parent_report_prose.sql`:

```sql
-- +goose Up
-- E1: the ONLY stored part of the parent report. Badges/states are computed
-- live on every read; only the LLM-gentled words are persisted, once per
-- (student, surface, scope). The primary key IS the first-open-wins lock:
-- generation inserts with ON CONFLICT DO NOTHING.
CREATE TABLE parent_report_prose (
    student_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    surface         text NOT NULL,
    scope_id        text NOT NULL,
    prose           jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (student_user_id, surface, scope_id)
);

-- +goose Down
DROP TABLE IF EXISTS parent_report_prose;
```

- [ ] **Step 2: Write the sqlc queries**

`apps/api/internal/store/queries/parent.sql`:

```sql
-- name: GetParentReportProse :one
SELECT prose FROM parent_report_prose
WHERE student_user_id = $1 AND surface = $2 AND scope_id = $3;

-- name: InsertParentReportProse :exec
INSERT INTO parent_report_prose (student_user_id, surface, scope_id, prose)
VALUES ($1, $2, $3, $4)
ON CONFLICT (student_user_id, surface, scope_id) DO NOTHING;
```

- [ ] **Step 3: Generate sqlc**

Run: `cd apps/api && make sqlc`
Expected: regenerates `internal/store/sqlc/*` with `GetParentReportProse` (returns `[]byte`) + `InsertParentReportProse`. Do not hand-edit the output.

- [ ] **Step 4: Write the failing test**

`apps/api/internal/api/parent_report_test.go` — reuse the package's existing testcontainer harness (see `teacher_read_test.go` / `teacher_weekly_test.go` for `newTestAPI`, seeded 吴老师 + students, and the teacher-auth request helper). Pick a seeded student who has a project evaluation (the same one `teacher_read_test.go` uses for `getStudentReport`).

```go
func TestGetParentReport_DeterministicNoProse(t *testing.T) {
	// ... stand up API, seed, get teacher session, resolve (classID, studentID, projectScopeID)
	// exactly as TestGetStudentReport does in teacher_read_test.go ...
	rr := doTeacherGET(t, srv, teacherCookie,
		fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s", classID, studentID, projectScopeID))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto ParentReportDTO
	mustJSON(t, rr.Body.Bytes(), &dto)
	if dto.ProseReady {
		t.Error("prose should be absent before composition")
	}
	if len(dto.DRows) != 6 || len(dto.ARows) != 6 {
		t.Fatalf("want 6 D + 6 A rows, got %d/%d", len(dto.DRows), len(dto.ARows))
	}
	// Badges are the parent 四台阶 labels, never L-codes or numbers.
	for _, d := range dto.DRows {
		switch d.Badge {
		case "起步", "发展", "熟练", "优秀", "暂无":
		default:
			t.Errorf("D badge %q not a 四台阶 label", d.Badge)
		}
	}
	for _, a := range dto.ARows {
		switch a.State {
		case "观察到主动信号", "偶有·多在引导后", "暂未观察到":
		default:
			t.Errorf("A state %q not a 三态 label", a.State)
		}
	}
}

func TestGetParentReport_ForeignTeacher404(t *testing.T) {
	// A teacher who does not own the student's class gets 404 (existence-hidden),
	// mirroring TestGetStudentReport_ForeignClass in teacher_read_test.go.
}
```

(Match the exact seed accessors and request helpers already in the test package; do not invent new harness. If the package’s helper is `authGET(...)` rather than `doTeacherGET`, use that.)

- [ ] **Step 5: Run test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestGetParentReport` (allow ≥600s)
Expected: FAIL — `getParentReport` / DTOs undefined, route not registered.

- [ ] **Step 6: Write the handler + DTOs**

`apps/api/internal/api/parent_report.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/parent"
	"mindimprint/api/internal/store/sqlc"
)

type ParentCoverDTO struct {
	Name      string `json:"name"`
	Subject   string `json:"subject"`
	Klass     string `json:"klass"`
	TypeLabel string `json:"typeLabel"`
	DateStr   string `json:"dateStr"`
	WarmLine  string `json:"warmLine"`
}

type ParentDRowDTO struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Badge   string `json:"badge"`
	Reading string `json:"reading"`
}

type ParentARowDTO struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Reading string `json:"reading"`
}

type ParentAdviceDTO struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ParentReportDTO is the whole parent page. Badges/states/rows are computed on
// this read; glance/overviews/opportunity/readings/warmLine/advice come from
// stored prose (nil until composed). ProseReady mirrors prose presence.
type ParentReportDTO struct {
	Cover       ParentCoverDTO    `json:"cover"`
	Glance      string            `json:"glance"`
	DOverview   string            `json:"dOverview"`
	AOverview   string            `json:"aOverview"`
	DRows       []ParentDRowDTO   `json:"dRows"`
	ARows       []ParentARowDTO   `json:"aRows"`
	Opportunity string            `json:"opportunity"`
	Advice      []ParentAdviceDTO `json:"advice"`
	ProseReady  bool              `json:"proseReady"`
	// prose sentinel mirrors the contract's `prose: "present" | null`.
	Prose *string `json:"prose"`
}

// parentReportData is everything both handlers need: the canonical report plus
// cover identity. GET renders it; POST composes prose over it.
type parentReportData struct {
	Report    agent.Report
	Name      string
	Subject   string
	Klass     string
	ScopeID   uuid.UUID
	Surface   string
	StudentID uuid.UUID
}

// loadParentReport runs the deterministic layer for one (student, project
// scope): fetch the canonical report + cover identity. No model call. E1
// serves surface "project" only.
func (a *API) loadParentReport(ctx context.Context, classID, userID uuid.UUID, surface string, scopeID uuid.UUID) (parentReportData, error) {
	if surface != "project" {
		return parentReportData{}, httpx.ErrNotFound("资源不存在")
	}
	row, err := a.d.Queries.GetStudentProjectEvaluationForTeacher(ctx, sqlc.GetStudentProjectEvaluationForTeacherParams{
		ScopeID: pgtype.UUID{Bytes: scopeID, Valid: true}, UserID: userID,
	})
	if err != nil {
		return parentReportData{}, err
	}
	var rep agent.Report
	if err := json.Unmarshal(row.Scores, &rep); err != nil {
		return parentReportData{}, err
	}
	user, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return parentReportData{}, err
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return parentReportData{}, err
	}
	return parentReportData{
		Report: rep, Name: user.DisplayName, Subject: row.ProjectTitle, Klass: cls.Name,
		ScopeID: scopeID, Surface: surface, StudentID: userID,
	}, nil
}

// parentReportDTO merges the deterministic projection with whatever prose
// exists. NA depth dims render 暂无 + 「暂无可计入的证据」 regardless of prose
// (敢于空白). No autonomy number ever appears.
func parentReportDTO(d parentReportData, prose *agent.ParentProse) ParentReportDTO {
	warm := ""
	if prose != nil {
		warm = prose.WarmLine
	}
	dto := ParentReportDTO{
		Cover: ParentCoverDTO{
			Name: d.Name, Subject: "研究项目 · " + d.Subject, Klass: d.Klass,
			TypeLabel: "项目报告", DateStr: parentDateStr(time.Now()), WarmLine: warm,
		},
	}
	for _, dim := range d.Report.DepthAxis {
		badge := parent.DBadge(dim.Level)
		reading := ""
		if badge == "暂无" {
			reading = "暂无可计入的证据"
		} else if prose != nil {
			reading = prose.DReadings[dim.Code]
		}
		dto.DRows = append(dto.DRows, ParentDRowDTO{Code: dim.Code, Name: dim.Name, Badge: badge, Reading: reading})
	}
	for _, sig := range d.Report.AutonomyAxis {
		reading := ""
		if prose != nil {
			reading = prose.AReadings[sig.Code]
		}
		dto.ARows = append(dto.ARows, ParentARowDTO{Code: sig.Code, Name: sig.Name, State: parent.AState(sig.Level), Reading: reading})
	}
	if prose != nil {
		dto.Glance, dto.DOverview, dto.AOverview, dto.Opportunity = prose.Glance, prose.DOverview, prose.AOverview, prose.Opportunity
		for _, ad := range prose.Advice {
			dto.Advice = append(dto.Advice, ParentAdviceDTO{Title: ad.Title, Text: ad.Text})
		}
		present := "present"
		dto.Prose = &present
		dto.ProseReady = true
	}
	return dto
}

// parentDateStr renders the cover date as 2026年7月25日.
func parentDateStr(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
}

// getParentReport handles GET .../parent-report/{surface}/{scopeId}. Computes
// the deterministic projection live and merges stored prose when present. This
// handler NEVER calls a model.
func (a *API) getParentReport(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	surface := r.PathValue("surface")
	scopeID, err := uuid.Parse(r.PathValue("scopeId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	data, err := a.loadParentReport(r.Context(), classID, userID, surface, scopeID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.getParentProse(r.Context(), userID, surface, scopeID)
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}

// getParentProse reads and decodes the stored bundle, or returns pgx.ErrNoRows.
func (a *API) getParentProse(ctx context.Context, userID uuid.UUID, surface string, scopeID uuid.UUID) (*agent.ParentProse, error) {
	raw, err := a.d.Queries.GetParentReportProse(ctx, sqlc.GetParentReportProseParams{
		StudentUserID: userID, Surface: surface, ScopeID: scopeID.String(),
	})
	if err != nil {
		return nil, err
	}
	var p agent.ParentProse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
```

Add `"fmt"` to the import block (used by `parentDateStr`).

- [ ] **Step 7: Register the routes**

In `apps/api/internal/api/api.go`, after line 133 (the weekly routes), add:

```go
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/parent-report/{surface}/{scopeId}", teacherOrAdmin(a.getParentReport))
	mux.Handle("POST /api/v1/classes/{id}/students/{userId}/parent-report/{surface}/{scopeId}/prose", teacherOrAdmin(a.postParentReportProse))
```

> `postParentReportProse` is implemented in Task 5. To keep this task compiling, add a temporary stub in `parent_report.go` that Task 5 replaces:
> ```go
> func (a *API) postParentReportProse(w http.ResponseWriter, r *http.Request) {
> 	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
> }
> ```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` (allow ≥600s)
Expected: PASS (full suite — migration + sqlc + new package).

- [ ] **Step 9: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0035_parent_report_prose.sql apps/api/internal/store/queries/parent.sql apps/api/internal/store/sqlc apps/api/internal/api/parent_report.go apps/api/internal/api/parent_report_test.go apps/api/internal/api/api.go
git commit -m "feat(e1): parent_report_prose storage + cost-free GET projection"
```

---

## Task 5: POST prose handler (the only spend)

**Files:**
- Modify: `apps/api/internal/api/parent_report.go` (replace the `postParentReportProse` stub)
- Test: `apps/api/internal/api/parent_report_test.go` (add POST tests)

**Interfaces:**
- Consumes: `a.d.EvalResolver(ctx)`, `a.d.Provider`, `agent.ComposeParent`, `gateway.EstimateCost`, `gateway.CostNumeric`, `a.d.Queries.RecordLLMCall`, `a.d.Queries.InsertParentReportProse`, `UserFromContext`. See `composeWeeklyProse` in `teacher_weekly.go` for the exact metering shape.
- Produces: `func (a *API) postParentReportProse(w,r)`; `func (a *API) composeParentProse(ctx, r, data parentReportData) (agent.ParentProse, bool)`.

- [ ] **Step 1: Write the failing test**

```go
func TestPostParentProse_ComposesOnceThenCostFree(t *testing.T) {
	// ... resolve (classID, studentID, projectScopeID) as in Task 4;
	//     the test API is wired with a stub/fake provider that returns a valid
	//     ParentProse JSON (mirror how teacher_weekly_test.go fakes ComposeWeekly's
	//     provider) ...
	url := fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s/prose", classID, studentID, projectScopeID)

	rr := doTeacherPOST(t, srv, teacherCookie, url)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto ParentReportDTO
	mustJSON(t, rr.Body.Bytes(), &dto)
	if !dto.ProseReady || dto.Glance == "" {
		t.Fatal("first POST should compose and return prose")
	}
	n1 := countLLMCalls(t, db, "parent_report")
	if n1 != 1 {
		t.Fatalf("want 1 parent_report llm_call, got %d", n1)
	}

	// Second POST: first-open-wins, no new spend.
	rr2 := doTeacherPOST(t, srv, teacherCookie, url)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second status=%d", rr2.Code)
	}
	if n2 := countLLMCalls(t, db, "parent_report"); n2 != 1 {
		t.Fatalf("second POST must not spend; llm_calls=%d", n2)
	}
}

func TestPostParentProse_RejectionNeverWalls(t *testing.T) {
	// provider returns prose that fails validation (e.g. leaks "D3") →
	// handler returns 200 with ProseReady=false and the deterministic rows,
	// and STILL records one metered llm_call (spend happened).
}
```

Use the package's existing fake-provider and `countLLMCalls`/db accessors (see `teacher_weekly_test.go`); do not invent new harness. If no `countLLMCalls` helper exists, count rows in `llm_call` where `purpose='parent_report'` with the package's db handle.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestPostParentProse` (allow ≥600s)
Expected: FAIL — stub returns 404 / no prose.

- [ ] **Step 3: Replace the stub**

In `apps/api/internal/api/parent_report.go`, replace the `postParentReportProse` stub with:

```go
// postParentReportProse handles POST .../parent-report/{surface}/{scopeId}/prose
// — the ONLY endpoint in E1 that spends. Generates once per (student, surface,
// scope); a second call returns the stored row without calling a model
// (first-open-wins). A failed composition never walls: the deterministic report
// still renders.
func (a *API) postParentReportProse(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	surface := r.PathValue("surface")
	scopeID, err := uuid.Parse(r.PathValue("scopeId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ctx := r.Context()
	data, err := a.loadParentReport(ctx, classID, userID, surface, scopeID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Already composed → return it, no spend.
	if existing, gerr := a.getParentProse(ctx, userID, surface, scopeID); gerr == nil {
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, existing))
		return
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	prose, spent := a.composeParentProse(ctx, r, data)
	if !spent {
		// 敢于空白: the badges, states and 暂无可计入的证据 still render.
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, nil))
		return
	}
	raw, merr := json.Marshal(prose)
	if merr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, nil))
		return
	}
	if ierr := a.d.Queries.InsertParentReportProse(ctx, sqlc.InsertParentReportProseParams{
		StudentUserID: userID, Surface: surface, ScopeID: scopeID.String(), Prose: raw,
	}); ierr != nil {
		httpx.WriteError(w, r, ierr)
		return
	}
	// Re-read: a concurrent teacher may have won the insert; the winner's row
	// is what both must see.
	stored, serr := a.getParentProse(ctx, userID, surface, scopeID)
	if serr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, &prose))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, stored))
}

// composeParentProse makes the flagship call and records its cost — including
// when the output is rejected, since a rejected composition still spent tokens.
// spent=false means "no prose this time", never an error to the client.
func (a *API) composeParentProse(ctx context.Context, r *http.Request, data parentReportData) (agent.ParentProse, bool) {
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("parent prose: no provider", "err", rerr)
		return agent.ParentProse{}, false
	}
	prose, usage, cerr := agent.ComposeParent(ctx, a.d.Provider, resolved, data.Report, data.Name, data.Subject)
	if u, ok := UserFromContext(ctx); ok && resolved.Provider != "" {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("parent llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Bytes: data.ScopeID, Valid: true},
			Surface: "teacher", Purpose: "parent_report",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("parent prose: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("parent prose: rejected", "err", cerr)
		return agent.ParentProse{}, false
	}
	return prose, true
}
```

Add `"log/slog"` and `"mindimprint/api/internal/gateway"` to the import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` (allow ≥600s)
Expected: PASS (full suite).

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/parent_report.go apps/api/internal/api/parent_report_test.go
git commit -m "feat(e1): parent-prose POST — flagship compose, first-open-wins, never walls"
```

---

## Task 6: web — ParentReport view + stub wiring

**Files:**
- Create: `apps/web/src/console/parentReportContent.ts`
- Create: `apps/web/src/console/ParentReport.tsx`
- Modify: `apps/web/src/console/StudentDetailView.tsx`
- Modify: `apps/web/src/console/TeacherReportView.tsx`
- Test: `apps/web/test/console/ParentReport.test.tsx`

**Interfaces:**
- Consumes: `ParentReport` type from `@mind-imprint/contracts`; the app `api` client (see how `StudentDetailView`/`TeacherReportView` call `api.*`); the fetched DTO from Task 4/5.
- Produces: `<ParentReport classId surface scopeId studentName onClose />` (a full-screen printable overlay); static content constants.

**Static content source:** copy the fixed教育 front-matter **verbatim** from the binding design `docs/design/teacher end/project/家长报告.dc.html` `SHARED` block into `parentReportContent.ts`:
`principles` (4), `dLevels` (4, with colors), `dDims` (6), `aStates` (3, with colors), `aSignals` (6), `howList` (4), `supply` (2), `glossary` (10), `footer`. These are not on the wire — they never change per student.

- [ ] **Step 1: Add the API client method**

In the app's `api` module (same file the console already imports `api` from), add:

```ts
async getParentReport(classId: string, surface: string, scopeId: string): Promise<ParentReport> {
  return this.get(`/classes/${classId}/students/${scopeId /* NOTE: see below */}...`);
}
```

Follow the EXACT pattern the console already uses for `getStudentReport` (path `/classes/{id}/students/{userId}/reports/{surface}/{scopeId}`). The parent paths are:
- GET `/classes/${classId}/students/${studentId}/parent-report/${surface}/${scopeId}`
- POST `/classes/${classId}/students/${studentId}/parent-report/${surface}/${scopeId}/prose`

Add both `getParentReport(classId, studentId, surface, scopeId)` and `generateParentReportProse(classId, studentId, surface, scopeId)` returning `ParentReport`, matching the client's existing method signatures and error handling.

- [ ] **Step 2: Write the static content constants**

`apps/web/src/console/parentReportContent.ts` — export typed constants transcribed verbatim from the dc-html `SHARED` block:

```ts
export const PRINCIPLES: { title: string; text: string }[] = [ /* 4, verbatim */ ];
export const D_LEVELS: { label: string; color: string; desc: string }[] = [ /* 起步/发展/熟练/优秀, verbatim */ ];
export const D_DIMS: { name: string; meaning: string }[] = [ /* 6, verbatim */ ];
export const A_STATES: { label: string; color: string; desc: string }[] = [ /* 3, verbatim */ ];
export const A_SIGNALS: { name: string; meaning: string }[] = [ /* 6, verbatim */ ];
export const HOW_LIST: { title: string; text: string }[] = [ /* 4, verbatim */ ];
export const SUPPLY: string[] = [ /* 2, verbatim */ ];
export const GLOSSARY: { term: string; def: string }[] = [ /* 10, verbatim */ ];
export const FOOTER = "本报告由思维印记基于孩子在平台上的过程记录生成……"; // verbatim

// Badge/state → color (client presentation; server sends only the label).
export const D_BADGE_COLOR: Record<string, { color: string; bg: string }> = {
  起步: { color: "#C4574D", bg: "#F7E6E4" }, 发展: { color: "#C68A3A", bg: "#F6EED9" },
  熟练: { color: "#3E7CA8", bg: "#E1EDF5" }, 优秀: { color: "#3E8A6E", bg: "#E4F0EA" },
  暂无: { color: "#8A92A3", bg: "#F1F2F6" },
};
export const A_STATE_COLOR: Record<string, { color: string; bg: string }> = {
  观察到主动信号: { color: "#3E8A6E", bg: "#E4F0EA" }, "偶有·多在引导后": { color: "#C68A3A", bg: "#F6EED9" },
  暂未观察到: { color: "#C4574D", bg: "#F7E6E4" },
};
```

- [ ] **Step 3: Write the failing test**

`apps/web/test/console/ParentReport.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ParentReport } from "@/console/ParentReport";
import { api } from "@/api";

const base = {
  cover: { name: "林知远", subject: "研究项目 · 嵌入式广告", klass: "研究组", typeLabel: "项目报告", dateStr: "2026年7月25日", warmLine: "" },
  glance: "", dOverview: "", aOverview: "",
  dRows: [
    { code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "" },
    { code: "D6", name: "反思与元认知", badge: "暂无", reading: "暂无可计入的证据" },
  ],
  aRows: [
    { code: "A1", name: "方向自主", state: "观察到主动信号", reading: "" },
    { code: "A4", name: "对抗与检验", state: "暂未观察到", reading: "" },
  ],
  opportunity: "", advice: [], prose: null as null,
};

describe("ParentReport", () => {
  it("renders 四台阶 badges and numberless 三态, shows the generate button when prose is null", async () => {
    vi.spyOn(api, "getParentReport").mockResolvedValue(base as never);
    render(<ParentReport classId="c" studentId="s" surface="project" scopeId="p" studentName="林知远" onClose={() => {}} />);
    expect(await screen.findByText("优秀")).toBeTruthy();
    expect(screen.getByText("观察到主动信号")).toBeTruthy();
    expect(screen.getByText("暂无可计入的证据")).toBeTruthy(); // 敢于空白
    // No A-axis number anywhere in the A rows.
    expect(screen.queryByText(/\b[0-5]\s*级/)).toBeNull();
    expect(screen.getByRole("button", { name: /生成家长版正文/ })).toBeTruthy();
  });

  it("composes on click and renders the readings", async () => {
    vi.spyOn(api, "getParentReport").mockResolvedValue(base as never);
    vi.spyOn(api, "generateParentReportProse").mockResolvedValue({
      ...base, prose: "present", glance: "方法对齐、证据充分",
      dRows: [{ code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "能把宽泛话题收窄。" }],
    } as never);
    render(<ParentReport classId="c" studentId="s" surface="project" scopeId="p" studentName="林知远" onClose={() => {}} />);
    fireEvent.click(await screen.findByRole("button", { name: /生成家长版正文/ }));
    await waitFor(() => expect(screen.getByText("方法对齐、证据充分")).toBeTruthy());
  });
});
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd apps/web && npm test -- ParentReport`
Expected: FAIL — cannot resolve `@/console/ParentReport`.

- [ ] **Step 5: Write the component**

`apps/web/src/console/ParentReport.tsx` — a full-screen printable overlay reproducing the dc-html **项目模式** layout in inline styles. Structure (in order): a screen-only control bar (标题 + 下载 PDF button = `window.print()` + a 关闭 button calling `onClose`); then the print body — cover → 这份报告怎么读 (`PRINCIPLES`) → 我们怎么看"能力"：两条轴 (`D_LEVELS`/`D_DIMS`, `A_STATES`/`A_SIGNALS`) → 判断是怎么得出来的 (`HOW_LIST`) → {name} 这次的表现 (glance card, then `dRows` with `D_BADGE_COLOR[badge]`, then `aRows` with `A_STATE_COLOR[state]`, then 机会与真实性 = `opportunity`) → 下一步 (`advice` + `SUPPLY`) → 名词解释 (`GLOSSARY`) → `FOOTER`. Add a `@media print { .no-print { display:none !important } }` style tag and hide the console rail via the same class.

State: `const [data, setData] = useState<ParentReport | null>(null)`; on mount `api.getParentReport(classId, studentId, surface, scopeId).then(setData)`. If `data.prose === null`, render a **生成家长版正文** button in the 这次的表现 section that calls `api.generateParentReportProse(...)` then `setData`. When `prose === null`, the readings/glance/overviews/opportunity/advice are empty strings — render the deterministic rows and the button; do not render empty prose sections.

Key correctness points (the reviewer will check these):
- Badge/state colors come ONLY from the label maps — never recompute a level.
- Never render any A-axis number.
- NA dim rows show badge 暂无 + 「暂无可计入的证据」 even before prose.
- The 下载 PDF button calls `window.print()`.

- [ ] **Step 6: Wire the stubs**

In `apps/web/src/console/StudentDetailView.tsx`: replace the inert `导出家长版·项目报告` `<span>` (the one with `title="家长版报告即将上线"`, around line 125) with a `<button>` that opens `<ParentReport>` for `(classId, student.id, "project", primaryReport.scopeId, student.displayName)` — track open state with `useState`. Leave the `导出家长版·阶段报告` span inert (E2). Guard: only enable when `primaryReport` exists (same condition as 查看完整能力报告).

In `apps/web/src/console/TeacherReportView.tsx`: replace the inert `导出家长版 PDF` `<span>` (around line 152) with a button opening `<ParentReport>` for the project scope currently displayed in that view (`surface:"project"`, the view's `scopeId`).

- [ ] **Step 7: Run tests + tsc**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS (all web tests) + no type errors.

- [ ] **Step 8: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/console/parentReportContent.ts apps/web/src/console/ParentReport.tsx apps/web/src/console/StudentDetailView.tsx apps/web/src/console/TeacherReportView.tsx apps/web/test/console/ParentReport.test.tsx apps/web/src/api.ts
git commit -m "feat(e1): parent report view + wire teacher-console export stubs"
```

(Adjust the `api` client path in `git add` to wherever the app's `api` module actually lives.)

---

## Self-Review

**Spec coverage:**
- §2a deterministic D badge / A numberless state → Task 1 (`internal/parent`) + Task 4 (`parentReportDTO`). ✓
- §2b static front-matter → Task 6 (`parentReportContent.ts`, verbatim). ✓
- §2c + §3 composer (flagship, validated, 敢于空白, 说人话 regex) → Task 2. ✓
- §4 migration 0035 first-open-wins → Task 4. ✓
- §5 two endpoints (GET cost-free / POST only spend, never walls) → Task 4 (GET) + Task 5 (POST). ✓
- §6 contract → Task 3. ✓
- §7 web view + print + stub wiring (项目 wired, 阶段 inert) → Task 6. ✓
- §8 invariants (isolation, metering Purpose:"parent_report", RL-5 numberless) → Tasks 2/4/5. ✓
- §10 acceptance → covered by Task 4/5/6 tests.

**Placeholder scan:** the static-content constants in Task 6 are marked "verbatim from the dc-html SHARED block" with an exact source path — a precise transcription instruction, not a TODO. All Go/TS/SQL code blocks are complete.

**Type consistency:** `ParentProse`/`ParentAdvice` (Task 2) ↔ `ParentReportProse`/`ParentAdvice` (Task 3) ↔ `ParentReportDTO` field names (Task 4) all use `glance/dOverview/aOverview/opportunity/warmLine/dReadings/aReadings/advice{title,text}`. `parent.DBadge(string)`/`parent.AState(int)` (Task 1) consumed with those exact signatures in Task 4. Route paths identical across Task 4 registration, Task 5 test, Task 6 client. `Purpose:"parent_report"` identical in Task 5 handler and its test.
