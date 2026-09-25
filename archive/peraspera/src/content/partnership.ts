// Per Aspera — 合作 (partnership) page (/partnership, /en/partnership) content.
// zh is the source of truth; en is an idiomatic (not literal) translation.
//
// This page is for schools. It moves 理念 → 方案 → details of the 方案:
//   理念   — why a school should bring AI into teaching thoughtfully (ANL).
//   方案   — two ways we help: build the school's AI teaching system, and run
//            AI winter/summer camps.
//   details — three components of "build the AI teaching system": a tight
//            summary of the 思维印记 evaluation product (with a link to its own
//            product page /mind-imprint), the AI-native course system, and
//            teacher training.
//
// The full 思维印记 deep-dive — the dark interface mockup, the two-faces /
// ten-dimension cognitive model, the SOLO L1–L4 levels, and the assessment
// principles — lives on its own product page at /mind-imprint (content in
// mind-imprint.ts). This page only summarizes it and links across, so it
// stays focused on the school story.
//
// Plain, warm, concrete wording — no jargon, no deadlines, no info sessions
// (说明会), no antithesis ("不是…而是" / not-X-but-Y).

import type { Bilingual } from "./site";

/* ---- Page hero --------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "合作", en: "Partnership" },
  title: {
    zh: "为学校提供 AI 时代的转型方案。",
    en: "AI-native transformation solutions for schools.",
  },
  sub: {
    zh: "如果一所学校也认同这样的学习方式——重视孩子怎么想问题,也重视他给出什么答案——我们可以陪学校一起,把 AI 踏踏实实地带进日常教学里。",
    en: "If a school shares this vision of learning — caring how a child thinks as much as what answer they hand in — we work alongside the school to bring AI into everyday teaching in a grounded, thoughtful way.",
  },
};

/* ---- 1 · 理念 (our philosophy) ------------------------------------------ */
export const philosophy: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  beliefs: { title: Bilingual; body: Bilingual }[];
} = {
  eyebrow: { zh: "理念", en: "Our philosophy" },
  title: {
    zh: "把 AI 认真地带进教学,是这个时代绕不开的事。",
    en: "Bringing AI into teaching, thoughtfully, is something no school can sidestep now.",
  },
  body: [
    {
      zh: "AI 已经在孩子手里了。他们用它查资料、写初稿、找思路——问题不在于挡不挡得住,而在于学校能不能帮他们学会,用一种真正能长出思考力的方式去用它。",
      en: "AI is already in children's hands. They use it to look things up, draft, and find a way in — the question is whether school helps them learn to use it in a way that actually grows their thinking.",
    },
    {
      zh: "我们把这件事叫做「AI 原生学习」(AI-Native Learning)。在这样的学习里,孩子练的是带着 AI 一起解决真实问题的能力,同时守住自己的判断力。该被看重的东西,从「交出了什么答案」转向「他是怎么想到这里的」。",
      en: "We call this AI-Native Learning. In it, a child practices solving real problems alongside AI while keeping their own judgment — and what deserves attention shifts from the answer handed in to how the child got there.",
    },
    {
      zh: "这不是加一门课、买一套工具就能完成的转变。它关乎课堂怎么设计、老师怎么带、学生的成长怎么被看见。我们愿意陪着认同这件事的学校,一步步把它落到日常里。",
      en: "This isn't a change you finish by adding one class or buying one tool. It touches how lessons are designed, how teachers guide, and how a student's growth is seen. We walk this out, step by step, with schools who believe in it.",
    },
  ],
  beliefs: [
    {
      title: { zh: "AI 会放大思考,也会替代思考", en: "AI can amplify thinking, or replace it" },
      body: {
        zh: "同一个工具,可以让一个孩子想得更深,也可以让他停止思考。差别不在工具,在怎么用——这正是可以教、也值得教的。",
        en: "The same tool can make a child think harder or stop thinking. The difference lies in how it's used — and that can be taught, and is worth teaching.",
      },
    },
    {
      title: { zh: "判断力要在真实任务里练", en: "Judgment is built on real tasks" },
      body: {
        zh: "核实一个说法、接住一个反例、分清哪些是自己的判断——这些能力,只有在孩子做自己真正在乎的题目时,才练得出来。",
        en: "Checking a claim, facing a counter-example, telling your own judgment apart from the machine's — these grow only when a child works on a task they genuinely care about.",
      },
    },
    {
      title: { zh: "成长要被看见", en: "Growth has to be visible" },
      body: {
        zh: "AI 时代最难看清的,是孩子的思考力到底在不在长。学校需要一种方式,把这条看不见的曲线变得看得见。",
        en: "The hardest thing to see in the AI era is whether a child's thinking is really growing. A school needs a way to make that invisible curve visible.",
      },
    },
  ],
};

