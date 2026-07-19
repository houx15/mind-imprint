# B · DualAxis Assessment Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat 10-dim CT rubric assessor with the two-axis DualAxis model (认知深度 scored 0–3/12 · 智识自主 observed · 跨轴 元认知) plus SOLO per-round + 提示词透镜, rendered by one shared report component across the project/course/chat surfaces.

**Architecture:** All three surfaces already share one `agent.Assess(...)` call + one rubric. We add the DualAxis engine, config, DTO, and contracts **additively** under permanent new names (`agent.Report` / `agent.AssessReport` / `studio.ReportDTO` / contracts `DualAxisReport` / `rubric.Model()`), flip each surface to them, clear stored rows with a clean-slate migration, then delete the flat originals in a final cleanup task. Every commit compiles; every package's own suite stays green at every commit.

**Tech Stack:** Go (`net/http`, `pgx`, sqlc, goose, `go:embed`), React + Vite + TypeScript + Zod contracts, Postgres. Assessor is one isolated flagship call.

**Spec:** `docs/superpowers/specs/2026-07-19-b-dualaxis-assessment-model-design.md`

## Global Constraints

Every task's requirements implicitly include these:

- **RL-5:** diagnostic only; no rank, no 人级档位判定, no aggregate. The **only** number in the whole report is the within-axis depth subtotal (`depthAxis.subtotal`, out of 12). Autonomy and cross-axis carry **no** score field.
- **Axiom, verbatim** (in config, assessor prompt, and report header): `两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定`
- **旗舰绝不降级:** the assessor resolves via `a.d.EvalResolver(ctx)`; tests assert persisted `tier == "flagship"` (or the resolver's tier). Never the coach loop.
- **Cost-on-reject:** every assessment call records an `llm_call` (`Purpose:"assessment"`) even when the report is rejected — the cost happened.
- **One flagship call:** the entire structured report comes from a single `gateway.Collect`. No second model call.
- **Uniform across surfaces:** identical report shape + one shared `<DualAxisReport>` component for project/course/chat.
- **Single-source config:** `packages/contracts/src/dualaxis.json` is the one truth; the Go copy at `apps/api/internal/rubric/dualaxis.json` is a byte-mirror produced by `make sync-rubric` — never hand-edit the Go copy.
- **make sqlc** from `apps/api` after any `queries/*.sql` change; never hand-edit `apps/api/internal/store/sqlc/*`. **make sync-rubric** after editing `dualaxis.json`.
- **Full Go packages**, never `-run` subsets, for migration/query/DTO/endpoint/config changes: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- Web tests from `apps/web`; contracts tests from `packages/contracts`. Icons inline SVG, never lucide-react.
- **Direct-merge to `main` + push** (no PR) at the end. Never `git add` a whole directory — name files. Pre-existing `M package.json` + untracked user files under `docs/` and repo root are **not** ours.
- Depth `0` = 证据不足 (rendered neutrally, off the scored bar). Missing SOLO/cross level → `NA`.

## File Structure

**Create:**
- `packages/contracts/src/dualaxis.json` — single-source config (axes, 6 dims, anchors, tiers, SOLO levels, axiom).
- `apps/api/internal/rubric/dualaxis.json` — synced Go byte-mirror (written by `make sync-rubric`).
- `apps/api/internal/rubric/dualaxis.go` — Go model types + `Model()` accessor.
- `apps/api/internal/rubric/dualaxis_test.go` — model parse/validation.
- `apps/api/internal/agent/assess_report.go` — `Report` structs + `AssessReport` engine + wire + enforcement + axiom checks.
- `apps/api/internal/agent/assess_report_prompt.go` — DualAxis system/user prompt.
- `apps/api/internal/agent/assess_report_test.go` — engine unit tests.
- `apps/api/internal/studio/report_dto.go` — `ReportDTO` + `ToReportDTO`.
- `apps/api/internal/studio/report_dto_test.go` — DTO mapping test.
- `apps/api/internal/store/migrations/0027_dualaxis_clean_slate.sql` — clears evaluation rows.
- `apps/api/internal/store/migrations/migrate_0027_test.go` — migration test.
- `packages/contracts/src/dualAxisReport.ts` — `DualAxisReport` Zod schema.
- `packages/contracts/test/dualAxisReport.test.ts` — contract tests.
- `apps/web/src/shell/report/DualAxisReport.tsx` — shared report component.
- `apps/web/src/shell/report/DualAxisReport.test.tsx` — component tests.

**Modify:**
- `apps/api/internal/agent/assess_input.go` — add `Round` type + `Rounds` field + `BuildAssessmentInput` rounds param.
- `apps/api/tools/syncrubric/main.go` + `apps/api/Makefile` — sync `dualaxis.json` too.
- `apps/api/internal/rubric/rubric.go` — deleted in cleanup (Task 10).
- `apps/api/internal/api/assessment.go`, `project_finish.go` — project surface → `AssessReport`/`ReportDTO` + rounds.
- `apps/api/internal/api/course_assessment.go`, `chat_assessment.go`, `course_assessment_input.go` — course/chat → `AssessReport`/`ReportDTO` + rounds.
- `apps/api/internal/api/growth_history.go` + `apps/api/internal/store/queries/evaluation.sql` consumers — decode new scores shape.
- `packages/contracts/src/rubric.ts` — add DualAxis model; keep `SoloLevel`.
- `packages/contracts/src/growthHistory.ts` — `report: DualAxisReport`.
- `packages/contracts/src/index.ts` — barrel exports.
- `apps/web/src/api/{projects,assessment,courseAssessment,chatAssessment,growth,index}.ts` — parse `DualAxisReport`.
- `apps/web/src/shell/growth/GrowthReport.tsx`, `shell/courses/CourseReport.tsx`, `shell/chat/ChatReport.tsx` — render `<DualAxisReport>`.
- Their `.test.tsx` files + api client tests — DualAxis fixtures.

**Delete (Task 10):** `apps/api/internal/rubric/rubric.go`, `apps/api/internal/rubric/ct-rubric.json`, `apps/api/internal/agent/assess.go` (old `Assess`/`Assessment`), `assess_prompt.go`, `anchors.json`, `apps/api/internal/studio/assessment_dto.go`, `packages/contracts/src/assessment.ts`, `packages/contracts/src/ct-rubric.json`, and the ct-rubric arm of `syncrubric`.

**Note on few-shot anchors (YAGNI):** the DualAxis engine takes **no** few-shot anchors (the flagship model + rich ladders + discipline notes suffice). `anchors.json`/`EmbeddedAnchors()` die in cleanup.

---

### Task 1: DualAxis config + Go rubric `Model()`

**Files:**
- Create: `packages/contracts/src/dualaxis.json`
- Create: `apps/api/internal/rubric/dualaxis.go`
- Create: `apps/api/internal/rubric/dualaxis_test.go`
- Modify: `apps/api/tools/syncrubric/main.go`, `apps/api/Makefile`

**Interfaces:**
- Produces: `rubric.Model() DualAxis`; `DualAxis{ID,Name,Axiom string; Axes map[string]Axis; Dimensions []AxisDim; PromptTiers []PromptTier; SoloLevels []SoloLevel}`; `AxisDim{ID,Axis,Name string; Anchors map[string]string; ObservationGuide,Guide string}`; `Axis{Name,Scoring string; Max int}`; `PromptTier{Tier,Label string}`; `SoloLevel{Level,Name string}`. Helpers `DepthDims() []AxisDim`, `AutonomyDim() AxisDim`, `CrossDim() AxisDim`.

- [ ] **Step 1: Create the config file** `packages/contracts/src/dualaxis.json` (authored, no placeholders):

```json
{
  "id": "dualaxis",
  "name": "AI 批判性思维 · 双轴模型",
  "axiom": "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  "axes": {
    "depth":    { "name": "认知深度", "scoring": "score", "max": 3 },
    "autonomy": { "name": "智识自主", "scoring": "observation", "max": 0 },
    "cross":    { "name": "元认知",   "scoring": "descriptive", "max": 0 }
  },
  "dimensions": [
    { "id": "D1", "axis": "depth", "name": "任务理解与问题表述", "anchors": {
      "0": "未提出可评估的任务表述 / 证据不足",
      "1": "直接抛出问题，不交代背景、目标或验收标准",
      "2": "给出任务背景与目标，但约束或验收标准仍模糊",
      "3": "主动交代任务背景、目标与验收标准，并能把绝对化命题改成有限定的判断" } },
    { "id": "D3", "axis": "depth", "name": "证据与信源意识", "anchors": {
      "0": "未涉及证据或来源 / 证据不足",
      "1": "完全信任来源，不追问出处",
      "2": "能追到原始来源并识别来源等级",
      "3": "溯到一手出处、识别反方证据，并能分析来源立场与证据适用范围" } },
    { "id": "D4", "axis": "depth", "name": "论证结构意识", "anchors": {
      "0": "未展开论证 / 证据不足",
      "1": "把观点当事实，不区分论点与论据",
      "2": "能拆分 claim–evidence–reasoning",
      "3": "识别证据与结论间缺失的 warrant，并能用让步段协调主张与反主张" } },
    { "id": "D5", "axis": "depth", "name": "反馈理解与修改理由", "anchors": {
      "0": "未对反馈作出可见处置 / 证据不足",
      "1": "被动接受或整段照搬 AI 输出",
      "2": "能采纳部分建议，但未说明理由",
      "3": "能分别说明采纳与拒绝的理由，先自改再求反馈，主体性清晰" } },
    { "id": "D2", "axis": "autonomy", "name": "学生主体性 / AI 依赖度",
      "observationGuide": "以观察语言描述学生的主体性与 AI 依赖度，不打分。记录：入场是否自设边界（如「不要直接重写」），是否先自改再求反馈，是否说明采纳/拒绝理由（A 类·能力锚定信号）；哪些行为是被 AI 追问/提示后才出现（引导后信号）；主动召唤反方/对手的次数（对手邀请）。" },
    { "id": "D6", "axis": "cross", "name": "元认知与反思",
      "guide": "跨轴维度，同时描述深度面与自主面，不单独打分。深度面：以 SOLO 层级（L1–L4）描述反思质量；自主面：标注反思是自发还是引导后完成。用「能反思，尚未自发反思」这类语言表达两面的差。" }
  ],
  "promptTiers": [
    { "tier": "P0", "label": "应答轮" },
    { "tier": "P1", "label": "要成品" },
    { "tier": "P2", "label": "要判断" },
    { "tier": "P3", "label": "要过程·设边界" }
  ],
  "soloLevels": [
    { "level": "L1", "name": "单点" },
    { "level": "L2", "name": "多点" },
    { "level": "L3", "name": "关联" },
    { "level": "L4", "name": "抽象扩展" }
  ]
}
```

- [ ] **Step 2: Write the failing test** `apps/api/internal/rubric/dualaxis_test.go`:

```go
package rubric

import "testing"

func TestModelParsesDualAxis(t *testing.T) {
	m := Model()
	if m.ID != "dualaxis" {
		t.Fatalf("id = %q, want dualaxis", m.ID)
	}
	if m.Axiom != "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定" {
		t.Fatalf("axiom mismatch: %q", m.Axiom)
	}
	if len(m.Dimensions) != 6 {
		t.Fatalf("dims = %d, want 6", len(m.Dimensions))
	}
	depth := DepthDims()
	if len(depth) != 4 {
		t.Fatalf("depth dims = %d, want 4", len(depth))
	}
	for _, d := range depth {
		if d.Axis != "depth" {
			t.Fatalf("dim %s axis = %q, want depth", d.ID, d.Axis)
		}
		for _, k := range []string{"0", "1", "2", "3"} {
			if d.Anchors[k] == "" {
				t.Fatalf("depth dim %s missing anchor %s", d.ID, k)
			}
		}
	}
	if AutonomyDim().ObservationGuide == "" {
		t.Fatalf("autonomy dim missing observationGuide")
	}
	if CrossDim().Guide == "" {
		t.Fatalf("cross dim missing guide")
	}
	if len(m.PromptTiers) != 4 || len(m.SoloLevels) != 4 {
		t.Fatalf("tiers=%d solo=%d, want 4/4", len(m.PromptTiers), len(m.SoloLevels))
	}
}
```

- [ ] **Step 3: Run it — expect FAIL** (no `dualaxis.json` embed yet):

Run: `cd apps/api && go build ./internal/rubric/...`
Expected: FAIL (`Model` undefined / no embedded file).

- [ ] **Step 4: Sync the config into the Go tree.** First extend the sync tool `apps/api/tools/syncrubric/main.go` to copy **both** files. The current tool copies `ct-rubric.json`; make it copy a list:

```go
package main

import (
	"log"
	"os"
	"path/filepath"
)

func main() {
	files := []string{"ct-rubric.json", "dualaxis.json"}
	for _, f := range files {
		src := filepath.Join("..", "..", "packages", "contracts", "src", f)
		dst := filepath.Join("internal", "rubric", f)
		data, err := os.ReadFile(src)
		if err != nil {
			log.Fatalf("syncrubric: read %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			log.Fatalf("syncrubric: write %s: %v", dst, err)
		}
		log.Printf("syncrubric: %s -> %s", src, dst)
	}
}
```

(If the existing tool differs structurally, preserve its copy mechanism and just add `dualaxis.json` to the set. The Makefile target `sync-rubric` at `apps/api/Makefile:12-13` already runs `go run ./tools/syncrubric` — no Makefile change needed unless the tool's package path changes; leave it.)

- [ ] **Step 5: Run the sync**

Run: `cd apps/api && make sync-rubric`
Expected: logs both copies; `apps/api/internal/rubric/dualaxis.json` now exists.

- [ ] **Step 6: Write the model** `apps/api/internal/rubric/dualaxis.go`:

```go
package rubric

import (
	_ "embed"
	"encoding/json"
)

//go:embed dualaxis.json
var dualaxisJSON []byte

type Axis struct {
	Name    string `json:"name"`
	Scoring string `json:"scoring"` // score | observation | descriptive
	Max     int    `json:"max"`
}

type AxisDim struct {
	ID               string            `json:"id"`
	Axis             string            `json:"axis"` // depth | autonomy | cross
	Name             string            `json:"name"`
	Anchors          map[string]string `json:"anchors"` // depth only: keys "0".."3"
	ObservationGuide string            `json:"observationGuide"`
	Guide            string            `json:"guide"`
}

type PromptTier struct {
	Tier  string `json:"tier"`
	Label string `json:"label"`
}

type SoloLevel struct {
	Level string `json:"level"`
	Name  string `json:"name"`
}

type DualAxis struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Axiom       string          `json:"axiom"`
	Axes        map[string]Axis `json:"axes"`
	Dimensions  []AxisDim       `json:"dimensions"`
	PromptTiers []PromptTier    `json:"promptTiers"`
	SoloLevels  []SoloLevel     `json:"soloLevels"`
}

var dualaxis = mustParseDual()

func mustParseDual() DualAxis {
	var m DualAxis
	if err := json.Unmarshal(dualaxisJSON, &m); err != nil {
		panic("rubric: bad embedded dualaxis.json: " + err.Error())
	}
	return m
}

// Model returns the parsed canonical DualAxis model.
func Model() DualAxis { return dualaxis }

func dimsByAxis(axis string) []AxisDim {
	out := make([]AxisDim, 0, 4)
	for _, d := range dualaxis.Dimensions {
		if d.Axis == axis {
			out = append(out, d)
		}
	}
	return out
}

// DepthDims returns the four scored depth-axis dimensions in config order.
func DepthDims() []AxisDim { return dimsByAxis("depth") }

// AutonomyDim returns the single autonomy-axis dimension.
func AutonomyDim() AxisDim {
	d := dimsByAxis("autonomy")
	if len(d) == 0 {
		return AxisDim{}
	}
	return d[0]
}

// CrossDim returns the single cross-axis dimension.
func CrossDim() AxisDim {
	d := dimsByAxis("cross")
	if len(d) == 0 {
		return AxisDim{}
	}
	return d[0]
}
```

- [ ] **Step 7: Run the test — expect PASS**

Run: `cd apps/api && go test ./internal/rubric/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add packages/contracts/src/dualaxis.json apps/api/internal/rubric/dualaxis.json apps/api/internal/rubric/dualaxis.go apps/api/internal/rubric/dualaxis_test.go apps/api/tools/syncrubric/main.go
git commit -m "feat(b): DualAxis single-source config + Go rubric.Model()"
```

---

### Task 2: Contracts — DualAxis model + `DualAxisReport` schema (additive)

**Files:**
- Modify: `packages/contracts/src/rubric.ts`
- Create: `packages/contracts/src/dualAxisReport.ts`
- Create: `packages/contracts/test/dualAxisReport.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `dualaxis.json` (Task 1), `SoloLevel` (existing in `rubric.ts`).
- Produces: `DUALAXIS_MODEL`, `PromptTier` (enum `P0|P1|P2|P3`), and the `DualAxisReport` Zod schema + inferred type with fields `depthAxis{dims[{code,name,score,evidence,promptEvidence}],subtotal}`, `autonomyAxis{code,name,observation,anchoredSignals[],promptedSignals[],adversaryInvites,promptEvidence}`, `crossAxis{code,name,depthLevel,initiative,prose,promptEvidence}`, `solo[{round,excerpt,level,rationale,initiative}]`, `promptLens{directiveRounds,totalRounds,boundarySettings,adversaryInvites,questions[{title,body}],bestPrompt{round,quote,annotation},takeaway{round,quote,annotation},perRound[{round,tier,label}]}`, `timeline[{round,task,prompt,pTag,dimTags[]}]`, `keyEvidence[{label,quote}]`, `guidance{anchored,prompted,risk,nextSteps[{title,body}]}`, `narrative`, `axiom`, `generatedAt`.

- [ ] **Step 1: Write the failing test** `packages/contracts/test/dualAxisReport.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { DualAxisReport } from "../src/dualAxisReport";

const sample = {
  depthAxis: {
    dims: [
      { code: "D1", name: "任务理解与问题表述", score: 3, evidence: "把绝对命题改成有限定判断", promptEvidence: "R4「我想把 thesis 改成…」" },
      { code: "D3", name: "证据与信源意识", score: 2, evidence: "溯到 NASA/Nature", promptEvidence: "" },
      { code: "D4", name: "论证结构意识", score: 3, evidence: "识别缺失 warrant", promptEvidence: "" },
      { code: "D5", name: "反馈理解与修改理由", score: 3, evidence: "说明采纳与拒绝理由", promptEvidence: "" },
    ],
    subtotal: 11,
  },
  autonomyAxis: { code: "D2", name: "学生主体性 / AI 依赖度", observation: "入场即设边界", anchoredSignals: ["R1 不要直接重写"], promptedSignals: ["R3 SIFT"], adversaryInvites: 0, promptEvidence: "" },
  crossAxis: { code: "D6", name: "元认知与反思", depthLevel: "L3", initiative: "引导后", prose: "能反思，尚未自发反思", promptEvidence: "" },
  solo: [{ round: 4, excerpt: "限定判断", level: "L3", rationale: "范围限定作为概念性组织者", initiative: "自发" }],
  promptLens: {
    directiveRounds: 3, totalRounds: 10, boundarySettings: 3, adversaryInvites: 0,
    questions: [{ title: "一问 · 任务说清了吗", body: "…" }],
    bestPrompt: { round: 8, quote: "请检查是否回扣 thesis，不要帮我润色", annotation: "材料✓ 任务✓ 边界✓ 验收✓" },
    takeaway: { round: 0, quote: "请扮演一个苛刻的审稿人…", annotation: "P4 模板" },
    perRound: [{ round: 1, tier: "P3", label: "要过程·设边界" }],
  },
  timeline: [{ round: 1, task: "上传草稿", prompt: "不要直接重写", pTag: "P3", dimTags: ["D1=2", "D2=3"] }],
  keyEvidence: [{ label: "任务理解", quote: "我想把 thesis 改成…" }],
  guidance: { anchored: "主动限定 thesis", prompted: "SIFT 核查", risk: "D3 仍停留在来源等级", nextSteps: [{ title: "下一步强化 D3", body: "跑一张 SIFT 记录" }] },
  narrative: "深度侧 L3 结构稳定复现，自主侧未主动召唤对手。",
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

describe("DualAxisReport", () => {
  it("parses a valid axis-structured report", () => {
    const r = DualAxisReport.parse(sample);
    expect(r.depthAxis.subtotal).toBe(11);
    expect(r.autonomyAxis.adversaryInvites).toBe(0);
    expect(r.crossAxis.depthLevel).toBe("L3");
  });

  it("rejects an unknown prompt tier", () => {
    const bad = { ...sample, promptLens: { ...sample.promptLens, perRound: [{ round: 1, tier: "P9", label: "x" }] } };
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });

  it("rejects an autonomy axis carrying a score field", () => {
    const bad = { ...sample, autonomyAxis: { ...sample.autonomyAxis, score: 3 } };
    // strict schema: extra keys rejected — autonomy must never carry a score
    expect(() => DualAxisReport.parse(bad)).toThrow();
  });
});
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd packages/contracts && npx vitest run test/dualAxisReport.test.ts`
Expected: FAIL (module `../src/dualAxisReport` not found).

- [ ] **Step 3: Create the schema** `packages/contracts/src/dualAxisReport.ts`:

```ts
import { z } from "zod";
import { SoloLevel } from "./rubric";

export const PromptTier = z.enum(["P0", "P1", "P2", "P3"]);
export type PromptTier = z.infer<typeof PromptTier>;

const DepthDim = z.object({
  code: z.string(),
  name: z.string(),
  score: z.number().int().min(0).max(3),
  evidence: z.string(),
  promptEvidence: z.string(),
}).strict();

const AutonomyAxis = z.object({
  code: z.string(),
  name: z.string(),
  observation: z.string(),
  anchoredSignals: z.array(z.string()),
  promptedSignals: z.array(z.string()),
  adversaryInvites: z.number().int().min(0),
  promptEvidence: z.string(),
}).strict(); // .strict() forbids a score field — the axiom, enforced by the type

const CrossAxis = z.object({
  code: z.string(),
  name: z.string(),
  depthLevel: SoloLevel,
  initiative: z.string(),
  prose: z.string(),
  promptEvidence: z.string(),
}).strict();

const SoloRow = z.object({
  round: z.number().int(),
  excerpt: z.string(),
  level: SoloLevel,
  rationale: z.string(),
  initiative: z.string(),
}).strict();

const PromptSample = z.object({
  round: z.number().int(),
  quote: z.string(),
  annotation: z.string(),
}).strict();

const PromptLens = z.object({
  directiveRounds: z.number().int().min(0),
  totalRounds: z.number().int().min(0),
  boundarySettings: z.number().int().min(0),
  adversaryInvites: z.number().int().min(0),
  questions: z.array(z.object({ title: z.string(), body: z.string() }).strict()),
  bestPrompt: PromptSample,
  takeaway: PromptSample,
  perRound: z.array(z.object({ round: z.number().int(), tier: PromptTier, label: z.string() }).strict()),
}).strict();

const TimelineRow = z.object({
  round: z.number().int(),
  task: z.string(),
  prompt: z.string(),
  pTag: z.string(),
  dimTags: z.array(z.string()),
}).strict();

// DualAxisReport: the whole growth report over the wire. Mirrors
// apps/api/internal/studio.ReportDTO byte-for-byte (camelCase). The ONLY number
// is depthAxis.subtotal (within-axis); autonomy/cross carry no score (RL-5 + axiom).
export const DualAxisReport = z.object({
  depthAxis: z.object({
    dims: z.array(DepthDim),
    subtotal: z.number().int().min(0).max(12),
  }).strict(),
  autonomyAxis: AutonomyAxis,
  crossAxis: CrossAxis,
  solo: z.array(SoloRow),
  promptLens: PromptLens,
  timeline: z.array(TimelineRow),
  keyEvidence: z.array(z.object({ label: z.string(), quote: z.string() }).strict()),
  guidance: z.object({
    anchored: z.string(),
    prompted: z.string(),
    risk: z.string(),
    nextSteps: z.array(z.object({ title: z.string(), body: z.string() }).strict()),
  }).strict(),
  narrative: z.string(),
  axiom: z.string(),
  generatedAt: z.string(),
}).strict();
export type DualAxisReport = z.infer<typeof DualAxisReport>;
```

- [ ] **Step 4: Add the DualAxis model to `rubric.ts`.** Append (keep everything existing — `SoloLevel`, `CT_RUBRIC`, etc. stay until Task 10):

```ts
import dualAxisJson from "./dualaxis.json";

export interface DualAxisDimension {
  id: string; axis: "depth" | "autonomy" | "cross"; name: string;
  anchors?: Record<string, string>; observationGuide?: string; guide?: string;
}
export interface DualAxisModel {
  id: string; name: string; axiom: string;
  axes: Record<string, { name: string; scoring: string; max: number }>;
  dimensions: DualAxisDimension[];
  promptTiers: { tier: string; label: string }[];
  soloLevels: { level: string; name: string }[];
}

const DualAxisModelSchema = z.object({
  id: z.string(), name: z.string(), axiom: z.string(),
  axes: z.record(z.object({ name: z.string(), scoring: z.string(), max: z.number() })),
  dimensions: z.array(z.object({
    id: z.string(), axis: z.enum(["depth", "autonomy", "cross"]), name: z.string(),
    anchors: z.record(z.string()).optional(),
    observationGuide: z.string().optional(), guide: z.string().optional(),
  })),
  promptTiers: z.array(z.object({ tier: z.string(), label: z.string() })),
  soloLevels: z.array(z.object({ level: z.string(), name: z.string() })),
});

export const DUALAXIS_MODEL: DualAxisModel = DualAxisModelSchema.parse(dualAxisJson) as DualAxisModel;

// assertModelComplete: every depth dim has all four 0–3 anchors; autonomy has a
// guide; cross has a guide.
export function assertModelComplete(m: DualAxisModel): void {
  for (const d of m.dimensions) {
    if (d.axis === "depth") {
      for (const k of ["0", "1", "2", "3"]) {
        if (!d.anchors?.[k]?.trim()) throw new Error(`dualaxis dim ${d.id} missing anchor ${k}`);
      }
    }
    if (d.axis === "autonomy" && !d.observationGuide?.trim()) throw new Error(`dualaxis dim ${d.id} missing observationGuide`);
    if (d.axis === "cross" && !d.guide?.trim()) throw new Error(`dualaxis dim ${d.id} missing guide`);
  }
}
assertModelComplete(DUALAXIS_MODEL);
```

Ensure `tsconfig`/vite `resolveJsonModule` is on (it already is — `ct-rubric.json` is imported the same way).

- [ ] **Step 5: Export from the barrel** `packages/contracts/src/index.ts` — add near the existing `export * from "./assessment";`:

```ts
export * from "./dualAxisReport";
```

(`DUALAXIS_MODEL`, `DualAxisModel`, `assertModelComplete` are exported via the existing `export * from "./rubric";`.)

- [ ] **Step 6: Run tests — expect PASS**

Run: `cd packages/contracts && npx vitest run test/dualAxisReport.test.ts test/rubric.test.ts && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 7: Commit**

```bash
git add packages/contracts/src/dualAxisReport.ts packages/contracts/src/rubric.ts packages/contracts/src/index.ts packages/contracts/test/dualAxisReport.test.ts
git commit -m "feat(b): contracts DualAxis model + DualAxisReport schema (additive)"
```

---

### Task 3: Go agent — per-round input + `AssessReport` engine + prompt (additive)

**Files:**
- Modify: `apps/api/internal/agent/assess_input.go`
- Create: `apps/api/internal/agent/assess_report.go`
- Create: `apps/api/internal/agent/assess_report_prompt.go`
- Create: `apps/api/internal/agent/assess_report_test.go`

**Interfaces:**
- Consumes: `rubric.Model()`, `rubric.DepthDims/AutonomyDim/CrossDim` (Task 1); `gateway.Provider/Resolved/Collect/ChatUsage`; `enforcement.BannedPhrasing`; existing `AssessmentInput`, `EventDigest`, `CardUse`, `DispositionUse`.
- Produces: `type Round struct{ N int; StudentPrompt, AiContext string }`; `AssessmentInput.Rounds []Round`; `BuildAssessmentInput(events, cards, dispositions, gates, wordCounts, reviewBands, graphSummary, rounds)` (rounds appended as the **8th** param); `agent.Report` (structs below); `AssessReport(ctx, prov, r, m rubric.DualAxis, in AssessmentInput) (Report, gateway.ChatUsage, error)`.

- [ ] **Step 1: Extend `assess_input.go`** — add `Round`, the `Rounds` field, and a rounds param to `BuildAssessmentInput`. Full replacement of the file:

```go
package agent

import "fmt"

type EventDigest struct {
	Type  string
	Order int
	Text  string
}

type CardUse struct {
	CardID    string
	Dimension string
	Spont     string // 自发 | 提示后
}

type DispositionUse struct {
	Kind   string // accept | rewrite | reject
	Reason string
}

// Round is one student turn in order: the student's prompt plus the AI ask/act
// that framed it. Drives SOLO per-round judging and the 提示词透镜.
type Round struct {
	N             int
	StudentPrompt string
	AiContext     string
}

type AssessmentInput struct {
	CardUses      []CardUse
	Dispositions  []DispositionUse
	GateProgress  []string
	SnapshotCount int
	WordCounts    []int
	ReviewBands   []string
	GraphSummary  string
	Timeline      []string
	Rounds        []Round
}

// BuildAssessmentInput digests the process record. Pure — no I/O. `rounds` is the
// ordered per-round student-turn stream (SOLO + prompt-lens); pass nil when a
// surface cannot supply it (dims fall to NA / empty honestly).
func BuildAssessmentInput(
	events []EventDigest,
	cards []CardUse,
	dispositions []DispositionUse,
	gates []string,
	wordCounts []int,
	reviewBands []string,
	graphSummary string,
	rounds []Round,
) AssessmentInput {
	timeline := make([]string, 0, len(events))
	for _, e := range events {
		timeline = append(timeline, fmt.Sprintf("%d. %s：%s", e.Order, e.Type, e.Text))
	}
	return AssessmentInput{
		CardUses:      cards,
		Dispositions:  dispositions,
		GateProgress:  gates,
		SnapshotCount: len(wordCounts),
		WordCounts:    wordCounts,
		ReviewBands:   reviewBands,
		GraphSummary:  graphSummary,
		Timeline:      timeline,
		Rounds:        rounds,
	}
}
```

> NOTE to implementer: the existing flat `Assess` (assess.go) also calls `BuildAssessmentInput`? It does **not** — `BuildAssessmentInput` is called only by the surface handlers (course/chat via the evidence helper, project via its own builder). Those call sites are updated in Tasks 5–6. Adding the 8th param **will break their current calls**, so this task's build will fail until you pass `nil` at those call sites. To keep this task self-contained and green, **also** append `, nil` to the three existing `BuildAssessmentInput(...)` / evidence-helper calls in `apps/api/internal/api/course_assessment_input.go` (the `agent.BuildAssessmentInput(...)` call) and `apps/api/internal/api/assessment.go` (`buildAssessmentInputFromProject`). Grep: `rg "BuildAssessmentInput\(" apps/api`. This is a mechanical arg add; Tasks 5–6 replace those bodies wholesale.

- [ ] **Step 2: Write the failing engine test** `apps/api/internal/agent/assess_report_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// reportProvider stubs a flagship reply (mirrors assessProvider in assess_test.go).
func reportProvider(reply string) gateway.Provider {
	return &gateway.StubProvider{Events: []gateway.StreamEvent{
		{TextDelta: reply},
		{Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Done: true},
	}}
}

const goodReport = `{
  "depthAxis":{"dims":[
    {"code":"D1","score":3,"evidence":"限定判断","promptEvidence":"R4"},
    {"code":"D3","score":2,"evidence":"NASA","promptEvidence":""},
    {"code":"D4","score":3,"evidence":"warrant","promptEvidence":""},
    {"code":"D5","score":3,"evidence":"理由","promptEvidence":""}]},
  "autonomyAxis":{"observation":"设边界","anchoredSignals":["R1"],"promptedSignals":["R3"],"adversaryInvites":0,"promptEvidence":""},
  "crossAxis":{"depthLevel":"L3","initiative":"引导后","prose":"能反思","promptEvidence":""},
  "solo":[{"round":4,"excerpt":"限定","level":"L3","rationale":"组织者","initiative":"自发"}],
  "promptLens":{"directiveRounds":3,"totalRounds":10,"boundarySettings":3,"adversaryInvites":0,
    "questions":[{"title":"一问","body":"…"}],
    "bestPrompt":{"round":8,"quote":"检查回扣","annotation":"齐备"},
    "takeaway":{"round":0,"quote":"苛刻审稿人","annotation":"P4"},
    "perRound":[{"round":1,"tier":"P3","label":"要过程·设边界"}]},
  "timeline":[{"round":1,"task":"上传","prompt":"不要重写","pTag":"P3","dimTags":["D1=2"]}],
  "keyEvidence":[{"label":"任务理解","quote":"改 thesis"}],
  "guidance":{"anchored":"限定 thesis","prompted":"SIFT","risk":"D3","nextSteps":[{"title":"强化 D3","body":"SIFT 记录"}]},
  "narrative":"深度 L3 稳定复现。"
}`

func TestAssessReportParsesAxes(t *testing.T) {
	rep, _, err := AssessReport(context.Background(), reportProvider(goodReport),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err != nil {
		t.Fatalf("AssessReport: %v", err)
	}
	if len(rep.DepthAxis.Dims) != 4 {
		t.Fatalf("depth dims = %d, want 4", len(rep.DepthAxis.Dims))
	}
	if rep.DepthAxis.Subtotal != 11 {
		t.Fatalf("subtotal = %d, want 11 (3+2+3+3)", rep.DepthAxis.Subtotal)
	}
	if rep.DepthAxis.Dims[0].Name != "任务理解与问题表述" {
		t.Fatalf("D1 name not filled from rubric: %q", rep.DepthAxis.Dims[0].Name)
	}
	if rep.Axiom != rubric.Model().Axiom {
		t.Fatalf("axiom not set from config: %q", rep.Axiom)
	}
	if rep.AutonomyAxis.Code != "D2" || rep.CrossAxis.Code != "D6" {
		t.Fatalf("axis codes = %q/%q, want D2/D6", rep.AutonomyAxis.Code, rep.CrossAxis.Code)
	}
}

func TestAssessReportSubtotalEqualsSumAndClamps(t *testing.T) {
	// score 5 clamps to 3; missing D5 → 0. subtotal = 3+0+3+0 = 6.
	reply := `{"depthAxis":{"dims":[
		{"code":"D1","score":5,"evidence":"x","promptEvidence":""},
		{"code":"D4","score":3,"evidence":"x","promptEvidence":""}]},
		"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"NA"},
		"narrative":"n"}`
	rep, _, err := AssessReport(context.Background(), reportProvider(reply),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err != nil {
		t.Fatalf("AssessReport: %v", err)
	}
	sum := 0
	for _, d := range rep.DepthAxis.Dims {
		if d.Score < 0 || d.Score > 3 {
			t.Fatalf("dim %s score %d out of range", d.Code, d.Score)
		}
		sum += d.Score
	}
	if rep.DepthAxis.Subtotal != sum {
		t.Fatalf("subtotal %d != Σ scores %d", rep.DepthAxis.Subtotal, sum)
	}
	if rep.DepthAxis.Subtotal != 6 {
		t.Fatalf("subtotal = %d, want 6", rep.DepthAxis.Subtotal)
	}
}

func TestAssessReportBannedPhrasingRejects(t *testing.T) {
	reply := `{"depthAxis":{"dims":[{"code":"D1","score":2,"evidence":"你应该这样写：先摆结论"}]},
		"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"L2"},"narrative":"n"}`
	_, _, err := AssessReport(context.Background(), reportProvider(reply),
		gateway.Resolved{Tier: "flagship"}, rubric.Model(), AssessmentInput{})
	if err == nil {
		t.Fatalf("expected banned-phrasing rejection")
	}
}

func TestAssessReportPromptCarriesLaddersAndAxiom(t *testing.T) {
	sys := assessReportSystemPrompt(rubric.Model())
	for _, want := range []string{"认知深度", "智识自主", "两轴永不合成总分", "不排名", "SOLO"} {
		if !strings.Contains(sys, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}
```

(If `gateway.StubProvider`'s event field names differ, mirror `assessProvider` in `apps/api/internal/agent/assess_test.go:12-18` exactly.)

- [ ] **Step 3: Run it — expect FAIL**

Run: `cd apps/api && go build ./internal/agent/...`
Expected: FAIL (`AssessReport`, `Report`, `assessReportSystemPrompt` undefined).

- [ ] **Step 4: Write the engine** `apps/api/internal/agent/assess_report.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// ---- Report: the whole DualAxis growth report (RL-5: the ONLY number is
// DepthAxis.Subtotal, within-axis; no field sums across axes). ----

type DepthDimScore struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Score          int    `json:"score"` // 0..3
	Evidence       string `json:"evidence"`
	PromptEvidence string `json:"promptEvidence"`
}

type DepthAxis struct {
	Dims     []DepthDimScore `json:"dims"`
	Subtotal int             `json:"subtotal"` // Σ Dims.Score, 0..12
}

type AutonomyAxis struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Observation      string   `json:"observation"`
	AnchoredSignals  []string `json:"anchoredSignals"`
	PromptedSignals  []string `json:"promptedSignals"`
	AdversaryInvites int      `json:"adversaryInvites"`
	PromptEvidence   string   `json:"promptEvidence"`
}

type CrossAxis struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	DepthLevel     string `json:"depthLevel"` // L1..L4|NA
	Initiative     string `json:"initiative"`
	Prose          string `json:"prose"`
	PromptEvidence string `json:"promptEvidence"`
}

type SoloRow struct {
	Round      int    `json:"round"`
	Excerpt    string `json:"excerpt"`
	Level      string `json:"level"` // L1..L4
	Rationale  string `json:"rationale"`
	Initiative string `json:"initiative"` // 自发 | 引导后
}

type LensQuestion struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type PromptSample struct {
	Round      int    `json:"round"`
	Quote      string `json:"quote"`
	Annotation string `json:"annotation"`
}

type PerRoundTier struct {
	Round int    `json:"round"`
	Tier  string `json:"tier"` // P0..P3
	Label string `json:"label"`
}

type PromptLens struct {
	DirectiveRounds  int            `json:"directiveRounds"`
	TotalRounds      int            `json:"totalRounds"`
	BoundarySettings int            `json:"boundarySettings"`
	AdversaryInvites int            `json:"adversaryInvites"`
	Questions        []LensQuestion `json:"questions"`
	BestPrompt       PromptSample   `json:"bestPrompt"`
	Takeaway         PromptSample   `json:"takeaway"`
	PerRound         []PerRoundTier `json:"perRound"`
}

type TimelineRow struct {
	Round   int      `json:"round"`
	Task    string   `json:"task"`
	Prompt  string   `json:"prompt"`
	PTag    string   `json:"pTag"`
	DimTags []string `json:"dimTags"`
}

type KeyEvidence struct {
	Label string `json:"label"`
	Quote string `json:"quote"`
}

type NextStep struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Guidance struct {
	Anchored  string     `json:"anchored"`
	Prompted  string     `json:"prompted"`
	Risk      string     `json:"risk"`
	NextSteps []NextStep `json:"nextSteps"`
}

type Report struct {
	DepthAxis    DepthAxis     `json:"depthAxis"`
	AutonomyAxis AutonomyAxis  `json:"autonomyAxis"`
	CrossAxis    CrossAxis     `json:"crossAxis"`
	Solo         []SoloRow     `json:"solo"`
	PromptLens   PromptLens    `json:"promptLens"`
	Timeline     []TimelineRow `json:"timeline"`
	KeyEvidence  []KeyEvidence `json:"keyEvidence"`
	Guidance     Guidance      `json:"guidance"`
	Narrative    string        `json:"narrative"`
	Axiom        string        `json:"axiom"`
}

// reportWire is what the model returns: depth dims keyed by code (no Name),
// no Axiom (engine fills it). Everything else mirrors Report.
type reportWire struct {
	DepthAxis struct {
		Dims []struct {
			Code           string `json:"code"`
			Score          int    `json:"score"`
			Evidence       string `json:"evidence"`
			PromptEvidence string `json:"promptEvidence"`
		} `json:"dims"`
	} `json:"depthAxis"`
	AutonomyAxis struct {
		Observation      string   `json:"observation"`
		AnchoredSignals  []string `json:"anchoredSignals"`
		PromptedSignals  []string `json:"promptedSignals"`
		AdversaryInvites int      `json:"adversaryInvites"`
		PromptEvidence   string   `json:"promptEvidence"`
	} `json:"autonomyAxis"`
	CrossAxis struct {
		DepthLevel     string `json:"depthLevel"`
		Initiative     string `json:"initiative"`
		Prose          string `json:"prose"`
		PromptEvidence string `json:"promptEvidence"`
	} `json:"crossAxis"`
	Solo        []SoloRow     `json:"solo"`
	PromptLens  PromptLens    `json:"promptLens"`
	Timeline    []TimelineRow `json:"timeline"`
	KeyEvidence []KeyEvidence `json:"keyEvidence"`
	Guidance    Guidance      `json:"guidance"`
	Narrative   string        `json:"narrative"`
}

func clampScore(s int) int {
	if s < 0 {
		return 0
	}
	if s > 3 {
		return 3
	}
	return s
}

var validSolo = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

func normSolo(l string) string {
	if validSolo[l] {
		return l
	}
	return "NA"
}

// AssessReport makes ONE isolated flagship call emitting the entire DualAxis
// report, runs banned-phrasing over every free-text field, fills dim names +
// axiom from the model, computes the depth subtotal (Σ scores), and emits every
// depth dim in model order (missing → score 0). Never in the coach loop.
func AssessReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, m rubric.DualAxis, in AssessmentInput) (Report, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assessReportSystemPrompt(m)},
			{Role: gateway.RoleUser, Content: assessReportUserInput(in)},
		},
	})
	if err != nil {
		return Report{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wire reportWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wire); err != nil {
		return Report{}, usage, fmt.Errorf("agent: report output not JSON: %w", err)
	}

	// Enforcement over every free-text field. Any hit rejects the whole report.
	texts := []string{wire.Narrative, wire.AutonomyAxis.Observation, wire.AutonomyAxis.PromptEvidence,
		wire.CrossAxis.Prose, wire.CrossAxis.PromptEvidence,
		wire.Guidance.Anchored, wire.Guidance.Prompted, wire.Guidance.Risk}
	for _, d := range wire.DepthAxis.Dims {
		texts = append(texts, d.Evidence, d.PromptEvidence)
	}
	texts = append(texts, wire.AutonomyAxis.AnchoredSignals...)
	texts = append(texts, wire.AutonomyAxis.PromptedSignals...)
	for _, s := range wire.Solo {
		texts = append(texts, s.Excerpt, s.Rationale)
	}
	for _, q := range wire.PromptLens.Questions {
		texts = append(texts, q.Title, q.Body)
	}
	texts = append(texts, wire.PromptLens.BestPrompt.Quote, wire.PromptLens.BestPrompt.Annotation,
		wire.PromptLens.Takeaway.Quote, wire.PromptLens.Takeaway.Annotation)
	for _, tl := range wire.Timeline {
		texts = append(texts, tl.Task, tl.Prompt)
	}
	for _, ke := range wire.KeyEvidence {
		texts = append(texts, ke.Label, ke.Quote)
	}
	for _, ns := range wire.Guidance.NextSteps {
		texts = append(texts, ns.Title, ns.Body)
	}
	for _, f := range texts {
		if f == "" {
			continue
		}
		if rule := enforcement.BannedPhrasing(f); rule != nil {
			return Report{}, usage, fmt.Errorf("agent: report rejected by banned-phrasing rule %q", rule.Name)
		}
	}

	// Depth dims: index the model's scores by code; emit every depth dim in
	// model order (missing → 0), Name from the rubric, score clamped 0..3.
	got := map[string]struct {
		score          int
		evidence, pe   string
	}{}
	for _, d := range wire.DepthAxis.Dims {
		got[d.Code] = struct {
			score        int
			evidence, pe string
		}{clampScore(d.Score), d.Evidence, d.PromptEvidence}
	}
	depth := DepthAxis{}
	for _, dim := range rubric.DepthDims() {
		g := got[dim.ID]
		depth.Dims = append(depth.Dims, DepthDimScore{
			Code: dim.ID, Name: dim.Name, Score: g.score, Evidence: g.evidence, PromptEvidence: g.pe,
		})
		depth.Subtotal += g.score
	}

	auto := rubric.AutonomyDim()
	cross := rubric.CrossDim()

	rep := Report{
		DepthAxis: depth,
		AutonomyAxis: AutonomyAxis{
			Code: auto.ID, Name: auto.Name,
			Observation: wire.AutonomyAxis.Observation, AnchoredSignals: wire.AutonomyAxis.AnchoredSignals,
			PromptedSignals: wire.AutonomyAxis.PromptedSignals, AdversaryInvites: wire.AutonomyAxis.AdversaryInvites,
			PromptEvidence: wire.AutonomyAxis.PromptEvidence,
		},
		CrossAxis: CrossAxis{
			Code: cross.ID, Name: cross.Name,
			DepthLevel: normSolo(wire.CrossAxis.DepthLevel), Initiative: wire.CrossAxis.Initiative,
			Prose: wire.CrossAxis.Prose, PromptEvidence: wire.CrossAxis.PromptEvidence,
		},
		Solo:        normSoloRows(wire.Solo),
		PromptLens:  wire.PromptLens,
		Timeline:    wire.Timeline,
		KeyEvidence: wire.KeyEvidence,
		Guidance:    wire.Guidance,
		Narrative:   wire.Narrative,
		Axiom:       m.Axiom,
	}
	if rep.AnchoredNilGuards(); true {
	}
	return rep, usage, nil
}

