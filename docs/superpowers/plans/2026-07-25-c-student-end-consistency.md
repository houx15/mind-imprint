# Spec C · Student-End Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the student-end migration to the canonical assessment object — repair the broken 能力素养 model, close two A-axis rendering gaps, and verify cross-surface consistency.

**Architecture:** Pure projection work. The 能力素养 model (`internal/ability`) is a deterministic cross-session merge of `agent.Report[]`; the per-session report surfaces render the shared `<DualAxisReport>`. This spec deletes vestigial SOLO/metacognition scaffolding, fixes a radar hardcoded to 4 spokes (depth is now 6 dims), honestly renames two mislabeled autonomy counters, and gives `given_not_taken` its own visual state. No LLM, no migration, no sqlc, no new contract concepts.

**Tech Stack:** Go (`internal/ability` pure package), Zod contracts (`packages/contracts`), React/TS (`apps/web`), vitest.

## Global Constraints

- **No LLM, no migration, no sqlc, no new contract concepts.** Everything reads reports the engine already produces. (spec §0, §1 DEC-5)
- **Client never calls a model directly.** This spec adds zero LLM calls. Keys only in `apps/api` server env.
- **Go and the Zod contract are two parallel definitions that must stay aligned.** `internal/ability.Model` (camelCase JSON tags) mirrors `packages/contracts/src/ability.ts`. Rename/delete in both.
- **RL-5: the two axes never compose into a total score.** The ability model's blocks (depth + autonomy) never combine; depth `level=-1` = 证据不足 (fewer than 2 contributing sessions); autonomy carries counts, never a level.
- **Student-only.** Do NOT touch the teacher renderer `apps/web/src/console/TeacherReportView.tsx`. (spec DEC-3)
- **No 说人话 lint on the student end.** The student keeps its existing register. (spec §4, finalized-model §10 note)
- Icons are inline SVG; never add `lucide-react` or any icon dep.
- **Go test command (projection changes → FULL packages, never `-run` subsets):** `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./internal/ability/... ./internal/api/...`. **Subagents: the Bash tool auto-backgrounds past 120s and `internal/api` needs ~210s — pass `timeout: 600000`.**
- **Web:** `cd apps/web && npm test` + `npx tsc --noEmit`. **Contracts:** `cd packages/contracts && npm test` + `npx tsc --noEmit`.
- **Never `git add` a whole directory.** Name each file explicitly (there are unrelated pre-existing `M`/untracked entries in the tree that are NOT ours).

---

### Task 1: Go `internal/ability` — delete metacognition, rename autonomy counters

**Files:**
- Modify: `apps/api/internal/ability/ability.go`
- Modify: `apps/api/internal/ability/ability_test.go`
- Modify: `apps/api/internal/api/ability_test.go:33-56` (decode struct drops metacognition)

**Interfaces:**
- Consumes: `agent.Report` (`.DepthAxis[].Code/.Level`, `.AutonomyAxis[].Code/.Level/.Opportunity`), `rubric.DepthDims()`.
- Produces (later tasks mirror these JSON keys in the contract): `ability.Model{ TotalSessions int; Depth []DepthAbility; Autonomy AutonomyAbility }` where `AutonomyAbility{ Sessions, BoundarySettings, AdversaryInvites, OpportunitiesTaken, OpportunitiesMissed int }` with JSON keys `sessions/boundarySettings/adversaryInvites/opportunitiesTaken/opportunitiesMissed`. **No `Metacognition` field or type exists anymore.**

- [ ] **Step 1: Update the tests to the new shape (write the failing test)**

Replace `apps/api/internal/ability/ability_test.go` in full with:

