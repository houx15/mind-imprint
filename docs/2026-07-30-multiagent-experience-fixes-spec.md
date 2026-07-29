# Multi-agent experience fixes — EA / EB / EC

> Phase 2 of the goal: after the writing-loop hardening, close the **experience** gaps in the multi-agent system. Grounded in a full end-to-end trace of the three flows the product owner named — rabbit-hole, proposal formation, reading-room context transfer. Three slices, one per flow, each spec→build→review→merge.

Guiding read from the audit: the plumbing largely EXISTS and is correct (brief-in is real and purposeful; leads materialize; 深挖一层 is honestly 克制). The gaps are about **coherence the student can feel** — context that transfers but is lossy/invisible, a graph that's buried, a shaping dialogue that doesn't actually shape.

## EA · Reading-room context transfer (the takeaways-out round-trip)

**Gap 1 (HIGH):** the reading sub-agent assembles a rich 5-field takeaway, but the spine 文献库 block surfaces only the one-line `proposalImpact` for 已归纳 sources (`projectcoach.go:302`) and a bare *count* for 在读 (`:309`). Findings / credibility / key_quotes never reach the writing coach — a write-only dead-end. She reads deeply; the main thread remembers one sentence.
**Gap 2 (HIGH):** leaving the reading room (`closeReadingSource`, `WorkspaceContainer.tsx:165`) shows NO acknowledgment of what she just read; summary-on-return composes once per project-open only. The reading feels like an island on exit.

**Build:**
- **Richer takeaway projection** (`buildSpineProjection`, projectcoach.go 文献库 block): for a 已归纳 source, surface a compact synthesis — `印记：<proposalImpact> · 发现：<top finding> · 可信度：<verdict>` (truncated), not just `proposalImpact`. Keep it one-to-two lines per source (projection stays compact). So a later writing-room coach turn actually "remembers" the substance. Backend-only; reuse the persisted takeaway fields already on the reference.
- **Carry-forward acknowledgment** on `closeReadingSource`: show a lightweight banner/toast in the workspace — "刚读完《<title>》——你确认的发现已经带进来了，写作时可以直接用" — so the transfer is visible. Frontend (WorkspaceContainer); no new endpoint (use the just-finalized source's title/takeaway already in hand).
- Tests: projection includes a finalized source's finding + credibility; the carry-forward banner renders after closing a read source.

## EB · Rabbit-hole liveness (make the exploration loop feel alive)

**Gap 3 (MEDIUM):** the exploration graph isn't a room — it's behind a `列表 ⇄ 探索图谱` toggle that defaults to `list` (`ReadingBlock.tsx:86`), and the "待追 N 条线索 / M 悬空来源" signal lives only in the coach's private spine (`projectcoach.go:336`), invisible in the nav. A student can never find it.
**Gap 6 (LOW):** the exploration empty-state copy (`ExplorationView.tsx:255`) promises leads grow automatically, but the read-but-didn't-card path produces none (`HasRecordContent`-gated seeding) — an overpromise.

**Build:**
- **Surface the signal:** show an open-leads / dangling-sources count on the 文献库 room (a small badge on the 探索图谱 toggle, and/or on the Rail's 阅读 room button) so the rabbit-hole advertises itself. Auto-switch the 文献库 to the 探索图谱 view when there are open leads and the student hasn't chosen list explicitly (gentle, not forced). Frontend; the counts already come from `getExploration`.
- **Honest empty-state copy:** reword `ExplorationView.tsx:255` so it doesn't promise automatic leads unconditionally — "读完一篇、归纳出线索后，它们会长到这里；也可以自己记一条" (already partly there; make the conditionality explicit and add the manual path prominently).
- Tests: the badge reflects a non-zero lead/dangling count; copy no longer overpromises.

## EC · Proposal formation (make 立题 a real shaping dialogue)

**Gap 4 (MEDIUM):** the forming coach uses the one generic cross-room posture (`project_coach.go:26`); it *passively sees* `（未填）` dim markers via the projection but nothing instructs it to walk objective→reason→activities→resources or nudge the empty ones. The four-dimension structure dies after the scripted intro.
**Gap 5 (LOW):** chat→panel is zero by design; the panel and chat are near-parallel.
**Minor race:** `onSend` (`PlanBlock.tsx:172`) doesn't flush the dim autosave debounce before calling `coach`, so a just-typed dim can be invisible to that turn.

**Build:**
- **A forming-aware nudge in the projection** (surgical, respects the single-posture design): in `buildSpineProjection`'s 开题四问 block, when on the forming/proposal_review surface, append a one-line steer — e.g. "（还没触及的维度：<未填的维度名>——可以顺着学生的话，把话题往这些维度带一步，一次一个）". This drives coverage WITHOUT a second prompt or a rigid script, and stays 克制 (one at a time, guide-not-fill). Backend (projectcoach.go), scoped to the forming surfaces.
- **Flush-before-send:** in `onSend`, flush the pending dim debounce (mirror `onGenerate`'s flush at `:137`) so the coach always sees the latest dims.
- (Gap 5 chat→panel dim-suggestion is deferred — it risks the 铁律 "AI never fills the proposal" line; the panel stays student-authored. Noted, not built.)
- Tests: projection names the untouched dimensions on the forming surface; the send path flushes dims first.

## Sequence
EA → EB → EC (context-transfer first — highest-value; then discoverability; then the forming shaping). Each merges after its own review. All are small/surgical (projection lines, a banner, a badge, copy, a nudge line, a debounce flush) — no new tables, no new heavy endpoints.
