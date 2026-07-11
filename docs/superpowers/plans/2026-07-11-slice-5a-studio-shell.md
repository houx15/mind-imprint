# Slice 5a — Studio shell + station rail + four-view frame + coach rail — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the whole student Writing Studio chrome — the 3-column shell + focus mode, the S0–S6 station rail (the contract map with gate progress), the four-view center frame (station-rail = view-switcher), and the AI 陪练 coach rail (thread + 三键处置 + 装备栏 + composer) — as controlled, fixture-backed React, mounted in the dev harness.

**Architecture:** A new `apps/web/src/studio/` tree of controlled components driven by one `StudioState` view-model fixture (the seam Slice 5b will populate from the Slice-4 backend). No backend, no SSE, no live model, no app-routing change — every interaction is a callback. The 素材 view reuses the Slice-1 `SourceDossier`. Same discipline as Slice 1 (new tree, controlled, fixtures, dev-harness mount, old `workspace/`/`StudentApp` untouched).

**Tech Stack:** React + Vite + TypeScript, Tailwind (`mk-` tokens), vitest + @testing-library/react. Icons = inline SVG (never lucide-react). `@mind-imprint/contracts` for `StudioEvent`.

## Global Constraints

- **New tree only.** All new files under `apps/web/src/studio/` (+ one dev-harness file + a `DevApp.tsx` edit + optional `tailwind.config.ts` token additions). Do NOT modify `apps/web/src/workspace/` (except importing `SourceDossier`), `shell/StudentApp.tsx`, `Root.tsx`, or app routing.
- **Controlled, no side effects.** Components take `state` + callbacks as props; no `fetch`, no SSE, no model, no timers-that-mutate. The dev-harness host (`StudioPanel`) holds the fixture in local React state so clicks work.
- **Build to the design** `docs/design/思维印记_工作区.dc.html` — the refreshed 2767-line version. Match tokens/structure/copy. Line ranges are cited per task; read them.
- **Design tokens:** Tailwind `mk-` where they exist (`mk-primary #2A3B7A`, `mk-primary-tint #EDEFF9`, `mk-green #4C9A82`, `mk-green-tint #E7F3EE`, `mk-accent #D98263`, `mk-accent-tint #FBEEE7`, `mk-amber #E8A33D`, `mk-ink #1C2333`, `mk-muted #8A92A3`, `mk-muted-2 #9AA1B0`, `mk-bg #F3F4F8`, `mk-surface #fff`, `mk-border #EAECF2`, radii `rounded-mk`/`rounded-mk-lg`); **inline `style={{}}` for hex not in the token set** (e.g. flag `#C96F4F`, purple `#7C6BB5`, amber-2 `#D9A23D`, station colors) — exactly as `SourceDossier` mixes them. Task 8 may add the 3–4 most-used new tokens to `tailwind.config.ts`.
- **Icons inline SVG**, lifted from the `.dc.html` paths.
- **Chinese UI copy is verbatim** from the design; English only for code identifiers.
- **Tests:** vitest + RTL, `render`/`screen`/`fireEvent`/`vi.fn()`. Run `cd apps/web && npx vitest run <path>` for one file; `npm test` (or `npx vitest run`) for the suite.
- **Commit trailer:** end every commit body with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

---

## File structure

- `apps/web/src/studio/Bean.tsx` — the mascot SVG (ported from `Bean.dc.html`).
- `apps/web/src/studio/state.ts` — `StudioState` + all sub-types.
- `apps/web/src/studio/fixtures.ts` — a full `StudioState` fixture (0457 China-sustainability, S4 current).
- `apps/web/src/studio/StationRail.tsx` — the S0–S6 contract map.
- `apps/web/src/studio/views/OnboardingView.tsx` — S0/S1/S2 onboarding (S0 = 任务解码 recognition).
- `apps/web/src/studio/views/StructureView.tsx` — 结构 (S4) Toulmin card-list shell.
- `apps/web/src/studio/views/WritingView.tsx` — 写作 (S5) edit/preview draft shell.
- `apps/web/src/studio/views/ReviewView.tsx` — 评估 (S6) readiness-gauge shell.
- `apps/web/src/studio/ViewFrame.tsx` — center: station header + view switch (reuses `SourceDossier` for 素材).
- `apps/web/src/studio/DispositionCard.tsx` — 三键处置 (≥15-char reason).
- `apps/web/src/studio/EquipmentBar.tsx` — 装备栏 popover.
- `apps/web/src/studio/MethodologyModal.tsx` — 工具说明书 modal.
- `apps/web/src/studio/CoachRail.tsx` — coach header + thread + tool-card slot + composer.
- `apps/web/src/studio/StudioShell.tsx` — 3-column layout + top bar + focus mode.
- `apps/web/src/studio/index.ts` — exports.
- `apps/web/src/dev/StudioPanel.tsx` — dev-harness host (fixture in local state).
- `apps/web/src/dev/DevApp.tsx` — **modify**: add a 工作室 tab.
- `apps/web/tailwind.config.ts` — **modify (Task 8, optional)**: add the few most-used Studio tokens.
- Each component gets a colocated `*.test.tsx`.