/* ---- 2 · 方案 overview: two kinds of solutions -------------------------- */
export const solutionsIntro: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "方案", en: "Our solution" },
  title: { zh: "两种合作方式。", en: "Two ways we work with a school." },
  sub: {
    zh: "一种是长期的——陪学校把 AI 教学体系搭起来;一种是集中的——我们为学校的学生带一期沉浸式的 AI 营。",
    en: "One is long-term — helping the school build its AI teaching system; one is concentrated — an immersive AI camp we run for the school's students.",
  },
};

export interface SolutionItem {
  title: Bilingual;
  body: Bilingual;
}

export interface Solution {
  tag: string;
  title: Bilingual;
  body: Bilingual;
  /** Sub-items shown as small cards under the body. Empty for solutions that
   *  are better described in a single paragraph (e.g. the camps). */
  items: SolutionItem[];
}

export const solutions: Solution[] = [
  {
    tag: "①",
    title: { zh: "帮学校搭建 AI 教学体系", en: "Build the school's AI teaching system" },
    body: {
      zh: "从课程、工具、老师三个方面一起入手,帮学校把 AI 真正用进日常教学里。三块内容会在下面逐一展开。",
      en: "We work on the courses, the tools, and the teachers together, so AI genuinely becomes part of everyday teaching. Each of the three is laid out below.",
    },
    items: [
      {
        title: { zh: "思维印记 · AI 评估工具", en: "The Mark of Thinking · an AI evaluation tool" },
        body: {
          zh: "我们自己的 AI 评估产品,持续看到学生真实的思考力是怎么成长的。",
          en: "Our own AI evaluation product, keeping track of how students' real thinking ability grows.",
        },
      },
      {
        title: { zh: "AI 原生课程体系", en: "AI-native course system" },
        body: {
          zh: "一套以真实思考为核心、可与学校共同设计的课程。",
          en: "A course system built around real thinking, co-designed with the school.",
        },
      },
      {
        title: { zh: "教师培训", en: "Teacher training" },
        body: {
          zh: "带老师用好 AI、上好这些课、看懂思维印记的评估。",
          en: "Helping teachers use AI well, teach these courses, and read the mind-imprint evaluations.",
        },
      },
    ],
  },
  {
    tag: "②",
    title: { zh: "AI 冬夏令营", en: "AI winter & summer camps" },
    body: {
      zh: "我们为学校的学生设计并带一期沉浸式的 AI 营——集中的几天里,孩子们围着一个真实问题,和 AI 一起动手想、动手做,练的是怎么提问、怎么核查、怎么把一个想法做成看得见的成果。营期结束时,每个孩子都能讲清楚自己是怎么想通这件事的。",
      en: "We design and run an immersive AI camp for a school's students. Over a few concentrated days, kids work through a real question together, thinking and building alongside AI — practicing how to ask good questions, check what AI tells them, and turn an idea into something real. By the end, every child can walk you through how they actually worked it out.",
    },
    items: [],
  },
];

