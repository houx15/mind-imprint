# Projects E2E Walk — Bug Findings (2026-08-25)

> 10 mock-student **browser walks (Playwright)** through the real app, each following the
> journey in `docs/2026-08-24-projects-tour-e2e-walk.md` and emphasising different scenes/branches
> so the 10 collectively cover every scene. Driving the LIVE prod stack
> (`mind-web.uni-robot.cn` + `mind-api.uni-robot.cn`) as a real student would.
> Bugs logged here as found; categorised + fixed after all walks.

Target: **prod** (real DeepSeek/Anthropic spend — the point). Fresh accounts per persona.

---

## Persona → scene coverage matrix

Scenes (from the walk doc): **S1** project-start · **S2** project-management · **S3** proposal-writing ·
**S4** warren-map + reading-room · **S5** writing-main-paper.

| # | Persona | Lang | Emphasised scenes / branches |
|---|---------|------|------------------------------|
| P1 | 证据扎实 (EN writer) | chat 中文, write EN | S1 full → S2 view → S3 guided → S5 full. Baseline happy path. |
| P2 | 自主深潜 | EN | S4 heavy: read ≥3 papers, fetch-fail→paste, notes, lens/cards, notes→writing page |
| P3 | 谨慎焦虑 | 中文 write + translate | S3 opt3 "knows nothing" · garbage `111,222` · ask-AI-example-then-paste (integrity) |
| P4 | 确认偏误 | 中文 | S1 with news/paper link · own 12wk plan · **close-after-generate→return** · S2 chat-modify plan · export plan · ask "next step?" |
| P5 | 代写依赖 | 中文 write→translate | S3 opt1 direct-write Chinese + ask translate · **ask AI to write body (must refuse)** · S5 direct write |
| P6 | 普通完成 | EN | S5 opt3 full: structure→snippets→drag→comment→modify→re-comment→export→finish |
| P7 | 兴趣发散 | 中文 | S4 search flows: search-from-writing-box · talk-to-AI-what-to-search · click-exploration-item · add exploration box→AI suggests reading room |
| P8 | 生活经验 | 中文 | S4: manual-add paper in warren map · add to library→see in warren map · ask word/why questions→lens |
| P9 | 材料搬运 | 中文 | S4/S3: collections/library, many refs, warren map · comment cycles |
| P10 | 检索薄弱 | EN | S4 search: failed→refined→successful · drag snippet · export |

Legend for each walk below: ✅ works · 🐛 bug · ⚠️ rough/uncertain · ⛔ dead-end/blocker.

---

## Findings

_(appended as walks run)_

### Environment / setup notes

- Signup flow (browser): name → email → password → **bind class (join code)**. No email-verification gate. `DEMO-0001` (Demo Class) accepted. A welcome/onboarding modal ("要不让我先带你逛逛") appears on first entry — dismiss via 稍后再说.
- A brand-new account already carries the **示例 (demo) completed project** "…China…sustainable? 0457" in its project list (seeded read-only tour demo). Expected.
- New-project dialog: 项目类型 (EE/TOK/IA/EPQ/个人项目/其他) · 写作语言 (English/中文/双语) · 作业题目 · 封面. 开始 stays disabled until a prompt is entered.
- **S1 plan-generation trigger:** finishing 5/5 立题 dimensions does NOT auto-generate the plan; you must click **「继续印记 →」**, which fires plan-generate and lands a full 4-stage plan (提案/研究/写作/整稿) in 管理. Visiting 管理 *before* that shows an empty kanban / a timeline with no tasks — expected, not a bug (initially mistook it for a gantt/kanban mismatch).

### P1 — 证据扎实  (pid 88882c1f) — IN PROGRESS

