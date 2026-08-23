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
        text: "「项目」是你带着自己真实的写作任务，和 AI 一起协作的地方——论文、报告、申请文书都行。这里的 AI 有一条**不会越过的边界：只陪你想，不替你写**正文。",
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
        placement: "center",
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
        placement: "center",
        text: "还没归类到具体子问题下的资料会先停在“未归档”区，你可以随时把它们拖到该去的地方。",
        advance: "next",
      },
      {
        id: "reading-warren-4",
        // No anchor: `explore-keyword` only renders inside a SELECTED question
        // node's find-actions (ExplorationSidebar.tsx NodePanel) — the demo
        // graph starts with nothing selected, so it can't resolve here without
        // simulating a click. Centered per the brief's fallback ruling.
        placement: "center",
        title: "AI 帮你想关键词",
        text: "在阅读区，选中一个问题节点，印记会帮你想检索关键词；有时还需要你把正文粘进来，因为 AI 拿不到全文。",
        advance: "next",
      },
      {
        id: "reading-warren-5",
        placement: "center",
        text: "点开一篇文献，就能「进入阅读室」深读它——真正逐句消化它、和它对话的地方。",
        advance: "next",
      },
    ],
  },
  {
    id: "reading-room",
    name: "精读一篇资料",
    steps: [
      // Fallback path: real 精读 immersive open isn't available this slice
      // (openDemoReadingRoom() just calls setReadingView("list")). Tightened
      // from 4 centered steps to 2, explicitly framed as "already read" so it
      // doesn't imply we're standing inside a live 精读 view.
      {
        id: "reading-room-0",
        placement: "center",
        title: "精读一篇资料时",
        text: "在示例项目里，这篇资料已经读完、收进了图书馆。精读一篇资料时，正文在中间，你可以像聊天一样在旁边跟 AI 讨论它——划句即问、召唤思维卡、随手记笔记。",
        advance: "next",
      },
      {
        id: "reading-room-1",
        placement: "center",
        text: "读完点「完成精读」，它就带着你的笔记收进文献库。",
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
        placement: "center",
        text: "写作时，印记会用思维卡在关键处陪你想——比如帮你检查论证、补反例，一次只提一个问题，不打断你的思路。",
        advance: "next",
      },
      {
        id: "writing-3",
        anchor: '[data-tour="writing-refpanel"]',
        placement: "right",
        text: "左边这栏是**引用面板**，把你在阅读房间留下的笔记和批注直接摆在手边，写到哪引到哪，不用来回切换窗口找资料。",
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
        text: "报告开头是**项目基本信息**和一段整体**综述**，帮你先建立一个整体印象。",
        advance: "next",
      },
      {
        id: "evaluation-report-2",
        anchor: "#s5",
        placement: "right",
        text: "再往下会拆到**深度**和**自主性**两个维度——你的思考钻得多深、你在多大程度上是自己推进的，而不是被 AI 牵着走。",
        advance: "next",
      },
      {
        id: "evaluation-report-3",
        anchor: "#s9",
        placement: "top",
        text: "报告末尾还会点出这次项目里的风险点或薄弱环节，供你下一次做得更好。这份报告就是「你的思维印记」——记录的是你怎么想的，不只是你写了什么。",
        advance: "next",
      },
    ],
  },
];

export const projectsJourney = projectsSegments;