func normSoloRows(rows []SoloRow) []SoloRow {
	for i := range rows {
		rows[i].Level = normSolo(rows[i].Level)
	}
	return rows
}

// AnchoredNilGuards keeps JSON output arrays non-null (nil slice → []).
func (r *Report) AnchoredNilGuards() {
	if r.AutonomyAxis.AnchoredSignals == nil {
		r.AutonomyAxis.AnchoredSignals = []string{}
	}
	if r.AutonomyAxis.PromptedSignals == nil {
		r.AutonomyAxis.PromptedSignals = []string{}
	}
	if r.Solo == nil {
		r.Solo = []SoloRow{}
	}
	if r.Timeline == nil {
		r.Timeline = []TimelineRow{}
	}
	if r.KeyEvidence == nil {
		r.KeyEvidence = []KeyEvidence{}
	}
	if r.PromptLens.Questions == nil {
		r.PromptLens.Questions = []LensQuestion{}
	}
	if r.PromptLens.PerRound == nil {
		r.PromptLens.PerRound = []PerRoundTier{}
	}
	if r.Guidance.NextSteps == nil {
		r.Guidance.NextSteps = []NextStep{}
	}
	if r.DepthAxis.Dims == nil {
		r.DepthAxis.Dims = []DepthDimScore{}
	}
}
```

> IMPLEMENTER NOTE: the `if r.AnchoredNilGuards(); true {}` line is a placeholder for "call the guard" — replace with a plain `rep.AnchoredNilGuards()` call before `return`. (Written this way only to keep the method referenced; use the clean form.)

- [ ] **Step 5: Write the prompt** `apps/api/internal/agent/assess_report_prompt.go`:

```go
package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/rubric"
)