**Design line references** (read these — they are the binding markup):
- Studio shell + top bar + focus: 757–780; station rail: 782–825; center header: 829–834;
  结构/S4: 959–1016; 素材/S3: 1018–1090; 写作/S5: 1092–1155; 评估/S6: 1157–1227; S0 任务解码: 836–871;
  coach rail: 1230–1405 (header 1232–1243, thread 1245–1280, 三键处置/CRAAP card ~1281–1343,
  装备栏 1344–1364, composer 1366–1404); methodology modal: 1408–1449; station data `STA` 2092–2100;
  equip `EQ` 2286–2293; S4 meta `S4META` 1947–1953; Bean: `Bean.dc.html`.

---

## Task 1: `Bean` mascot + `StudioState` types + fixture

**Files:**
- Create: `apps/web/src/studio/Bean.tsx`, `apps/web/src/studio/Bean.test.tsx`
- Create: `apps/web/src/studio/state.ts`
- Create: `apps/web/src/studio/fixtures.ts`, `apps/web/src/studio/fixtures.test.tsx`

**Interfaces:**
- Produces: `Bean({color, size}: {color?: string; size?: number})`; all `state.ts` types below; `STUDIO_FIXTURE: StudioState`.
- Consumes: `SourceFixture`/`SOURCE_FIXTURES` from `../workspace/material/fixtures`.

- [ ] **Step 1: Write `state.ts`** (the view-model — the 5a↔5b seam)

```ts
import type { SourceFixture } from "../workspace/material/fixtures";

export type StationCode = "S0" | "S1" | "S2" | "S3" | "S4" | "S5" | "S6";
export type StationView = "结构" | "素材" | "写作" | "评估" | "onboarding";
export type StationState = "done" | "current" | "locked";

export type Station = {
  code: StationCode;
  name: string;
  view: StationView;
  state: StationState;
  gate?: { total: number; passed: number };
  backflow?: boolean;
};

export type CoachMessage =
  | { kind: "student"; body: string }
  | { kind: "ai"; body: string; tag?: string }
  | { kind: "flag"; label: string; body: string };

export type EquipCard = { id: string; name: string; spont: "自发" | "提示后" };

export type StructureCardFx = {
  id: string;
  role: string;         // 核心主张 / 理据·推理 / 支撑证据 / 反方·钢人 / 让步·转折
  status: "done" | "active" | "empty";
  preview?: string;     // collapsed text when done
  question?: string;    // AI question when active
};

export type GaugeFx = {
  table: string;        // 表A .. 表H
  total: number;
  lit: number;
  note: string;
  level: "full" | "partial" | "empty";
};

export type RubricRow = { official: string; plain: string; weak: boolean };
export type OnboardingFx = {
  restatePrompt: string;
  rubricRows: RubricRow[];
  planSteps: string[];  // 立题 / 找素材 / 评估来源 / 搭论证 / 成稿 / 反思归档
};

export type StudioState = {
  project: { title: string; qualLabel: string };
  stations: Station[];
  activeStation: StationCode;
  focusMode: boolean;
  coach: {
    anchor: string;
    messages: CoachMessage[];
    equipment: EquipCard[];
  };
  views: {
    material: SourceFixture[];
    structure: StructureCardFx[];
    writing: { draft: string; mode: "edit" | "preview" };
    review: GaugeFx[];
    onboarding: OnboardingFx;
  };
};

export type StudioCallbacks = {
  onSelectStation: (code: StationCode) => void;
  onToggleFocus: () => void;
  onDisposition: (choice: "accept" | "revise" | "reject", reason: string) => void;
  onOpenMethodology: (cardId: string) => void;
  onComposerSend: (text: string) => void;
};
```

