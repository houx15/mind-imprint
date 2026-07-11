# Slice 5a — Studio shell + station rail + four-view frame + coach rail (fixture-backed)

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 5a of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`); the first half of roadmap Slice 5, split per the plan |
| **Sources** | The binding design `docs/design/思维印记_工作区.dc.html` (the refreshed 2767-line version — the Writing Studio: 工作台/四视图/S0–S6 rail/AI 陪练 coach rail/装备栏) — primary for all UI · product spec §5.2 (S0–S6 + four views), §6 (surfaces), §7 (four red lines) · agent-spec §5.2 (the plan artifact, the coach) |
| **Depends on** | Slice 1 (`annotate` primitive + `SourceDossier` — reused by the 素材 view) · Slices 0–4 conceptually (the `StudioState` fixture mirrors the backend projection 5b will build from skill contracts + gate reports + plan) |
| **Delivers** | The whole Studio **chrome** as controlled, fixture-backed, testable React UI: the 3-column shell + focus mode, the S0–S6 station rail (the contract map, with gate progress), the four-view center frame (station-rail = view-switcher), and the AI 陪练 coach rail (thread + 三键处置 + 装备栏 + composer) — built to the `.dc.html`, mounted in the dev harness. No backend, no live model, no app-routing change. |

## 0. Scope and boundary

Slice 5a builds the Studio's **shell and chrome** — every region the design shows around the
per-view content — as controlled React components driven by a single `StudioState` fixture, and
mounts it in the dev harness. It is the UI counterpart of Slices 0–4: the station rail renders
what the contract DAG + gate engine compute, the coach rail renders what the runtime loop emits,
and the `StudioState` shape is the exact seam **5b** will populate from the live backend.

This mirrors Slice 1's discipline: a **new tree** (`apps/web/src/studio/`), controlled components
(state + callbacks passed in, no internal fetch/model), fixtures with real content (the
China-sustainability / 0457 scenario), a dev-harness mount, and **zero changes to the old
task-centric `workspace/` or `shell/StudentApp`** — the routing flip and the old workspace's
retirement are 5b's job.

**In scope (controlled UI + fixtures, no backend):**
- **StudioShell** — the 3-column layout (station rail │ center │ coach rail) inside a Studio content
  column with the top bar (back · 写作工作室 · project title · qual pill · 专注模式 toggle) and
  **focus mode** (collapse nav + station rail to icon strips; the coach rail stays 388px).
- **StationRail (the contract map)** — the S0–S6 `任务旅程` list: per-station state (done ✓ /
  current ring / locked 🔒 with soft-lock preview), the current station's **gate strip**
  (`本环节门禁 N 项，已过 M` + dot segments), the `有据修正` backflow pill. Click switches the
  active station → view.
- **ViewFrame (center)** — renders the active station's view: the station-name header + the view
  container. The four **view shells** with representative fixture content: 素材 reuses the Slice-1
  `SourceDossier`; 结构 = the Toulmin card-list shell; 写作 = the edit/preview draft shell; 评估 =
  the readiness-gauge shell; S0–S2 = the onboarding cards (S0 `任务解码` is the built first-entry
  recognition moment). Deep per-view interactions are Slices 6–9.
- **CoachRail** — header (Bean mascot + `AI 陪练` + `正在看：{anchor}` live status) + conversation
  thread (student / AI / orange **flag** bubbles, with rubric tag pills) + the contextual tool card
  (**三键处置** accept/我自己改/不采纳 + a ≥15-char reason field with a live counter, or the **CRAAP**
  tool card in 素材) + the **装备栏** collapsible popover (card chips with 自发/提示后 badges →
  methodology modal) + the composer (装备栏 toggle · attach · textarea · mic · send).
- **StudioState** view-model type + a fixture, and a **Bean** mascot component (ported from
  `Bean.dc.html`).

**Out of scope:**
- **→ 5b:** a projects API; deriving `StudioState` from the Slice-4 graph/gate/plan; the live coach
  (post_intervention / check_gate / plan) over SSE; real disposition persistence; the app-routing
  flip + retiring the old chat `workspace/`; the Slice-3 deferred debt (task_id-nullable migration +
  project-scope `GetCardInstance`).
- **→ Slices 6–9:** deep per-view interactions — the live annotate dossier + source log (6); the
  `graph` primitive + Toulmin map editing (7); the draft silent-edit buffer + whole-draft review
  work-order (8); the readiness-gauge computation + reflection + export (9). 5a builds the shells
  those slices fill.

## 1. Decisions (flagged for review)

- **New `apps/web/src/studio/` tree, controlled + fixture-backed, dev-harness mounted.** No routing
  change; the old `workspace/`/`StudentApp` are untouched (retired in 5b). Same discipline as Slice 1.
- **The station rail IS the four-view switcher** (the design's real behavior — the `.dc.html`'s
  independent view-switcher and its `board` var are vestigial/dead). A `Station` carries a `view`
  (`结构|素材|写作|评估` or an onboarding kind); clicking a station sets the active station and the
  center renders that station's view. No separate segmented control.
- **`StudioState` is a frontend view-model, not a wire contract.** It lives in
  `apps/web/src/studio/state.ts`. 5b will add a `projectToStudioState(...)` projection from the
  backend (skill contracts via `TopoOrder` → stations; gate reports → gate progress + done/current/
  locked; the plan's route frontier → current + coach anchor). Keeping it frontend-local avoids
  coupling the wire contracts to a UI shape.
- **Reuse Slice-1 `SourceDossier`** for the 素材 view shell (it already renders the annotate
  dossier over fixtures). The other three view shells are new, shallow containers.
- **View shells, not deep interactions.** 5a renders each view's frame + representative fixture
  content at the design's structure; the deep interactions land in Slices 6–9. The boundary: 5a
  proves the *frame, switching, and chrome*; it does not build the graph editor, the live draft
  buffer, or gauge computation.
- **Follow the built design, skip the unbuilt stubs.** The `.dc.html` half-wires a "recognized"
  entry interstitial and stubs resume/report screens; 5a builds S0 `任务解码` as the recognition
  moment and omits the unbuilt screens.
- **Port `Bean.dc.html` as a `Bean` React component** (the breathing/blinking mascot SVG, colorized
  by `avatarColor`). The design imports it via `<dc-import name="Bean">` in the nav, coach header,
  and entry screen.

## 2. The `StudioState` view-model (the 5a↔5b seam)

The single object that drives the whole shell. 5a supplies a fixture; 5b supplies a projection.

```ts
type StationView = "结构" | "素材" | "写作" | "评估" | "onboarding";
type StationState = "done" | "current" | "locked";