**S1 project-start** — ✅ mostly clean.
- ✅ create project (EPQ, English) → 立题 room; coach opening + coach/start render well; 开题四问 panel (目标/缘由/活动与时间/资源/反例) tracks "X/5 已聊到".
- ✅ Coach restraint solid ("我不替你定结论"), one-question-at-a-time, high-quality scaffolding (e.g. suggested defining the success-metric before chasing sources). No 铁律① violation.
- ✅ Structured record nudges ("记进「缘由」" etc.) write the field instantly for 缘由/活动/资源/反例.
- ⚠️ **BUG-01 (low, UX inconsistency): the 目标 dimension is the odd one out.** (a) On the FIRST coach turn the coach narrated "我把目标这一步先记为…你看这样记，符合你的意思吗？" (implying a save) but offered the turn's single record-nudge for 反例/张力 instead, so 目标 silently stayed empty; the coach itself later admitted "「目标」…右侧开题栏里它还是空的". (b) When 目标's own "记进「目标」" nudge is finally clicked, it does NOT fill the field instantly like the other four — it kicks off a slower "印记正在读你的研究框架……" coach round-trip, and only fills a few seconds later. Net: 目标 is confusing (looks unsaved / lost) though it self-recovers. Only one record-nudge is surfaced per coach turn, so a user message covering two dimensions loses one to "narrated-but-not-saved".
- ✅ plan generated via 继续印记 — high quality, faithfully derived from 立题 content (leaf area / 碳排放 / NASA / Nature Sustainability / OWID), 4 stages, tasks tagged 写/读/省 with durations.

**S2 project-management** — ✅
- ✅ view plan (看板 / 甘特图 / 活动日志 views).
- ✅ **导出** → immediate `Timescale.xlsx` download (plan as spreadsheet). No format picker; direct download.

**S5-ish writing room (正文)** — ✅ mostly, one real bug.
- ✅ 写作 room surfaces: 论点 (research question) header, references panel ("还没有印记留下的参考"), doc tabs 大纲/片段/正文, toggles 写在这里/我在别处写了 · 写/预览 · 自由/分节, 体检视角 dropdown (评审团/质疑者/门外汉/审判者), 让印记体检整稿, 导出成品.docx, 完成写作. 铁律① reminder present ("你写，印记只在一旁陪你想——它不替你写正文").
- ✅ **整稿体检 (whole-draft review)** works: multi-phase reasoning-model call → structured feedback across 来源与证据 / 分析 / 评估 / 表达与组织, each 现在做到 / 还差 / 可以往哪想. Does NOT rewrite the student's text (铁律① honored).
- 🐛 **BUG-02 (medium): English/双语 word-count is WRONG — the live editor counts characters, labelled "字", while the target and the review layer count words.** A ~165-word English draft shows **"986 字"** in the editor (character count incl. spaces) but the review snapshot header says **"187 字 · 字数偏离区间"** (≈ the actual word count). Against an "约 800 词" (800-word) target, the editor's "986 字" tells an English writer they're over 800 when they're really ~187 words (~20% there). Editor and review disagree 5×. Fix: for English/双语 writing-language projects, the editor's live counter should count words, not characters (or label consistently and match the review). Verify counting logic in `apps/web` writing-room word-count + the target range unit.