- [ ] **Step 2: Write `Bean.tsx`** (port `Bean.dc.html` — the breathing/blinking mascot)

```tsx
type BeanProps = { color?: string; size?: number };

// Ported from the Claude Design Bean.dc.html: a rounded "bean" body with two
// blinking eyes; eye color contrasts with the fill (light eyes on dark beans).
export function Bean({ color = "#2A3B7A", size = 38 }: BeanProps) {
  const m = /^#?([0-9a-f]{6})$/i.exec(color.trim());
  let eye = "#17223B";
  if (m) {
    const n = parseInt(m[1]!, 16);
    const r = (n >> 16) & 255, g = (n >> 8) & 255, b = n & 255;
    const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
    eye = lum < 0.55 ? "#FFFFFF" : "#17223B";
  }
  return (
    <svg viewBox="0 0 120 120" width={size} height={size} style={{ display: "block", overflow: "visible" }} aria-hidden="true">
      <g style={{ transformOrigin: "60px 60px" }}>
        <path
          d="M59.5 24.8 C75.7 23.7 93.3 31.9 98.9 47.2 C105.1 64.1 96.2 82.8 80.5 91.2 C64.6 99.7 42.4 96.4 30.2 83.4 C18.5 71.0 18.8 51.2 30.9 38.7 C37.8 31.6 48.1 25.6 59.5 24.8Z"
          fill={color}
        />
        <ellipse cx="49.4" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
        <ellipse cx="68.3" cy="58.2" rx="2.6" ry="3.9" fill={eye} />
      </g>
    </svg>
  );
}
```

- [ ] **Step 3: Write `fixtures.ts`** — a complete `StudioState` for the 0457 scenario, S4 current

The fixture must have 7 stations S0–S6 in order (names 任务解码/立题/视角与素材/信源评估/论证构建/成稿打磨/反思归档; views 评估/结构/素材/素材/结构/写作/评估 per the design's `STA`, lines 2092–2100), with S0–S2 `done`, S3 `done`+`backflow:true`, S4 `current` with `gate:{total:5, passed:2}`, S5/S6 `locked`. `activeStation:"S4"`, `focusMode:false`. `coach.anchor:"论证图 · 治理决心主张"`, 3 messages (one `student`, one `ai` with `tag:"锚定 D5"`, one `flag` label `孤儿证据`), `equipment` = the 6 `EQ` cards (钢人卡/让步段卡/Toulmin 图/SIFT 横向阅读/CRAAP 五维/横向核查 with 自发/提示后 badges, lines 2286–2293). `views.material: SOURCE_FIXTURES` (reuse Slice 1). `views.structure`: 5 `StructureCardFx` (S4META roles 核心主张/理据·推理/支撑证据/反方·钢人/让步·转折, lines 1947–1953; 2 `done` with previews, 1 `active` with a question, 2 `empty`). `views.writing`: real China-sustainability draft prose + `mode:"edit"`. `views.review`: 8 `GaugeFx` (表A–表H). `views.onboarding`: restate prompt + 4 rubric rows (official vs plain) + 6 plan steps. Use real content (AGENTS.md — no lorem ipsum).

- [ ] **Step 4: Write the tests** — `Bean.test.tsx` + `fixtures.test.tsx`

```tsx
// Bean.test.tsx
import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Bean } from "./Bean";

describe("Bean", () => {
  it("renders an svg and picks a light eye on a dark fill, dark eye on a light fill", () => {
    const { container: dark } = render(<Bean color="#2A3B7A" />);
    expect(dark.querySelector("svg")).toBeTruthy();
    expect(dark.querySelectorAll("ellipse")[0]!.getAttribute("fill")).toBe("#FFFFFF");
    const { container: light } = render(<Bean color="#E8A33D" />);
    expect(light.querySelectorAll("ellipse")[0]!.getAttribute("fill")).toBe("#17223B");
  });
});
```

```tsx
// fixtures.test.tsx
import { describe, it, expect } from "vitest";
import { STUDIO_FIXTURE } from "./fixtures";

describe("STUDIO_FIXTURE", () => {
  it("has 7 stations S0-S6 in order with exactly one current", () => {
    const codes = STUDIO_FIXTURE.stations.map((s) => s.code);
    expect(codes).toEqual(["S0", "S1", "S2", "S3", "S4", "S5", "S6"]);
    expect(STUDIO_FIXTURE.stations.filter((s) => s.state === "current")).toHaveLength(1);
  });
  it("the active station resolves to a station with a view", () => {
    const active = STUDIO_FIXTURE.stations.find((s) => s.code === STUDIO_FIXTURE.activeStation);
    expect(active).toBeTruthy();
    expect(active!.view).toBeTruthy();
  });
});
```

- [ ] **Step 5: Run tests + commit**

Run: `cd apps/web && npx vitest run src/studio/Bean.test.tsx src/studio/fixtures.test.tsx`
Expected: PASS.
```bash
git add apps/web/src/studio/Bean.tsx apps/web/src/studio/Bean.test.tsx apps/web/src/studio/state.ts apps/web/src/studio/fixtures.ts apps/web/src/studio/fixtures.test.tsx
git commit -m "feat(web): Studio Bean mascot + StudioState view-model + fixture

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: `StationRail` — the S0–S6 contract map

**Files:**
- Create: `apps/web/src/studio/StationRail.tsx`, `apps/web/src/studio/StationRail.test.tsx`

**Interfaces:**
- Consumes: `Station`, `StationCode` (state.ts).
- Produces: `StationRail({ stations, active, focus, onSelect }: { stations: Station[]; active: StationCode; focus: boolean; onSelect: (c: StationCode) => void })`.

Build to design lines 782–825. Behavior: header `任务旅程` (+ a `?` help affordance with the design's title text). Each station is a clickable row showing: a numbered/icon circle colored by state (`done` → green `#4C9A82` filled + check; `current` → navy ring `box-shadow:0 0 0 3px #E4E8F5`; `locked` → grey outline + padlock), the name (`nameStyle` bolded for current), a `有据修正` pill when `backflow`, and — **only for the current station** — the gate strip `本环节门禁 {total} 项，已过 {passed}` followed by `total` dot segments (`passed` navy `#2A3B7A`, rest `#D6DBE8`). In `focus` mode render an icon-only 64px rail (names/gate hidden); otherwise 246px. Every row (including `locked`) calls `onSelect(code)` — soft-lock preview (design tooltip line 787: 可点击上锁环节先行预览). Rail width via a `style` width, not a Tailwind class.

- [ ] **Step 1: Write the failing test**

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StationRail } from "./StationRail";
import { STUDIO_FIXTURE } from "./fixtures";