const reportPosture = `你是「思维印记」的过程评估者。依据可观察的行为证据，判断学生在与 AI 协作中「怎么思考」——不给分数以外的结论、不排名、不下判决式结论。
本模型是双轴模型：第一轴「认知深度」按 0–3 打分（四维小计满分 12）；第二轴「智识自主」只用观察语言描述、绝不打分；跨轴「元认知」同时描述深度面与自主面、不单独打分。
测量公理（必须原样体现，不得改写）：两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定。
另需产出：SOLO 判层（对每一轮学生回应判 L1–L4，标注判据与发起方 自发/引导后；准确性是门槛不是刻度；序数不作均值；指令轮与核查行为不判层）；提示词透镜（对提示词而非学生分档 P0–P4，统计主动指令轮/边界设定/对手邀请）。
严禁替学生改写或撰写作文内容；只描述与诊断其思考路径。只输出 JSON。`

func assessReportSystemPrompt(m rubric.DualAxis) string {
	var b strings.Builder
	b.WriteString(reportPosture)

	b.WriteString("\n\n第一轴 · 认知深度（0–3 打分，覆盖以下每维）：\n")
	for _, d := range rubric.DepthDims() {
		b.WriteString(fmt.Sprintf("%s %s：0 %s ｜ 1 %s ｜ 2 %s ｜ 3 %s\n",
			d.ID, d.Name, d.Anchors["0"], d.Anchors["1"], d.Anchors["2"], d.Anchors["3"]))
	}

	auto := rubric.AutonomyDim()
	b.WriteString(fmt.Sprintf("\n第二轴 · 智识自主（不打分，仅观察）：\n%s %s：%s\n", auto.ID, auto.Name, auto.ObservationGuide))

	cross := rubric.CrossDim()
	b.WriteString(fmt.Sprintf("\n跨轴 · 元认知（不单独打分）：\n%s %s：%s\n", cross.ID, cross.Name, cross.Guide))

	b.WriteString("\nSOLO 层级：")
	for _, s := range m.SoloLevels {
		b.WriteString(fmt.Sprintf("%s %s；", s.Level, s.Name))
	}
	b.WriteString("\n提示词档位：")
	for _, t := range m.PromptTiers {
		b.WriteString(fmt.Sprintf("%s %s；", t.Tier, t.Label))
	}

	b.WriteString(`

输出格式（严格 JSON；depthAxis.dims 覆盖 D1/D3/D4/D5，autonomyAxis 绝不含 score 字段）：
{"depthAxis":{"dims":[{"code":"D1","score":0,"evidence":"…","promptEvidence":"…"}]},
"autonomyAxis":{"observation":"…","anchoredSignals":["…"],"promptedSignals":["…"],"adversaryInvites":0,"promptEvidence":"…"},
"crossAxis":{"depthLevel":"L1|L2|L3|L4|NA","initiative":"自发|引导后|混合","prose":"…","promptEvidence":"…"},
"solo":[{"round":1,"excerpt":"…","level":"L3","rationale":"…","initiative":"自发|引导后"}],
"promptLens":{"directiveRounds":0,"totalRounds":0,"boundarySettings":0,"adversaryInvites":0,
"questions":[{"title":"…","body":"…"}],"bestPrompt":{"round":0,"quote":"…","annotation":"…"},
"takeaway":{"round":0,"quote":"…","annotation":"…"},"perRound":[{"round":1,"tier":"P0|P1|P2|P3","label":"…"}]},
"timeline":[{"round":1,"task":"…","prompt":"…","pTag":"P0|P1|P2|P3","dimTags":["D1=2"]}],
"keyEvidence":[{"label":"…","quote":"…"}],
"guidance":{"anchored":"…","prompted":"…","risk":"…","nextSteps":[{"title":"…","body":"…"}]},
"narrative":"…"}`)
	return b.String()
}

