// Per Aspera — 课程 / Courses (academy) page (/academy, /en/academy)
// content. zh is the source of truth; en is an idiomatic (not literal)
// translation.
//
// This is the FAMILIES service (2C): our AI-assisted interactive courses, for
// families who share our way of learning. The courses run on our core product,
// 思维印记 (Mind Imprint). The library is presented as TWO THREADS — 思辨
// (critical thinking, modules 1-5) and 产品思维 (product thinking, module 6) —
// broken into SIX MODULES. Modules 1-5 are drawn from docs/03_课程库_单课设计;
// module 6 (AI 与产品) is added per the refactor plan. Plain, warm, concrete
// wording — no jargon, no deadlines, no info sessions (说明会), no antithesis,
// and no mention of time zones. Classes are live and AI-assisted, with no
// pre-recorded video lectures.

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "课程", en: "Courses" },
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
      zh: "课堂不大,六到十个孩子在线上视频里围成一圈,一起讨论、互相追问。每一节都是老师带着孩子实时上的直播小课,没有提前录好的视频课。AI 随时可以拿来查证、拓展思路,判断和结论始终留给孩子自己。",
      en: "Classes are small — six to ten kids gathered in a live video discussion, talking things through and questioning each other. Every session is taught live by a teacher in real time, with no pre-recorded video lectures. AI is on hand as a tool to check facts and widen ideas, and the judgment and the conclusion always stay with the child.",
    },
    {
      zh: "一年分三个学期,每个学期我们都会更新一批课程——跟着当下发生的事和孩子们感兴趣的方向走,不是一份用十年不变的教材。同一个模块下,孩子这学期讨论的话题,和下学期很可能完全不一样。",
      en: "We run three terms a year, and refresh part of the course list every term — following what's actually happening in the world and what kids are curious about, not a syllabus that stays the same for a decade. Within the same module, what a child discusses this term and what they discuss next term can look quite different.",
    },
    {
      zh: "带课的老师本身在这些领域里做过真事——写过时评、做过研究、参与过真实的项目,不是只会照本宣科的讲师。",
      en: "The people who teach have actually done the work in their field — writing commentary, doing research, working on real projects — not reciting from a script.",
    },
    {
      zh: "上课全程在线,每周一次,和孩子平时的学校生活并行,不用另外请假或安排接送。",
      en: "Everything runs online, once a week, alongside your child's regular school life — no extra absence, no commute.",
    },
  ],
};

/* ---- Runs on our product: 思维印记 (Mind Imprint) ----------------------- */
export const productNote: { text: Bilingual; linkLabel: Bilingual; linkHref: string } = {
  text: {
    zh: "这些课都在我们自己的 AI 产品「思维印记」上进行。孩子和 AI 协作的每一步都会被记录下来,变成一条看得见的思考过程,也是我们做过程评估的依据。",
    en: "These classes all run on our own AI product, Mind Imprint. Every step of a child's work with AI is recorded into a visible thinking process, which is also the basis for how we look at that process.",
  },
  linkLabel: { zh: "了解思维印记", en: "About Mind Imprint" },
  linkHref: "/mind-imprint",
};

/* ---- The six modules ------------------------------------------------------ */
export interface AcademyModule {
  title: Bilingual;
  blurb: Bilingual;
  topics: Bilingual[];
}

export const modulesIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "课程库", en: "The course library" },
  title: { zh: "六个模块,分成两条主线。", en: "Six modules, along two main threads." },
  sub: {
    zh: "一条是「思辨」——怎么找信息、怎么判断、怎么把一个问题想清楚,对应前五个模块;一条是「产品思维」——从一个真实问题出发,和 AI 一起动手做出点东西,对应最后一个模块。下面每个模块里都有很多具体的课,挑几个例子给你看看孩子会讨论什么。",
    en: "One thread is critical thinking (思辨) — how to find information, judge it, and think a question all the way through, covered by the first five modules. The other is product thinking (产品思维) — starting from a real problem and building something with AI, covered by the last module. Each module below holds many specific lessons; here are a few examples of what kids actually discuss.",
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

/* ---- Participation levels ------------------------------------------------ */
export interface AcademyLevel {
  name: Bilingual;
  desc: Bilingual;
  points: Bilingual[];
}

export const levelsIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "怎么上课", en: "Ways to join" },
  title: { zh: "从试一门课,到一条完整的路径。", en: "From trying one class to a full track." },
  sub: {
    zh: "不用一上来就定死。孩子和家长可以先从投入不多的方式开始,觉得合适再往深走。",
    en: "You don't have to decide everything up front. Start with a lighter commitment, and go deeper once it feels right.",
  },
};

