# Guided Tour P7 — Full projects-walk refinement · Design

**Status:** DESIGNED (2026-08-23) · builds on P6 ([[guided-tour-p6-reading-writing-walk-2026-08-23]], PROD backend `1a673579` / frontend `2a69a210`)
**Spec author:** houyx15
**Owner doc for writing-project behaviour:** `docs/2026-08-09-all-statuses.md`

## Why

Post-P6 user walkthrough found issues in every projects-tour segment: shallow/mis-ordered
reading flow, a fake stub paper, missing feature intros (activity log, 检索卡, 还需要探索 box,
building-block outline, 提案/正文 modes, AI-批注 trigger, to-explore box), a wrong sidebar
default, a mislabeled step, two bugs (未归档 node location, literal `\n\n` in the essay), and a
too-thin evaluation-axis intro. The demo stays **read-only + finished**; we seed real data,
fix bugs, add read-only teaching variants, and restructure the tour.

## Decisions (user-confirmed)

- **Add a real map-node read-state badge** to the warren graph (data-driven from
  `reading_status`), so the tour can genuinely show a node changing after reading.
- **One comprehensive P7**, sequenced reading → writing → plan/eval + fixes, one deploy.
- **Real paper:** author an ORIGINAL full-length (~10-paragraph) article on the demo's topic
  (China/greening/sustainability). Do NOT reproduce a copyrighted paper verbatim; keep the same
  thesis so the seeded reading transcript + 批注 still fit.