```go
package ability

import (
	"testing"
	"time"

	"mindimprint/api/internal/agent"
)

// depthReport builds a Report carrying only the given depth-dim levels (by
// code) — the ability aggregator reads only Code+Level off DepthAxis.
func depthReport(levels map[string]string) agent.Report {
	dims := make([]agent.DepthDim, 0, len(levels))
	for code, level := range levels {
		dims = append(dims, agent.DepthDim{Code: code, Level: level})
	}
	return agent.Report{DepthAxis: dims}
}

// sig builds an AutonomySignal with just the fields the aggregator reads.
func sig(code string, level int, opportunity string) agent.AutonomySignal {
	return agent.AutonomySignal{Code: code, Level: level, Opportunity: opportunity}
}

func at(day int) time.Time { return time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC) }

func TestAggregateDepthRecencyWeightedAndLowNGuard(t *testing.T) {
	// D1 contributes [L1=1 (older), L3=3 (newer)] → weighted (0.6*1 + 1*3)/1.6 = 2.25 → level 2, evidence 2.
	// D3 contributes only [L2=2] once → below the 2-session guard → level -1.
	// D4 is "NA" both times → NA carries no evidence (never a low score) → level -1, evidence 0.
	// D5/D6 never appear → level -1, evidence 0.
	samples := []Sample{
		{Report: depthReport(map[string]string{"D1": "L1", "D3": "L2", "D4": "NA"}), CreatedAt: at(1)},
		{Report: depthReport(map[string]string{"D1": "L3", "D4": "NA"}), CreatedAt: at(2)},
	}
	m := Aggregate(samples)
	if m.TotalSessions != 2 {
		t.Fatalf("totalSessions = %d, want 2", m.TotalSessions)
	}
	if len(m.Depth) != 6 {
		t.Fatalf("depth dims = %d, want 6 (D1-D6 always)", len(m.Depth))
	}
	byCode := map[string]DepthAbility{}
	for _, d := range m.Depth {
		byCode[d.Code] = d
	}
	if d := byCode["D1"]; d.Level != 2 || d.EvidenceCount != 2 {
		t.Fatalf("D1 = level %d evidence %d, want level 2 evidence 2", d.Level, d.EvidenceCount)
	}
	if byCode["D1"].LevelLabel == "" {
		t.Fatalf("D1 level label empty, want the L2 anchor text")
	}
	if d := byCode["D3"]; d.Level != -1 || d.EvidenceCount != 1 {
		t.Fatalf("D3 = level %d evidence %d, want level -1 evidence 1 (low-N)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D4"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D4 = level %d evidence %d, want level -1 evidence 0 (NA excluded)", d.Level, d.EvidenceCount)
	}
	if d := byCode["D5"]; d.Level != -1 || d.EvidenceCount != 0 {
		t.Fatalf("D5 = level %d evidence %d, want level -1 evidence 0 (never scored)", d.Level, d.EvidenceCount)
	}
}

func TestAggregateAutonomy(t *testing.T) {
	// A3 level sums into BoundarySettings; A4 into AdversaryInvites.
	// OpportunitiesTaken counts given_taken; OpportunitiesMissed counts given_not_taken.
	// not_supplied counts toward neither. Metacognition/SOLO is gone from the model.
	samples := []Sample{
		{
			Report: agent.Report{
				DepthAxis:    []agent.DepthDim{{Code: "D6", Level: "L3"}},
				AutonomyAxis: []agent.AutonomySignal{sig("A1", 1, "given_taken"), sig("A2", 1, "given_taken"), sig("A3", 2, "given_taken"), sig("A4", 0, "not_supplied")},
			},
			CreatedAt: at(1),
		},
		{
			Report: agent.Report{
				DepthAxis:    []agent.DepthDim{{Code: "D6", Level: "L3"}},
				AutonomyAxis: []agent.AutonomySignal{sig("A3", 1, "given_not_taken"), sig("A4", 0, "not_supplied")},
			},
			CreatedAt: at(2),
		},
		{
			Report: agent.Report{
				DepthAxis: []agent.DepthDim{{Code: "D6", Level: "L4"}},
			},
			CreatedAt: at(3),
		},
	}
	m := Aggregate(samples)
	if m.Autonomy.Sessions != 3 {
		t.Fatalf("autonomy sessions = %d, want 3", m.Autonomy.Sessions)
	}
	if m.Autonomy.BoundarySettings != 3 || m.Autonomy.AdversaryInvites != 0 {
		t.Fatalf("autonomy sums = %+v, want boundary 3 adversary 0", m.Autonomy)
	}
	if m.Autonomy.OpportunitiesTaken != 3 || m.Autonomy.OpportunitiesMissed != 1 {
		t.Fatalf("autonomy opportunities = taken %d missed %d, want 3/1", m.Autonomy.OpportunitiesTaken, m.Autonomy.OpportunitiesMissed)
	}
}

func TestAggregateEmpty(t *testing.T) {
	m := Aggregate(nil)
	if m.TotalSessions != 0 || len(m.Depth) != 6 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 6", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail to COMPILE**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -count=1 ./internal/ability/...`