/* ---- 3a · details: 思维印记 (the mark of thinking) — summary + link ------------ */
export const miSummary: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  pilot: Bilingual;
  linkLabel: Bilingual;
  linkHref: string;
} = {
  eyebrow: { zh: "方案详解 · 思维印记", en: "In detail · The Mark of Thinking" },
  title: {
    zh: "思维印记：一款让思考过程被看见的 AI 评估产品。",
    en: "The Mark of Thinking: an AI evaluation product that makes thinking visible.",
  },
  body: [
    {
      zh: "思维印记是我们自己的 AI 评估产品。学生带着真实的任务和 AI 协作，这一路他是怎么想的会被完整记录下来，再被评估。它看的是思考的过程；成品始终是学生自己的。",
      en: "The Mark of Thinking is our own AI evaluation product. A student brings a real task and works through it with AI, and the whole way they thought is recorded, then evaluated. It looks at the thinking process; the finished work stays the student's own.",
    },
    {
      zh: "对一所学校来说，它补上了一份成绩单看不到的那部分：AI 时代，学生的思考能力到底在不在真实地成长。它把过程变成看得见、能长期追踪的东西，给老师的判断多一份依据。",
      en: "For a school, it fills in the part a report card can't show: whether a student's thinking ability is genuinely growing in the AI era. It turns the process into something visible and trackable over time, giving teachers more to go on.",
    },
  ],
  pilot: {
    zh: "我们正开始和一些学校一起试点这套评估。",
    en: "We're beginning to pilot this evaluation together with a few schools.",
  },
  linkLabel: { zh: "完整了解思维印记", en: "Explore the product in full" },
  linkHref: "/mind-imprint",
};

/* ---- 3b · details: 课程体系 (course system) ----------------------------- */
export interface CourseModule {
  code: string;
  name: Bilingual;
  body: Bilingual;
}

export const courseSystem: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  modules: CourseModule[];
  closing: Bilingual;
} = {
  eyebrow: { zh: "方案详解 · 课程体系", en: "In detail · course system" },
  title: {
    zh: "一套 AI 原生、以真实思考为核心的课程。",
    en: "An AI-native course system built around real thinking.",
  },
  body: [
    {
      zh: "我们带给学校的,是一套围绕真实问题、以讨论驱动的课程。课上不背标准答案,而是让学生带着 AI 去调查、去辩论、去动手做——把「和 AI 一起想清楚一件事」这件本事,拆成可以一节一节练的内容。",
      en: "What we bring is a course system built around real questions and driven by discussion. Instead of memorizing set answers, students investigate, debate, and build alongside AI — turning \"thinking something through with AI\" into a skill practiced lesson by lesson.",
    },
    {
      zh: "整套课程按几个模块组织,可以按学校的年级、学生和培养目标来取用、拼接。",
      en: "The whole system is organized into a few module families, which can be drawn from and combined to fit the school's grades, students, and goals.",
    },
  ],
  modules: [
    {
      code: "01",
      name: { zh: "信息素养", en: "Information literacy" },
      body: {
        zh: "溯源、横向核查、判断可信度——在 AI 什么都能生成的年代,先学会分辨真假。",
        en: "Sourcing, lateral reading, judging credibility — learning to tell true from false in an age when AI can generate anything.",
      },
    },
    {
      code: "02",
      name: { zh: "知识工具与思辨", en: "Thinking tools & reasoning" },
      body: {
        zh: "论证怎么拆、反例怎么接、让步段怎么写——一套可以反复用的思考工具。",
        en: "How to break down an argument, take on a counter-example, write a concession — a set of thinking tools students use again and again.",
      },
    },
    {
      code: "03",
      name: { zh: "五大知识领域 (TOK)", en: "Five domains of knowing (TOK)" },
      body: {
        zh: "从不同学科看同一个问题,理解知识是怎么来的、又有哪些边界。",
        en: "Looking at one question across disciplines — understanding where knowledge comes from and where its limits lie.",
      },
    },
    {
      code: "04",
      name: { zh: "AI 协作与伦理", en: "AI collaboration & ethics" },
      body: {
        zh: "怎么和 AI 分工、怎么署名、哪些事该由自己拿主意——用得好也用得正。",
        en: "How to divide the work with AI, how to attribute it, and what to decide for yourself — using it both well and honestly.",
      },
    },
    {
      code: "05",
      name: { zh: "元认知与反身", en: "Metacognition & reflection" },
      body: {
        zh: "回头看自己是怎么想的、哪里被 AI 带着走了——把反思变成习惯。",
        en: "Looking back at how you thought, and where AI carried you along — making reflection a habit.",
      },
    },
    {
      code: "06",
      name: { zh: "AI 与产品 · 建造", en: "AI & building" },
      body: {
        zh: "计算思维、和 AI 一起动手、从一个问题做到一个原型——把想法变成看得见的东西。",
        en: "Computational thinking, building with AI, going from a problem to a prototype — turning an idea into something you can see.",
      },
    },
  ],
  closing: {
    zh: "这套课程不是一份现成的教材直接搬进课堂。我们会和学校一起,按学生的实际情况共同设计,再落到每一节课里。",
    en: "This isn't a ready-made textbook dropped into the classroom. We co-design it with the school around the students' real situation, then bring it down into each lesson.",
  },
};

