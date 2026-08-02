# Studio Batch 5 — Writing / Reading / Review feedback (2026-08-02)

Branch: `studio-batch5-writing-review` (off main `b343130`).
Source: user feedback batch (21 items) on proposal-forming, writing room, reading room / exploration, review/retrospective, and a 12-card deletion list.

## Decisions (confirmed with user)
- **D1 (#20/#21):** New explicit **完成写作** milestone. Before it: review room is view-only (outline visible, nothing enterable/generable). After it: writing goes read-only, review unlocks → student self-reflects (AI supports with review/reflection cards) → **then** AI writes its reflection (mirror) → 完成回顾 → 定稿 → 评估.
- **D2 (#19):** Delete + **rewire to survivors**. **`toulmin` KEPT** (user reversed — graph builder/classifier/structure view untouched). 11 cards deleted: source-map, corpus-hook, framing, steelman, aok-methods, science-knowing, ethics-roleplay, sift_craap, certainty-spectrum, ai-collaboration, checkpoint. `sift_craap`→`craap` (acceptance/demo + drop custom renderer/teaching). `steelman`→`concession` (one-sided moment, attest, decks, skills repertoire/student_written). `certainty-spectrum` overclaim-moment **retires**. `framing` off FORMING_DECK + allow-map. Count 45→34.
- **D3 (#8-second):** Writing cards live **both** inline (chips under AI's latest reply) **and** in a persistent labeled shelf (大纲 / 片段 / 正文·检查), with 评审团·质疑者·门外汉·审判者 under 正文·检查.

## Cross-cutting principle (recurs in #1, #8-first, #13, #21)
Every card finish should: **AI feedback on the student's actual content → student chooses to apply/add → content changes.** Never a bare "提交了". Empty card = no spend, no artifact. (铁律①: AI never writes the deliverable — feedback + scaffolds only.)

## Slices
- **S1 — Card deletion + rewire (#19).** ✅ DONE `5e8530d` (45→34, toulmin kept). Delete 11 JSONs + registry lines; `make sync-cards`; rewire sift_craap→craap, toulmin→argument-map; fix decks (WRITING/THINKING/FORMING), moments (moment.go), attest, skills writing-project.json, projection cardMeth/structure, card_persist allow-map; drop SiftCraapRenderer/teaching; update count assertions (45→33) + related[] + coverage doc + AGENTS.md acceptance script. STATUS: pending
- **S2 — Forming coach feedback (#1, #2, #13-general).** Shared `POST /cards/reflect` (card content → coach reply, no-spend on empty); wire into forming card-finish (replace canned "提交了"); forming coach prompt gives substantive feedback vs the 4 standards (目标/动机/活动·时间/资源) before advancing; nudge collecting literature during question/motivation. STATUS: pending
- **S3 — Writing room (#6, #7, #8-first, #9-first, #8-second, #9-second, #10).** Snippet edit mode (dbl-click→edit, finish icon, taller box); outline→snippets = real foldable sections + nested slots (fix name-as-text regression); card→AI feedback→add full-paragraph snippet to a chosen section (reuse /cards/reflect); author sentence_frames on surviving writing cards; card placement inline+shelf; examiner free-question path; sidebar shows used card + styled selected-paragraph (kill literal 【就这一段】); diagnose+fix 体检没跑完 (surface real cause: snapshot-commit / SSE / review_rejected). STATUS: pending
- **S4 — Reading / exploration / materials (#3, #4, #5, #12, #13-rabbit, #16, #18).** Topic-aware reading empty-state (kill stale opener); persist + show abstract + metadata (Crossref on success path; new reference field); #5 = confirm readable-extraction already opens the link, live-proxy out; exploration graph: attach existing 来源 under a 线索 while keeping it open + add untracked source from graph view + graph as default even when empty; rabbit-hole card → pick a 线索 + tell thoughts → dig-deeper suggestions (fix empty-seed invisibility); materials box categorization + 线索 as filter tag on 材料 tab. STATUS: pending
- **S5 — Review / finalize / evaluation (#20, #21).** 完成写作 milestone + project state; review view-only lock until writing done; review cards surfaced (add `reflection` to coachProposeSurfaces + render CoachProposal/shelf in ReviewBlock); reorder so mirror (AI reflection) is gated until student's reflection.done; finalize→evaluation unchanged downstream. STATUS: pending

## Item → slice map
1→S2 · 2→S2 · 3→S4 · 4→S4 · 5→S4(report only) · 6→S3 · 7→S3 · 8-first→S3 · 9-first→S3 · 8-second→S3 · 9-second→S3 · 10→S3 · 12→S4 · 13-general→S2/S3 · 13-rabbit→S4 · 16→S4 · 18→S4 · 19→S1 · 20→S5 · 21→S5

## Progress ledger
(append one line per slice when review is clean)
- S4a: complete (commit e2863a6, db 0048; contracts 296 / web 672 / go incl internal/api testcontainers green; sqlc hand-edit reviewed — column/Scan order consistent across 8 queries)
- S3b: complete (commit b12d7e2, web 668 / go agent green; #10 root cause = missing stripFences() in review.go; noted follow-up: paragraph-scope commitSnapshot flips word_budget gate)
- S3a: complete (commit 2ad6019, contracts 294 / web 662 / go agent+cards green; card→paragraph reuses /cards/reflect surface=writing)
- S2: complete (commit e048e87, contracts 292 / web 653 / go internal/api 470s green; self-reviewed — forming scope aligned, spend metered before bail)
- S1: complete (commit 5e8530d, contracts 289 / go build+cards/agent/studio/skills / web 651 / internal/api 471s all green; self-reviewed — seeded-steelman degrades gracefully in projectEquipment)