describe("StationRail", () => {
  it("renders 7 stations and the current station's gate strip", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} />);
    expect(screen.getByText("任务解码")).toBeInTheDocument();
    expect(screen.getByText("反思归档")).toBeInTheDocument();
    expect(screen.getByText(/本环节门禁 5 项，已过 2/)).toBeInTheDocument();
  });

  it("shows the 有据修正 backflow pill on S3", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={() => {}} />);
    expect(screen.getByText("有据修正")).toBeInTheDocument();
  });

  it("clicking a LOCKED station still fires onSelect (soft-lock preview)", () => {
    const onSelect = vi.fn();
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={false} onSelect={onSelect} />);
    fireEvent.click(screen.getByText("反思归档")); // S6, locked
    expect(onSelect).toHaveBeenCalledWith("S6");
  });

  it("in focus mode hides station names", () => {
    render(<StationRail stations={STUDIO_FIXTURE.stations} active="S4" focus={true} onSelect={() => {}} />);
    expect(screen.queryByText("任务解码")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `cd apps/web && npx vitest run src/studio/StationRail.test.tsx` → FAIL (module missing).
- [ ] **Step 3: Implement `StationRail.tsx`** to the design (lines 782–825) satisfying the test. Use inline SVG for the check/padlock/help icons (lift from the design). Gate dots: render `total` spans, first `passed` navy, rest grey.
- [ ] **Step 4: Run to verify it passes.**
- [ ] **Step 5: Commit** (`feat(web): Studio station rail — the S0–S6 contract map with gate progress`).

---

## Task 3: `OnboardingView` — S0 任务解码 (recognition) + S1/S2

**Files:**
- Create: `apps/web/src/studio/views/OnboardingView.tsx`, `.../OnboardingView.test.tsx`

**Interfaces:**
- Consumes: `OnboardingFx`, `Station` (state.ts).
- Produces: `OnboardingView({ station, data }: { station: Station; data: OnboardingFx })`.

Build to design lines 836–871 (S0 任务解码). S0 renders three cards: (1) 用自己的话，说清这份任务在考什么 — subtitle + a textarea (`restatePrompt` as placeholder), (2) 评分表 · 翻成人话 — the `rubricRows` list (official grey vs plain), each clickable to toggle a 待加强 marker (local `useState<Set<number>>`), (3) 我的写作计划 — a horizontal step tracker over `planSteps` (6 dots + labels). S1/S2 (立题/视角与素材) render a lightweight shell (station name + a "此环节的深入交互将在后续切片接入" placeholder card) — these deepen in Slices 6–7. Branch on `station.code`.

- [ ] **Step 1: Failing test**

```tsx
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OnboardingView } from "./OnboardingView";
import { STUDIO_FIXTURE } from "../fixtures";

const s0 = STUDIO_FIXTURE.stations.find((s) => s.code === "S0")!;

describe("OnboardingView (S0 任务解码)", () => {
  it("renders the restate card, rubric rows, and the plan tracker", () => {
    render(<OnboardingView station={s0} data={STUDIO_FIXTURE.views.onboarding} />);
    expect(screen.getByText(/说清这份任务在考什么/)).toBeInTheDocument();
    expect(screen.getByText(/评分表/)).toBeInTheDocument();
    const firstPlain = STUDIO_FIXTURE.views.onboarding.rubricRows[0]!.plain;
    expect(screen.getByText(firstPlain)).toBeInTheDocument();
  });

  it("clicking a rubric row toggles its 待加强 marker", () => {
    render(<OnboardingView station={s0} data={STUDIO_FIXTURE.views.onboarding} />);
    const firstPlain = STUDIO_FIXTURE.views.onboarding.rubricRows[0]!.plain;
    fireEvent.click(screen.getByText(firstPlain));
    expect(screen.getAllByText("待加强").length).toBeGreaterThan(0);
  });
});
```

- [ ] **Step 2–5:** run-fail → implement → run-pass → commit (`feat(web): Studio onboarding view — S0 任务解码 recognition + S1/S2 shells`).

---

## Task 4: `StructureView` + `WritingView` + `ReviewView` shells

**Files:**
- Create: `apps/web/src/studio/views/StructureView.tsx`, `WritingView.tsx`, `ReviewView.tsx` + one test file each.

**Interfaces:**
- Produces: `StructureView({ cards }: { cards: StructureCardFx[] })` (design 959–1016); `WritingView({ draft, mode }: { draft: string; mode: "edit" | "preview" })` (design 1092–1155); `ReviewView({ gauges }: { gauges: GaugeFx[] })` (design 1157–1227).

Shells (deep interactions → Slices 7/8/9):
- **StructureView** — the gate banner (`本环节门禁…` + orange "not clean" / green "all clean" banner keyed off whether any card is `empty`) then the 5 `StructureCardFx` as a card list: role chip + status chip + collapsed `preview` (done) or the AI `question` bubble (active) + a disabled "① 选择相关素材 / ② 把这一步写成句子" hint (the live picker/textarea is Slice 7).
- **WritingView** — the 编辑·安静 / 预览·批注 sub-tab row; `edit` → a read-only full-height textarea showing `draft` + hint "写作时印记不会打断你——想听意见，点「整稿体检」"; `preview` → rendered paragraphs + a disabled 整稿体检 button (the live work-order is Slice 8).
- **ReviewView** — the 就绪度 8-lamp gauge grid (2×4 over `gauges`): each table shows `table` name, a lamps bar (`lit` filled of `total`, colored by `level`: full `#4C9A82`, partial `#D9A23D`, empty `#AEB4C2`), and `note`. The self-score/reflection/declaration blocks are Slice 9 — render only the gauge grid here.

- [ ] **Step 1: Failing tests** (one per view). Example for StructureView:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StructureView } from "./StructureView";
import { STUDIO_FIXTURE } from "../fixtures";

describe("StructureView (结构/S4 shell)", () => {
  it("renders the gate banner and the 5 Toulmin role cards", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/门禁/)).toBeInTheDocument();
    for (const c of STUDIO_FIXTURE.views.structure) {
      expect(screen.getByText(c.role)).toBeInTheDocument();
    }
  });
});
```
WritingView test: asserts the edit textarea shows the draft + the two sub-tabs render; toggling to `preview` (pass `mode="preview"`) shows the 整稿体检 button. ReviewView test: asserts all 8 table names render and the lit-lamp count matches a fixture gauge.

- [ ] **Step 2–5:** run-fail → implement all three → run-pass → commit (`feat(web): Studio 结构/写作/评估 view shells (deep interactions deferred to 6-9)`).

---

## Task 5: `ViewFrame` — the center switcher (station rail = view switcher)

**Files:**
- Create: `apps/web/src/studio/ViewFrame.tsx`, `ViewFrame.test.tsx`

**Interfaces:**
- Consumes: all view components + `SourceDossier` (`../workspace/material/SourceDossier`) + `Station`, `StudioState` (state.ts).
- Produces: `ViewFrame({ state }: { state: StudioState })` — renders the active station's header (station name, design 829–834) + the matching view, switching on the active station's `view`/`code`.

Resolution: find `active = state.stations.find(s => s.code === state.activeStation)`. Then: `active.view === "素材"` → `<SourceDossier sources={state.views.material} />`; `"结构"` → `<StructureView cards={state.views.structure} />`; `"写作"` → `<WritingView {...state.views.writing} />`; `"评估"` → `<ReviewView gauges={state.views.review} />`; `"onboarding"` → `<OnboardingView station={active} data={state.views.onboarding} />`.

- [ ] **Step 1: Failing test**

```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ViewFrame } from "./ViewFrame";
import { STUDIO_FIXTURE } from "./fixtures";

