# Slice 9 — 0457 readiness gauge, made real (评估) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the static 就绪度 fixture with a real 0457 readiness gauge projected from the whole-draft review's per-table judgment, lit by one new per-criterion integer `points`.

**Architecture:** The whole-draft review model (already judging each 0457 table's band) also emits `points` = how many of that table's descriptor points the draft evidences. The skill config declares each table's total. A new `projectReadiness` joins the latest snapshot's **board-voice** review with the config → `[]GaugeDTO` on the studio projection, mapped 1:1 to the 评估 view. No migration (points ride in the existing `review_item` intervention body), no second model call.

**Tech Stack:** Go (`net/http`, sqlc, testcontainers), TypeScript + React + vitest, Zod contracts. Spec: `docs/superpowers/specs/2026-07-16-slice-9-readiness-gauge-design.md`.

## Global Constraints

- **DEC-9.1** Gauge only — the 评估 view renders only block ① 就绪度. Blocks ②③④ stay deferred; do NOT add them.
- **DEC-9.2** 0457 only, pluggable — one interface, one 0457 lamps renderer. Do NOT build other skins.
- **DEC-9.3** 4 review tables only — 表D/E/F/H. No 表A/B/C/G, no cross-station projection.
- **DEC-9.4** Extend the review output — lamps lit by one new `points` int on the review; NO second model call, NO migration, NO separate assessor engine.
- **RL-3** Never a predicted grade. Lamps = which descriptor cell. Tooltip copy is unchanged: 「就绪度显示你的草稿现在落在评分表的哪一格，用来定位下一步，不是预估分数」.
- **Board-voice-only** drives readiness (assessment of record). An intervention anchor with no `voice` key OR `voice=="board"` is board; sceptic/layperson/executioner never light gauges.
- **Clamp site is the projection only.** `ProposeReview` copies the model's `points` verbatim (no clamp); `projectReadiness` is the single clamp to `[0, total]`.
- **Table totals (verbatim):** 表D 来源与证据 = 4 · 表E 分析 = 4 · 表F 评估 = 3 · 表H 表达与组织 = 3.
- **Level derivation:** `full` if `lit == total && total > 0`; `empty` if `lit == 0`; else `partial`.
- **Level colours (verbatim):** full `#4C9A82` · partial `#D9A23D` · empty `#AEB4C2`.
- **Summary line:** `已点亮 {Σlit}/{Σtotal} 格`, computed in the view from the tables — NOT carried on the wire.
- Go tests: `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker; FULL packages for the gate. Skills → `make sync-skills` (from `apps/api`). Web → `npm test`; contracts → `npm test`; `tsc --noEmit` clean.
- Never `git add` a whole dir with untracked user files; add named paths only. The pre-existing `M package.json` + untracked user files (`docs/03_课程库…`, `docs/2026-07-06-spec.md`, `docs/astranova/`, the Toddle zip, `*.png`) are NOT ours — leave them.
- Binding design: `docs/design/思维印记_工作区.dc.html` REVIEW view (lines ~1157–1184 for the gauge block).

---

### Task 1: Skill config — `points` per review criterion

**Files:**
- Modify: `apps/api/internal/skills/skill.go` (`ReviewCriterion` struct ~64–70; `Validate` ~100–123)
- Modify: `apps/api/internal/skills/specs/writing-project.json` (`review_criteria`)
- Modify: `packages/contracts/skills/writing-project.json` (synced copy — via `make sync-skills`)
- Test: `apps/api/internal/skills/skill_test.go`

**Interfaces:**
- Produces: `skills.ReviewCriterion{Code string, Name string, Points int}` — `Points` is the table's total lamp count, ≥ 1. Consumed by Task 2 (prompt total) and Task 3 (gauge `Total`).

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/skills/skill_test.go`:

```go
func TestReviewCriterionPointsParsed(t *testing.T) {
	s, err := Load([]byte(`{"id":"x","kind":"project","contracts":{},
		"review_criteria":[{"code":"表D","name":"来源与证据","points":4}]}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s.ReviewCriteria) != 1 || s.ReviewCriteria[0].Points != 4 {
		t.Fatalf("want points 4, got %+v", s.ReviewCriteria)
	}
}