Expected: FAIL — build error, `m.Autonomy.OpportunitiesTaken undefined` (field is still `AnchoredSignals`).

- [ ] **Step 3: Rewrite `apps/api/internal/ability/ability.go`**

Replace the file in full with (metacognition block deleted, autonomy counters renamed):

```go
// Package ability projects a student's per-session DualAxis reports into a
// current-standing 能力素养 model. Pure — no store, no LLM (RL-5: the person-level
// view is a merge of many sessions' evidence, never a single-session 档位).
package ability

import (
	"math"
	"sort"
	"strconv"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/rubric"
)

// decay is the per-session recency weight base: the newest contributing session
// weighs 1, each older one 0.6× the next.
const decay = 0.6

type Sample struct {
	Report    agent.Report
	CreatedAt time.Time
}

// DepthAbility is one scored depth dim's merged current standing. Level -1 means
// "证据不足 · 需更多任务" (fewer than 2 contributing sessions) — the axiom made literal.
type DepthAbility struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Level         int    `json:"level"`
	LevelLabel    string `json:"levelLabel"`
	EvidenceCount int    `json:"evidenceCount"`
}

// AutonomyAbility is 智识自主 aggregated as observation counts — never a level.
// OpportunitiesTaken/Missed count A-signals by Opportunity (机会供给先于判定):
// given_taken = an opportunity offered and taken; given_not_taken = offered and
// not taken (a miss, not a low score). not_supplied counts toward neither.
type AutonomyAbility struct {
	Sessions            int `json:"sessions"`
	BoundarySettings    int `json:"boundarySettings"`
	AdversaryInvites    int `json:"adversaryInvites"`
	OpportunitiesTaken  int `json:"opportunitiesTaken"`
	OpportunitiesMissed int `json:"opportunitiesMissed"`
}

type Model struct {
	TotalSessions int             `json:"totalSessions"`
	Depth         []DepthAbility  `json:"depth"`
	Autonomy      AutonomyAbility `json:"autonomy"`
}

// levelToInt maps a depth dim's L1..L4 level to its ordinal 1..4. "NA" (no
// evidence) and any unrecognized value return ok=false — the caller must skip
// it, exactly as the old shape skipped a Score<1 (no evidence, never a low
// score).
func levelToInt(level string) (int, bool) {
	switch level {
	case "L1":
		return 1, true
	case "L2":
		return 2, true
	case "L3":
		return 3, true
	case "L4":
		return 4, true
	default:
		return 0, false
	}
}

// Aggregate merges the samples into a current-standing model. Defensive: sorts by
// CreatedAt ascending so recency weighting holds regardless of input order.
func Aggregate(samples []Sample) Model {
	sorted := make([]Sample, len(samples))
	copy(sorted, samples)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt.Before(sorted[j].CreatedAt) })

	m := Model{TotalSessions: len(sorted)}

	// Depth: one merged level per depth dim, in rubric order (always length 6).
	for _, dim := range rubric.DepthDims() {
		var scores []int // contributing (L1-L4, i.e. NOT NA) in oldest->newest order
		for _, s := range sorted {
			for _, d := range s.Report.DepthAxis {
				if d.Code != dim.ID {
					continue
				}
				if lvl, ok := levelToInt(d.Level); ok {
					scores = append(scores, lvl)
				}
			}
		}
		da := DepthAbility{Code: dim.ID, Name: dim.Name, EvidenceCount: len(scores), Level: -1}
		if len(scores) >= 2 {
			k := len(scores)
			var num, den float64
			for i, sc := range scores {
				w := math.Pow(decay, float64(k-1-i))
				num += w * float64(sc)
				den += w
			}
			da.Level = int(math.Round(num / den))
			da.LevelLabel = dim.Anchors["L"+strconv.Itoa(da.Level)]
		}
		m.Depth = append(m.Depth, da)
	}

	// Autonomy: A3 ("边界主权"-shaped signal) sums into BoundarySettings, A4
	// ("对抗与检验"-shaped signal) into AdversaryInvites; OpportunitiesTaken/
	// Missed count signals by Opportunity.
	for _, s := range sorted {
		m.Autonomy.Sessions++
		for _, a := range s.Report.AutonomyAxis {
			switch a.Code {
			case "A3":
				m.Autonomy.BoundarySettings += a.Level
			case "A4":
				m.Autonomy.AdversaryInvites += a.Level
			}
			switch a.Opportunity {
			case "given_taken":
				m.Autonomy.OpportunitiesTaken++
			case "given_not_taken":
				m.Autonomy.OpportunitiesMissed++
			}
		}
	}

	return m
}
```