describe("ViewFrame (station rail = view switcher)", () => {
  it("S4 active renders the 结构 view with its role cards + the station header", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S4" }} />);
    expect(screen.getByText("论证构建")).toBeInTheDocument();
    expect(screen.getByText(STUDIO_FIXTURE.views.structure[0]!.role)).toBeInTheDocument();
  });
  it("switching activeStation to S3 renders the 素材 SourceDossier", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S3" }} />);
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();
  });
  it("S0 active renders the onboarding recognition", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S0" }} />);
    expect(screen.getByText(/说清这份任务在考什么/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2–5:** run-fail → implement → run-pass → commit (`feat(web): Studio ViewFrame — station-rail-driven four-view switcher`).

---

## Task 6: `DispositionCard` + `EquipmentBar` + `MethodologyModal` (coach sub-pieces)

**Files:**
- Create: `apps/web/src/studio/DispositionCard.tsx` + test; `EquipmentBar.tsx` + test; `MethodologyModal.tsx` + test.

**Interfaces:**
- `DispositionCard({ tag, anchor, body, onDisposition }: { tag: string; anchor: string; body: string; onDisposition: (c: "accept"|"revise"|"reject", reason: string) => void })` — design ~1281–1343 (the 三键处置 variant). Three buttons 接受/我自己改/不采纳; a reason `<textarea>` with a live rune-count; submit **disabled until the reason is ≥15 characters (rune count via `[...reason].length`, matching the backend)**; the hint turns green at ≥15. On submit, calls `onDisposition(choice, reason)`.
- `EquipmentBar({ cards, open, onToggle, onOpen }: { cards: EquipCard[]; open: boolean; onToggle: () => void; onOpen: (id: string) => void })` — design 1344–1364. When `open`, the popover lists chip buttons (name + 自发/提示后 badge, green vs grey); a chip click calls `onOpen(id)`. A toolbox toggle calls `onToggle`.
- `MethodologyModal({ cardId, onClose }: { cardId: string | null; onClose: () => void })` — design 1408–1449. Renders nothing when `cardId` is null; otherwise a scrim + centered card titled `工具说明书 · 我不懂为什么` with why/how/help sections (static copy keyed by `cardId`, with a sensible default) + a close button calling `onClose`.

- [ ] **Step 1: Failing tests.** DispositionCard is the important one:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DispositionCard } from "./DispositionCard";

describe("DispositionCard (三键处置)", () => {
  it("requires a >=15-rune reason before submit", () => {
    const onDisposition = vi.fn();
    render(<DispositionCard tag="D5" anchor="治理决心主张" body="这条主张还没有素材支撑" onDisposition={onDisposition} />);
    fireEvent.click(screen.getByText("不采纳"));
    const box = screen.getByRole("textbox");
    fireEvent.change(box, { target: { value: "太短" } });
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(onDisposition).not.toHaveBeenCalled();
    fireEvent.change(box, { target: { value: "这条追问和我的方向不一致，我想先按自己的思路推进" } });
    fireEvent.click(screen.getByText(/提交|钉/));
    expect(onDisposition).toHaveBeenCalledWith("reject", expect.stringContaining("方向"));
  });
});
```
EquipmentBar test: `open=true` shows a chip; clicking it fires `onOpen`. MethodologyModal test: `cardId=null` renders nothing; a non-null id shows `工具说明书`; close fires `onClose`.

- [ ] **Step 2–5:** run-fail → implement the three → run-pass → commit (`feat(web): Studio coach sub-pieces — 三键处置 disposition, 装备栏, 工具说明书`).

---

## Task 7: `CoachRail` — header + thread + tool-card slot + composer

**Files:**
- Create: `apps/web/src/studio/CoachRail.tsx`, `CoachRail.test.tsx`

**Interfaces:**
- Consumes: `Bean`, `DispositionCard`, `EquipmentBar`, `MethodologyModal`, `CoachMessage`, `EquipCard`.
- Produces: `CoachRail({ anchor, messages, equipment, activeView, onDisposition, onOpenMethodology, onSend }: { anchor: string; messages: CoachMessage[]; equipment: EquipCard[]; activeView: StationView; onDisposition: StudioCallbacks["onDisposition"]; onOpenMethodology: (id: string) => void; onSend: (t: string) => void })`.

Build to design 1230–1405. Fixed 388px column: header (`<Bean size={30}/>` + `AI 陪练` + `正在看：{anchor}` with the pulsing `#4C9A82` dot) → scrollable thread (`messages`: student navy-right bubbles / ai grey-left bubbles with an optional `锚定 {tag}` pill / flag orange-left cards with the label) → the contextual tool card (when `activeView === "素材"` a CRAAP tool-card placeholder; else `<DispositionCard>` fed from the last `ai` message's tag/body — for 5a wire it from a representative fixture prop or the last ai message) → `<EquipmentBar>` (local `open` state + `MethodologyModal` via local `methId` state, `onOpen` sets it, `onOpenMethodology` also notified) → composer (装备栏 toggle · attach icon · textarea bound to local state · mic icon · send button calling `onSend` then clearing).

- [ ] **Step 1: Failing test**

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachRail } from "./CoachRail";
import { STUDIO_FIXTURE } from "./fixtures";

const c = STUDIO_FIXTURE.coach;

describe("CoachRail", () => {
  it("renders the anchor status, the thread, and the 装备栏 toggle", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    expect(screen.getByText(new RegExp(c.anchor))).toBeInTheDocument();
    expect(screen.getByText("孤儿证据")).toBeInTheDocument(); // the flag message label
  });
  it("sends composer text", () => {
    const onSend = vi.fn();
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={onSend} />);
    fireEvent.change(screen.getByPlaceholderText(/发给印记/), { target: { value: "我加了一条证据" } });
    fireEvent.click(screen.getByLabelText(/发送|send/i));
    expect(onSend).toHaveBeenCalledWith("我加了一条证据");
  });
});
```

- [ ] **Step 2–5:** run-fail → implement → run-pass → commit (`feat(web): Studio coach rail — thread + tool-card + 装备栏 + composer`).

---

## Task 8: `StudioShell` + `StudioPanel` harness mount + `index.ts` + tokens

**Files:**
- Create: `apps/web/src/studio/StudioShell.tsx`, `StudioShell.test.tsx`, `apps/web/src/studio/index.ts`, `apps/web/src/dev/StudioPanel.tsx`, `StudioPanel.test.tsx`
- Modify: `apps/web/src/dev/DevApp.tsx` (add a 工作室 tab); optionally `apps/web/tailwind.config.ts` (add `mk-flag #C96F4F`, `mk-purple #7C6BB5`, `mk-amber-2 #D9A23D` if used widely).

