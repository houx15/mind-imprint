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
        text: "印记认为，**思辨力**是 AI 时代最重要的能力，而它可以通过系统化的学习和操练一点点长出来。所以我们准备了一系列小课，带你练。",
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
        text: "课程按主题分成了几类，你可以用这些标签快速筛选，找到当下最想练的那一类。",
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
        text: "点开任意一门课的卡片，就能看到它的介绍并开始学习。挑一门你感兴趣的试试吧。",
        advance: "next",
      },
    ],
  },
  {
    id: "courses-player",
    name: "课程里怎么互动",
    steps: [
      {
        id: "courses-player-0",
        anchor: '[data-testid="course-region"]',
        placement: "center",
        text: "课程是一屏一屏推进的：印记会先讲解，再请你回答小问题。想清楚了再往下走——**不用赶**。",
        advance: "next",
      },
      {
        id: "courses-player-1",
        anchor: '[data-testid="ask-bubble"]',
        placement: "left",
        text: "学的过程中有疑问，随时在这里问我，我就在你身边。",
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
        anchor: '[data-tour="course-report"]',
        placement: "center",
        title: "这是一份学习报告的样子",
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
        text: "这门课练到的**思维工具卡**会收进这里——它们会在你的图鉴里慢慢集齐。",
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
        anchor: '[data-tour="courses-history"]',
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
        placement: "center",
        text: "这是你的**思维工具卡图鉴**。每张卡是一种思考方法，练过就会被点亮、攒起星星。",
        advance: "next",
      },
      {
        id: "courses-tujian-1",
        anchor: '[data-tour="tujian-grid"]',
        placement: "center",
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