- [ ] **Step 4: Drop metacognition from the HTTP test's decode struct**

In `apps/api/internal/api/ability_test.go`, edit the anonymous decode struct (lines 33-56) to remove the `Metacognition` field and its assertion. Replace the struct declaration + the two metacognition-related lines so the block reads:

```go
	var m struct {
		TotalSessions int `json:"totalSessions"`
		Depth         []struct {
			Code  string `json:"code"`
			Level int    `json:"level"`
		} `json:"depth"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode ability model: %v — body=%s", err, rec.Body)
	}
	if m.TotalSessions != 0 || len(m.Depth) != 6 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 6", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
	// deterministic: no model call happened.
```

(Delete the `Metacognition struct{ Distribution map[string]int ... }` field and the `if m.Metacognition.Distribution == nil { ... }` assertion; keep everything else in the test, including the `countAllLLMCalls` check.)

- [ ] **Step 5: Run the FULL Go packages to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./internal/ability/... ./internal/api/...`
Expected: PASS both packages (`internal/api` takes ~210s). No other package imports `ability.Metacognition`, so nothing else breaks.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/ability/ability.go apps/api/internal/ability/ability_test.go apps/api/internal/api/ability_test.go
git commit -m "feat(spec-c): drop metacognition, rename autonomy counters in ability model"
```

---

### Task 2: Contract + `AbilityModel.tsx` — mirror the shape, fix the 6-spoke radar, delete the SOLO panel

**Files:**
- Modify: `packages/contracts/src/ability.ts`
- Modify: `apps/web/src/shell/growth/AbilityModel.tsx`
- Modify: `apps/web/test/shell/growth/AbilityModel.test.tsx`
- Modify: `apps/web/test/api/ability.test.ts:8-18`

**Interfaces:**
- Consumes: `ability.Model` JSON from Task 1 (`autonomy.opportunitiesTaken/opportunitiesMissed`, no `metacognition`).
- Produces: contract `AbilityModel` with `autonomy: { sessions, boundarySettings, adversaryInvites, opportunitiesTaken, opportunitiesMissed }` and NO `metacognition`. Consumed by `AbilityModel.tsx` and `api/ability.ts` (`AbilityModel.parse`).

- [ ] **Step 1: Update the web tests (write the failing tests)**

Replace `apps/web/test/shell/growth/AbilityModel.test.tsx` in full with:

```tsx
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AbilityModel } from "@/shell/growth/AbilityModel";
import { api } from "@/api";

afterEach(() => { vi.restoreAllMocks(); });