func TestReviewCriterionPointsMustBePositive(t *testing.T) {
	_, err := Load([]byte(`{"id":"x","kind":"project","contracts":{},
		"review_criteria":[{"code":"表D","name":"来源与证据","points":0}]}`))
	if err == nil {
		t.Fatal("want error for points < 1, got nil")
	}
}

func TestSeededWritingSkillHasCriterionPoints(t *testing.T) {
	all, err := Registry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	sk, ok := all["writing-project"]
	if !ok {
		t.Fatal("writing-project skill missing")
	}
	want := map[string]int{"表D": 4, "表E": 4, "表F": 3, "表H": 3}
	if len(sk.ReviewCriteria) != len(want) {
		t.Fatalf("want %d criteria, got %d", len(want), len(sk.ReviewCriteria))
	}
	for _, c := range sk.ReviewCriteria {
		if want[c.Code] != c.Points {
			t.Fatalf("%s: want points %d, got %d", c.Code, want[c.Code], c.Points)
		}
	}
}
```

> Note: `Registry()` is the loader used by the other seeded-skill tests in this file — confirm the exact name/signature already used by an existing test (e.g. `TestRegistry…`) and match it; if it takes no args and returns `(map[string]Skill, error)` the code above is correct as written.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/skills/ -run 'TestReviewCriterionPoints|TestSeededWritingSkillHasCriterionPoints' -v`
Expected: FAIL — `Points` field undefined / points is 0.

- [ ] **Step 3: Add the `Points` field + validation**

In `apps/api/internal/skills/skill.go`, replace the `ReviewCriterion` struct (drop the "NOT the Slice-9 readiness band engine" annotation):

```go
// ReviewCriterion is one mark-scheme table the whole-draft review assesses the
// draft against (0457's 表D/E/F/H). Points is the table's total descriptor-point
// count — the number of lamps the Slice-9 readiness gauge renders for it.
type ReviewCriterion struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}
```

In `Validate()`, immediately before `if _, err := s.TopoOrder(); err != nil {`:

```go
	for _, c := range s.ReviewCriteria {
		if c.Points < 1 {
			return fmt.Errorf("skill %s: review criterion %s needs points >= 1", s.ID, c.Code)
		}
	}
```

- [ ] **Step 4: Add `points` to both JSON copies**

In `apps/api/internal/skills/specs/writing-project.json`, set `review_criteria` to (preserving any surrounding fields/order):

```json
  "review_criteria": [
    { "code": "表D", "name": "来源与证据", "points": 4 },
    { "code": "表E", "name": "分析", "points": 4 },
    { "code": "表F", "name": "评估", "points": 3 },
    { "code": "表H", "name": "表达与组织", "points": 3 }
  ]
```

Then sync the contracts copy:

Run: `cd apps/api && make sync-skills`
Expected: `packages/contracts/skills/writing-project.json` updated to match; `git status` shows both JSON files changed, no other drift.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/skills/ -v`
Expected: PASS (all skills tests, including the three new ones).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/skills/skill.go apps/api/internal/skills/skill_test.go apps/api/internal/skills/specs/writing-project.json packages/contracts/skills/writing-project.json
git commit -m "feat(refactor2): Slice 9 T1 — review criterion points (0457 table totals)"
```

---

### Task 2: Review output — `points` on `ReviewItem` + prompt

**Files:**
- Modify: `apps/api/internal/agent/review.go` (`ReviewItem` ~17–24; `reviewItemWire` ~26–34; posture consts ~36–108; `ProposeReview` ~116–165)
- Test: `apps/api/internal/agent/review_test.go`

