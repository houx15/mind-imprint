import type { TourSegment } from "../types";

const EXAMPLE_SLUG = "__example__"; // example-report view is opened by StudentApp (Task 9)

export const coursesSegments: TourSegment[] = [
  {
    id: "courses-intro",
    name: "为什么有这些课程",
    steps: [
      {
        id: "courses-intro-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("courses"); },
        placement: "center",
        title: "先聊聊课程",
        text: "印记认为，**思辨力**是 AI 时代最重要的能力，它可以通过系统化的学习与操练逐步获得。所以我们准备了一系列小课来练它。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-categories",
    name: "课程分类",
    steps: [
      {
        id: "courses-categories-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("courses"); },
        anchor: '[data-tour="courses-categories"]',
        placement: "bottom",
        text: "课程按主题分了几类——有的练**溯源与信息甄别**，有的练**论证与思辨结构**，有的练**研究方法**。用这些标签快速筛到你当下最想练的一类。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-enter",
    name: "进入一门课",
    steps: [
      {
        id: "courses-enter-0",
        anchor: '[data-tour="courses-grid"]',
        placement: "top",
        text: "点开任意一门课的卡片，就能看到它的介绍并开始学习。请挑一门你感兴趣的开始。",
        advance: "action",
        // Target the whole card (its wrapping `[data-course-card]` div carries
        // the onClick), not the inner CTA button — so a click anywhere on the
        // card the copy points at advances the tour, matching what actually
        // navigates to the detail page.
        actionEvent: { selector: '[data-tour="courses-grid"] [data-course-card]', type: "click" },
      },
      {
        id: "courses-enter-1",
        // The card click above lands on the course's browse/detail page, not
        // the live player — this is the real button that actually starts it.
        anchor: '[data-tour="course-detail-start"]',
        // "top": the 开始学习 button sits at the very bottom of a long detail
        // page, so a bubble below it would clamp back up onto the button.
        placement: "top",
        text: "看完这页介绍，点这个按钮就能正式进入课程，开始学习。",
        advance: "action",
        actionEvent: { selector: '[data-tour="course-detail-start"]', type: "click" },
      },
    ],
  },
  {
    id: "courses-player",
    name: "课程里怎么互动",
    steps: [
      {
        id: "courses-player-0",
        // `[data-testid="course-region"]` (not `.course-nav__next`): the
        // course opens on the Opening scene, before the "下一步" nav renders —
        // course-region is the one real anchor guaranteed present the moment
        // this step shows.
        anchor: '[data-testid="course-region"]',
        placement: "bottom",
        text: "这就是课程播放器：内容会一屏一屏推进，印记先讲解，再请你回答小问题。跟着往下走就好——**不用赶**。",
        advance: "next",
      },
      {
        id: "courses-player-1",
        anchor: '[data-tour="courses-ask-box"]',
        placement: "left",
        text: "在课程里遇到疑问，随时用这个提问框问我，我就在你身边。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-report-example",
    name: "看懂学习报告",
    steps: [
      {
        id: "courses-report-example-0",
        onEnter: (nav) => nav.openCourse(EXAMPLE_SLUG),
        // Deliberately centered — an overview of the whole report before the
        // sub-section spotlights below; no anchor (a centered step ignores it).
        placement: "center",
        title: "这是一份学习报告的示例",
        text: "每学完一门课，你都会拿到这样一份报告。下面我带你看看它由哪几块组成。",
        advance: "next",
      },
      {
        id: "courses-report-example-1",
        anchor: '[data-tour="course-report-stats"]',
        placement: "bottom",
        text: "这里是你的**用时**和**完成的阶段数**，一眼看到你走了多远。",
        advance: "next",
      },
      {
        id: "courses-report-example-2",
        anchor: '[data-tour="course-report-quiz"]',
        placement: "bottom",
        text: "小测表现记录了你答对了几题，还能逐题回看自己的作答。",
        advance: "next",
      },
      {
        id: "courses-report-example-3",
        anchor: '[data-tour="course-report-cards"]',
        placement: "top",
        text: "这门课练到的**思维工具卡**会收进这里——练过的卡都会进你的图鉴，等你集齐。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-history",
    name: "学习记录",
    steps: [
      {
        id: "courses-history-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("history"); },
        // Deliberately centered — for a new user this list is empty ("刚开始这里
        // 是空的"), so a spotlight would frame nothing; no anchor.
        placement: "center",
        text: "你学过、正在学的课程都会记录在“学习记录”里，随时能回来继续，或重看报告。刚开始这里是空的，学起来就有了。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-tujian",
    name: "图鉴与工具卡",
    steps: [
      {
        id: "courses-tujian-0",
        onEnter: (nav) => { nav.setTab("courses"); nav.setCoursesSub("gallery"); },
        anchor: '[data-tour="tujian-grid"]',
        placement: "top",
        text: "这是你的**思维工具卡图鉴**。每张卡是一种**可复用的思考方法**——比如 CRAAP 用来给资料做溯源体检、让步段用来处理反例。练过一次，卡就会被点亮、攒起星星。",
        advance: "next",
      },
      {
        id: "courses-tujian-1",
        anchor: '[data-tour="tujian-grid"]',
        placement: "top",
        text: "点开任意一张卡，看看它讲什么。",
        advance: "action",
        actionEvent: { selector: '[data-tour="tujian-grid"] button', type: "click" },
      },
      {
        id: "courses-tujian-2",
        anchor: '[data-tour="card-detail-tabs"]',
        placement: "bottom",
        text: "卡片里有“介绍”和“我的练习历史”。练习历史会记录你在项目里用过它几次，以及能在哪些课程里学到它。",
        advance: "next",
      },
    ],
  },
];

export const coursesJourney = coursesSegments;