func assessReportUserInput(in AssessmentInput) string {
	var b strings.Builder
	b.WriteString("过程记录：\n")
	if len(in.Rounds) > 0 {
		b.WriteString("逐轮学生提示词与语境：\n")
		for _, r := range in.Rounds {
			b.WriteString(fmt.Sprintf("R%d 语境：%s ｜ 学生：%s\n", r.N, r.AiContext, r.StudentPrompt))
		}
	}
	if len(in.Timeline) > 0 {
		b.WriteString("时间线：\n" + strings.Join(in.Timeline, "\n") + "\n")
	}
	for _, c := range in.CardUses {
		b.WriteString(fmt.Sprintf("工具卡：%s（维度 %s，%s）\n", c.CardID, c.Dimension, c.Spont))
	}
	for _, d := range in.Dispositions {
		b.WriteString(fmt.Sprintf("对反馈的处置：%s —— %s\n", d.Kind, d.Reason))
	}
	if len(in.GateProgress) > 0 {
		b.WriteString("关卡进度：" + strings.Join(in.GateProgress, "；") + "\n")
	}
	if in.SnapshotCount > 0 {
		b.WriteString(fmt.Sprintf("草稿快照：%d 次，字数 %v\n", in.SnapshotCount, in.WordCounts))
	}
	if len(in.ReviewBands) > 0 {
		b.WriteString("整稿体检：" + strings.Join(in.ReviewBands, "、") + "\n")
	}
	if in.GraphSummary != "" {
		b.WriteString("论证结构：" + in.GraphSummary + "\n")
	}
	return b.String()
}
```

- [ ] **Step 6: Fix the guard call.** In `assess_report.go`, replace the placeholder `if rep.AnchoredNilGuards(); true {\n\t}` with:

```go
	rep.AnchoredNilGuards()
