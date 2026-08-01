# Studio batch-4 — 2026-08-01

Branch `feat/studio-batch4-2026-08-01` off main `069db1d`. 18-item studio-feedback list, delivered as 5 slices. **MERGED (ff) to main + DEPLOYED to prod. Prod HEAD `5810be4`, db v45 (migration 0045 applied).** Suites: web 645 · contracts 306 · go ./... all ok. Whole-branch review clean (safe to merge+deploy).

## Sequence (user-chosen: "annoyances first")

### Slice 1 — editing bugs + gantt + cards leave an artifact (`86f20c4`, `ff452d0`)
- **#1/2/3** outline focus-loss: `OutlinePane.save()` stopped swapping server-minted ids onto mounted rows (`key={n.id}` remount dropped focus ~700ms after each keystroke). Keep local ids stable across saves (mirrors `useSnippets`).
- **#11** mind map keyboard: Enter=sibling, Tab=child; map inputs register into `inputRefs` so the new node takes focus.
- **#10** gantt: order stages by earliest task `start`, not board/load order.
- **#7/#9** cards→artifact: `compileCard.ts` projects a completed card's own answers into a 片段 (editable/insertable) + a message naming the card; pinned paragraph shows in the sent bubble.
- **#13** rabbit-hole → mints a 线索 from the next-direction + refreshes the graph.

### Slice 2 — card taxonomy, placement & templates (`d7ca503`, `aa8577b`)
- **#17/#18** phase decks: 立题 = 提问/语言框定/视角对照矩阵/检索方向审视; 找资料/读 = 事实观点价值/视角对照矩阵; 写作 adds 确定度光谱 + 事实观点价值. Server `studioDeckCards` allowlist mirrors summonable ids.
- **#8** always-visible card shelf (dropped the 工具卡 toggle; writing deck default-open); voices as a 正文 check tool — the selection chip now offers 问印记 + 体检这段 (paragraph-scoped review).
- **#6** `step.sentence_frames` (optional schema) → a visible 参考句式 block (skeletons only, 铁律①); on pee/concession/steelman + Go mirror.

### Slice 3 — materials organization: nested sections + drag (`597fc51`, `07872d3`)
- **#5** `snippet.section` nullable label (outline heading OR 线索; ids re-minted so keyed by TEXT). Migration 0045, hand-edited sqlc (section last in every SELECT/RETURNING + Scan), `normalizeSection` blank→NULL. SnippetsPane foldable sections + ⠿ drag + 归到 select; 未归类 + orphan groups never drop a snippet.
- **#15** placing off the 片段 tab now toasts.
- **#16** sidebar 片段 browse tags each snippet with its section + a category filter (clamps a vanished category back to 全部).

### Slice 4 — per-lead 深挖 with the student's thinking (`552a468`)
- **#12/#13** each open 线索 has "深挖这条" → focuses the 深挖 panel on that lead + a thought box → directions centered on THAT thread; still 克制 (proposes, never auto-adds). `ExplorationGuideInput.FocusLead+Thought`; `POST /exploration/guide` accepts `{leadId,thought}` (absent = whole-graph, unchanged); metering/IDOR discipline preserved.
- **DEFERRED** (needs a migration): 分支 as nested child leads (`parent_lead_id`) + creating a brand-new 来源 under a 线索 (connecting an existing source already works).

### Slice 5 — link auto-fetch: DOI + sturdier web (`b4f54e7`, `5810be4`)
- **#4** `materialize/`: DOI → Crossref → publisher landing URL + title (best-effort, falls back to original URL; resolved URL scheme re-validated); browser-like headers (UA + Accept) vs bot-walls; prefer `<article>/<main>/[role=main]` extraction with whole-page fallback.
- **DEFERRED** (needs a heavy dep): PDF body extraction — a raw-PDF DOI still lands in the paste-fallback (now with a recovered title).

## Reviews
Per-slice reviews (slices 1–3) folded in. Slice 4+5 covered by the final whole-branch review — no Critical/High; folded: rabbit-hole always-refresh, sentence_frames in on_demand steps, sidebar filter clamp + toast timer, DOI-resolved-URL scheme validation.

## Deploy
Full deploy (migration + Go + web): server `git pull` → `compose build api web` → `compose run --rm api --migrate-up` (0045 → v45) → `compose up -d api web` (db untouched). Smoke green (web 200, api healthz, prod-smoke: auth + DeepSeek SSE + voice TTS).