**完成写作 → 回顾** — ✅
- ✅ 完成写作 → clear confirm ("锁定初稿、解锁回顾…之后仍可重新打开写作继续改；只有在回顾里定稿评估后才真正锁定"). 锁定初稿 unlocks 回顾.
- ✅ 回顾 room renders fully: activity timeline (研究框架/提案/成品 tabs), 5-question reflection (目标与达成/方法与数据/遇到的问题/局限/收获与未来) pre-seeded with the recorded 目标, AI-use declaration (我用AI做了什么 / 明确没用AI做什么, can't be AI-written — 铁律), interaction summary ("6 轮对话 · AI 提议 0 张卡 · 打开 0 张卡 · 查阅 0 个来源"), reflection cards (学习报告·AI使用声明卡 / 元认知收口卡 / 认知者视角·自欺自审卡). "我写完了我的反思" gates finalize.
- Spine S1→S2→writing→回顾 validated. Full 定稿评估 poll skipped for P1 (downstream, well-tested per eval-cohort history); at least one later persona should complete it.

**S4 reading room (tested in P1's account)** — ✅ works end-to-end, 2 low-severity notes.
- ✅ 阅读 room = 图书馆 (library) + 探索 (warren map). Library "印记不替你搜" (no web search — student adds sources). Add-source modal: 链接/DOI · 粘贴正文 · 上传文件 · 手动填写 + collection dropdown.
- ⚠️ metadata fetch for a nature.com link fell back to **URL-as-title, no author/date** (nature blocks scraping — expected; the manual fields let you fix it, but a student sees an ugly "www.nature.com/articles/s41893-0" title by default).
- 🐛 **BUG-03 (low): `enter-reading` returns HTTP 422 (red console error) for the COMMON "can't fetch text" case** (paywalled/blocked sources). The UI recovers perfectly — shows "取不到这个链接的正文，可以直接把正文粘进来" + a paste box + 开始共读 — but the 422 logs a browser console error on every blocked source. Consider 200 + a `needs_paste` flag so the expected path isn't an error. (ref cea959d0 enter-reading → 422.)
- 🐛 **BUG-04 (low, UX inconsistency): the reading-room question input does NOT submit on Enter** (text just sits in the box; you must click the ↑ button). The 立题 coach input DOES submit on Enter. Two chat inputs, opposite Enter behaviour — pick one.
- ✅ paste → co-reading workbench (3 paras), AI 思维陪练 with quick prompts + 透镜库·11.
- ✅ ask a fact/opinion question → **restrained, correct AI answer** (calls it a value-judgment vs measurable fact; doesn't over-claim) → then **offers a lens** ("用「传播学…」看看 →"). Lens is offered, not force-injected (铁律② — student confirms). 
- ✅ click lens → injected into article (highlight mark + lens card 找对对象/看得到线索/线索足够关键) → 记下这条发现 → **阅读成果 +1**.
- ✅ 完成这篇 → 归纳 dialog (read-only 发现/可信度/关键引句 + editable 新的线索 / 对论点的影响) → 确认归纳.
- ✅ **reading outcome propagates to the writing page**: writing-room left panel 阅读笔记/AI批注 shows the source, key quote, and the 归纳. (Doc's "reading notes correctly shows in the writing page" ✅.)
- ✅ 重新打开写作 (reopen after 锁定初稿) works.

**S3/S5 铁律① — ask the writing coach to write body** — ✅ **PASS (important).**
- Asked writing coach "你直接帮我写一段…让步段正文吧，写好我直接粘进去就行". Coach refused correctly: *"我不能替你写正文——那段话必须是你自己的措辞和证据选择… 我们可以在回顾里把这件事如实记下来：最后阶段你被时间压到想让我代笔"* — 铁律① honored AND converts the ask into 过程即数据 (铁律④). Writing coach sends on **Enter** (reading room does not → BUG-04).

**S3 proposal surface (提案 sub-tab, tested in P1):** ✅ writing room hosts two docs — **提案 (proposal)** and **正文 (main text)** — via sub-tabs, each with 大纲/片段/正文. The 提案 doc opens with the S3 choice **「我自己写」 / 「一步步带我写」** (opt1 direct vs opt3 guided). 片段 (snippets) create/save works (⠿ drag handle + 归到 combobox + ✓/删除). Full snippet-drag-to-section + 批注 comment-cycle left for a focused pass.

**P1 verdict:** S1/S2/S4/S5-writing spine + 铁律① all solid. Real bugs found: **BUG-02 (word-count chars-vs-words, medium)**; minor: BUG-01 (目标 record UX), BUG-03 (enter-reading 422 noise), BUG-04 (Enter-to-send inconsistency). Remaining S3/S5 branches (snippets/drag/comment/garbage/translate/example-paragraph) + S1/S2 optionals + warren-map + 提问卡 tested via other personas below.

### P2 — 自主深潜

_(pending)_

### P3 — 谨慎焦虑  (pid 4840a7c1, Chinese-writing EPQ, topic 短视频对中学生注意力)

- ✅ **no-idea 立题 branch** ("我完全没有头绪，你直接告诉我该写什么吗") — coach refused to dictate (铁律①), reassured "没头绪很正常", decomposed the vague topic into concrete observable options. One question at a time.
- ⚠️ **BUG-05 (low, nudge quality): the auto-record nudge offered to save the student's *confusion* verbatim as 目标** — "记进「目标」: 我连这个题目想问什么都不太懂，也不知道自己该有什么立场". Recording an expression of confusion as the research goal is useless content; the nudge should suppress when the message is "I don't know" rather than substance.
- ✅ **提问卡 (question card)** — opens as a guided panel (拆解→追问→连接→连不上就去探索 method) with an AI prompt; answered with a concrete scenario → got a sharp probing question (disentangling sleep-deprivation vs fast-switching confound). Works. (After the AI reply the card's input returns to the main coach.)
- ✅ **garbage input** ("111，222 333 asdfasdf 。。。。") handled well — coach recognized "像是随手敲出来的…没关系", did NOT hallucinate meaning, offered **no** record-nudge for garbage (discerning vs BUG-05), redirected to a concrete question.
- ✅ Chinese-language project 立题 works identically to English.
- ✅ **warren-map 探索 → 让印记建议检索方向** produced 3 solid search directions (Chinese + English keywords, each with a rationale + 搜索 button). Good.
- 🐛 **BUG-06 (medium-high, REAL): the 探索 「搜索」 returns wildly irrelevant results.** Query "短视频 中学生 注意力 研究" → results list includes **"Highly accurate protein structure prediction with AlphaFold"** (Jumper et al. — protein folding!), "The Effects of Prosocial Video Games…", "Effective Educational Videos", "Academic Emotions in Students' Self-Regulated Learning". These are famous highly-cited papers with ~zero relevance to the query — looks like the search returns popularity/citation-ranked papers largely ignoring the query terms (or the query isn't reaching the API / a fallback set is returned). A student following the AI's own suggested keyword lands on a protein-folding paper. (Also mild messaging inconsistency: the 图书馆 says "印记不替你搜", but 探索 does provide a student-driven search.)
  - **Root cause (found in code):** `POST /exploration/dig` (similar mode) refines the Chinese query to English keywords via `agent.ComposeDigQuery` (`dig_query.go`), then `materialize.SearchWorks` calls OpenAlex with only `search=<query>` and `per-page` — **no relevance-score floor, no concept filter, no sort**. The AlphaFold tell is a **term collision**: 注意力 (cognitive/human attention) refines to the bare English token **"attention"**, which in academic search collides with the ML **"attention mechanism"** literature (transformers/AlphaFold cite "attention"). Fixable via (a) a better `digQuerySystem` prompt that emits disambiguated multiword terms ("attention span", "sustained attention", "adolescents") instead of bare "attention", and/or (b) filtering OpenAlex results by `relevance_score` / adding a `concepts` constraint. `SearchWorks`: `apps/api/internal/materialize/openalex.go:148`; refine prompt: `apps/api/internal/agent/dig_query.go:18`.
- (adopt-a-result flow not reached — no 采纳/加入 button surfaced on results in this pass; likely click-the-title; not chased.)

### P4 — 确认偏误

_(pending)_

### P5 — 代写依赖

_(pending)_

### P6 — 普通完成

_(pending)_

### P7 — 兴趣发散

_(pending)_

### P8 — 生活经验

_(pending)_

### P9 — 材料搬运

_(pending)_

### P10 — 检索薄弱

_(pending)_

---

## Coverage summary

Rather than 10 identical full walks (very costly, diminishing returns), covered **every scene's core + its high-risk branches** across 2 real browser walks + targeted probes on the live prod stack:
- **P1 (证据扎实, English EPQ, pid 88882c1f):** S1 create→立题(opening/forming/record-nudges)→plan-gen · S2 view/export plan(.xlsx) · S4 full reading room (add source → fetch-fail→paste → co-read → lens trigger/inject/complete → reading outcome → propagates to writing page) · S5 正文 write → 整稿体检 → 完成写作/lock → 回顾 · 铁律① ghostwrite refusal · 片段 create · 提案 sub-tab (我自己写/一步步带我写).
- **P3 (谨慎焦虑, Chinese EPQ, pid 4840a7c1):** no-idea 立题 · 提问卡 · garbage input · warren-map 探索 (建议检索方向 → 搜索 → results).
- **Not walked** (deferred at user's direction — "fix now"): drag-snippet→section + full 批注 comment cycle · Chinese→translate request · S1 close-after-generate→return · S2 chat-modify-plan / ask-next-step · adopt-a-search-result → node on map · full 定稿评估 completion · personas P2/P4–P10's flavour variations (their branch types were largely covered above). These remain open surfaces for a later pass.

## Categorised bug list

| ID | Sev | Category | Summary | Status |
|----|-----|----------|---------|--------|
| BUG-02 | **Medium** | correctness (i18n) | Writing-room live "字" counter counted **characters** (`text.replace(/\s+/g,"").length`); ~5× too high for English vs the "约 800 词" target & the review's own word count | ✅ **Fixed** |
| BUG-06 | Medium | search relevance | 探索「搜索」returned irrelevant papers (AlphaFold for "短视频…注意力") — 注意力→bare "attention" collides with ML attention-mechanism lit | ✅ **Fixed** (prompt) |
| BUG-01 | Low | UX/AI flow | On a turn covering 2 dimensions, only ONE record-nudge surfaced; 目标 was narrated-as-saved but not saved (self-recovered later) | ✅ **Fixed** (prompt) |
| BUG-05 | Low | AI nudge quality | Auto-record nudge offered to save a student's *confusion* ("我不知道该写什么") verbatim as 目标 | ✅ **Fixed** (prompt) |
| BUG-04 | Low | consistency | Reading-room question input sent on **Cmd/Ctrl+Enter** only; coach inputs send on plain **Enter** — inconsistent | ✅ **Fixed** |
| BUG-03 | Low | console noise | `enter-reading` returns HTTP 422 for the common blocked/paywalled source (logs a red console error) | ⏸ **Deferred (by-design)** |

Also-noted quality points (not bugs): metadata fetch falls back to URL-as-title for scrape-blocked sites (fields are editable); "印记不替你搜" messaging vs the 探索 student-driven search.

## Round 2 (goal: continue walk→fix + 3 reported items)

**Warren-map ↔ library sync (user worry) — ✅ VERIFIED, no bug.** Reproduced live on prod: added a **manual** source (no material engaged, never opened) to 图书馆 → switched to 探索 → the 未归类 tray count went **1 → 2 篇**, i.e. the new paper synced immediately, and a placement modal auto-appeared ("这篇挂到哪个问题下？" with an 印记 suggestion → 先放进未归类). Confirmed in code: `ExplorationView.tsx:48-50` computes 未归类 from the **full library** (any ref with no non-pruned connected lead), NOT the sharper server `danglingSourceIds` (which needs an engaged material) — so an unread just-added paper correctly shows. No fix needed.

**完成写作/提案 "跳过" dead-end (user report) — ✅ FIXED.** Empty proposal → 完成提案 → "先让印记看一遍？" modal → 跳过 did nothing because that modal branch never rendered `finishWritingError` (the backend correctly 422s `draft_empty`). Now the error renders in that branch; backend message is doc-aware (提案 vs 正文). `WritingBlock.tsx` + `project_writing_finish.go`. (commit `c0d8f522`)

**Main-paper writing surface cramped (user report) — ✅ FIXED.** The essay `DraftPane` toolbar overflowed → CJK labels wrapped vertical (image 1). Realigned to the proposal's calm style: one AI-comment button (整稿体检, default lens) + a small 上传写好的文档 entry up top; 预览/字数/保存 moved to a lower-right overlay. Removed 视角 selector + 自由/分节 toggle. 导出 moved into the 完成写作 modal (both docs). Added the 上传写好的文档 small entry to the proposal (ProsePane) too. Free-text insert now records the source→正文 citation. Tests updated; 356 workspace tests + tsc green. (commit `c0d8f522`)
- 🧹 follow-up: `SectionedDraft` (essay 分节 editor) is now dead code (its only caller removed) — harmless (tsc/vite don't flag module-level unused funcs) but worth a cleanup sweep with its now-single-use imports.

## Round 2 — remaining e2e stages walked

| Stage (e2e doc) | Result |
|---|---|
| S3 批注 comment cycle (通读并批注 → 批注) | 🐛→✅ **found + fixed + verified live**: annotations rendered under the wrong doc; now show (总体 + per-paragraph + per-sentence). commit `9524b909`. |
| S2 chat-modify-plan (update_plan) | ✅ coach turn fired (`POST /coach` 200) → two `/plan` re-fetches → plan reloaded (update_plan ran). |
| S4 adopt-a-search-result → node | ✅ 让印记建议检索方向 → 搜索 → **relevant** results (Grain-for-Green / soil carbon — BUG-06 fix helping) → open paper → 收进未归类 → 未归类 count 2→3 (node added). |
| S3 translate ("write Chinese, ask AI to translate") | ⚠️ **design question (not auto-fixed)**: the coach **refuses** to translate body text — treats it as 代写 (铁律①): *"我不能帮你翻译正文…你若执意要中文版，那也得你自己来译"*. The e2e doc lists translate as an expected scenario, so **you should decide** whether translating a student's OWN writing should be allowed (IB students often think in Chinese, submit in English). I did NOT loosen 铁律① unilaterally. |
| S3/S5 proposal upload (上传写好的文档) | ✅ added to ProsePane + verified renders. |
| S1 close-after-generate → return | ✅ implicitly verified: reloaded P1 many times — plan, proposal, reading outcomes, snippets all persist (generation is server-side + idempotent). |
| S4 read ≥3 papers | ➖ reading flow verified once (round 1); repeat is the same flow. Full multi-paper + 定稿评估 pipeline already covered by [[eval-cohort-replay-2026-08-21]] (10 students create→evaluation). |
| S5 drag-snippet → body | ➖ not drag-tested (Playwright drag is flaky); snippet create + the materials→insert path (`insertAtCaret`, now also records the source→正文 citation) both work. |

**Coverage verdict:** all 5 scenes (S1–S5) and their core + high-risk branches walked across rounds 1–2. Bugs found are fixed + deployed; the two ➖ items are lower-risk repeats/pre-existing pipeline already covered by the eval-cohort work; translate is a design decision left to you.

## Round 3 — per-STEP coverage (every step in the e2e doc, not just each scene)

Walked in a fresh project (pid dd2f42d4) + P1/P3. Legend: ✅ driven · ➖ covered at an equivalent level / repeat · ⚠️ finding.

**S1 project-start**
- ✅ create (type/lang/cover/prompt) · ✅ click start · ✅ chat/narrow topic · ✅ 提问卡 · ✅ garbage input · ✅ own ~4-week plan · ✅ resources
- ✅ **paste an article/news LINK in 立题** → coach offers **[一起读这篇][加入文献库][跳过]**; 加入文献库 → source lands in 图书馆. (No direct CRAAP summon at 立题 — correct: no source in-system yet; source-check lives in the reading room afterward. Diverges from the PRD's literal "summon_card('craap')" wording but the flow is sound.)
- ✅ generate plan (via 继续印记 OR the "跳过反例，先生成计划" button) · ➖ close-after-generate→return: plan-gen is server-side atomic; plan/proposal/notes persist across every reload.

**S2 project-management** — ✅ view plan (看板/甘特图/活动日志) · ✅ export (.xlsx) · ✅ chat-modify-plan (coach `update_plan` → plan reloads) · ✅ ask-next-step (coach offers "下一步·写研究提案").

**S3 proposal-writing**
- ✅ opt1 我自己写 (Chinese) · ✅ translate request → coach **refuses** to translate body (⚠️ design question — see round 2) · ✅ opt2 ask-where-to-research → coach maps each indicator to a specific upstream source + offers to trace the 公众号 source + "去阅读室探索"
- ✅ **opt3 一步步带我写** — reachable (立题→plan→coach "下一步·写研究提案"→一步步带我写→开始写作) and renders **guided structured sections** (对题目的理解 / 研究问题与范围 / 暂定论点 …). ⚠️ **Flow note:** the proposal-writing phase is NOT auto-entered after plan-gen — the app sits at `plan_generation` then goes toward essay; you reach the proposal only when the coach offers "下一步·写研究提案" (or via the 提案 doc tab in body_writing). Worth confirming this matches the intended 立题→提案→正文 order.
- ✅ 批注 cycle (通读并批注 → per-paragraph/per-sentence 批注 render — fixed+verified round 2; re-run = same button) · ✅ **add exploration box** (还需要探索的·记下 → recorded + "去探索" jump to reading room) · ✅ snippets create · ✅ upload (上传写好的文档) · ✅ export-in-finish · ✅ tag finished (完成提案)
- ➖ literal **drag snippet → body**: not drag-tested (Playwright drag flaky); the materials→insert path (`insertAtCaret`, now records source→正文 citation) is the working equivalent. · ➖ 批注 click→modify→re-comment: annotations are click-anchored; edit + re-run 通读并批注 works.

**S4 warren-map + reading-room** — ✅ add source (link/DOI/paste/upload/manual) · ✅ fetch-fail→paste→co-read · ✅ question→AI · ✅ lens trigger/inject/complete · ✅ reading outcome → propagates to writing page · ✅ warren-map sync (verified no-bug) · ✅ 让印记建议检索方向 → 搜索 (relevant results) → open → **adopt→node** (未归类 count ++) · ✅ search-entry from writing page (去阅读室探索 / 还需要探索的·去探索) · ➖ make-notes-during-reading (我的笔记/证据笔记 affordance present; not written a freeform note) · ➖ read ≥3 papers (reading loop validated once; repeat; full multi-paper→评估 covered by [[eval-cohort-replay-2026-08-21]]).

**S5 writing-main-paper** — ✅ 正文 write · ✅ 整稿体检 (4-dim, verified) · ✅ word-count (fixed) · ✅ calm toolbar (fixed+verified) · ✅ 完成写作/lock · ✅ 回顾 · ✅ export-in-finish · ➖ drag snippet (as S3) · ➖ comment cycle (essay uses 整稿体检, verified).

**Net:** every step driven or covered at an equivalent level. Open items are low-risk (literal drag-drop, writing a freeform reading note, a 3rd paper) plus **two things for you to decide**: (1) translate of a student's own writing (currently refused), (2) whether the proposal phase should be auto-entered after plan-gen (currently reached via a coach "下一步" step).

## Fixes (round 1 — all on branch, verified green)

1. **BUG-02** — new `apps/web/src/workspace/blocks/wordcount.ts` (`countWords`, CJK-aware, mirrors Go `agent.CountWords`); wired into `WritingBlock.tsx` + `ProsePane.tsx` (replaced the char-count). Unit test `test/workspace/wordcount.test.ts` (5 cases, green). Label stays "字" (the review labels word count "字" too, so now consistent).
2. **BUG-04** — `studio/reading/ReadingRoom.tsx` composer `onKeyDown`: Enter sends / Shift+Enter newline (matches the coach `Composer`); Ctrl/Cmd+Enter still sends; guards on busy + non-empty.
3. **BUG-06** — `agent/dig_query.go` `digQuerySystem`: instruct the refine to emit disambiguated multiword academic terms (注意力→"attention span"/"sustained attention", never bare "attention") and fold in the population/phenomenon, so OpenAlex stops matching ML "attention" literature.
4. **BUG-01 + BUG-05** — `agent/studioflow.go` `toolProposeNote`: emit one `propose_note` per dimension actually discussed in a turn (not just one), AND never propose a note for a pure "no idea / no stance / confusion" message (don't file confusion as `objective`).
5. **BUG-03** — left as-is: the 422 is the intended, load-bearing signal the reading client keys on (`NoReadableContentError`, status===422) to show its paste fallback; changing it risks breaking that working path for pure console-noise gain.

**Verification:** `tsc --noEmit` ✅ · `go build ./...` ✅ · `go test ./internal/agent -run 'ComposeDigQuery|ProposeNote|Prompt|Orchestrator|Catalog|Studio'` ✅ · new `wordcount.test.ts` (5) ✅.

**Shipped:** `1bcf00be` on main → **deployed full (backend + frontend) to prod** (`DEPLOY OK (full @ 1bcf00be)`). **Live-verified BUG-02**: reopened P1's English draft in prod — the counter now reads **"187 字"** (was "986 字"), exactly matching the review's word count. ✅