**Interfaces:**
- Consumes: `skills.ReviewCriterion.Points` (Task 1).
- Produces: `agent.ReviewItem.Points int` (`json:"points"`) — lit lamps as the model produced them, NOT clamped here. Persisted verbatim into the intervention body by the existing `mustJSON(it)` at `apps/api/internal/api/writing.go:348`. Consumed by Task 3.

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/agent/review_test.go`:

```go
func TestReviewSystemPromptAsksForPoints_AllVoices(t *testing.T) {
	for _, v := range []Voice{VoiceBoard, VoiceSceptic, VoiceLayperson, VoiceExecutioner} {
		p := reviewSystemPrompt(v, false)
		if !strings.Contains(p, "points") {
			t.Fatalf("voice %s: prompt missing points instruction", v)
		}
	}
}

func TestProposeReviewParsesPoints(t *testing.T) {
	prov := &fakeProvider{text: `[{"criterion_code":"表D","band":"到达 identify","evidence":"有一手源","missing":"孤儿证据没接上","fix":"把它接到主张","points":3}]`}
	criteria := []skills.ReviewCriterion{{Code: "表D", Name: "来源与证据", Points: 4}}
	items, _, err := ProposeReview(context.Background(), prov, gateway.Resolved{}, criteria,
		[]string{"第一段"}, "摘要", VoiceBoard, false)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(items) != 1 || items[0].Points != 3 {
		t.Fatalf("want points 3, got %+v", items)
	}
}

func TestProposeReviewPromptCarriesCriterionTotal(t *testing.T) {
	prov := &fakeProvider{text: `[{"criterion_code":"表D","band":"b","evidence":"e","missing":"m","fix":"f","points":2}]`}
	criteria := []skills.ReviewCriterion{{Code: "表D", Name: "来源与证据", Points: 4}}
	if _, _, err := ProposeReview(context.Background(), prov, gateway.Resolved{}, criteria,
		[]string{"第一段"}, "摘要", VoiceBoard, false); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if !strings.Contains(prov.lastUser, "4") {
		t.Fatalf("user prompt should carry the table total 4; got %q", prov.lastUser)
	}
}
```

> Note: this file already has a fake provider used by the existing `ProposeReview` tests (Slice 8/8b). REUSE it — do not add a second. If its type/field names differ from `fakeProvider{text, lastUser}`, adapt the three tests to the existing fake, and if it does not already capture the outgoing user message, add a `lastUser` capture to it (its `Collect`/chat method records `req.Messages[len-1].Content`). Confirm the existing helper's name before writing.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestReviewSystemPromptAsksForPoints|TestProposeReviewParsesPoints|TestProposeReviewPromptCarriesCriterionTotal' -v`
Expected: FAIL — `Points` undefined; prompt has no points instruction; total not in prompt.

- [ ] **Step 3: Add `Points` to both structs**

In `apps/api/internal/agent/review.go`, add `Points` to `ReviewItem`:

```go
type ReviewItem struct {
	CriterionCode string `json:"criterion_code"`
	CriterionName string `json:"criterion_name"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
	Points        int    `json:"points"` // descriptor points evidenced, 0..criterion total (RL-3: which cell, not a grade)
}
```

and to `reviewItemWire`:

```go
type reviewItemWire struct {
	CriterionCode string `json:"criterion_code"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
	Points        int    `json:"points"`
}
```

- [ ] **Step 4: Add the (voice-invariant) points instruction + total in the prompt, and copy `Points` across**

In `review.go`, add a shared const near the postures:

```go
// reviewPointsInstruction is appended for EVERY voice — points is assessment
// data (which descriptor cell the draft reaches), not part of the coaching
// lens, so it is voice-invariant. RL-3: points names a cell, never a grade.
const reviewPointsInstruction = `每个对象另外给出 points：这张表当前收到的证据够到第几分点，
取 0 到该表总分点之间的整数（题面已给出每张表的总分点）。points 只表示"落在评分表的哪一格"，不是预估分数。`
```

In `reviewSystemPrompt`, append it for all voices (before the over-budget lens):

```go
func reviewSystemPrompt(voice Voice, overBudget bool) string {
	var base string
	switch voice {
	case VoiceSceptic:
		base = reviewPostureSceptic
	case VoiceLayperson:
		base = reviewPostureLayperson
	case VoiceExecutioner:
		base = reviewPostureExecutioner
	default:
		base = reviewPosturePrompt
	}
	base = base + "\n" + reviewPointsInstruction
	if overBudget {
		return base + "\n" + reviewOverBudgetLens
	}
	return base
}
```

In `ProposeReview`, carry each table's total into the codes list (change the loop that builds `codes`):

```go
	name := map[string]string{}
	codes := make([]string, 0, len(criteria))
	for _, c := range criteria {
		name[c.Code] = c.Name
		codes = append(codes, fmt.Sprintf("%s（%s，共 %d 分点）", c.Code, c.Name, c.Points))
	}
```

and copy `Points` when building each `ReviewItem` (the append near the end):

```go
		items = append(items, ReviewItem{
			CriterionCode: wv.CriterionCode, CriterionName: nm,
			Band: wv.Band, Evidence: wv.Evidence, Missing: wv.Missing, Fix: wv.Fix,
			Points: wv.Points,
		})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -v`
Expected: PASS (full agent package — confirms the new fields/prompt didn't break existing review/voice/budget tests).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/review.go apps/api/internal/agent/review_test.go
git commit -m "feat(refactor2): Slice 9 T2 — review emits per-criterion points (voice-invariant)"
```

---

### Task 3: Projection — `GaugeDTO` + `projectReadiness`

**Files:**
- Modify: `apps/api/internal/studio/dto.go` (`StudioProjection` ~7–37; add `GaugeDTO` + `Readiness` field)
- Modify: `apps/api/internal/studio/projection.go` (`Project` return ~300–311; add `projectReadiness`)
- Modify: `apps/api/internal/studio/dto_parity_test.go` (Go-side key set — Zod side is Task 4)
- Test: `apps/api/internal/studio/projection_test.go`

**Interfaces:**
- Consumes: `agent.ReviewItem.Points` (Task 2); `skills.ReviewCriterion.Points` (Task 1); the existing board-voice anchor convention (`projectWriting` reads `struct{ Kind, ID, Voice string }` from `iv.Anchor`, projection.go ~351).
- Produces: `studio.GaugeDTO{Code, Name string, Lit, Total int, Note, Level string}` and `StudioProjection.Readiness []GaugeDTO` (`json:"readiness"`, always non-nil `[]`). Consumed by Task 4 (Zod parity) and Task 5 (client).

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/studio/projection_test.go`. These reuse the package's existing Docker-backed seeding harness — model them on the existing `projectWriting`/review projection tests (same helper that seeds a project, a `draft_snapshot`, and `review_item` interventions). The four cases:

```go
func TestProjectReadiness_LitFromBoardReview(t *testing.T) {
	// Seed: latest snapshot S; a board-voice review_item for 表D with points 3.
	// Skill config: 表D total 4, 表E 4, 表F 3, 表H 3.
	// Expect gauge 表D {lit:3,total:4,level:"partial",note:<missing>}; 表E/F/H {lit:0,level:"empty"}.
	proj := seedAndProject(t, /* board review_item: 表D band/missing, points 3 */)
	g := gaugeByCode(proj.Readiness, "表D")
	if g.Lit != 3 || g.Total != 4 || g.Level != "partial" {
		t.Fatalf("表D want 3/4 partial, got %+v", g)
	}
	if e := gaugeByCode(proj.Readiness, "表E"); e.Lit != 0 || e.Level != "empty" {
		t.Fatalf("表E want 0 empty, got %+v", e)
	}
	if len(proj.Readiness) != 4 {
		t.Fatalf("want 4 gauges, got %d", len(proj.Readiness))
	}
}

func TestProjectReadiness_BoardVoiceOnly(t *testing.T) {
	// Seed: only a SCEPTIC-voice review_item for 表D with points 4. No board item.
	// Expect all gauges empty (sceptic never lights readiness).
	proj := seedAndProject(t, /* sceptic review_item: 表D points 4 */)
	if g := gaugeByCode(proj.Readiness, "表D"); g.Lit != 0 || g.Level != "empty" {
		t.Fatalf("表D want 0 empty (sceptic ignored), got %+v", g)
	}
}

func TestProjectReadiness_EmptyBeforeReview(t *testing.T) {
	// Seed: a latest snapshot, NO review_item interventions.
	// Expect 4 gauges, all lit 0 / empty, from config alone.
	proj := seedAndProject(t /* no reviews */)
	if len(proj.Readiness) != 4 {
		t.Fatalf("want 4 config gauges pre-review, got %d", len(proj.Readiness))
	}
	for _, g := range proj.Readiness {
		if g.Lit != 0 || g.Level != "empty" || g.Total < 1 {
			t.Fatalf("pre-review gauge should be 0/N empty, got %+v", g)
		}
	}
}

func TestProjectReadiness_ClampsAndFull(t *testing.T) {
	// Seed: board review_item for 表F (total 3) with points 9 (out of range).
	// Expect lit clamped to 3, level "full".
	proj := seedAndProject(t, /* board review_item: 表F points 9 */)
	if g := gaugeByCode(proj.Readiness, "表F"); g.Lit != 3 || g.Level != "full" {
		t.Fatalf("表F want 3/3 full (clamped), got %+v", g)
	}
}
```

Add a small local helper at the bottom of the test file:

```go
func gaugeByCode(gs []GaugeDTO, code string) GaugeDTO {
	for _, g := range gs {
		if g.Code == code {
			return g
		}
	}
	return GaugeDTO{}
}
```

> Note: match `seedAndProject` to the ACTUAL seeding helper this test file already uses for review-projection cases (find the existing test that seeds a `review_item` intervention with a `{kind,id,voice}` anchor + a body of marshalled `agent.ReviewItem`, and reuse its setup verbatim, only changing the body's `points` and `voice`). A board item = anchor `voice:"board"` (or the `voice` key omitted). Assert against `StudioProjection.Readiness`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectReadiness -v`
Expected: FAIL — `Readiness`/`GaugeDTO`/`projectReadiness` undefined.

- [ ] **Step 3: Add `GaugeDTO` + the `Readiness` field**

In `apps/api/internal/studio/dto.go`, add after the `WritingDTO` block:

```go
// GaugeDTO is one 0457 mark-scheme table on the 就绪度 readiness display: the
// descriptor cell the latest BOARD-voice review placed the draft in. Lit/Total
// are lamp counts (which cell), never a predicted grade (RL-3). Note is the
// review's own `missing` — what's absent, so the next step is legible; "" when
// the table is full. Level is derived from lit/total.
type GaugeDTO struct {
	Code  string `json:"code"`  // 表D..表H
	Name  string `json:"name"`  // 来源与证据 / 分析 / 评估 / 表达与组织
	Lit   int    `json:"lit"`   // clamp(review.points, 0, total); 0 when no board review
	Total int    `json:"total"` // skill config points
	Note  string `json:"note"`  // review.missing; "" when full / no review
	Level string `json:"level"` // "full" | "partial" | "empty"
}
```

Add the field to `StudioProjection` (after `Writing WritingDTO`):

```go
	// Readiness projects the 评估 view's 就绪度 gauge (Slice 9): one card per 0457
	// review table (表D/E/F/H), lit from the latest board-voice whole-draft
	// review ⋈ skill config. Always the full config set (4 unlit cards
	// pre-review). See projectReadiness (projection.go).
	Readiness []GaugeDTO `json:"readiness"`
```

- [ ] **Step 4: Add `projectReadiness` and wire it into `Project`**

In `apps/api/internal/studio/projection.go`, add to the `Project` return:

```go
		Writing:       projectWriting(sk, d),
		Readiness:     projectReadiness(sk, d),
	}, nil
```

Add the function (place it right after `projectWriting`):

```go
// projectReadiness projects the 评估 view's 就绪度 gauge: one GaugeDTO per skill
// review criterion (0457 表D/E/F/H, config order), lit from the latest snapshot's
// BOARD-voice review (sceptic/layperson/executioner are coaching lenses and never
// light readiness — Slice 9 DEC-9.2/board-only). Always emits the full config set,
// so the display is stable and honest before any review (4 unlit cards). RL-3: lit
// is a descriptor cell, never a grade; the projection is the single clamp site.
func projectReadiness(sk skills.Skill, d ProjectData) []GaugeDTO {
	out := make([]GaugeDTO, 0, len(sk.ReviewCriteria))
	// Board-voice review items for the latest snapshot, keyed by criterion code.
	byCode := map[string]agent.ReviewItem{}
	if d.LatestSnapshot != nil {
		for _, iv := range d.Interventions {
			if iv.Type != "review_item" {
				continue
			}
			var a struct{ Kind, ID, Voice string }
			_ = json.Unmarshal(iv.Anchor, &a)
			if a.Kind != "draft_snapshot" || a.ID != d.LatestSnapshot.ID.String() {
				continue
			}
			if a.Voice != "" && a.Voice != "board" {
				continue // only the board voice is the assessment of record
			}
			var it agent.ReviewItem
			if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
				continue
			}
			byCode[it.CriterionCode] = it
		}
	}
	for _, c := range sk.ReviewCriteria {
		g := GaugeDTO{Code: c.Code, Name: c.Name, Total: c.Points, Level: "empty"}
		if it, ok := byCode[c.Code]; ok {
			lit := it.Points
			if lit < 0 {
				lit = 0
			}
			if lit > c.Points {
				lit = c.Points
			}
			g.Lit = lit
			g.Note = it.Missing
			switch {
			case g.Total > 0 && g.Lit == g.Total:
				g.Level = "full"
			case g.Lit == 0:
				g.Level = "empty"
			default:
				g.Level = "partial"
			}
		}
		out = append(out, g)
	}
	return out
}
```

- [ ] **Step 5: Update the Go-side parity key set**

In `apps/api/internal/studio/dto_parity_test.go`, add `readiness` to the top-level `StudioProjection` key set and a `GaugeDTO` entry (`code, name, lit, total, note, level`), matching how the file lists other DTOs' expected keys. (The Zod side is added in Task 4; this file checks the Go structs against the hand-listed expected keys, so it stays green here.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -v`
Expected: PASS (full studio package, including the Docker-backed readiness cases and parity).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/studio/dto.go apps/api/internal/studio/projection.go apps/api/internal/studio/projection_test.go apps/api/internal/studio/dto_parity_test.go
git commit -m "feat(refactor2): Slice 9 T3 — projectReadiness (board-voice review ⋈ config)"
```

---

### Task 4: Contracts — `Gauge` Zod schema + `readiness`

**Files:**
- Modify: `packages/contracts/src/studioState.ts` (add `Gauge`; add `readiness` to the `StudioProjection` schema)
- Test: `packages/contracts/src/studioState.test.ts` (or the existing studioState test file)

**Interfaces:**
- Consumes: the Go `GaugeDTO`/`Readiness` wire shape (Task 3).
- Produces: `Gauge` Zod schema + type `{code, name, lit, total, note, level}`; `StudioProjection.readiness: Gauge[]`. Consumed by Task 5.

- [ ] **Step 1: Write the failing test**

Add to the contracts studioState test file:

```ts
import { Gauge, StudioProjection } from "./studioState";

test("Gauge parses a readiness table", () => {
  const g = Gauge.parse({ code: "表D", name: "来源与证据", lit: 3, total: 4, note: "孤儿证据", level: "partial" });
  expect(g.total).toBe(4);
});

test("Gauge rejects an unknown level", () => {
  expect(() => Gauge.parse({ code: "表D", name: "x", lit: 0, total: 4, note: "", level: "sorta" })).toThrow();
});

test("StudioProjection carries a readiness array", () => {
  const keys = Object.keys((StudioProjection as any).shape);
  expect(keys).toContain("readiness");
});
```

> Note: match the import path/style and the projection-shape assertion to how this test file already checks other fields (e.g. `writing`). If the file inspects the schema differently, mirror that; the intent is (1) `Gauge` validates + rejects a bad level, (2) `readiness` is a required key on `StudioProjection`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/contracts && npm test -- studioState`
Expected: FAIL — `Gauge` is not exported; `readiness` not in shape.

- [ ] **Step 3: Add the `Gauge` schema + `readiness` field**

In `packages/contracts/src/studioState.ts`, add (near the other view schemas):

```ts
export const Gauge = z.object({
  code: z.string(),
  name: z.string(),
  lit: z.number().int(),
  total: z.number().int(),
  note: z.string(),
  level: z.enum(["full", "partial", "empty"]),
});
export type Gauge = z.infer<typeof Gauge>;
```

Add `readiness: z.array(Gauge),` to the `StudioProjection` object schema (beside `writing`).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/contracts && npm test`
Expected: PASS (full contracts suite).

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/src/studioState.test.ts
git commit -m "feat(refactor2): Slice 9 T4 — Gauge Zod schema + StudioProjection.readiness"
```

---

### Task 5: Client — map `readiness`, render the gauge per design

**Files:**
- Modify: `apps/web/src/studio/state.ts` (`GaugeFx` ~17–23)
- Modify: `apps/web/src/studio/StudioContainer.tsx` (`toStudioState` ~33–48)
- Modify: `apps/web/src/studio/views/ReviewView.tsx`
- Modify: `apps/web/src/studio/fixtures.ts` (`review:` block ~194–202)
- Test: `apps/web/src/studio/views/ReviewView.test.tsx`

**Interfaces:**
- Consumes: `StudioProjection.readiness: Gauge[]` (Task 4).
- Produces: the 评估 view rendering real gauges + summary. `GaugeFx` is now the contract `Gauge` type (`{code, name, lit, total, note, level}`).

- [ ] **Step 1: Write the failing tests**

Replace/extend `apps/web/src/studio/views/ReviewView.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { ReviewView } from "./ReviewView";
import type { GaugeFx } from "../state";

const gauges: GaugeFx[] = [
  { code: "表D", name: "来源与证据", lit: 3, total: 4, note: "孤儿证据没接上", level: "partial" },
  { code: "表E", name: "分析", lit: 0, total: 4, note: "", level: "empty" },
  { code: "表F", name: "评估", lit: 3, total: 3, note: "", level: "full" },
  { code: "表H", name: "表达与组织", lit: 3, total: 3, note: "", level: "full" },
];

test("renders code and name for each table", () => {
  render(<ReviewView gauges={gauges} />);
  expect(screen.getByText("表D")).toBeInTheDocument();
  expect(screen.getByText("来源与证据")).toBeInTheDocument();
});

test("renders the summary line from lamp sums", () => {
  render(<ReviewView gauges={gauges} />);
  // Σlit = 3+0+3+3 = 9, Σtotal = 4+4+3+3 = 14
  expect(screen.getByText("已点亮 9/14 格")).toBeInTheDocument();
});

test("empty readiness shows 0/N and unlit cards", () => {
  const empty: GaugeFx[] = gauges.map((g) => ({ ...g, lit: 0, note: "", level: "empty" as const }));
  render(<ReviewView gauges={empty} />);
  expect(screen.getByText("已点亮 0/14 格")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/web && npm test -- ReviewView`
Expected: FAIL — `GaugeFx` has no `code`/`name`; no summary text.

- [ ] **Step 3: Re-type `GaugeFx` from the contract**

In `apps/web/src/studio/state.ts`, replace the hand-written `GaugeFx` with the contract type. Add `Gauge` to the existing contracts import and:

```ts
export type GaugeFx = Gauge;
```

(Remove the old `GaugeFx = { table; total; lit; note; level }` block. Re-export `Gauge` in the `export type { … }` line as needed so consumers keep importing `GaugeFx` from `../state`.)

- [ ] **Step 4: Map `readiness` in `toStudioState`**

In `apps/web/src/studio/StudioContainer.tsx`, change `review: []` to:

```tsx
      review: p.readiness,
```

- [ ] **Step 5: Render code+name+summary in `ReviewView`**

Rewrite `apps/web/src/studio/views/ReviewView.tsx` to match the binding design (`.dc.html` REVIEW view): per-card `code` + `name` (separate), the `lit/total` countLabel, the lamp row (`total` lamps, first `lit` filled), the `note`; plus the summary line top-right. Keep the 就绪度 heading + tooltip verbatim.

```tsx
import type { GaugeFx } from "../state";

export type ReviewViewProps = {
  gauges: GaugeFx[];
};

const LEVEL_COLOR: Record<GaugeFx["level"], string> = {
  full: "#4C9A82",
  partial: "#D9A23D",
  empty: "#AEB4C2",
};

function GaugeCard({ g }: { g: GaugeFx }) {
  const color = LEVEL_COLOR[g.level];
  const lamps = Array.from({ length: g.total }, (_, i) => i < g.lit);
  return (
    <div
      data-testid={`gauge-${g.code}`}
      style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "14px 16px" }}
    >
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 800, color }}>{g.code}</span>
          <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{g.name}</span>
        </div>
        <span style={{ fontSize: 11.5, fontWeight: 800, color }}>
          {g.lit}/{g.total}
        </span>
      </div>
      <div style={{ display: "flex", gap: 5, margin: "11px 0 9px" }}>
        {lamps.map((lit, i) => (
          <span
            key={i}
            data-lamp={lit ? "lit" : "empty"}
            style={{ flex: 1, height: 6, borderRadius: 4, background: lit ? color : "#EEF0F5" }}
          />
        ))}
      </div>
      <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.55 }}>{g.note}</div>
    </div>
  );
}

export function ReviewView({ gauges }: ReviewViewProps) {
  const litSum = gauges.reduce((n, g) => n + g.lit, 0);
  const totalSum = gauges.reduce((n, g) => n + g.total, 0);
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
      <div style={{ maxWidth: 720, margin: "0 auto" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, marginBottom: 8 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 7 }}>
            <div style={{ fontSize: 17, fontWeight: 800, color: "#1C2333" }}>就绪度</div>
            <span
              title="就绪度显示你的草稿现在落在评分表的哪一格，用来定位下一步，不是预估分数"
              style={{
                width: 16, height: 16, borderRadius: "50%", background: "#EEF0F5", color: "#9AA1B0",
                fontSize: 11, fontWeight: 700, display: "flex", alignItems: "center",
                justifyContent: "center", cursor: "help",
              }}
            >
              ?
            </span>
          </div>
          <div style={{ fontSize: 13.5, fontWeight: 700, color: "#6B7384" }}>
            已点亮 {litSum}/{totalSum} 格
          </div>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12, marginTop: 14 }}>
          {gauges.map((g) => (
            <GaugeCard key={g.code} g={g} />
          ))}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Replace the static fixture with a realistic 4-table readiness**

In `apps/web/src/studio/fixtures.ts`, replace the `review:` array (the 表A–表H block) with the 4 review tables:

```ts
    review: [
      { code: "表D", name: "来源与证据", lit: 3, total: 4, note: "已溯到一手源；仍有 1 条孤儿证据", level: "partial" },
      { code: "表E", name: "分析", lit: 2, total: 4, note: "「变绿→可持续」的跳步还没补上", level: "partial" },
      { code: "表F", name: "评估", lit: 1, total: 3, note: "对来源风险的评估还不足", level: "partial" },
      { code: "表H", name: "表达与组织", lit: 3, total: 3, note: "结构清楚、语言干净", level: "full" },
    ],
```

- [ ] **Step 7: Run tests + tsc to verify green**

Run: `cd apps/web && npm test -- ReviewView && npx tsc --noEmit`
Expected: PASS; tsc clean (confirms `GaugeFx` re-type + fixture + `toStudioState` all consistent).

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/studio/state.ts apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/views/ReviewView.tsx apps/web/src/studio/views/ReviewView.test.tsx apps/web/src/studio/fixtures.ts
git commit -m "feat(refactor2): Slice 9 T5 — 评估 gauge renders real readiness (code+name+summary)"
```

---

## Final gate (before the whole-branch review)

- `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...` — all packages `ok`.
- `cd apps/web && npm test && npx tsc --noEmit` — green + clean.
- `cd packages/contracts && npm test` — green.
- `cd apps/api && make sync-skills` — no drift (both `writing-project.json` copies already synced in T1).
- `git status` — only Slice-9 files touched; the pre-existing `M package.json` + untracked user files remain untouched.
```
