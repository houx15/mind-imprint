import type { TourSegment } from "../types";

export const projectsSegments: TourSegment[] = [
  {
    id: "projects-intro",
    name: "AI 陪写的项目",
    steps: [
      {
        id: "projects-intro-0",
        onEnter: (nav) => nav.setTab("projects"),
        placement: "center",
        title: "再聊聊项目",
        text: "「项目」是你带着自己真实的写作任务，和 AI 一起协作的地方——论文、报告、申请文书都行。整个项目分成五个房间，从立题一路走到回顾，等下我会带你逐个走一遍。这里的 AI 有一条**不会越过的边界：只陪你想，不替你写**正文。",
        advance: "next",
      },
      {
        id: "projects-intro-1",
        placement: "center",
        text: "更特别的是：你怎么想、卡在哪、怎么被点拨着走出来，这个**过程**会被记录下来，最后长成一份**过程评估报告**，而不只是看你交出的成品。",
        advance: "next",
      },
    ],
  },
  {
    id: "projects-open-demo",
    name: "打开一个示例项目",
    steps: [
      {
        id: "projects-open-demo-0",
        onEnter: (nav) => nav.openDemoProject(),
        placement: "center",
        text: "我先带你打开一个示例项目，走一遍五个房间，你就知道自己动手时该怎么用了。",
        advance: "next",
      },
    ],
  },
  {
    id: "project-rooms-bar",
    name: "五个房间",
    steps: [
      {
        id: "project-rooms-bar-0",
        anchor: '[data-tour="room-bar"]',
        placement: "bottom",
        text: "一个项目分成五个房间：**立题**（想清楚要研究什么）→ **管理**（拆解任务、排进度）→ **阅读**（收集和消化资料）→ **写作**（真正落笔）→ **回顾**（收尾、看过程评估）。可以随时切换，不是线性锁死的流程。",
        advance: "next",
      },
    ],
  },
  {
    id: "forming",
    name: "立题：想清楚要研究什么",
    steps: [
      {
        id: "forming-0",
        onEnter: (nav) => nav.setStudioRoom("forming"),
        placement: "center",
        text: "先来看**立题**房间——这是项目的起点，你在这里把一个模糊的想法，收敛成一个能研究、能验证的问题。",
        advance: "next",
      },
      {
        id: "forming-1",
        anchor: '[data-tour="forming-proposal"]',
        placement: "right",
        text: "这张表卡住了立题要想清楚的几件事：研究问题、背景与动机、初步假设……填得越具体，后面 AI 陪你想得就越准。",
        advance: "next",
      },
      {
        id: "forming-2",
        demoModal: { kind: "question-card" },
        title: "AI 不会替你定论",
        text: "没头绪时，可以点开**提问卡**，让印记用一串问题帮你把题目想清楚——而不是直接帮你定一个题目，定题这件事，得是你自己的。",
        advance: "next",
      },
      {
        id: "forming-3",
        anchor: '[data-tour="coach-rail"]',
        placement: "left",
        text: "右边这条是**陪练栏**——贯穿五个房间都在。你随时可以在这里问 AI，它会一次只问你一个问题，帮你把卡住的地方想透，而不是一股脑替你答完。",
        advance: "next",
      },
    ],
  },
  {
    id: "plan-manage",
    name: "管理：拆解与排期",
    steps: [
      {
        id: "plan-manage-0",
        // ① 立题→管理 transition (P7 Task 9): the plan骨架 is a deterministic
        // system step derived from the研究问题 — NOT AI-writing-for-the-student
        // (铁律 scope is body text only), so this copy is safe.
        onEnter: (nav) => nav.setStudioRoom("plan"),
        placement: "center",
        text: "写好研究问题后，进到**管理**房间——印记会照着你的思路先拟一份计划的骨架，你再按自己的节奏调整。这里就是把问题拆成一个个能执行的任务、排上时间的地方。",
        advance: "next",
      },
      {
        id: "plan-manage-1",
        anchor: '[data-tour="manage-viewtoggle"]',
        placement: "bottom",
        text: "同一份计划有三种看法：**甘特图**看时间线、**看板**看任务状态、**日志**看你实际做了什么。切换着用，哪种顺手用哪种。",
        advance: "next",
      },
      {
        id: "plan-manage-2",
        // Real-scene (P7 Task 9): pin the 甘特图 view so this anchor resolves
        // regardless of the demo's last-left plan view.
        onEnter: (nav) => nav.setPlanView("gantt"),
        anchor: '[data-tour="manage-gantt"]',
        placement: "top",
        text: "甘特图里每一条是一个任务，拖动就能调整它的时间安排，一眼看出项目还剩多少路要走。",
        advance: "next",
      },
      {
        id: "plan-manage-3",
        // ② 活动日志 (P7 Task 9): drive the plan room to the 日志 view so the
        // seeded activity feed (`manage-activity-log`) is on screen — a real
        // spotlight of "过程即数据" at the task level.
        onEnter: (nav) => nav.setPlanView("log"),
        anchor: '[data-tour="manage-activity-log"]',
        placement: "top",
        text: "切到**日志**——你做过的每一步：和 AI 聊了什么、读了什么、写了什么，都会自动记在这里，回顾时一目了然，不用自己补记。",
        advance: "next",
      },
      {
        id: "plan-manage-4",
        anchor: '[data-tour="manage-export"]',
        placement: "bottom",
        text: "计划或日志随时可以**导出**带走——它是你的，不锁在这里。",
        advance: "next",
      },
    ],
  },
  {
    id: "reading-warren",
    name: "阅读：探索与收集",
    steps: [
      {
        id: "reading-warren-0",
        onEnter: (nav) => {
          nav.setStudioRoom("reading");
          nav.setReadingView("graph");
        },
        placement: "center",
        text: "接下来是**阅读**房间——research 的主战场。先看它的探索视图，帮你围绕研究问题找资料、理线索。",
        advance: "next",
      },
      {
        id: "reading-warren-1",
        anchor: '[data-tour="reading-viewtoggle"]',
        placement: "bottom",
        text: "阅读房间有两种视图：**图书馆**（管理读过的资料）和**探索**（围绕问题找线索）。现在看到的是探索视图。",
        advance: "next",
      },
      {
        id: "reading-warren-2",
        anchor: '[data-tour="warren-question"]',
        placement: "bottom",
        text: "地图中心是你的主问题，周围长出的是从它派生的子问题——探索资料时，思路会顺着这张图自然展开，而不是漫无目的地搜。",
        advance: "next",
      },
      {
        id: "reading-warren-3",
        anchor: '[data-tour="warren-unfiled"]',
        placement: "top",
        text: "还没归到具体问题下的资料，会先停在**未归类**区——你可以随时把它挂到该去的问题下面。",
        advance: "next",
      },
      {
        id: "reading-warren-4",
        // ⑥ 让印记建议检索方向 (P7 Task 9) — REAL spotlight of the controls
        // column's 检索卡/建议方向 entry (`search-card-trigger`), which renders
        // on the idle exploration view (map or hole, no node selected). The
        // sibling「让印记建议检索方向」button in the same box has no anchor, so
        // this trigger button is the reliable real target for the "怎么找" beat.
        anchor: '[data-tour="search-card-trigger"]',
        placement: "left",
        text: "不知道搜什么、上哪找？让印记按你的研究问题**建议几个检索方向**；旁边这张**检索卡**还会教你怎么判断一条来源可不可信。",
        advance: "next",
      },
      {
        id: "reading-warren-5",
        // ⑦ 还需要探索 (P7 Task 9) — REAL spotlight of the seeded resource-needs
        // box (migration 0087 seeded 3 rows into studio_state).
        anchor: '[data-tour="needs-resources"]',
        placement: "left",
        text: "还差哪些资料？记在**「还需要探索」**里——它是你给自己留的找料清单，读到一半想起还缺什么，随手补一条。",
        advance: "next",
      },
      {
        id: "reading-warren-6",
        // ④ drill-in (P7 Task 9) — REAL, reachable action: the map's root
        // question cards carry `warren-question`; clicking one zooms into that
        // question's real 2nd layer (Level-2 mindmap; read-only, no server
        // write). The document-capture click delegate matches via
        // `closest('[data-tour="warren-question"]')`, so a click on any root
        // card (or its inner text) both zooms AND advances the step.
        anchor: '[data-tour="warren-question"]',
        placement: "bottom",
        text: "点开一条问题线，**钻进去**——印记会顺着这个问题，帮你往下补充相关来源。",
        advance: "action",
        actionEvent: { selector: '[data-tour="warren-question"]', type: "click" },
      },
      {
        id: "reading-warren-7",
        // ⑤ search + 采纳/丢弃 (P7 Task 9) — NARRATED FALLBACK. The live find
        // controls (`explore-find`/`explore-keyword`) and the candidate panel
        // (`explore-suggestions`) only render after a Level-2 mindmap node is
        // SELECTED, and Level-2 nodes carry NO `data-tour` anchor and there is
        // no selection nav hook — so the tour can't deterministically click one
        // (and an `action` step gated on an unreachable control would dead-end
        // the tour, which only advances on the action). We narrate honestly.
        // 铁律②: 印记 proposes, the student decides.
        placement: "center",
        title: "印记建议，你来决定",
        text: "钻进去后，用「**找相似文献**」或换个关键词，让印记补充候选来源。它给出的每条候选都带着**标题、作者·年份·期刊、摘要**——**采纳还是丢弃，由你说了算**，印记只负责建议，绝不替你收进来。",
        advance: "next",
      },
      {
        id: "reading-warren-8",
        // ⑧⑨ add + where-to-enter-reading (P7 Task 9) — NARRATED. adopt/paste/
        // enter-reading all 403 on the read-only demo, and `paper-enter-reading`
        // lives on a selection-gated paper detail panel (see above). We narrate
        // the gesture and hand off to the real immersive room via the deep-link
        // in the next segment.
        placement: "center",
        text: "把有用的来源**采纳、收进来**（没有全文时，也能直接把正文粘进来）。挑好一篇，点「**进入阅读室**」，就能和印记逐句深读它——我这就带你进去看一篇。",
        advance: "next",
      },
      {
        id: "reading-warren-9",
        // 检索卡 (P7 Task 9) — REAL: `openSearchCard()` opens the static
        // `SearchCardModal` (GET-only, no 403) via a ref-guarded one-shot. Kept
        // LAST in the segment so the modal it leaves open is torn down cleanly
        // by the next segment's `openDemoReadingRoom` (which unmounts the whole
        // exploration view) — no stuck overlay, and no `action` step follows it.
        onEnter: (nav) => nav.openSearchCard(),
        anchor: '[data-tour="search-card"]',
        placement: "bottom",
        title: "检索卡：给来源做体检",
        text: "读一篇之前，先看看这张**检索卡**——它把「怎么判断来源靠不靠谱」拆成一串可操作的问题（谁写的、什么时候、有没有依据……）。挑好、读透一篇来源前，常回来用它。",
        advance: "next",
      },
    ],
  },
  {
    id: "reading-room",
    name: "精读一篇资料",
    steps: [
      // Real-scene 精读: `openDemoReadingRoom()` (P6, Task 5) opens the REAL
      // immersive Reading Room on the demo's Nature paper as a read-only replay
      // (seeded transcript + demoMode; every write path disabled). All five
      // rr-* anchors render in demoMode (onSaveNote is still passed, so
      // rr-notes mounts; its textarea is merely disabled).
      {
        id: "reading-room-0",
        onEnter: (nav) => nav.openDemoReadingRoom(),
        anchor: '[data-tour="rr-article"]',
        placement: "left",
        title: "精读室",
        text: "这就是精读室——示例里，印记正陪 Phoebe 读那篇 Nature 论文。整篇正文都在这里，你可以逐句读、随时停下来追问。注意正文里几处**高亮的句子**——那是思维卡帮你划出来的关键句。",
        advance: "next",
      },
      {
        id: "reading-room-1",
        anchor: '[data-tour="rr-chat"]',
        placement: "right",
        text: "一边读一边问印记：划一句、点一段，就能就这里和它讨论。示例里能看到用 CRAAP 透镜逐条盘问来源的真实对话。",
        advance: "next",
      },
      {
        id: "reading-room-2",
        anchor: '[data-tour="rr-deck"]',
        placement: "right",
        text: "卡住时，从「透镜库」召一张思维卡，用一套现成的方法拆解这篇来源——比如溯源、辨可信度。示例里这张卡已经跑过，把正文里几处关键句**划了出来**（就是你刚看到的高亮），一眼看清它在哪儿站得住、哪儿站不住。",
        advance: "next",
      },
      {
        id: "reading-room-3",
        anchor: '[data-tour="rr-notes"]',
        placement: "left",
        text: "「我的笔记」是你自己的空间——随手记下想法、疑问、要引用的点。它只属于你，不会喂给评估。",
        advance: "next",
      },
      {
        id: "reading-room-4",
        anchor: '[data-tour="rr-finish"]',
        placement: "bottom",
        text: "读完点「完成这篇」，它就带着你的笔记和判断收进文献库，写作时随手可取。",
        advance: "next",
      },
    ],
  },
  {
    id: "reading-nodedone",
    name: "读完，回到地图",
    steps: [
      {
        id: "reading-nodedone-0",
        // ⑫ back-to-map + node-read (P7 Task 9): `setReadingView("graph")`
        // closes the immersive 精读 room (P6 close-on-view) and shows the map;
        // `markDemoNodeRead("…0290")` client-badges the root whose reference
        // (…0260) the tour just "read" — the demo project is write-blocked, and
        // migration 0088 seeded that reference 'reading' (not 'done') on purpose
        // so this shows a real un-badged → 已读 transition without a write.
        onEnter: (nav) => {
          nav.setStudioRoom("reading");
          nav.setReadingView("graph");
          nav.markDemoNodeRead("00000000-0000-0000-0000-000000000290");
        },
        anchor: '[data-tour="warren-node-read"]',
        placement: "bottom",
        text: "回到探索图谱——刚读完的那篇，让它所在的问题节点亮起了**「已读」**。你的阅读进度，就这样自然地长在图上，不用另外记。",
        advance: "next",
      },
      {
        id: "reading-nodedone-1",
        // ⑬ switch view via the top button (P7 Task 9) — REAL spotlight of the
        // list/graph toggle, present in both reading views.
        anchor: '[data-tour="reading-viewtoggle"]',
        placement: "bottom",
        text: "阅读房间顶部这个开关，在**探索图谱**和**文献库**之间随时切换——探索时理线索，收料后翻清单。",
        advance: "next",
      },
    ],
  },
  {
    id: "reading-library",
    name: "文献库",
    steps: [
      {
        id: "reading-library-0",
        onEnter: (nav) => {
          nav.setStudioRoom("reading");
          nav.setReadingView("list");
        },
        placement: "center",
        text: "切到**文献库**视图，看看已经读过、收进来的资料。",
        advance: "next",
      },
      {
        id: "reading-library-1",
        anchor: '[data-tour="library-table"]',
        placement: "top",
        text: "每精读完一篇资料，它就会带着你的笔记和评估收进这张表。点开任意一行，能回到当时的笔记和标注——写作要引用时，来这里翻。",
        advance: "next",
      },
      {
        id: "reading-library-2",
        // ⑭ add-source (P7 Task 9) — REAL spotlight; the actual add flow 403s on
        // the demo, so we narrate rather than click.
        anchor: '[data-tour="library-add"]',
        placement: "bottom",
        text: "想加新资料？点「**添加来源**」——粘一个链接、贴一个 DOI，或者直接把正文粘进来，都能收进这张表。",
        advance: "next",
      },
      {
        id: "reading-library-3",
        // ⑮ metadata edit (P7 Task 9) — REAL spotlight of the right-hand preview
        // panel, which renders for the first row by default (ReadingBlock falls
        // back to rows[0]); patch-metadata 403s on the demo, so we narrate.
        anchor: '[data-tour="library-preview"]',
        placement: "left",
        text: "点开一行，右边是它的**题录信息**：标题、作者、分类、年份、链接。哪里不全或不对，直接在这里改、随时补。",
        advance: "next",
      },
      {
        id: "reading-library-4",
        // enter-reading from the library (P7 Task 9) — REAL spotlight; the button
        // renders in the preview panel for a non-pending reference. enter-reading
        // 403s on the demo, so we narrate.
        anchor: '[data-tour="library-enter-reading"]',
        placement: "top",
        text: "想重读某一篇？在这里点「**进入阅读室**」，就能再钻进去逐句深读，接着上次的笔记往下想。",
        advance: "next",
      },
    ],
  },
  {
    id: "writing",
    name: "写作：你自己动笔",
    steps: [
      {
        id: "writing-0",
        onEnter: (nav) => nav.setStudioRoom("writing"),
        placement: "center",
        text: "到了**写作**房间——这里最重要的一条边界：**正文永远是你自己写的，AI 不会替你代笔**。它能帮你理清结构、查论证漏洞、找例证，但落笔的手是你的。",
        advance: "next",
      },
      {
        id: "writing-1",
        // ⑯ 提案/正文 两种模式 (P7 Task 10) — REAL spotlight of the doc-mode
        // toggle (`writing-docswitch`, WritingBlock.tsx:380). The demo seeds BOTH
        // a proposal and an essay, so the switch renders. Land on the essay 大纲
        // so ⑯→⑰ flow without a jarring jump.
        onEnter: (nav) => nav.setWritingView({ doc: "essay", tab: "outline" }),
        anchor: '[data-tour="writing-docswitch"]',
        placement: "bottom",
        text: "写作房间有**两种模式**：先在「**提案**」里把研究方案想清楚，再切到「**正文**」正式写论文。点这里就能在两者之间来回切换。",
        advance: "next",
      },
      {
        id: "writing-2",
        // ⑰ 大纲 building-blocks / subquestions (P7 Task 10) — REAL spotlight of
        // the seeded outline (`writing-outline`, added on OutlinePane's root).
        // The essay 大纲 carries the thesis→subquestion branches (each becomes a
        // body paragraph). This is a deterministic system-derived structure, not
        // AI writing body text (铁律 scope is prose only) — copy is safe.
        onEnter: (nav) => nav.setWritingView({ doc: "essay", tab: "outline" }),
        anchor: '[data-tour="writing-outline"]',
        placement: "right",
        text: "「大纲」把你的**主问题拆成一条条可回答的子问题**——每条子问题就长成一段主体论证。先搭好这张骨架，正文才有顺着走的线。",
        advance: "next",
      },
      {
        id: "writing-3",
        // ⑱ sidebar default = 阅读笔记 (P7 Task 10) — the demo's ReferencePanel
        // now DEFAULTS to the 阅读笔记 tab (T6), so this is a real spotlight of
        // the panel with your notes already摊开; no forceTab needed.
        anchor: '[data-tour="writing-refpanel"]',
        placement: "right",
        text: "左边这栏默认摊开你的**阅读笔记**——写到哪、要引哪一篇，随手就能翻到，不用来回切窗口找资料。",
        advance: "next",
      },
      {
        id: "writing-4",
        // ⑲ 片段 real guiding box (P7 Task 10) — the 片段引导/写作卡 lives ONLY in
        // the PROPOSAL doc's 片段 tab. Drive there so `writing-aicard` renders as
        // a FILLED read-only guide (T5 `ProposalGuideReadOnly`).
        // 铁律①: the card SCAFFOLDS her thinking — it never writes body text.
        onEnter: (nav) => nav.setWritingView({ doc: "proposal", tab: "snippets" }),
        anchor: '[data-tour="writing-aicard"]',
        placement: "right",
        text: "在「片段」里，印记把每个论证部分拆成一个个**可填的引导框**，帮你想清楚每段要论证什么。**正文始终是你自己写的**——它只陪你想，绝不替你落笔。",
        advance: "next",
      },
      {
        id: "writing-5",
        // ⑳ AI批注 (P7 Task 10, relabeled from the old 引用面板 mislabel) — select
        // the 批注 tab so the real seeded 批注 (migration 0083) render.
        // 🚨 CONSTRAINT (T6): `selectRefPanelTab("anno")` is OVERRIDDEN by the
        // showSnippets→snippets effect if the writing room is on the 正文/draft
        // tab. writing-4 left us on the PROPOSAL 片段 tab (showSnippets off), so
        // this switch STICKS — we deliberately spotlight 批注 BEFORE moving to
        // 正文 in the next step.
        onEnter: (nav) => nav.selectRefPanelTab("anno"),
        anchor: '[data-tour="writing-annotations"]',
        placement: "right",
        text: "切到「**AI批注**」——印记通读你的初稿后留下的分层批注：**绿色**是亮点、**蓝色**是建议、**红色**是要处理的问题。点一条，就跳到正文里对应的那句话。",
        advance: "next",
      },
      {
        id: "writing-6",
        // ㉑ how to trigger 批注 + move to 正文 (P7 Task 10) — REAL spotlight of the
        // DISABLED read-only「让印记通读并批注」button (T5 `writing-review-trigger`,
        // renders on the proposal 正文/prose tab for the demo). This is the FIRST
        // step onto the draft tab — the 批注 spotlight above already happened, so
        // the showSnippets override can't clobber it retroactively.
        onEnter: (nav) => nav.setWritingView({ doc: "proposal", tab: "draft" }),
        anchor: '[data-tour="writing-review-trigger"]',
        placement: "bottom",
        text: "写好一稿，点「**让印记通读并批注**」，它就会从头读一遍、逐段给批注——就是你刚看到的那些。看完提案，我们切到正文继续写。",
        advance: "next",
      },
      {
        id: "writing-7",
        // ㉓ 正文 left sidebar (P7 Task 10) — on the essay 正文, the panel's
        // 片段/阅读笔记/AI批注 tabs are all at hand while you write. Spotlight the
        // whole `writing-refpanel` (its root anchor is stable regardless of which
        // tab is active — showSnippets flips it to 片段 here, which is fine).
        onEnter: (nav) => nav.setWritingView({ doc: "essay", tab: "draft" }),
        anchor: '[data-tour="writing-refpanel"]',
        placement: "right",
        text: "正式写正文时，左边这栏还在：**片段、阅读笔记、AI批注**三个页签都摆在手边，边写边取，不打断思路。",
        advance: "next",
      },
      {
        id: "writing-8",
        // ㉔ to-explore box (P7 Task 10) — REAL spotlight of the 还需要探索的 box at
        // the top of the writing ReferencePanel (`needs-resources`, always shown
        // while writing). Seeded rows render for the demo.
        anchor: '[data-tour="needs-resources"]',
        placement: "right",
        text: "写着写着发现还缺点什么？记进**「还需要探索」**里——它是你给自己留的找料清单，回头去补，不怕当场卡住。",
        advance: "next",
      },
      {
        id: "writing-9",
        // 完成写作 (P6, Task 9 → kept) — the essay 正文's lock. For the demo it
        // always renders DISABLED (read-only), so `writing-finish` resolves and
        // this is a real spotlight of the real lock.
        onEnter: (nav) => nav.setWritingView({ doc: "essay", tab: "draft" }),
        anchor: '[data-tour="writing-finish"]',
        placement: "bottom",
        text: "写完初稿，点「完成写作」会**锁定初稿、解锁回顾**——别担心，之后仍可「重新打开写作」继续改。",
        advance: "next",
      },
    ],
  },
  {
    id: "reflection",
    name: "回顾：收尾与复盘",
    steps: [
      {
        id: "reflection-0",
        onEnter: (nav) => nav.setStudioRoom("reflection"),
        placement: "center",
        text: "最后是**回顾**房间——项目收尾的地方，也是过程评估的入口。",
        advance: "next",
      },
      {
        id: "reflection-1",
        anchor: '[data-tour="reflection-prompts"]',
        placement: "top",
        text: "回顾几个复盘问题，帮你把这个项目里学到的东西沉淀下来，而不是写完就忘。回答完、确认收尾后，项目就进入「完成」状态。",
        advance: "next",
      },
      {
        id: "reflection-2",
        // Real-scene (P6, Task 9): the point of no return. For the demo this
        // button always renders (disabled, read-only), so `review-finalize`
        // resolves and this is a real spotlight of the real lock — the student's
        // answer to "after you finish, can I still change my writing?".
        anchor: '[data-tour="review-finalize"]',
        placement: "top",
        text: "全部回顾写完、确认无误后，点「定稿并开始评估」。**注意：定稿后，正文和回顾都会锁定、无法再修改**，印记会据此生成过程评估——所以一定是真的改完了，再定稿。",
        advance: "next",
      },
    ],
  },
  {
    id: "evaluation-report",
    name: "过程评估报告",
    steps: [
      {
        id: "evaluation-report-0",
        // Open the shared demo project's 过程评估报告 (world-readable, fetched by
        // id → a non-owner tour user sees it). Kept center while the report
        // loads; the next steps anchor its sections once they're rendered.
        onEnter: (nav) => nav.openDemoReport(),
        placement: "center",
        title: "你的思维印记",
        text: "项目完成后，AI 会基于整个过程——你在立题、阅读、写作里留下的每一步——生成一份**过程评估报告**，而不是只看最终交上来的稿子。我这就带你看一份真实的示例报告。",
        advance: "next",
      },
      {
        id: "evaluation-report-1",
        anchor: "#s1",
        placement: "right",
        text: "报告开头是**项目基本信息**——课题、类型、起止时间、里程碑，还有几个关键计数：和 AI 聊了多少轮、读了几篇资料、写了多少字。",
        advance: "next",
      },
      {
        id: "evaluation-report-2",
        anchor: "#s2",
        placement: "right",
        text: "**综述**用一段话讲清这个项目整体是怎么推进的——资料、写作、和 AI 协作各自的样子，最后给出改进建议和推荐课程。",
        advance: "next",
      },
      {
        id: "evaluation-report-3",
        anchor: "#s3",
        placement: "top",
        text: "**过程时间线**把你做过的每一步按时间排开——什么时候聊、什么时候读、什么时候写，每一步用了几轮 AI。过程被看见，而不只是结果。",
        advance: "next",
      },
      {
        id: "evaluation-report-4",
        anchor: "#s4",
        placement: "top",
        text: "**材料清单**列出你用过的每一份资料：最终怎么判定、能支撑什么、不能支撑什么、用在了哪里。",
        advance: "next",
      },
      {
        id: "evaluation-report-5",
        anchor: "#s5",
        placement: "right",
        text: "**认知深度 D**——你的思考钻得有多深。六个维度各有一个评级、对应的行为证据，和下一步建议。",
        advance: "next",
      },
      {
        id: "eval-dim-D2",
        // ㉕ per-dimension spotlight (P7 Task 10) — each维度卡 carries the real
        // `axis-dim-{code}` anchor (T8). Point at 1–2 representative dims per
        // axis (not all 12) using each dim's own「这一维看的是」(means) copy.
        anchor: '[data-tour="axis-dim-D2"]',
        placement: "right",
        text: "点开一张维度卡，它先告诉你「**这一维看的是**」什么。比如 **D2 证据与信源**：你会不会找资料、判断它可不可信、说清每份资料能支持什么。",
        advance: "next",
      },
      {
        id: "eval-dim-D6",
        anchor: '[data-tour="axis-dim-D6"]',
        placement: "right",
        text: "再比如 **D6 反思与元认知**：你能不能回头审视自己的思路，说清自己的判断是怎么来的——每一维都配着你的行为证据和下一步建议。",
        advance: "next",
      },
      {
        id: "evaluation-report-6",
        anchor: "#s6",
        placement: "right",
        text: "**智识自主 A**——你在多大程度上是自己在推进思考，而不是被 AI 牵着走。同样六个维度、带证据。",
        advance: "next",
      },
      {
        id: "eval-dim-A4",
        anchor: '[data-tour="axis-dim-A4"]',
        placement: "right",
        text: "自主轴也一样细。**A4 对抗与检验**看的是：你会不会主动请人挑刺、找自己论证里的问题，而不是只等着被表扬。",
        advance: "next",
      },
      {
        id: "eval-dim-A5",
        anchor: '[data-tour="axis-dim-A5"]',
        placement: "right",
        text: "**A5 判断署名**看的是：你愿不愿意为自己的结论负责，说清「这是我的判断」——而不是把判断权交给 AI。",
        advance: "next",
      },
      {
        id: "evaluation-report-7",
        anchor: "#s7",
        placement: "right",
        text: "**提问透镜**回看你问 AI 的那些问题——你怎么问，往往最能反映你怎么想。",
        advance: "next",
      },
      {
        id: "evaluation-report-8",
        anchor: "#s8",
        placement: "top",
        text: "**工具卡与子代理**记录你这一路召唤过哪些思维工具（CRAAP、溯源、让步段……）以及它们帮你做了什么。",
        advance: "next",
      },
      {
        id: "evaluation-report-9",
        anchor: "#s9",
        placement: "top",
        text: "末尾是**风险提示**——这次项目里的薄弱环节或需要警惕的地方，供你下次做得更好。这份报告就是「你的思维印记」：记录你怎么想，不只是你写了什么。",
        advance: "next",
      },
    ],
  },
];

export const projectsJourney = projectsSegments;