**Interfaces:**
- Consumes: `StationRail`, `ViewFrame`, `CoachRail`, `Bean`, `StudioState`, `StudioCallbacks`.
- Produces: `StudioShell({ state, callbacks }: { state: StudioState; callbacks: StudioCallbacks })`; `StudioPanel()` (fixture host).

`StudioShell` (design 757–780 + the 3-column body 780): the top bar (hidden when `state.focusMode`) — back chevron + 写作工作室 + divider + `state.project.title` + qual pill `state.project.qualLabel` + a 专注模式 toggle button (calls `callbacks.onToggleFocus`) — then the body row: `<StationRail>` (width by focus) + `<ViewFrame state={state}/>` (center, `flex:1 min-width:520px background:#F3F4F8`) + `<CoachRail>` (388px, fed from `state.coach` + `callbacks`). A focus-exit floating button when `focusMode`.

`StudioPanel` (harness host, mirrors `MaterialPanel`): holds `STUDIO_FIXTURE` in `useState`; wires `callbacks` to mutate that local state so clicks work — `onSelectStation` sets `activeStation`, `onToggleFocus` flips `focusMode`, the rest push to a visible event log (like `MaterialPanel`). Add a `工作室` tab to `DevApp` (`Tab` union gains `"studio"`).

- [ ] **Step 1: Failing tests**