// Six depth dims (D1–D6) so the radar exercises all six spokes; two low-N (-1).
const model = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "背景与目标清楚", evidenceCount: 4 },
    { code: "D2", name: "证据与信源", level: 3, levelLabel: "比较了信源立场", evidenceCount: 3 },
    { code: "D3", name: "论证结构", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "视角与偏见", level: 2, levelLabel: "承认样本局限", evidenceCount: 2 },
    { code: "D5", name: "反馈处理与修订", level: -1, levelLabel: "", evidenceCount: 0 },
    { code: "D6", name: "反思与元认知", level: 3, levelLabel: "能复盘策略", evidenceCount: 2 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, opportunitiesTaken: 18, opportunitiesMissed: 9 },
};

describe("AbilityModel", () => {
  it("renders the caption, a scored dim, the low-N guard, honest autonomy labels, and a 6-spoke radar with no NaN", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    const { container } = render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(screen.getByText(/任务理解与问题表述/)).toBeTruthy();
    expect(screen.getAllByText(/证据不足/).length).toBeGreaterThan(0); // D3 + D5
    expect(screen.getByText(/把握机会/)).toBeTruthy();  // relabeled autonomy panel
    expect(screen.getByText(/错过机会/)).toBeTruthy();
    expect(screen.getByText("D6")).toBeTruthy();        // 6th radar spoke label present
    expect(container.innerHTML).not.toContain("NaN");   // radar no longer degenerate
  });

  it("no longer renders a metacognition / SOLO panel or 自发/引导后 wording", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    const { container } = render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(container.textContent).not.toMatch(/SOLO/);
    expect(container.textContent).not.toMatch(/自发/);
    expect(container.textContent).not.toMatch(/引导后/);
  });

  it("renders an empty state when there are no sessions", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue({ ...model, totalSessions: 0, depth: model.depth.map((d) => ({ ...d, level: -1, evidenceCount: 0 })) } as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
});
```

Then in `apps/web/test/api/ability.test.ts`, replace the `body` object's `autonomy` line and delete its `metacognition` line (lines 16-17) so the body reads:

```ts
      autonomy: { sessions: 2, boundarySettings: 3, adversaryInvites: 0, opportunitiesTaken: 3, opportunitiesMissed: 1 },
    };
```

(i.e. remove the `metacognition: { ... }` line entirely — the contract no longer has that field, and `.parse` would strip the old `anchoredSignals`/`promptedSignals` leaving the required new fields undefined.)

- [ ] **Step 2: Run the web tests to verify they fail**

Run: `cd apps/web && npx vitest run test/shell/growth/AbilityModel.test.tsx test/api/ability.test.ts`
Expected: FAIL — `AbilityModel.test.tsx` still finds SOLO panel text / no 把握机会; `ability.test.ts` parse rejects (missing `opportunitiesTaken`). (tsc will also fail once the contract changes in Step 3 until the component is updated in the same step.)

- [ ] **Step 3: Update the contract `packages/contracts/src/ability.ts`**

Replace the file in full:

```ts
import { z } from "zod";

// AbilityModel: the student-level 能力素养 model — a cross-session merge of the
// DualAxis reports. Mirrors apps/api/internal/ability.Model (camelCase). RL-5: the
// blocks never combine into a total; a depth level of -1 = 证据不足 (fewer than
// 2 contributing sessions); 智识自主 carries counts, never a level.
export const AbilityDepth = z.object({
  code: z.string(),
  name: z.string(),
  level: z.number().int(),        // -1 = insufficient, else 1..4 (L1..L4 ordinal)
  levelLabel: z.string(),
  evidenceCount: z.number().int().min(0),
});