/* ---- 3c · details: 教师培训 (teacher training) -------------------------- */
export interface Principle {
  title: Bilingual;
  body: Bilingual;
}

export const teacherTraining: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual[];
  items: Principle[];
  closing: Bilingual;
} = {
  eyebrow: { zh: "方案详解 · 教师培训", en: "In detail · teacher training" },
  title: {
    zh: "带着老师,一起把这些课上起来。",
    en: "Getting teachers ready to teach this.",
  },
  body: [
    {
      zh: "再好的课程和工具,最后都要靠老师带进课堂。所以教师培训是这套方案里很实的一环——它是给学校老师的一套实操计划,帮他们在 AI 时代把课上得更有底气。",
      en: "Even the best course and the best tool still reach students through a teacher. So teacher training is a very practical part of this — a hands-on program for the school's staff, helping them teach with confidence in the AI era.",
    },
  ],
  items: [
    {
      title: { zh: "在教学里用好 AI", en: "Use AI well in teaching" },
      body: {
        zh: "怎么用 AI 备课、设计讨论、给反馈,又不让它替学生把该想的都想了——老师先自己走一遍。",
        en: "How to use AI to prepare, design discussion, and give feedback, without letting it do the thinking students should do — teachers walk it themselves first.",
      },
    },
    {
      title: { zh: "带得动这些课", en: "Run these courses" },
      body: {
        zh: "以真实问题、讨论驱动的课堂,和讲授式很不一样。我们带老师练怎么抛问题、怎么接学生的回应、怎么把讨论收回来。",
        en: "A question-driven, discussion-based classroom runs very differently from a lecture. We help teachers practice posing questions, taking up student responses, and bringing a discussion back together.",
      },
    },
    {
      title: { zh: "看懂思维印记", en: "Understand The Mark of Thinking" },
      body: {
        zh: "一份过程评估摊在面前,老师能读出学生强在哪、卡在哪,再决定下一步怎么带——把评估变成教学的依据。",
        en: "With a process evaluation in front of them, teachers can read where a student is strong and where they're stuck, then decide how to guide next — turning the evaluation into something teaching can act on.",
      },
    },
  ],
  closing: {
    zh: "培训会按学校的节奏来,可以是集中的工作坊,也可以是一学期里持续的陪伴。",
    en: "The training runs at the school's pace — as a concentrated workshop, or as ongoing support across a term.",
  },
};

/* ---- 4 · Closing CTA ---------------------------------------------------- */
export const closingCta: { title: Bilingual; sub: Bilingual; ctaLabel: Bilingual; ctaHref: string } = {
  title: { zh: "想了解合作，留下联系方式。", en: "Interested in partnering? Leave your contact." },
  sub: {
    zh: "告诉我们学校的大致情况和想合作的方向,我们会尽快联系你。",
    en: "Tell us a bit about your school and what kind of partnership you have in mind — we'll get back to you soon.",
  },
  ctaLabel: { zh: "留下联系方式", en: "Leave your contact" },
  ctaHref: "/contact",
};