- **"Mock search/add/dive"** = real-scene where reachable (canned `dig` returns real
  candidates; the immersive room via the `pendingDemoReading` deep-link) + **spotlight-and-narrate**
  the write-only controls (采纳/add-source/enter/paste all 403 on the demo — show them, explain,
  don't execute). Minimise bespoke mock components.

## Feasibility ground truth (from investigation)

Backend 403s every non-GET on the demo. Reachable read-only: all GET reads, the static
`SearchCardModal`, the seeded warren graph, the immersive room via `getMaterialSource` +
`pendingDemoReading`, and `dig` (backend `cannedDig` short-circuits demo → 3 real candidates,
no 403). 403 on demo: adopt/attach/create-lead/create-edge/enter-reading/paste/patch-metadata/
add-log/put-resource-needs/review-trigger. Seed gaps to fill: no `parent_lead_id` (no 2nd
layer), no material `anchors` (no inline highlight), no `note_resource_need` rows (empty box),
stub 3-sentence paper, literal `\n\n` essay body. Feature gaps: warren map node shows no
read-state; `ProposalGuidePane` renders an **empty** box when `locked`; the review-trigger
button is hidden when `locked`.

## Design

### Part 1 — Seed / data (new migrations, demo-only, all GET-readable)

- **P1a · essay `\n\n` fix** — the demo essay `edit_buffer …02f0` body in `0082:254-265`
  concatenates plain `'…'` literals, so `\n\n` is stored literally. Migration UPDATEs the
  buffer content to real newlines (rewrite with `E'…'`/real breaks). Demo-only; real projects
  unaffected.
- **P1b · real full-length paper + inline-highlight anchors** — replace material `…0271`
  `blocks` with an original ~10-paragraph article (same thesis: satellite greening, China/India
  share, agriculture/tree-planting mechanism, the "greening ≠ sustainable" caution, emissions
  counter-point). Seed `anchors` on the material so the reading room renders `<mark>` highlights
  the tour can point at (a card "lined out" these spans). Keep `…0270` as the short 自媒体
  source (contrast is intentional). Update the canned reading transcript if needed to reference
  the fuller article.
- **P1c · warren 2nd-layer + 还需要探索** — seed one `exploration_lead` with `parent_lead_id`
  set to a root (plus 1–2 siblings) so clicking that root reveals a real 2nd layer; seed 2–3
  `note_resource_need` rows so the 还需要探索 box shows populated.

### Part 2 — Frontend features / unlocks (all `isDemo`-scoped or read-state-driven; read-only)

- **P2a · 未归档 node location** (`WarrenMap.tsx:384-393`) — replace the hardcoded `{x:0,y:320}`
  with a slot computed from the root nodes' bounding box (below `max(y)+gap`), so it never
  overlaps a root. Not demo-specific — a general layout fix.
- **P2b · map-node read badge** (`WarrenMap.tsx` `WarrenNodeView`) — show a small "已读 ✓"
  affordance on a node whose contained reference(s) are `reading_status='done'` (data-driven;
  general feature). Anchor `warren-node-read` for the tour.
- **P2c · ProposalGuide read-only variant** (`ProposalGuide.tsx:433-460`) — when `locked`
  (demo), render a READ-ONLY filled guide (structure + example prompts/answers, disabled) instead
  of an empty div, so `writing-aicard` frames a real "可填的引导框". 铁律①: it shows the
  SCAFFOLD, never AI-authored body text.
- **P2d · review-trigger read-only copy** (`WritingBlock.tsx:497-507`) — for `isDemo`, always
  render a DISABLED "让印记通读并批注" button (mirror the 完成写作 disabled pattern) + anchor
  `writing-review-trigger`, so the tour can show how 批注 is triggered.
- **P2e · sidebar default → 阅读笔记** (`ReferencePanel.tsx:109`) — change the `isDemo` default
  from `"anno"` to `"notes"` (guard: only if `hasNotes`, else `tabs[0]`). A later tour step
  selects the `anno` tab explicitly.
- **P2f · nav hooks + tab-select** — add tour nav mechanisms (mirroring `setWritingView`'s
  one-shot pattern) to: force the plan room's `活动日志` view (`setPlanView("log")`), select the
  ReferencePanel `批注`/`阅读笔记` tab, and open the `SearchCardModal`. Thread StudentApp →
  ProjectsTab → WorkspaceContainer/PlanBlock/ReferencePanel.
- **P2g · new `data-tour` anchors** — activity-log view, 检索卡 modal + its trigger button,
  `NeedsResourcesBox` root, `writing-docswitch` (exists), add-source button + `Preview` panel +
  enter-reading button in the list view, and per-dimension axis cards in the eval report (reuse
  the existing `data-dim-bar={code}` or add `data-tour`).

### Part 3 — Restructured projects tour (`tour/segments/projects.ts`)

New/expanded segments (real-scene where reachable, else spotlight-narrate; one idea per step):

1. **立题 → 管理 transition (①):** after 立题, a step: "写好研究问题后，印记会照着你的思路先
   拟一份计划，你再按自己的节奏调整。"
2. **管理 activity log (②):** `setPlanView("log")` → spotlight the seeded 活动日志 feed
   ("你做过的每一步——聊了什么、读了什么、写了什么——都会自动记在这里").
3. **阅读探索 — directions & search (⑤⑥⑦):** open 检索卡 (static, real) → 让印记建议检索方向
   (narrate) → 还需要探索 box (seeded, real) → select a node → 关键词/找相似 → real canned
   candidates in `explore-suggestions` ("印记给出候选来源，你来采纳或丢弃" — narrate 采纳).
4. **node drill-down + 未归档 (④③):** click a root node → real 2nd layer; point at the fixed
   未归档 zone.
5. **add + dive (⑧⑨⑫):** narrate "把选中的来源收进来 / 有时直接把正文粘进来" (spotlight the
   add/paste + `paper-enter-reading` controls), then deep-link into the immersive room.
6. **精读 real (⑩⑪):** real full-length paper (`rr-article`), a card lining out text
   (`rr-deck` + a highlighted `<mark>` span), reading-with-AI (`rr-chat`), notes, finish.
7. **back to map, node changed (⑫):** close immersive → graph → spotlight the now-已读 node
   (`warren-node-read`) — "读完后，这个节点就标成已读了".
8. **switch view + library (⑫⑬⑭⑮):** spotlight `reading-viewtoggle` (top button) → list view →
   add-source (narrate the familiar add), `Preview` metadata edit (narrate), enter-reading
   (narrate).
9. **writing (⑯–㉔):** 提案/正文 docswitch (⑯) → 大纲 building-blocks/subquestions (⑰, point at
   the seeded outline branches) → sidebar default 阅读笔记 (⑱) → 片段 real guiding box (⑲, P2c)
   → how to trigger 批注 (P2d) → select 批注 tab, show seeded 批注 (⑳ relabeled) → move to 正文
   (㉑) → 正文 left sidebar: 片段/阅读笔记/AI批注 (㉓) → 还需要探索 box in writing (㉔) → 完成写作
   (existing).
10. **回顾 (existing):** 定稿并开始评估 lock warning.
11. **评估 axis dimensions (㉕):** after the whole-section `#s5`/`#s6` steps, add per-dimension
    spotlights — D (D1 任务理解…D6 反思元认知) and A (A1 方向自主…A6 求真优先) — using each
    `AxisDimCard`'s `means` (这一维看的是) as copy; point at 1–2 representative dimensions per
    axis (not all 12) to keep it digestible.

## 铁律 compliance

Read-only throughout (no writes triggered; all write controls spotlighted-not-clicked). 铁律①:
the guiding box + review show the SCAFFOLD and AI's review role, never AI-authored body text.
No dark patterns. The map read-badge + guide read-only variant are general features, not
demo-only hacks.

## Testing

- Backend: migrations apply; demo essay body has real newlines; material `…0271` returns the
  full article + anchors over `getMaterialSource`; the child lead + resource-needs read back;
  `internal/api` + `internal/agent` green.
- Frontend: segment tests (new steps: anchors exist, non-center spotlights, action selectors,
  new nav hooks fire); ProposalGuide locked read-only variant renders; review-trigger disabled
  copy for isDemo; sidebar default = notes for isDemo; WarrenMap unfiled-slot + read-badge.
- Prod-browser smoke of the WHOLE journey (jsdom can't hit-test): every new spotlight lands on a
  real element on the finished read-only demo; the reading flow order is correct (search →
  add/dive → 精读 real paper + highlight → back-to-map node-read → switch-view → library);
  writing shows the filled guide + 批注 trigger + real 批注; 0 console errors (no stray 403).

## Out of scope

- Making adopt/add-source/enter/paste/metadata-edit actually mutate on the demo (stays 403;
  spotlight-narrate only). Per-user writable demo. Live LLM.
- Reproducing any copyrighted paper text.