```

- [ ] **Step 7: Run the engine tests + full agent package — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/...`
Expected: PASS (existing flat `Assess` tests still pass; new report tests pass).

- [ ] **Step 8: Build the whole API to confirm the `nil` arg additions compile**

Run: `cd apps/api && go build ./...`
Expected: OK (the three `BuildAssessmentInput` call sites now pass `, nil`).

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/agent/assess_input.go apps/api/internal/agent/assess_report.go apps/api/internal/agent/assess_report_prompt.go apps/api/internal/agent/assess_report_test.go apps/api/internal/api/course_assessment_input.go apps/api/internal/api/assessment.go
git commit -m "feat(b): AssessReport DualAxis engine + per-round input (additive)"
```

---

### Task 4: Go studio — `ReportDTO` + `ToReportDTO` (additive)

**Files:**
- Create: `apps/api/internal/studio/report_dto.go`
- Create: `apps/api/internal/studio/report_dto_test.go`

**Interfaces:**
- Consumes: `agent.Report` and its sub-structs (Task 3).
- Produces: `studio.ReportDTO` (mirrors `agent.Report` json tags + adds `generatedAt`); `studio.ToReportDTO(r agent.Report, generatedAt string) ReportDTO`.

- [ ] **Step 1: Write the failing test** `apps/api/internal/studio/report_dto_test.go`:

```go
package studio

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestToReportDTOCarriesAxesAndGeneratedAt(t *testing.T) {
	r := agent.Report{
		DepthAxis: agent.DepthAxis{
			Dims:     []agent.DepthDimScore{{Code: "D1", Name: "任务理解与问题表述", Score: 3, Evidence: "x"}},
			Subtotal: 3,
		},
		AutonomyAxis: agent.AutonomyAxis{Code: "D2", Name: "学生主体性 / AI 依赖度", Observation: "o", AnchoredSignals: []string{}, PromptedSignals: []string{}},
		CrossAxis:    agent.CrossAxis{Code: "D6", Name: "元认知与反思", DepthLevel: "L3"},
		Narrative:    "n",
		Axiom:        "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
	}
	dto := ToReportDTO(r, "2026-07-19T00:00:00Z")
	if dto.GeneratedAt != "2026-07-19T00:00:00Z" {
		t.Fatalf("generatedAt = %q", dto.GeneratedAt)
	}
	if dto.DepthAxis.Subtotal != 3 || len(dto.DepthAxis.Dims) != 1 {
		t.Fatalf("depth axis not carried: %+v", dto.DepthAxis)
	}
	b, _ := json.Marshal(dto)
	s := string(b)
	for _, want := range []string{`"depthAxis"`, `"autonomyAxis"`, `"crossAxis"`, `"subtotal":3`, `"generatedAt"`, `"axiom"`} {
		if !contains(s, want) {
			t.Fatalf("marshalled DTO missing %q: %s", want, s)
		}
	}
	// autonomy must NOT carry a score field
	if contains(s, `"autonomyAxis":{"code":"D2","name":"学生主体性 / AI 依赖度","score"`) {
		t.Fatalf("autonomy axis leaked a score field")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

(If the `studio` package already has a `contains` helper, drop the local one.)

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/api && go build ./internal/studio/...`
Expected: FAIL (`ReportDTO`, `ToReportDTO` undefined).

- [ ] **Step 3: Write the DTO** `apps/api/internal/studio/report_dto.go`:

```go
package studio

import "mindimprint/api/internal/agent"

// ReportDTO is the DualAxis growth report over the wire. Mirrors
// packages/contracts/src/dualAxisReport.ts (camelCase) byte-for-byte, adding
// generatedAt. The only number is DepthAxis.Subtotal (RL-5 + axiom).
type ReportDTO struct {
	DepthAxis    agent.DepthAxis     `json:"depthAxis"`
	AutonomyAxis agent.AutonomyAxis  `json:"autonomyAxis"`
	CrossAxis    agent.CrossAxis     `json:"crossAxis"`
	Solo         []agent.SoloRow     `json:"solo"`
	PromptLens   agent.PromptLens    `json:"promptLens"`
	Timeline     []agent.TimelineRow `json:"timeline"`
	KeyEvidence  []agent.KeyEvidence `json:"keyEvidence"`
	Guidance     agent.Guidance      `json:"guidance"`
	Narrative    string              `json:"narrative"`
	Axiom        string              `json:"axiom"`
	GeneratedAt  string              `json:"generatedAt"`
}

// ToReportDTO adds the generation timestamp to a Report. Pure.
func ToReportDTO(r agent.Report, generatedAt string) ReportDTO {
	r.AnchoredNilGuards() // ensure no null arrays over the wire
	return ReportDTO{
		DepthAxis: r.DepthAxis, AutonomyAxis: r.AutonomyAxis, CrossAxis: r.CrossAxis,
		Solo: r.Solo, PromptLens: r.PromptLens, Timeline: r.Timeline, KeyEvidence: r.KeyEvidence,
		Guidance: r.Guidance, Narrative: r.Narrative, Axiom: r.Axiom, GeneratedAt: generatedAt,
	}
}
```

- [ ] **Step 4: Run test — expect PASS**

Run: `cd apps/api && go test ./internal/studio/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/studio/report_dto.go apps/api/internal/studio/report_dto_test.go
git commit -m "feat(b): studio.ReportDTO + ToReportDTO (additive)"
```

---

### Task 5: Project surface → DualAxis

**Files:**
- Modify: `apps/api/internal/api/assessment.go` (`generateProjectReport`, `dtoFromEvaluationRow`, `buildAssessmentInputFromProject`, `getAssessment`)
- Modify: `apps/api/internal/api/project_finish.go` (return type of `finishProject`'s report)
- Modify: `apps/api/internal/api/project_finish_test.go`, `assessment_test.go` (project cases)

**Interfaces:**
- Consumes: `agent.AssessReport`, `rubric.Model()`, `studio.ToReportDTO` / `studio.ReportDTO`, `agent.Round`.
- Produces: `generateProjectReport(ctx, projectID) (studio.ReportDTO, error)`; project GET/finish endpoints now emit `ReportDTO`; stored `scores` = marshalled `agent.Report`.

- [ ] **Step 1: Update the project report generator.** In `apps/api/internal/api/assessment.go`:
  - Change `generateProjectReport` to return `(studio.ReportDTO, error)`.
  - Build rounds: add a helper `roundsFromProject(d studio.ProjectData) []agent.Round` that walks `d.Events` in order, pairing each student-message event with the preceding AI/context event. (Mirror how `eventDigestsFromProject` reads events; a student turn = event whose `Type` is the student-message type — reuse the same type check `eventDigestsFromProject` uses to identify student vs AI. Number them 1..N.) Pass its result as the 8th arg to `buildAssessmentInputFromProject`'s `agent.BuildAssessmentInput(...)` call (replace the `, nil` added in Task 3).
  - Replace the `agent.Assess(ctx, a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())` call with:
    ```go
    report, usage, err := agent.AssessReport(ctx, a.d.Provider, resolved, rubric.Model(), in)
    ```
  - Record cost EXACTLY as today (`store.RecordLLMCall(ctx, agent.LLMCallRow{ProjectID: projectID, Surface: "studio", Purpose: "assessment", Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens)})`) BEFORE the reject check — unchanged.
  - On reject (`err != nil` from banned-phrasing / parse), return `studio.ReportDTO{}, errAssessmentRejected` after recording cost — unchanged shape of the reject branch.
  - Marshal `report` (the whole `agent.Report`) to `scoresJSON` and persist via `InsertProjectEvaluation(sqlc.InsertProjectEvaluationParams{ProjectID: pgUUID(projectID), Scores: scoresJSON, Narrative: report.Narrative, Model: resolved.Model, Tier: resolved.Tier, PromptTokens: ..., CompletionTokens: ..., CostEstimate: ...})`. (Same params; `Scores` now holds the axis-structured doc; `Narrative` = `report.Narrative`.)
  - Return `studio.ToReportDTO(report, time.Now-ish)`. Use the SAME timestamp source the code already uses — persist then re-read is not needed; return `studio.ToReportDTO(report, row.CreatedAt.Format(time.RFC3339))` where `row` is the `InsertProjectEvaluation` `RETURNING *` result (the insert returns the row).

- [ ] **Step 2: Update `dtoFromEvaluationRow`.** It currently unmarshals `row.Scores` into `[]agent.DimensionScore` then calls `studio.ToAssessmentDTO`. Replace with:

```go
func dtoFromEvaluationRow(row sqlc.Evaluation) (studio.ReportDTO, error) {
	var report agent.Report
	if err := json.Unmarshal(row.Scores, &report); err != nil {
		return studio.ReportDTO{}, err
	}
	return studio.ToReportDTO(report, row.CreatedAt.Format(time.RFC3339)), nil
}
```

- [ ] **Step 3: Update `getAssessment`** — its return path uses `dtoFromEvaluationRow`; the `pgx.ErrNoRows → 200 null` branch is unchanged; the success branch now returns `ReportDTO`. No other change.

- [ ] **Step 4: Update `finishProject`** in `project_finish.go` — it holds the report value from `generateProjectReport`; change its local type to `studio.ReportDTO`. The `errAssessmentRejected` → 422 mapping, the `SetProjectFinished` + `project_finished` event, and the final `writeJSON(w, http.StatusOK, report)` are unchanged.

- [ ] **Step 5: Update the project tests.** In `project_finish_test.go`:
  - `assessReply` / the success stub must now be a valid DualAxis JSON. Replace the success-path provider `assessStubProvider(assessReply)` fixtures with a DualAxis body. Add a shared const in `assessment_test.go`:
    ```go
    const dualAxisReply = `{"depthAxis":{"dims":[
      {"code":"D1","score":3,"evidence":"限定判断","promptEvidence":"R4"},
      {"code":"D3","score":2,"evidence":"NASA","promptEvidence":""},
      {"code":"D4","score":3,"evidence":"warrant","promptEvidence":""},
      {"code":"D5","score":3,"evidence":"理由","promptEvidence":""}]},
      "autonomyAxis":{"observation":"设边界","anchoredSignals":["R1"],"promptedSignals":["R3"],"adversaryInvites":0,"promptEvidence":""},
      "crossAxis":{"depthLevel":"L3","initiative":"引导后","prose":"能反思","promptEvidence":""},
      "solo":[{"round":4,"excerpt":"限定","level":"L3","rationale":"组织者","initiative":"自发"}],
      "promptLens":{"directiveRounds":3,"totalRounds":10,"boundarySettings":3,"adversaryInvites":0,
        "questions":[{"title":"一问","body":"…"}],"bestPrompt":{"round":8,"quote":"检查回扣","annotation":"齐备"},
        "takeaway":{"round":0,"quote":"苛刻审稿人","annotation":"P4"},"perRound":[{"round":1,"tier":"P3","label":"要过程·设边界"}]},
      "timeline":[{"round":1,"task":"上传","prompt":"不要重写","pTag":"P3","dimTags":["D1=2"]}],
      "keyEvidence":[{"label":"任务理解","quote":"改 thesis"}],
      "guidance":{"anchored":"限定 thesis","prompted":"SIFT","risk":"D3","nextSteps":[{"title":"强化 D3","body":"SIFT 记录"}]},
      "narrative":"深度 L3 稳定复现。"}`
    ```
  - `TestFinishProject_SuccessMarksFinishedAndPersistsFlagshipReport`: change the provider to `assessStubProvider(dualAxisReply)`; decode the response body into a struct exposing `DepthAxis.Subtotal` and assert `== 11`; keep the `tier == "flagship"`, `countProjectEvaluations == 1`, and `project_finished` event assertions unchanged.
  - The reject test (`TestFinishProject_RejectedAssessmentKeepsProjectActive`) keeps its `你应该这样写…` stub but wrapped in a minimal valid-shape body so it reaches enforcement: `{"depthAxis":{"dims":[{"code":"D1","score":2,"evidence":"你应该这样写：先摆结论"}]},"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"L2"},"narrative":"n"}`. Assert 422 `assessment_rejected`, project stays `active`, `countProjectEvaluations == 0`, `countLLMCalls == 1`.
  - The gate-unmet and already-finished tests are unchanged except any success stub they rely on → `dualAxisReply`.

- [ ] **Step 6: Run the full API package — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS. (Course/chat tests still use the OLD flat `Assess` at this point — untouched, still green.)

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/assessment.go apps/api/internal/api/project_finish.go apps/api/internal/api/project_finish_test.go apps/api/internal/api/assessment_test.go
git commit -m "feat(b): project surface → AssessReport / ReportDTO"
```

---

### Task 6: Course + chat surfaces → DualAxis

**Files:**
- Modify: `apps/api/internal/api/course_assessment_input.go` (`buildAssessmentInputFromEvidence` + a rounds helper)
- Modify: `apps/api/internal/api/course_assessment.go`, `chat_assessment.go`
- Modify: `apps/api/internal/api/course_assessment_test.go`, `chat_assessment_test.go`

**Interfaces:**
- Consumes: `agent.AssessReport`, `rubric.Model()`, `studio.ToReportDTO`, `agent.Round`, `dtoFromEvaluationRow` (now returns `ReportDTO`, from Task 5).
- Produces: course/chat GET+POST endpoints emit `ReportDTO`; stored `scores` = `agent.Report`.

- [ ] **Step 1: Add rounds to the shared evidence builder.** In `course_assessment_input.go`, add:

```go
// roundsFromEvidence pairs student-message events into ordered rounds. Course/chat
// have no separate AI-context projection, so AiContext is left empty; the student
// prompt alone still drives SOLO + prompt-lens.
func roundsFromEvidence(events []studio.Event) []agent.Round {
	rounds := make([]agent.Round, 0)
	n := 0
	for _, e := range events {
		if !isStudentEvent(e) { // reuse the same student-vs-AI check eventDigestsFromProject uses
			continue
		}
		n++
		rounds = append(rounds, agent.Round{N: n, StudentPrompt: e.Text, AiContext: ""})
	}
	return rounds
}
```

Then change `buildAssessmentInputFromEvidence` to pass the rounds as the 8th arg (replacing the `, nil` added in Task 3):

```go
func buildAssessmentInputFromEvidence(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(events), cardUsesFromEvidence(cards), dispositionUsesFromEvidence(cards),
		nil, nil, nil, "", roundsFromEvidence(events))
}
```

(If there is no existing `isStudentEvent`/event-type predicate, derive the student-event check from whatever `eventDigestsFromProject` already keys on — inspect it and reuse the exact same condition so rounds and the timeline agree on what a "student turn" is. Do not invent a new event type.)

- [ ] **Step 2: Update `generateCourseAssessment`** in `course_assessment.go`:
  - Replace `agent.Assess(ctx, a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())` with `report, usage, err := agent.AssessReport(ctx, a.d.Provider, resolved, rubric.Model(), in)`.
  - Cost recording via `store.RecordCourseLLMCall(ctx, sess.UserID, "assessment", resolved, int32(usage.InputTokens), int32(usage.OutputTokens))` — unchanged, still before reject check.
  - On reject → 422 inline — unchanged.
  - Marshal `report` → scores; persist `InsertSessionEvaluation(...)` with `Narrative: report.Narrative`; return `dtoFromEvaluationRow(row)` (now `ReportDTO`) OR `studio.ToReportDTO(report, row.CreatedAt.Format(time.RFC3339))`.

- [ ] **Step 3: Update `generateChatAssessment`** in `chat_assessment.go` — identical pattern with `store.RecordChatLLMCall(ctx, u.ID, "assessment", resolved, int32(usage.InputTokens), int32(usage.OutputTokens))` and `InsertThreadEvaluation(...)`.

- [ ] **Step 4: Update the course/chat tests.** In `course_assessment_test.go` and `chat_assessment_test.go`: the success provider becomes `assessStubProvider(dualAxisReply)` (the const from Task 5, now in `assessment_test.go`); decode responses and assert `depthAxis.subtotal == 11` and `tier == "flagship"`; the reject stub becomes the minimal `你应该这样写…` body from Task 5 Step 5; assert 422 + one `llm_call` recorded + zero evaluations.

- [ ] **Step 5: Run the full API package — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS. All three surfaces now emit DualAxis.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/course_assessment_input.go apps/api/internal/api/course_assessment.go apps/api/internal/api/chat_assessment.go apps/api/internal/api/course_assessment_test.go apps/api/internal/api/chat_assessment_test.go
git commit -m "feat(b): course + chat surfaces → AssessReport / ReportDTO"
```

---

### Task 7: Growth-history decode + clean-slate migration 0027

**Files:**
- Create: `apps/api/internal/store/migrations/0027_dualaxis_clean_slate.sql`
- Create: `apps/api/internal/store/migrations/migrate_0027_test.go`
- Modify: `apps/api/internal/api/growth_history.go`
- Modify: `apps/api/internal/api/growth_history_test.go` (or the query test that seeds evaluations)

**Interfaces:**
- Consumes: `agent.Report`, `studio.ToReportDTO` (from Tasks 3–4); the `ListGrowthHistory` sqlc query (unchanged SQL — it selects `scores`,`narrative`).
- Produces: `growthHistoryEntry.Report studio.ReportDTO`; migration 0027 (up: `DELETE FROM evaluations`; down: no-op).

- [ ] **Step 1: Write the migration** `apps/api/internal/store/migrations/0027_dualaxis_clean_slate.sql`:

```sql
-- +goose Up
-- DualAxis replaces the flat 10-dim report shape. The product is not in use;
-- existing evaluation rows are flat-shaped and cannot be upgraded (one-time,
-- no-regenerate), so clear them. New rows are axis-structured.
DELETE FROM evaluations;

-- +goose Down
-- Irreversible data clear; nothing to restore.
SELECT 1;
```

- [ ] **Step 2: Write the migration test** `apps/api/internal/store/migrations/migrate_0027_test.go` — mirror `migrate_0026_test.go`'s harness. Seed one evaluation row before Up, assert zero rows after Up:

```go
package migrations_test

// Mirror migrate_0026_test.go's container/goose setup exactly (same imports,
// same newMigrationDB helper). Only the assertions below are new.

func TestMigrate0027ClearsEvaluations(t *testing.T) {
	ctx := context.Background()
	db := newMigrationDB(t) // same helper the 0026 test uses; migrates up to 0026

	// Seed a minimal evaluation row against a seeded project (reuse the seed the
	// other migration tests use for a valid project_id FK).
	seedOneEvaluation(t, db) // insert scores='[]'::jsonb, narrative='x', model/tier/status='done', project_id=<seeded>
	var before int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM evaluations`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before == 0 {
		t.Fatal("seed failed: no evaluation row before 0027")
	}

	if err := goose.UpByOne(db, "."); err != nil { // apply 0027
		t.Fatalf("apply 0027: %v", err)
	}
	var after int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM evaluations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 0 {
		t.Fatalf("evaluations after 0027 = %d, want 0", after)
	}
}
```

(Match the ACTUAL goose driver calls the 0026 test uses — `goose.UpByOne`/`UpToContext`/`DownToContext` signatures in this repo. If the harness migrates fully to head, instead migrate to 0026 then `UpByOne`. Reuse the repo's existing seed helper for a valid `project_id`; if none exists, insert a school→class→user→project chain the way `migrate_0026_test.go`/`growth_history_query_test.go` seed it.)

- [ ] **Step 3: Update the growth-history handler.** In `growth_history.go`, the `growthHistoryEntry` struct's `Report` field type changes from `studio.AssessmentDTO` to `studio.ReportDTO`, and the unmarshal of `row.Scores` changes:

```go
var report agent.Report
if err := json.Unmarshal(row.Scores, &report); err != nil {
	// skip malformed row rather than 500 the whole list
	continue
}
entries = append(entries, growthHistoryEntry{
	Surface:   row.Surface,
	ScopeID:   uuidText(row.ScopeID),
	Label:     row.Label,
	Sublabel:  row.Sublabel,
	CreatedAt: row.CreatedAt.Format(time.RFC3339),
	Report:    studio.ToReportDTO(report, row.CreatedAt.Format(time.RFC3339)),
})
```

- [ ] **Step 4: Update the history test.** In `growth_history_test.go` / `growth_history_query_test.go`: the seeded evaluation `scores` fixtures must now be axis-structured `agent.Report` JSON (marshal an `agent.Report{DepthAxis:{Dims:[{Code:"D1",Score:3}],Subtotal:3}, ...Narrative:"n", Axiom:"…"}` instead of the flat `[]DimensionScore`). Assert the returned entry's `Report.DepthAxis.Subtotal` and the owner-filtering behavior (unchanged).

- [ ] **Step 5: Run the full API + migrations packages — expect PASS**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS (all Go packages green — engine, DTO, all three surfaces, history, migrations).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0027_dualaxis_clean_slate.sql apps/api/internal/store/migrations/migrate_0027_test.go apps/api/internal/api/growth_history.go apps/api/internal/api/growth_history_test.go apps/api/internal/api/growth_history_query_test.go
git commit -m "feat(b): growth-history decodes DualAxis + 0027 clean-slate migration"
```