export const AbilityModel = z.object({
  totalSessions: z.number().int().min(0),
  depth: z.array(AbilityDepth),
  autonomy: z.object({
    sessions: z.number().int().min(0),
    boundarySettings: z.number().int().min(0),
    adversaryInvites: z.number().int().min(0),
    opportunitiesTaken: z.number().int().min(0),
    opportunitiesMissed: z.number().int().min(0),
  }),
});
export type AbilityModel = z.infer<typeof AbilityModel>;
```

- [ ] **Step 4: Update `apps/web/src/shell/growth/AbilityModel.tsx`**

Four edits:

(a) Replace the top constant + `radarPoints` (lines 5-18) with an N-spoke version:

```tsx
const RADAR_MAX = 4; // depth ordinals are 1..4 (L1..L4); insufficient (-1) → center

// N spokes evenly spaced from 12 o'clock clockwise; value 1..4 → radius fraction.
function radarPoints(levels: number[], cx: number, cy: number, r: number): string {
  const n = levels.length;
  return levels
    .map((lv, i) => {
      const frac = lv < 0 ? 0 : lv / RADAR_MAX;
      const a = ((-90 + (i * 360) / n) * Math.PI) / 180;
      return `${cx + Math.cos(a) * r * frac},${cy + Math.sin(a) * r * frac}`;
    })
    .join(" ");
}
```

(b) In the radar `<svg>` block, make the ring polygons and the axis labels 6-aware. Replace the ring `.map` and the label `.map` so the rings span all depth dims and each label's angle is computed, not indexed:

```tsx
            {[0.33, 0.66, 1].map((ring) => (
              <polygon key={ring} points={radarPoints(model.depth.map(() => RADAR_MAX * ring), cx, cy, r)} fill="none" stroke="#ECEEF4" strokeWidth="1" />
            ))}
            <polygon points={radarPoints(levels, cx, cy, r)} fill="rgba(42,59,122,.14)" stroke="#2A3B7A" strokeWidth="2" strokeLinejoin="round" />
            {model.depth.map((d, i) => {
              const a = ((-90 + (i * 360) / model.depth.length) * Math.PI) / 180;
              return <text key={d.code} x={cx + Math.cos(a) * (r + 16)} y={cy + Math.sin(a) * (r + 16)} textAnchor="middle" fontSize="10.5" fontWeight="600" fill="#6B7384">{d.code}</text>;
            })}
```

Also fix the stale comment above the radar: `{/* depth radar over the 4 scored dims */}` → `{/* depth radar over the 6 depth dims */}`.

(c) Relabel the 智识自主 observation panel line — replace the `自发信号 / 引导后` text with the honest counts:

```tsx
        <div style={{ fontSize: 12.5, color: "#4C5653", marginTop: 8, lineHeight: 1.7 }}>
          跨 {model.autonomy.sessions} 次会话：边界设定 ×{model.autonomy.boundarySettings} · 对手邀请 ×{model.autonomy.adversaryInvites} · 把握机会 {model.autonomy.opportunitiesTaken} / 错过机会 {model.autonomy.opportunitiesMissed}
        </div>
```

(d) Delete the entire `{/* 跨轴 元认知 distribution panel */}` block (the `<div>` rendering `元认知 · SOLO 分布` / `highestSolo` / `distribution` / `spontaneous` / `prompted`).

- [ ] **Step 5: Run contracts + web to verify pass**

Run: `cd packages/contracts && npm test && npx tsc --noEmit`
Expected: PASS.
Run: `cd apps/web && npx vitest run test/shell/growth/AbilityModel.test.tsx test/api/ability.test.ts && npx tsc --noEmit`
Expected: PASS both; tsc clean (no lingering `metacognition`/`anchoredSignals` references).

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/ability.ts apps/web/src/shell/growth/AbilityModel.tsx apps/web/test/shell/growth/AbilityModel.test.tsx apps/web/test/api/ability.test.ts
git commit -m "feat(spec-c): 6-spoke ability radar, honest autonomy labels, drop SOLO panel"
```

---

### Task 3: `<DualAxisReport>` — distinguish given_not_taken, render autonomy promptEvidence

**Files:**
- Modify: `apps/web/src/shell/report/DualAxisReport.tsx:54-71` (`AutonomySignalCard`)
- Modify: `apps/web/test/shell/report/DualAxisReport.test.tsx`