export const levels: AcademyLevel[] = [
  {
    name: { zh: "单科体验", en: "Try a module" },
    desc: {
      zh: "先挑一个模块里的一两门课上上看,一周一次课。适合刚接触这种上课方式、想先感受一下的孩子和家长。",
      en: "Pick one or two classes from a single module and try them — once a week. A good fit if this way of learning is new to you and you want a feel for it first.",
    },
    points: [
      { zh: "一周一次课,时间投入不大", en: "One class a week — a light commitment" },
      { zh: "可以只上感兴趣的那一个模块", en: "Focus on just the one module that interests you" },
      { zh: "随时可以加课,也可以先只试这一门", en: "Add more later, or simply try this one for now" },
    ],
  },
  {
    name: { zh: "多模块组合", en: "A few modules together" },
    desc: {
      zh: "覆盖两三个模块,一周两到三次课。孩子开始在不同的思考工具之间来回练习,逐渐把它们连成一整套习惯。",
      en: "Covering two or three modules, a few classes a week. Kids start moving between different thinking tools and gradually turn them into one connected habit.",
    },
    points: [
      { zh: "一周两到三次课", en: "A few classes each week" },
      { zh: "模块之间互相呼应,不是孤立的课", en: "Modules connect to each other, not isolated lessons" },
      { zh: "适合想系统练一练、又不想排太满的孩子", en: "For kids who want it to add up to something, without over-scheduling" },
    ],
  },
  {
    name: { zh: "完整个性化路径", en: "A full, personalized track" },
    desc: {
      zh: "六个模块都会覆盖到,一周固定几次课。我们会持续跟进孩子的变化,动态调整接下来该学什么——这是最贴近我们说的“为每个孩子设计方案”的一条路径。",
      en: "All six modules, on a regular weekly schedule. We keep track of how the child is changing and adjust what comes next — this is the path that most fully lives up to what we mean by a plan designed for one child.",
    },
    points: [
      { zh: "覆盖全部六个模块", en: "Covers all six modules" },
      { zh: "固定的每周节奏,长期跟进", en: "A steady weekly rhythm, followed over time" },
      { zh: "方案随孩子的表现持续调整", en: "The plan keeps adjusting to how the child is doing" },
    ],
  },
];

/* ---- Winter / summer camps ----------------------------------------------- */
export const camps: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "另一种方式", en: "Another way in" },
  title: {
    zh: "也可以来一次寒暑假的 AI 营。",
    en: "You can also join a winter or summer AI camp.",
  },
  body: [
    {
      zh: "想先轻松地感受一下这种学习方式,寒假或暑假的 AI 营是个不错的开始。几天时间,孩子和一小群同伴集中泡在一个真实的问题里,和 AI 一起动手,从头把一件事想清楚、做出来。",
      en: "For a lighter first taste of this way of learning, a winter or summer AI camp is a good place to start. Over a few days, your child and a small group of peers dive into one real question together, working hands-on with AI to think it through and build something from scratch.",
    },
    {
      zh: "营期节奏紧凑、投入完整,几天下来,孩子就能真切地体会到平时课上那种一起讨论、一起追问的感觉。",
      en: "Camps are short and immersive, and a few days in, kids get a real feel for the kind of discussing and questioning that our regular classes are built on.",
    },
  ],
};

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

/* ---- Beyond the classroom: project showcase ----------------------------- */
export const showcase: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "走出教室", en: "Beyond the classroom" },
  title: {
    zh: "学期快结束时,孩子们会展示自己做的东西。",
    en: "Near the end of each term, kids show what they've made.",
  },
  body: [
    {
      zh: "每学期快结束时,我们会办一次线上的成果展示。孩子们轮流讲讲自己这学期琢磨透的一个问题,或者做出来的一个小项目、一个原型,说说自己是怎么想清楚的、又是怎么改过来改过去的。",
      en: "Near the end of each term, we hold an online showcase. Kids take turns talking through a question they worked through that term, or a small project or prototype they built — and how their thinking changed along the way.",
    },
    {
      zh: "家长可以旁听,看看孩子这学期真正学到的是什么样子,而不只是一张成绩单。同伴之间也会互相提问、给反馈——这本身也是一次思考的练习。",
      en: "Parents are welcome to sit in and see what a child actually learned that term — not just a grade on paper. Kids also question and give feedback to each other, which is its own kind of thinking practice.",
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