---

### Task 8: Web — shared `<DualAxisReport>` component (additive)

**Files:**
- Create: `apps/web/src/shell/report/DualAxisReport.tsx`
- Create: `apps/web/src/shell/report/DualAxisReport.test.tsx`

**Interfaces:**
- Consumes: the `DualAxisReport` contract type (Task 2).
- Produces: `export function DualAxisReport({ report }: { report: DualAxisReportT }): JSX.Element` — renders 总览 → 双轴读数 → SOLO → 提示词透镜 → 交互证据 timeline → 关键原话 → 建议. (Import the contract type aliased to avoid name collision: `import { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";`.)

- [ ] **Step 1: Write the failing test** `apps/web/src/shell/report/DualAxisReport.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DualAxisReport } from "./DualAxisReport";
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

const report: DualAxisReportT = {
  depthAxis: { dims: [
    { code: "D1", name: "任务理解与问题表述", score: 3, evidence: "限定判断", promptEvidence: "R4" },
    { code: "D3", name: "证据与信源意识", score: 2, evidence: "NASA", promptEvidence: "" },
    { code: "D4", name: "论证结构意识", score: 3, evidence: "warrant", promptEvidence: "" },
    { code: "D5", name: "反馈理解与修改理由", score: 3, evidence: "理由", promptEvidence: "" },
  ], subtotal: 11 },
  autonomyAxis: { code: "D2", name: "学生主体性 / AI 依赖度", observation: "入场即设边界", anchoredSignals: ["R1"], promptedSignals: ["R3"], adversaryInvites: 0, promptEvidence: "" },
  crossAxis: { code: "D6", name: "元认知与反思", depthLevel: "L3", initiative: "引导后", prose: "能反思，尚未自发反思", promptEvidence: "" },
  solo: [{ round: 4, excerpt: "限定判断", level: "L3", rationale: "组织者", initiative: "自发" }],
  promptLens: { directiveRounds: 3, totalRounds: 10, boundarySettings: 3, adversaryInvites: 0,
    questions: [{ title: "一问 · 任务说清了吗", body: "…" }],
    bestPrompt: { round: 8, quote: "检查是否回扣 thesis", annotation: "齐备" },
    takeaway: { round: 0, quote: "扮演苛刻审稿人", annotation: "P4 模板" },
    perRound: [{ round: 1, tier: "P3", label: "要过程·设边界" }] },
  timeline: [{ round: 1, task: "上传草稿", prompt: "不要直接重写", pTag: "P3", dimTags: ["D1=2"] }],
  keyEvidence: [{ label: "任务理解", quote: "我想把 thesis 改成…" }],
  guidance: { anchored: "主动限定 thesis", prompted: "SIFT 核查", risk: "D3 仍停留在来源等级", nextSteps: [{ title: "下一步强化 D3", body: "跑一张 SIFT 记录" }] },
  narrative: "深度侧 L3 结构稳定复现，自主侧未主动召唤对手。",
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-19T00:00:00Z",
};

describe("DualAxisReport", () => {
  it("renders the depth subtotal as /12", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/11/)).toBeTruthy();
    expect(screen.getByText(/12/)).toBeTruthy();
  });
  it("renders the axiom verbatim", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/两轴永不合成总分/)).toBeTruthy();
  });
  it("shows the autonomy axis without a numeric score", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/学生主体性/)).toBeTruthy();
    expect(screen.getByText(/观察/)).toBeTruthy(); // 观察 badge, not a score
  });
  it("renders SOLO rows and prompt-lens counts", () => {
    render(<DualAxisReport report={report} />);
    expect(screen.getByText(/L3/)).toBeTruthy();
    expect(screen.getByText(/对手邀请/)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `cd apps/web && npx vitest run src/shell/report/DualAxisReport.test.tsx`
Expected: FAIL (module not found).

- [ ] **Step 3: Build the component** `apps/web/src/shell/report/DualAxisReport.tsx`. Render every section; follow the DualAxis reference structure in the app's visual language. Depth cards show the integer score badge; autonomy shows an 「观察」 badge (no number); cross shows a 「跨轴」 badge; SOLO as a table; prompt-lens shows the three counts + three question cards + best-prompt + takeaway; timeline lists rounds with P-tag + dim-tags; then 关键原话 + 建议. Structure:

```tsx
import type { DualAxisReport as DualAxisReportT } from "@mind-imprint/contracts";