**Interfaces:**
- Consumes: contract `AutonomySignal` (`opportunity: "given_taken" | "given_not_taken" | "not_supplied"`, `level: 0..5`, `promptEvidence: string`) — unchanged, already defined in `dualAxisReport.ts`.
- Produces: no new exports; a three-state autonomy card. **No contract change.**

- [ ] **Step 1: Add the failing tests**

Append two `it(...)` blocks inside the `describe("DualAxisReport", ...)` in `apps/web/test/shell/report/DualAxisReport.test.tsx` (the shared `report` fixture already has A3 & A6 as `given_not_taken`, A1 as `given_taken`):

```tsx
  it("marks given_not_taken as 机会已给·未接住, and given_taken carries no such marker", () => {
    render(<DualAxisReport report={report} />);
    const a3 = screen.getByText("边界主权").closest("article")!; // given_not_taken
    expect(a3.textContent).toMatch(/机会已给·未接住/);
    const a1 = screen.getByText("方向自主").closest("article")!; // given_taken
    expect(a1.textContent).not.toMatch(/机会已给·未接住/);
  });

  it("renders autonomy promptEvidence when non-empty", () => {
    const withPrompt: DualAxisReportT = {
      ...report,
      autonomyAxis: report.autonomyAxis.map((a) =>
        a.code === "A1" ? { ...a, promptEvidence: "R2：请只帮我列选项，别替我选。" } : a),
    };
    render(<DualAxisReport report={withPrompt} />);
    expect(screen.getByText(/R2：请只帮我列选项/)).toBeTruthy();
  });
```

- [ ] **Step 2: Run the report tests to verify they fail**

Run: `cd apps/web && npx vitest run test/shell/report/DualAxisReport.test.tsx`
Expected: FAIL — no `机会已给·未接住` marker; autonomy `promptEvidence` not rendered.

- [ ] **Step 3: Rewrite `AutonomySignalCard` in `apps/web/src/shell/report/DualAxisReport.tsx`**

Replace the function (lines 54-71) with a three-state card that also renders `promptEvidence`:

```tsx
function AutonomySignalCard({ a }: { a: DualAxisReportT["autonomyAxis"][number] }) {
  const notSupplied = a.opportunity === "not_supplied";
  const missed = a.opportunity === "given_not_taken";
  return (
    <article style={{ padding: "11px 0", borderBottom: "1px solid #F3F4F7" }}>
      <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 6, gap: 10, flexWrap: "wrap" }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{a.name}</span>
        {notSupplied ? (
          <span style={{ fontSize: 12, fontWeight: 700, color: "#8A92A3", background: "#F1F2F6", padding: "2px 10px", borderRadius: 999, fontStyle: "italic" }}>
            暂无·机会未提供
          </span>
        ) : (
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
            <span style={{ fontSize: 12, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 10px", borderRadius: 999 }}>Lv {a.level}</span>
            {missed ? (
              <span style={{ fontSize: 12, fontWeight: 700, color: "#B0682A", background: "#F7ECDD", padding: "2px 10px", borderRadius: 999 }}>机会已给·未接住</span>
            ) : null}
          </span>
        )}
      </header>
      <p style={{ fontSize: 12.5, color: "#8A92A3", margin: 0 }}>{a.evidence}</p>
      {a.promptEvidence ? (
        <p style={{ fontSize: 12, color: "#6B7384", margin: "4px 0 0" }}>提示词证据：{a.promptEvidence}</p>
      ) : null}
    </article>
  );
}
```

- [ ] **Step 4: Run the report tests to verify they pass**

Run: `cd apps/web && npx vitest run test/shell/report/DualAxisReport.test.tsx`
Expected: PASS. Note the pre-existing test "shows not_supplied as 暂无·机会未提供 with no numeric level" still passes (not_supplied path unchanged, still no `Lv`); "renders six autonomy-axis signals with level 0–5" still passes (given_taken A1 still shows `Lv 3`).