type Station = {
  code: "S0"|"S1"|"S2"|"S3"|"S4"|"S5"|"S6";
  name: string;                 // 任务解码 / 立题 / 视角与素材 / 信源评估 / 论证构建 / 成稿打磨 / 反思归档
  view: StationView;
  state: StationState;
  gate?: { total: number; passed: number }; // shown under the current station
  backflow?: boolean;           // 有据修正 pill
};

type CoachMessage =
  | { kind: "student"; body: string }
  | { kind: "ai"; body: string; tag?: string }        // tag = 锚定 {criterion}
  | { kind: "flag"; label: string; body: string };     // orange callout, e.g. 孤儿证据

type EquipCard = { id: string; name: string; spont: "自发" | "提示后" };

type StudioState = {
  project: { title: string; qualLabel: string };       // 0457 个人报告
  stations: Station[];                                  // S0..S6 in order
  activeStation: Station["code"];                       // freely switchable
  focusMode: boolean;
  coach: {
    anchor: string;                                     // 正在看：{anchor}
    messages: CoachMessage[];
    equipment: EquipCard[];
  };
  // per-view fixture content (shallow shells in 5a; Slices 6–9 replace with live data)
  views: {
    material: SourceFixture[];      // reuse Slice-1 fixtures for SourceDossier
    structure: StructureCardFx[];   // 5 Toulmin cards (role/status/preview)
    writing: { draft: string; mode: "edit" | "preview" };
    review: GaugeFx[];              // 表A..表H lamps
    onboarding: OnboardingFx;       // S0 restate + rubric rows + plan steps
  };
};
```

Callbacks (the controlled surface): `onSelectStation(code)`, `onToggleFocus()`, `onDisposition(choice, reason)`, `onOpenMethodology(cardId)`, `onToggleEquip()`, `onComposerSend(text)`. In 5a these drive local fixture state / no-op logging; 5b wires them to the API.

## 3. Component tree

```
studio/
  Bean.tsx                 // mascot SVG (ported from Bean.dc.html), props {color, size}
  state.ts                 // StudioState + sub-types (above)
  fixtures.ts              // a full StudioState fixture (0457 China-sustainability, S4 current)
  StudioShell.tsx          // 3-column layout + top bar + focus mode; owns activeStation/focus via props
  StationRail.tsx          // 任务旅程 list, per-station state + gate strip + soft-lock
  ViewFrame.tsx            // center: station header + switch on active station.view
  views/
    OnboardingView.tsx     // S0/S1/S2 cards (S0 = 任务解码 recognition)
    StructureView.tsx      // 结构 (S4) Toulmin card-list shell
    WritingView.tsx        // 写作 (S5) edit/preview draft shell
    ReviewView.tsx         // 评估 (S6) readiness-gauge shell
    // 素材 (S3) view = reuse workspace/material/SourceDossier (Slice 1)
  CoachRail.tsx            // header + thread + tool card + 装备栏 + composer
  DispositionCard.tsx      // 三键处置 (accept/我自己改/不采纳 + ≥15-char reason)
  EquipmentBar.tsx         // 装备栏 popover (chips + 自发/提示后 badges)
  MethodologyModal.tsx     // 工具说明书 · 我不懂为什么
  index.ts                 // exports