export function DualAxisReport({ report }: { report: DualAxisReportT }) {
  const { depthAxis, autonomyAxis, crossAxis, solo, promptLens, timeline, keyEvidence, guidance, narrative, axiom } = report;
  return (
    <div className="dualaxis-report">
      {/* 总览 */}
      <section>
        <div>深度 {depthAxis.subtotal} / 12（单轴小计）</div>
        <div>自主 · 边界设定 ×{promptLens.boundarySettings} · 对手邀请 ×{autonomyAxis.adversaryInvites}</div>
        <p>{narrative}</p>
        <p className="axiom">{axiom}</p>
      </section>

      {/* 双轴读数 */}
      <section>
        <h3>第一轴 · 认知深度（{depthAxis.subtotal} / 12）</h3>
        {depthAxis.dims.map((d) => (
          <article key={d.code}>
            <header>{d.name}<span className="score-badge">{d.score}</span></header>
            <p>{d.evidence}</p>
            {d.promptEvidence
              ? <p className="prompt-evidence">提示词证据：{d.promptEvidence}</p>
              : <p className="prompt-evidence absent">提示词证据：空</p>}
          </article>
        ))}
        <h3>第二轴 · 智识自主（观察 · 不计分）</h3>
        <article>
          <header>{autonomyAxis.name}<span className="obs-badge">观察</span></header>
          <p>{autonomyAxis.observation}</p>
          <p>能力锚定：{autonomyAxis.anchoredSignals.join("、")}</p>
          <p>引导后：{autonomyAxis.promptedSignals.join("、")}</p>
          <p>对手邀请：{autonomyAxis.adversaryInvites} 次</p>
        </article>
        <h3>跨轴 · 元认知</h3>
        <article>
          <header>{crossAxis.name}<span className="cross-badge">跨轴</span></header>
          <p>深度面 {crossAxis.depthLevel}／自主面 {crossAxis.initiative}</p>
          <p>{crossAxis.prose}</p>
        </article>
      </section>

      {/* SOLO 判层 */}
      <section>
        <h3>SOLO 判层</h3>
        <table>
          <thead><tr><th>轮次</th><th>回应</th><th>判层</th><th>判据</th><th>发起方</th></tr></thead>
          <tbody>
            {solo.map((s) => (
              <tr key={s.round}><td>R{s.round}</td><td>{s.excerpt}</td><td>{s.level}</td><td>{s.rationale}</td><td>{s.initiative}</td></tr>
            ))}
          </tbody>
        </table>
      </section>

      {/* 提示词透镜 */}
      <section>
        <h3>提示词透镜</h3>
        <div className="lens-stats">
          <span>主动指令轮 {promptLens.directiveRounds} / {promptLens.totalRounds}</span>
          <span>边界设定 {promptLens.boundarySettings} 次</span>
          <span>对手邀请 {promptLens.adversaryInvites} 次</span>
        </div>
        {promptLens.questions.map((q, i) => (
          <div key={i}><strong>{q.title}</strong><p>{q.body}</p></div>
        ))}
        <div><strong>本次最佳提示词（R{promptLens.bestPrompt.round}）</strong><blockquote>{promptLens.bestPrompt.quote}</blockquote><p>{promptLens.bestPrompt.annotation}</p></div>
        <div><strong>带走的一条升级提示</strong><blockquote>{promptLens.takeaway.quote}</blockquote><p>{promptLens.takeaway.annotation}</p></div>
      </section>

      {/* 交互证据 timeline */}
      <section>
        <h3>交互证据</h3>
        {timeline.map((t) => (
          <details key={t.round}>
            <summary>R{t.round} · {t.task} <span className="p-tag">{t.pTag}</span> {t.dimTags.map((dt) => <span key={dt} className="dim-tag">{dt}</span>)}</summary>
            <blockquote>{t.prompt}</blockquote>
          </details>
        ))}
      </section>

      {/* 关键原话 */}
      <section>
        <h3>关键思维证据</h3>
        {keyEvidence.map((k, i) => (<article key={i}><strong>{k.label}</strong><p>{k.quote}</p></article>))}
      </section>

      {/* 建议 */}
      <section>
        <h3>参与度与下一步</h3>
        <article><strong>A 类 · 自发完成</strong><p>{guidance.anchored}</p></article>
        <article><strong>引导后完成</strong><p>{guidance.prompted}</p></article>
        <article><strong>主要风险</strong><p>{guidance.risk}</p></article>
        {guidance.nextSteps.map((n, i) => (<article key={i}><strong>{n.title}</strong><p>{n.body}</p></article>))}
      </section>
    </div>
  );
}
```

(Apply the project's existing report styling/classes — inspect `CourseReport.tsx` for the card/section class conventions and reuse them; the structure above is the contract, the styling matches the app. Icons, if any, inline SVG.)

- [ ] **Step 4: Run tests + tsc — expect PASS**

Run: `cd apps/web && npx vitest run src/shell/report/DualAxisReport.test.tsx && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/report/DualAxisReport.tsx apps/web/src/shell/report/DualAxisReport.test.tsx
git commit -m "feat(b): shared <DualAxisReport> web component (additive)"
```

---

### Task 9: Web — flip clients + growthHistory + three renderers

**Files:**
- Modify: `apps/web/src/api/{projects,assessment,courseAssessment,chatAssessment,growth,index}.ts`
- Modify: `packages/contracts/src/growthHistory.ts`
- Modify: `apps/web/src/shell/growth/GrowthReport.tsx`, `shell/courses/CourseReport.tsx`, `shell/chat/ChatReport.tsx`
- Modify: their `.test.tsx` + the api client `.test.ts` fixtures

**Interfaces:**
- Consumes: `<DualAxisReport>` (Task 8), `DualAxisReport` contract (Task 2).
- Produces: all four report endpoints parse `DualAxisReport`; `GrowthHistoryEntry.report` is a `DualAxisReport`; the three renderers render `<DualAxisReport report={...}>`.

- [ ] **Step 1: Update the contract** `packages/contracts/src/growthHistory.ts` — change `report: Assessment` → `report: DualAxisReport` (import it). Barrel already exports it.

- [ ] **Step 2: Flip the api clients.** In each, swap the parse type from `Assessment` to `DualAxisReport`:
  - `projects.ts` `finishProject`: `return DualAxisReport.parse(raw);`
  - `assessment.ts` `getAssessment`: parse `DualAxisReport` (keep the `null` empty-state).
  - `courseAssessment.ts` / `chatAssessment.ts`: both GET+POST parse `DualAxisReport`.
  - `growth.ts` `getGrowthHistory`: `GrowthHistory.parse(raw).entries` (entries now carry `DualAxisReport`) — no change beyond the contract type.
  - `index.ts`: update the facade return types (`Promise<DualAxisReport>` / `Promise<DualAxisReport | null>`).

- [ ] **Step 3: Flip the three renderers** to render the shared component:
  - `GrowthReport.tsx`: `HistoryRow` renders `<DualAxisReport report={entry.report} />` in the expanded body (drop `LevelChip`/`DimensionRow`/`SOLO_LABELS` flat logic). Empty-state copy unchanged (keep "留下一次思维印记" — do NOT reintroduce 生成).
  - `CourseReport.tsx`: replace the `assessment.dimensions.map(DimensionRow)` block with `<DualAxisReport report={assessment} />`. Keep the auto-generate flow (`getCourseAssessment` then `generateCourseAssessment`) and back/portal nav.
  - `ChatReport.tsx`: replace the flat dimensions/narrative block with `<DualAxisReport report={assessment} />`. Keep the **opt-in** GET-on-mount / POST-on-click flow (铁律 2 — never auto-POST).

- [ ] **Step 4: Update the tests + fixtures.** Every mocked `Assessment` fixture (`GrowthReport.test.tsx`, `CourseReport.test.tsx`, `ChatReport.test.tsx`, and api client tests `projects.test.ts` etc.) becomes a `DualAxisReport` fixture (reuse the object from Task 8's test). Assert the rendered report shows the subtotal + axiom (the renderers now delegate to `<DualAxisReport>`); keep the flow assertions (opt-in button on chat, auto-generate on course, empty state on growth). **Run the WEB suite for these — do not skip it (A3 lesson: a contract change that skips the web suite goes red silently).**

- [ ] **Step 5: Run web + contracts + tsc — expect PASS**

Run: `cd packages/contracts && npx vitest run && npx tsc --noEmit && cd ../../apps/web && npx vitest run && npx tsc --noEmit`
Expected: PASS across contracts + web, tsc clean both.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/growthHistory.ts apps/web/src/api/projects.ts apps/web/src/api/assessment.ts apps/web/src/api/courseAssessment.ts apps/web/src/api/chatAssessment.ts apps/web/src/api/growth.ts apps/web/src/api/index.ts apps/web/src/shell/growth/GrowthReport.tsx apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/chat/ChatReport.tsx apps/web/src/shell/growth/GrowthReport.test.tsx apps/web/src/shell/courses/CourseReport.test.tsx apps/web/src/shell/chat/ChatReport.test.tsx apps/web/src/api/projects.test.ts
git commit -m "feat(b): flip web clients + renderers to DualAxisReport"
```