```tsx
// StudioShell.test.tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioShell } from "./StudioShell";
import { STUDIO_FIXTURE } from "./fixtures";

const noop = { onSelectStation: () => {}, onToggleFocus: () => {}, onDisposition: () => {}, onOpenMethodology: () => {}, onComposerSend: () => {} };

describe("StudioShell", () => {
  it("renders the three regions: station rail, center view, coach rail", () => {
    render(<StudioShell state={STUDIO_FIXTURE} callbacks={noop} />);
    expect(screen.getByText("任务解码")).toBeInTheDocument();       // rail
    expect(screen.getByText("论证构建")).toBeInTheDocument();       // center header (S4 active)
    expect(screen.getByText("AI 陪练")).toBeInTheDocument();        // coach rail
  });
  it("专注模式 toggle fires onToggleFocus", () => {
    const onToggleFocus = vi.fn();
    render(<StudioShell state={STUDIO_FIXTURE} callbacks={{ ...noop, onToggleFocus }} />);
    fireEvent.click(screen.getByText(/专注/));
    expect(onToggleFocus).toHaveBeenCalled();
  });
});
```

```tsx
// StudioPanel.test.tsx — the harness wiring actually switches views on station click
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioPanel } from "./StudioPanel";

describe("StudioPanel (harness)", () => {
  it("clicking a station in the rail switches the center view", () => {
    render(<StudioPanel />);
    expect(screen.getByText("论证构建")).toBeInTheDocument();     // S4 default
    fireEvent.click(screen.getByText("信源评估"));                 // S3
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();      // 素材 view now shown
  });
});
```