dev/
  StudioPanel.tsx          // mounts StudioShell over the fixture in the dev harness
```

`StudioShell` is controlled: it takes `state: StudioState` + the callbacks; a thin `StudioPanel`
holds the fixture in local React state so clicks (station switch, focus, disposition) visibly work
in the harness without a backend.

## 4. Layout & tokens (from the design)

- **Regions:** nav rail 74px (62px focus) │ station rail 246px (64px focus) │ center `flex:1;
  min-width:520px; background:#F3F4F8` │ coach rail 388px (fixed). Top bar 56px (hidden in focus).
- **Tokens** (match the `.dc.html`): navy `#2A3B7A` (hover `#22305F`, tint `#EDEFF9`); green
  `#4C9A82` (tint `#E7F3EE`); orange flag `#C96F4F`/`#D98263` (tint `#FBEEE7`); amber `#D9A23D`;
  purple `#7C6BB5`; text `#1C2333`/`#2B3346`/`#5B6373`; borders `#EAECF2`/`#E1E4ED`; canvas
  `#F3F4F8`; cards `#fff`. Gate colors: passed `#2A3B7A`/`#4C9A82`, empty `#AEB4C2`. Font
  `'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif`. Radii: pills 999px, buttons 9–12px,
  cards 13–16px, modal 20px. Keyframes `mkPulse` (coach status dot), `mkFade` (装备栏 / work-order),
  `mkPop`/`mkScrim` (modal).
- **Icons = inline SVG** (project rule — never lucide-react); lift the paths from the `.dc.html`.

## 5. Testing (vitest + React Testing Library, controlled, no network)

- **StudioShell** — renders the 3 columns; 专注模式 toggle collapses nav + station rail (asserts
  width/props change) and the coach rail stays.
- **StationRail** — renders S0–S6 with the fixture's states (done/current/locked icons); the current
  station shows its gate strip (`本环节门禁 N 项，已过 M`) with the right passed/total dot split;
  clicking a **locked** station still previews it (soft-lock) and fires `onSelectStation`.
- **ViewFrame** — switching `activeStation` renders the correct view (S3→SourceDossier, S4→Structure,
  S5→Writing, S6→Review, S0→Onboarding); the header shows the station name.
- **CoachRail** — renders the thread (student/ai/flag bubbles + a `锚定` tag); the `正在看：{anchor}`
  status; the 装备栏 toggle opens/closes the popover; a chip click fires `onOpenMethodology`.
- **DispositionCard** — the reason field enforces ≥15 chars (rune count, matching the backend):
  a short reason keeps 接受/提交 disabled + shows the hint; ≥15 chars enables it and turns green;
  submitting fires `onDisposition(choice, reason)`.
- **OnboardingView (S0)** — renders the restate textarea + rubric rows (official vs plain) +
  the 6-step plan tracker; marking a rubric row toggles 待加强.
- **Fixture integrity** — the `StudioState` fixture has 7 stations S0–S6 in order with exactly one
  `current`, and the active station's view resolves.
- **Bean** — renders an SVG with the given color; eye color contrasts on dark vs light fills.

No backend, no SSE, no model — every test drives props/fixtures and asserts rendered structure +
callback wiring, exactly like Slice 1's annotate/dossier tests.

## 6. Open questions for the plan

1. Whether `StudioPanel` (harness host) should persist its fixture edits to localStorage or keep
   them in memory. Default: in-memory (the harness is a demo surface; Slice 1's panel was in-memory).
2. How much of each view shell to render in 5a vs 6–9. Default (locked in §0): the frame + header +
   representative fixture content at the design's structure, reusing `SourceDossier` for 素材;
   no graph editor, no live draft buffer, no gauge computation.
3. Whether `Bean` lives in `studio/` or a shared `components/`. Default: `studio/Bean.tsx` for 5a
   (its only consumer); promote to shared if Chat/Course reuse it later.

## 7. Acceptance criteria

- The whole Studio chrome renders from a single `StudioState` fixture: the 3-column shell + focus
  mode, the S0–S6 station rail with per-station state + the current station's gate strip, the
  four-view center frame that switches on station click (station rail = view switcher), and the AI
  陪练 coach rail (thread + 三键处置 + 装备栏 + composer).
- The 素材 view reuses the Slice-1 `SourceDossier`; the other three views + S0–S2 render as shells.
- 三键处置 enforces a ≥15-char (rune-count) reason before submit, matching the backend gate.
- Controlled + fixture-backed: no backend, no SSE, no live model; every interaction is a callback.
- Built to the `.dc.html` tokens/structure; icons are inline SVG; the old `workspace/`/`StudentApp`
  and app routing are untouched. Mounted in the dev harness.
- The `StudioState` type is the documented seam 5b will populate from the Slice-4 backend.
