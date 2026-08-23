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
        onEnter: (nav) => nav.setStudioRoom("plan"),
        placement: "center",
        text: "**管理**房间用来把研究问题拆成一个个能执行的任务，并给它们排上时间。",
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
        anchor: '[data-tour="manage-gantt"]',
        placement: "top",
        text: "甘特图里每一条是一个任务，拖动就能调整它的时间安排，一眼看出项目还剩多少路要走。",
        advance: "next",
      },
      {
        id: "plan-manage-3",
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
        text: "还没归类到具体子问题下的资料会先停在“未归档”区，你可以随时把它们拖到该去的地方。",
        advance: "next",
      },
      {
        id: "reading-warren-4",
        // REAL, reachable action: the map's root question cards carry
        // `warren-question`; clicking one zooms into that question (Level-2,
        // read-only — no server write). This is the best-effort real spotlight
        // of the "find sources" gesture the search flow starts from.
        anchor: '[data-tour="warren-question"]',
        placement: "bottom",
        text: "点开一条问题线索，钻进去——印记会顺着这个问题，帮你补充相关来源。",
        advance: "action",
        actionEvent: { selector: '[data-tour="warren-question"]', type: "click" },
      },
      {
        id: "reading-warren-5",
        // Fallback (centered) for the search + 采纳/丢弃 flow. The live find
        // controls (`explore-keyword`/`explore-find`) + candidate panel
        // (`explore-suggestions`) only render after a node is SELECTED inside
        // the Level-2 mindmap, which the tour nav can't drive (no selection
        // setter; Level-2 nodes carry no `warren-*` anchor). So we narrate the
        // step honestly instead of dead-ending an action on an unreachable
        // anchor. The candidate cards show 标题 / 作者·年份·期刊 / 摘要.
        placement: "center",
        title: "印记建议，你来决定",
        text: "钻进去后，用关键词或「找相似文献」让印记补充来源。它给出的每条候选都带着**标题、作者·年份·期刊、摘要**——采纳还是丢弃，由你说了算，印记只负责建议。",
        advance: "next",
      },
      {
        id: "reading-warren-6",
        placement: "center",
        text: "把一条来源采纳进来后，点「进入阅读室」就能逐句深读它——那是下一站。",
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
        text: "这就是精读室——示例里，印记正陪 Phoebe 读那篇 Nature 论文。正文在这里，你可以逐句读、随时停下来追问。",
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
        text: "卡住时，从「透镜库」召一张思维卡，用一套现成的方法拆解这篇来源——比如溯源、辨可信度。",
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
        text: "切到**图书馆**视图，看看已经读过、收进来的资料。",
        advance: "next",
      },
      {
        id: "reading-library-1",
        anchor: '[data-tour="library-table"]',
        placement: "top",
        text: "每精读完一篇资料，它就会带着你的笔记和评估收进这张表。点开任意一行，能回到当时的笔记和标注——写作要引用时，来这里翻。",
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
        anchor: '[data-tour="writing-tabs"]',
        placement: "bottom",
        text: "写作分**大纲、片段、正文**三个层次——先搭骨架，再攒素材片段，最后合成完整正文，不用一上来就憋大段文字。",
        advance: "next",
      },
      {
        id: "writing-2",
        // 铁律①: describe the card as SCAFFOLDING her own thinking — never as
        // writing body text. (In the finished demo the essay is locked, so this
        // 片段 guide may not render and the step degrades to a centered bubble;
        // the copy reads correctly either way.)
        anchor: '[data-tour="writing-aicard"]',
        placement: "right",
        text: "在「片段」里，印记把每个论证部分拆成一个个可填的引导框，帮你想清楚每一段要论证什么。**正文始终是你自己写的**——它只陪你想，绝不替你落笔。",
        advance: "next",
      },
      {
        id: "writing-3",
        anchor: '[data-tour="writing-refpanel"]',
        placement: "right",
        text: "左边这栏是**引用面板**，把你在阅读房间留下的笔记和批注直接摆在手边，写到哪引到哪，不用来回切换窗口找资料。",
        advance: "next",
      },
      {
        id: "writing-4",
        // Real seeded 批注 (migration 0083). Lives in the panel's「AI批注」tab —
        // not the default tab, so in the demo it may sit behind a tab click and
        // the step degrades to centered; the copy stands on its own.
        anchor: '[data-tour="writing-annotations"]',
        placement: "right",
        text: "印记通读你的初稿后，会在这里留下分层批注：**绿色**是亮点、**蓝色**是建议、**红色**是要处理的问题。点一条，就跳到正文里对应的那句话。",
        advance: "next",
      },
      {
        id: "writing-5",
        // The 完成写作 lock. In the finished demo this button is replaced by
        // 「重新打开写作」, so the anchor may not resolve and the step degrades to
        // a centered bubble — the copy still describes the real lock behavior.
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
        // The point of no return. In the finished demo this button is replaced
        // by the archived-state label, so the anchor may not resolve and the
        // step degrades to a centered bubble — the copy still carries the real
        // warning (the student's answer to "after you finish, can I still
        // change my writing?").
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
        id: "evaluation-report-6",
        anchor: "#s6",
        placement: "right",
        text: "**智识自主 A**——你在多大程度上是自己在推进思考，而不是被 AI 牵着走。同样六个维度、带证据。",
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