- [ ] **Step 2: Run to verify they fail.**
- [ ] **Step 3: Implement** `StudioShell`, `index.ts` (export the public surface), `StudioPanel`, and the `DevApp` 工作室 tab; add tokens if used.
- [ ] **Step 4: Run to verify they pass** + run the whole web suite: `cd apps/web && npx vitest run` — all green (existing tests unaffected — new tree, `DevApp` only gains a tab).
- [ ] **Step 5: Typecheck + commit**

Run: `cd apps/web && npx tsc --noEmit` → clean.
```bash
git add apps/web/src/studio/ apps/web/src/dev/StudioPanel.tsx apps/web/src/dev/StudioPanel.test.tsx apps/web/src/dev/DevApp.tsx apps/web/tailwind.config.ts
git commit -m "feat(web): Studio shell + dev-harness mount (3-column layout, focus mode, live view-switching)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final gate (before the whole-branch review)

- [ ] `cd apps/web && npx vitest run` — full web suite green (new studio tests + all pre-existing).
- [ ] `cd apps/web && npx tsc --noEmit` — clean.
- [ ] The old `workspace/`, `shell/StudentApp.tsx`, `Root.tsx`, and app routing are untouched (only `DevApp.tsx` gained a tab; `SourceDossier` is imported, not modified).
- [ ] Visual spot check via the dev harness (`?dev`/`?demo` per `Root.tsx` — confirm how the harness is reached) is optional; the tests are the gate.

## Self-review notes (plan author)

- **Spec coverage:** StudioShell+focus (T8) · StationRail/contract map (T2) · four-view frame + station-rail-switcher + SourceDossier reuse (T5) · view shells (T3/T4) · CoachRail + 三键处置(≥15 rune) + 装备栏 + methodology (T6/T7) · StudioState seam + Bean + fixture (T1) · dev-harness mount (T8). All spec §0/§7 items map to a task.
- **Type consistency:** `StudioState`/`Station`/`StudioCallbacks`/`StationView` used identically across T1–T8; `DispositionCard` choice strings `"accept"|"revise"|"reject"` match `StudioCallbacks.onDisposition`.
- **Deferred (documented):** deep per-view interactions → Slices 6–9; live backend/coach/routing/retirement → 5b. The view shells intentionally render disabled affordances where the live interaction lands later.
- **Known approximation:** the CRAAP tool-card variant in the coach rail (素材 view) is a placeholder in 5a; the live CRAAP card is Slice 6.
