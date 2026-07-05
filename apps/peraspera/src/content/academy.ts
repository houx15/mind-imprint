// Per Aspera — 学院 / 课程项目 (academy) page (/academy, /en/academy)
// content. zh is the source of truth; en is an idiomatic (not literal)
// translation.
//
// This page introduces our part-time, online courses — for families who share
// our way of learning but haven't applied to (or aren't interested in
// applying to) schools like Astra Nova. The course library is presented as
// SIX MODULES, not a long list of individual lessons. Modules 1-5 are drawn
// from docs/03_课程库_单课设计; module 6 (AI 与产品) is new, added per the
// refactor plan. Plain, warm, concrete wording — no jargon, no deadlines, no
// info sessions (说明会), no antithesis, and no mention of time zones.

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "学院", en: "Academy" },
  title: {
    zh: "一套业余时间就能上的课，陪孩子练真正的思考。",
    en: "Part-time courses that build real thinking, alongside school.",
  },
  sub: {
    zh: "全程在线，每周一次，不用请假、不用来回接送。孩子和同伴一起讨论一个真实的问题，练的是怎么想问题的过程。",
    en: "Fully online, once a week — no school absence, no commute. Kids discuss a real question together with peers, practicing the process of working through it.",
  },
};

/* ---- Intro: our course idea --------------------------------------------- */
export const introSection: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
} = {
  eyebrow: { zh: "我们怎么上课", en: "How our classes work" },
  title: {
    zh: "从一个真实的问题开始,一起把它想清楚。",
    en: "Start from a real question, and think it through together.",
  },
  body: [
    {
      zh: "每一节课都从一件真实的事情说起——一段视频、一篇引发争议的报道、一个身边就能遇到的选择。孩子们先各自说说自己的第一反应,再一起对照证据、互相追问,看看这个判断站不站得住。老师在旁边引导,但结论要孩子自己想出来。",
      en: "Every class starts with something real — a video, a report that sparked debate, a choice a kid might actually face. Everyone shares their first reaction, then the group checks it against evidence and questions each other, to see whether the judgment holds up. The teacher guides the discussion, but the conclusion is the child's own.",
    },
    {
      zh: "课堂不大,几个孩子围在一起讨论,AI 是可以随时拿来查证、拓展思路的工具,但怎么判断、怎么下结论,始终是孩子自己的事。",
      en: "Classes are small — a handful of kids in real discussion. AI is there as a tool to check facts and widen ideas, but the judgment and the conclusion always stay with the child.",
    },
    {
      zh: "上课全程在线,每周一次,和孩子平时的学校生活并行,不用另外请假或安排接送。",
      en: "Everything runs online, once a week, alongside your child's regular school life — no extra absence, no commute.",
    },
  ],
};

/* ---- The six modules ------------------------------------------------------ */
export interface AcademyModule {
  title: Bilingual;
  blurb: Bilingual;
  topics: Bilingual[];
}

export const modulesIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "课程库", en: "The course library" },
  title: { zh: "六个模块,练全一整套思考能力。", en: "Six modules, one full set of thinking abilities." },
  sub: {
    zh: "我们按能力分成六个模块,每个模块下面有很多具体的课,挑几个例子给你看看孩子会讨论什么。",
    en: "We group lessons by ability into six modules. Each module holds many specific lessons — here are a few examples of what kids actually discuss.",
  },
};