- [ ] **Step 5: Full web suite + tsc**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS all; tsc clean.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/report/DualAxisReport.tsx apps/web/test/shell/report/DualAxisReport.test.tsx
git commit -m "feat(spec-c): distinguish given_not_taken + render autonomy promptEvidence"
```

---

### Task 4: Cross-surface consistency verification (checklist, expected zero code)

**Files:**
- Modify (only if a real gap is found): the surface at fault.
- No test file by default — the deliverable is the recorded verification.

**Interfaces:** none. This is a read-only audit against spec §4.

- [ ] **Step 1: Verify chat/course render core-only**

Read `apps/web/src/shell/chat/ChatReport.tsx` and `apps/web/src/shell/courses/CourseReport.tsx`. Confirm each passes a `DualAxisReport` object into `<DualAxisReport report={...} />` and does not itself synthesize `officialProjection`/`workAndProcess`. Then confirm the backend does not emit those two blocks for chat/course scope — grep the Go assessment path:

Run: `cd apps/api && grep -rnE "OfficialProjection|WorkAndProcess" internal/agent internal/api | grep -iE "chat|course"`
Expected: no chat/course assignment of those fields (they are project-surface-only per Spec B). Record the result. If chat/course DO populate them, that is a real gap → open a fix.

- [ ] **Step 2: Verify no teacher-only affordance leaked into the shared student component**

Run: `cd apps/web && grep -nE "👍|👎|证据地图|EvidenceMap|thumbsUp|calibrat" src/shell/report/DualAxisReport.tsx`
Expected: no matches (those live only in `console/TeacherReportView.tsx` / `console/EvidenceMap.tsx`). Record the result.

- [ ] **Step 3: Verify four surfaces share one contract with no stale field reads**

Run: `cd apps/web && grep -rnE "anchoredSignals|promptedSignals|metacognition|highestSolo|\.solo" src`
Expected: no matches (Task 2 removed the last of these). If any remain, they are stale reads → fix.

- [ ] **Step 4: Record the verification and commit**

Write the three results (each PASS/`clean`, or the gap found + fix) into the task's report. If all three are clean, there is no code change — commit only the progress-ledger/report update produced by the SDD flow, or skip the commit if there is nothing tracked to commit. If a gap was fixed, commit the fix:

```bash
# only if a real gap was fixed in Steps 1-3:
git add <the-file-that-was-fixed>
git commit -m "fix(spec-c): <the consistency gap that was found>"
```

---

## Self-Review

**Spec coverage:**
- §2 Bucket 1 (ability repair): radar 6-spoke + `RADAR_MAX`→4 (Task 2 Step 4a/4b); delete metacognition Go+contract+panel (Task 1 Step 3, Task 2 Step 3/4d); rename autonomy counters (Task 1 Step 3, Task 2 Step 3/4c). ✓
- §3 Bucket 2 (A-axis rendering): given_not_taken third state + autonomy promptEvidence (Task 3). ✓
- §4 Bucket 3 (consistency verification): Task 4. ✓
- §5 invariants (RL-5, no-LLM, 机会供给先于判定): no total introduced; zero model calls added; given_not_taken visually separated. ✓
- §6 file list: every listed file has a task; `TeacherReportView.tsx` untouched. ✓
- §7 explicit-not-doing: no redesign, no teacher mirror, no student lint — none appear in tasks. ✓

**Placeholder scan:** No TBD/TODO; every code step carries full code; no "similar to Task N".

**Type consistency:** Go `OpportunitiesTaken`/`OpportunitiesMissed` (JSON `opportunitiesTaken`/`opportunitiesMissed`) match the contract fields and the web reads (`model.autonomy.opportunitiesTaken/Missed`) and the test mocks. `Metacognition` removed everywhere it was referenced (Go struct, Go tests, HTTP decode struct, contract, component panel, component test, client test). `RADAR_MAX` = 4 consistent with depth ordinals 1..4. Radar angle formula `-90 + i*360/n` used identically in `radarPoints` and the label loop.