(Name every file you actually changed; run `git status` first and add only B's files — never the pre-existing `M package.json` or untracked user files.)

---

### Task 10: Remove the flat model (cleanup)

**Files:**
- Delete: `apps/api/internal/agent/assess.go`, `assess_prompt.go`, `assess_input_test.go` (if it tests the old flat path only — otherwise keep the round-building tests), `anchors.json`, `apps/api/internal/agent/assess_test.go`
- Delete: `apps/api/internal/studio/assessment_dto.go`, `assessment_dto` tests / the flat arm of `dto_parity_test.go`
- Delete: `apps/api/internal/rubric/rubric.go`, `apps/api/internal/rubric/ct-rubric.json`
- Delete: `packages/contracts/src/assessment.ts`, `packages/contracts/src/ct-rubric.json`, `packages/contracts/test/assessment.test.ts`
- Modify: `packages/contracts/src/rubric.ts` (remove `CT_RUBRIC`/`FULL_RUBRIC`/`assertRubricComplete`/`ct-rubric.json` import + the flat `RubricDimension`/`Rubric` interfaces — keep `SoloLevel`, `ScoredLevel`, `SOLO_LABELS`, and the DualAxis model added in Task 2), `packages/contracts/src/index.ts` (drop `export * from "./assessment";`), `apps/api/tools/syncrubric/main.go` (drop `ct-rubric.json` from the sync set)

**Interfaces:**
- Consumes: nothing new. Produces: a codebase with only the DualAxis model.

- [ ] **Step 1: Verify no live references remain.** Grep for every old symbol and confirm only the files-to-delete reference them:

```bash
rg -n "agent\.Assess\b|agent\.Assessment\b|EmbeddedAnchors|ToAssessmentDTO|AssessmentDTO|rubric\.CT\b|CT_RUBRIC|FULL_RUBRIC|assertRubricComplete|ct-rubric\.json|from \"\.\/assessment\"|@mind-imprint/contracts\".*\bAssessment\b" apps packages
```

Expected after Tasks 1–9: matches only inside the files this task deletes/edits. If a live consumer remains, it was missed in Tasks 5–9 — fix it there in spirit (update the reference) before deleting.

- [ ] **Step 2: Delete the flat files** listed above (`git rm`). Remove `SoloLevel` import usages? No — `SoloLevel` stays (used by `dualAxisReport.ts`).

- [ ] **Step 3: Trim `rubric.ts`** — remove the `ct-rubric.json` import, `RubricDimension`/`Rubric` interfaces, `RubricDimensionSchema`/`RubricSchema`, `CT_RUBRIC`, `FULL_RUBRIC`, `assertRubricComplete` and its call. Keep `SoloLevel`, `ScoredLevel`, `SOLO_LABELS`, and everything the DualAxis model (Task 2) added.

- [ ] **Step 4: Trim `index.ts`** — remove `export * from "./assessment";`.

- [ ] **Step 5: Trim `syncrubric/main.go`** — set `files := []string{"dualaxis.json"}`.

- [ ] **Step 6: Run the entire suite — expect PASS**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./... \
  && cd ../../packages/contracts && npx vitest run && npx tsc --noEmit \
  && cd ../../apps/web && npx vitest run && npx tsc --noEmit
```

Expected: PASS everywhere, tsc clean. No dangling imports.

- [ ] **Step 7: Commit**

```bash
git add -A apps/api/internal/agent apps/api/internal/studio apps/api/internal/rubric apps/api/tools/syncrubric packages/contracts/src packages/contracts/test
git status   # verify ONLY B's files staged; unstage any stray user file
git commit -m "refactor(b): remove flat 10-dim assessment model"
```

(`git add -A <dir>` is scoped to B-owned dirs here, but still run `git status` and unstage anything not B's before committing.)

---

## Self-Review

**Spec coverage:**
- §1 model (6 dims, axes, 0–3 depth, unscored autonomy, cross, axiom, authored anchors) → Task 1 (config) + Task 2 (contracts model).
- §2.1 enriched per-round input → Task 3 (`Round`/`Rounds`) + Tasks 5–6 (surface round builders).
- §2.2 one flagship call → Task 3 (`AssessReport`, single `gateway.Collect`).
- §2.3 axis-structured DTO + structural axiom → Task 3 (engine + subtotal) + Task 4 (DTO) + Task 2 (`.strict()` autonomy = no score field).
- §2.4 enforcement over all text + defaults → Task 3.
- §3 single-source config + Go/TS parsing + sequenced removal → Tasks 1, 2, 10.
- §4.1 one shared component, three render sites → Tasks 8–9. (Correct: ReviewView is NOT a render site.)
- §4.2 clean-slate migration → Task 7.
- §5 tests + invariants → every task's tests; flagship/cost-on-reject in Tasks 5–6; RL-5/axiom structural in Tasks 2–4.
- §6 out-of-scope (C radar, mind-map, few-shot anchors) → not built; noted.

**Placeholder scan:** the only intentional deferrals are "reuse the repo's student-event predicate / 0026 test harness / report styling" — these point at concrete existing code the implementer inspects, not invented behavior. The `AnchoredNilGuards` placeholder line is explicitly corrected in Task 3 Step 6. Config `…` never appears — all anchors authored in Task 1.

**Type consistency:** `agent.Report`/`ReportDTO`/`DualAxisReport` fields match across Go and TS (camelCase). `AssessReport(ctx, prov, r, m, in)` signature is identical in Tasks 3, 5, 6. `BuildAssessmentInput` gains its 8th `rounds` param in Task 3 and every caller passes it (Tasks 3 `nil` stopgap → 5/6 real). `dtoFromEvaluationRow` returns `studio.ReportDTO` consistently after Task 5.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-19-b-dualaxis-assessment-model.md`.