export const modules: AcademyModule[] = [
  {
    title: { zh: "信息素养", en: "Information literacy" },
    blurb: {
      zh: "一条信息传到孩子面前之前,经过了谁的手、藏了什么目的。练的是怎么找到源头、怎么核实。",
      en: "Before any piece of information reaches a kid, it's passed through someone's hands and often someone's agenda. This module practices tracing it back to the source and checking it.",
    },
    topics: [
      { zh: "CRAAP 与 SIFT 核查法", en: "The CRAAP and SIFT checklists" },
      { zh: "识破企业的“漂绿”说法", en: "Spotting corporate greenwashing" },
      { zh: "顺着资金往回查", en: "Tracing who's funding a claim" },
      { zh: "拆解一部气候纪录片", en: "Deconstructing a climate documentary" },
      { zh: "抓出 AI 编造的假消息", en: "Catching AI-generated misinformation" },
      { zh: "写作文时守住诚信底线", en: "Academic writing with integrity" },
    ],
  },
  {
    title: { zh: "知识工具与思辨", en: "Thinking tools" },
    blurb: {
      zh: "给孩子几件顺手的思维工具——分清事实和观点、看懂数据、拆穿站不住脚的论证。",
      en: "Hands-on tools for everyday reasoning — telling fact from opinion, reading data honestly, and taking a shaky argument apart.",
    },
    topics: [
      { zh: "事实、观点与价值判断", en: "Fact, opinion, and value judgment" },
      { zh: "我们到底能确定到什么程度", en: "How sure can we really be" },
      { zh: "数据与统计怎么看", en: "Reading data and statistics" },
      { zh: "论证与常见的逻辑谬误", en: "Arguments and logical fallacies" },
      { zh: "语言怎么悄悄影响我们怎么想", en: "How language shapes the way we think" },
    ],
  },
  {
    title: { zh: "五大知识领域", en: "Five areas of knowledge" },
    blurb: {
      zh: "自然科学、数学、人文社科、艺术、历史——每个领域“算知道”的标准都不一样,孩子要学会分辨。",
      en: "Natural science, mathematics, the human sciences, the arts, history — each field has its own standard for what counts as “knowing,” and kids learn to tell them apart.",
    },
    topics: [
      { zh: "自然科学怎么算“知道”", en: "How natural science claims to know" },
      { zh: "数学靠严密的证明来确立结论", en: "Mathematics establishes its conclusions through rigorous proof" },
      { zh: "人文社科的研究人凭什么算数", en: "What makes human & social science research count" },
      { zh: "艺术:有据解读,还是自己脑补", en: "The arts: grounded reading, or projection" },
      { zh: "哥伦布是英雄还是恶棍", en: "Columbus: hero or villain" },
      { zh: "两源对照法 OPCVL", en: "Cross-checking two sources (OPCVL)" },
    ],
  },
  {
    title: { zh: "AI 协作与伦理", en: "Working with AI, and its ethics" },
    blurb: {
      zh: "怎么和 AI 一起想问题、又不把判断力交出去;再往深一层,看这件事在不同知识领域里牵出的伦理问题。",
      en: "How to think alongside AI without handing over your own judgment — and the ethical questions this raises across different areas of knowledge.",
    },
    topics: [
      { zh: "和 AI 协作,但自己拿主意", en: "Collaborating with AI while keeping your own judgment" },
      { zh: "五大知识领域里的伦理问题", en: "The ethical dimension across the five areas of knowledge" },
    ],
  },
  {
    title: { zh: "元认知与反身", en: "Metacognition & reflection" },
    blurb: {
      zh: "把镜头转回自己:我这套推理是怎么来的,我有没有被自己或别人绕进去。",
      en: "Turning the lens back on yourself: where did this reasoning come from, and did I get fooled — by someone else, or by myself.",
    },
    topics: [
      { zh: "审视自己的推理过程", en: "Examining your own reasoning" },
      { zh: "认知者的视角:我有没有被骗", en: "The knower's perspective: did I get fooled" },
    ],
  },
  {
    title: { zh: "AI 与产品", en: "Building with AI" },
    blurb: {
      zh: "从一个真实的问题出发,和 AI 一起动手,把它做成一个能用的原型或小产品。",
      en: "Starting from a real problem, working with AI hands-on, and turning it into a working prototype or small product.",
    },
    topics: [
      { zh: "计算思维入门", en: "An introduction to computational thinking" },
      { zh: "和 AI 一起动手做东西", en: "Building things together with AI" },
      { zh: "从一个真实问题到一个能用的原型", en: "From a real problem to a working prototype" },
    ],
  },
];

/* ---- Personalized plan ---------------------------------------------------- */
export const personalizedPlan: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  points: { title: Bilingual; body: Bilingual }[];
} = {
  eyebrow: { zh: "怎么安排给孩子", en: "How we plan it for your child" },
  title: {
    zh: "每个孩子,我们都设计一套自己的方案。",
    en: "We design a plan for each child, individually.",
  },
  body: [
    {
      zh: "每个孩子擅长的地方不一样、感兴趣的方向不一样、现在所在的起点也不一样。我们不会让所有孩子按同一个顺序上完六个模块。",
      en: "Every child has different strengths, different interests, and a different starting point. We don't run every child through the six modules in the same fixed order.",
    },
    {
      zh: "第一次交流,我们会先了解孩子:平时喜欢琢磨什么问题、在学校里觉得吃力或觉得有意思的是什么、家长希望孩子在哪方面变强。根据这些,我们从孩子本身出发,挑出最合适的模块和课,搭一套开始的方案。",
      en: "In our first conversation, we get to know your child: what questions they like to chew on, what feels hard or exciting at school, and what you'd like them to grow stronger at. From there, we start from the child and pick the modules and lessons that fit, and put together a plan to begin with.",
    },
    {
      zh: "方案不是定死的。孩子在课上的表现、说的话、追问的方向,都会让我们看到新的东西,然后调整接下来该学什么、该往哪个模块走。",
      en: "The plan isn't fixed. How a child shows up in class — what they say, what they push back on — tells us something new each time, and we adjust what comes next and which module to move into.",
    },
  ],
  points: [
    {
      title: { zh: "从孩子出发", en: "Starting from the child" },
      body: {
        zh: "先了解孩子的兴趣和起点,再决定先上哪个模块,让方案跟着孩子走。",
        en: "We understand the child's interests and starting point first, then decide which module to begin with — not follow one fixed sequence.",
      },
    },
    {
      title: { zh: "随成长调整", en: "Adjusting as they grow" },
      body: {
        zh: "每一段时间,我们都会看看孩子的变化,把接下来的模块和课重新配一遍。",
        en: "At regular intervals, we look at how the child has changed and re-plan which modules and lessons come next.",
      },
    },
    {
      title: { zh: "跨年龄都能学", en: "For a wide age range" },
      body: {
        zh: "不管孩子年纪大小、有没有申请这类学校的打算,只要认同这样的学习方式,都可以来上课。",
        en: "Whatever your child's age, and whether or not you're aiming for a school like Astra Nova, if this way of learning resonates, they're welcome to join.",
      },
    },
  ],
};

/* ---- Closing CTA ------------------------------------------------------- */
export const closingCta: { title: Bilingual; sub: Bilingual; ctaLabel: Bilingual; ctaHref: string } = {
  title: { zh: "想了解课程,留下联系方式。", en: "Interested? Leave your contact." },
  sub: {
    zh: "告诉我们孩子的大致情况,我们会尽快联系你,一起聊聊适合的模块和方案。",
    en: "Tell us a bit about your child, and we'll get back to you soon to talk through the modules and plan that fit.",
  },
  ctaLabel: { zh: "留下联系方式", en: "Leave your contact" },
  ctaHref: "/contact",
};
