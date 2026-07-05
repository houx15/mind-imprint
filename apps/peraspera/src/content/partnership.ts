// Per Aspera — 合作 (partnership) page (/partnership, /en/partnership) content.
// zh is the source of truth; en is an idiomatic (not literal) translation.
//
// This page is for schools, not parents. It introduces two kinds of work we
// do with a school, then introduces our AI-evaluation product, 思维印记
// (mind imprint), accurately — matching how it describes itself on
// apps/site (EvaluationPage.astro / HomePage.astro): an AI tool (like an
// agent workspace) with two special layers, interactive 思维工具卡
// (thinking tool-cards) and 过程评估 (process evaluation); it evaluates the
// thinking process, not the finished work; the cognitive model splits
// "using AI well" into two faces (生成式驾驭 / 批判式防护) across ten
// dimensions; growth is read on a four-level SOLO scale (L1–L4); the
// evaluation is private to the student and offered, never pushed.
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
    zh: "如果一所学校也认同这样的学习方式——重视孩子怎么想问题,而不只是给出什么答案——我们可以陪学校一起,把 AI 踏踏实实地带进日常教学里。",
    en: "If a school shares this vision of learning — caring how a child thinks, not only what answer they hand in — we work alongside the school to bring AI into everyday teaching in a grounded, thoughtful way.",
  },
};

/* ---- Two kinds of solutions --------------------------------------------- */
export const solutionsIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "我们能提供什么", en: "What we offer" },
  title: { zh: "两种合作方式。", en: "Two ways we work with schools." },
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
      zh: "这不是买几个账号、装一套软件那么简单。我们从课程、工具、老师三个方面,帮学校把 AI 真正用进日常教学里。",
      en: "This isn't just handing out a few accounts or installing some software. We work on the courses, the tools, and the teachers together, so AI genuinely becomes part of everyday teaching.",
    },
    items: [
      {
        title: { zh: "AI 课程设计", en: "AI course design" },
        body: {
          zh: "根据学校的学生和培养目标,设计一套教孩子怎么和 AI 一起想问题、怎么守住自己判断力的课程。",
          en: "We design a course, tailored to the school's students and goals, that teaches children how to think alongside AI while keeping their own judgment.",
        },
      },
      {
        title: { zh: "思维印记：AI 评估工具", en: "Mind imprint: an AI evaluation tool" },
        body: {
          zh: "把我们自己的 AI 评估产品「思维印记」带给学校,持续看到学生真实的思考能力是怎么成长的。（详见下文）",
          en: "We bring our own AI evaluation product, mind imprint, to the school, so it can keep track of how students' real thinking ability is growing. (More below.)",
        },
      },
      {
        title: { zh: "教师培训", en: "Teacher training" },
        body: {
          zh: "带老师们熟悉 AI 时代的教学方式——怎么设计课堂讨论、怎么看懂学生和 AI 协作时留下的痕迹,而不只是学一遍工具怎么用。",
          en: "We help teachers get comfortable with AI-era teaching — how to design classroom discussion and read what a student's work with AI actually shows — not just how to click through a tool.",
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

/* ---- Introduce 思维印记 (mind imprint) ------------------------------------ */
export const miIntro: { eyebrow: Bilingual; title: Bilingual; body: Bilingual[] } = {
  eyebrow: { zh: "思维印记", en: "Mind imprint" },
  title: {
    zh: "一款只评估思考过程的 AI 工具。",
    en: "An AI tool that evaluates only the thinking process.",
  },
  body: [
    {
      zh: "思维印记是一个 AI 工具,形态有点像一个带 AI 助理的工作台。学生带着自己真实的任务进来——一篇作文、一个课题、一份报告——在和 AI 协作的过程中,被引导做更结构化的思考;这整个过程会被完整记录下来,再被评估。",
      en: "Mind imprint is an AI tool that works something like a workspace with an AI assistant built in. A student brings in a real task — an essay, a project, a report — and while working with the AI, is guided toward more structured thinking. The whole process is recorded, and then evaluated.",
    },
    {
      zh: "它有两层独有的能力。第一层是互动的「思维工具卡」——在合适的时刻,AI 会请学生停下来,自己先想一步。第二层是「过程评估」——评估看的是学生这一路是怎么和 AI 一起把问题想清楚的:有没有去核实一个说法的来源,撞到一个反例时是绕开还是认真接住,哪些地方是自己的判断、哪些地方是直接照搬 AI 的话。",
      en: "It has two special layers. The first is interactive thinking tool-cards — at the right moment, the AI asks the student to pause and think for themselves. The second is process evaluation — it looks at how the student actually worked through the problem with AI: whether they checked where a claim came from, whether they faced a counter-example head-on or dodged it, and which parts were the student's own judgment versus copied straight from the AI.",
    },
    {
      zh: "一篇漂亮的作文,可能是 AI 一键生成的,也可能是学生认真想了很久的,单看成品,分数分不出这两者的差别。过程能看出这个差别——这也是为什么它对学校有用:它能补上一份成绩单看不到的那部分,让老师和学校真正看清楚,AI 时代的孩子,思考能力到底在不在长。",
      en: "A polished essay could be one AI click, or it could be hours of real thought — looking at the finished piece alone, a grade can't tell the two apart. The process shows the difference. That's exactly why it's useful for a school: it fills in the part a report card alone can't show, so teachers and the school can actually see whether a child's thinking ability is genuinely growing in the AI era.",
    },
  ],
};

export interface Dimension {
  code: string;
  name: Bilingual;
}

export interface Face {
  face: "drive" | "guard";
  title: Bilingual;
  sub: Bilingual;
  dims: Dimension[];
}

export const miModel: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual; faces: Face[] } = {
  eyebrow: { zh: "认知模型", en: "The cognitive model" },
  title: { zh: "两面、十个维度。", en: "Two faces, ten dimensions." },
  sub: {
    zh: "我们把「善用 AI」拆成一枚硬币的两面——主动驾驭 AI,与批判地守住自己的判断——再往下分成十个具体维度,逐一评估。",
    en: "We split \"using AI well\" into two sides of one coin — actively driving AI, and critically guarding your own judgment — and evaluate ten concrete dimensions underneath.",
  },
  faces: [
    {
      face: "drive",
      title: { zh: "生成式驾驭", en: "Generative drive" },
      sub: {
        zh: "主动驾驭 AI,把它变成放大思考的工具",
        en: "Actively driving AI — making it a tool that amplifies thinking",
      },
      dims: [
        { code: "D1", name: { zh: "提问清晰度", en: "Prompt clarity" } },
        { code: "D10", name: { zh: "协作编排", en: "Collaborative orchestration" } },
        { code: "D4", name: { zh: "多视角与让步", en: "Perspective & concession" } },
        { code: "D5", name: { zh: "论证拆解", en: "Argument analysis" } },
        { code: "D7", name: { zh: "论证质量", en: "Argument quality" } },
      ],
    },
    {
      face: "guard",
      title: { zh: "批判式防护", en: "Critical guard" },
      sub: {
        zh: "不被 AI 俘获,守住判断与诚信",
        en: "Not getting captured by AI — guarding judgment and integrity",
      },
      dims: [
        { code: "D2", name: { zh: "信源辨识", en: "Source discernment" } },
        { code: "D3", name: { zh: "横向验证", en: "Lateral verification" } },
        { code: "D6", name: { zh: "反思与元认知", en: "Reflection & metacognition" } },
        { code: "D8", name: { zh: "信息再生产", en: "Information reproduction" } },
        { code: "D9", name: { zh: "AI 边界与伦理", en: "AI limits & ethics" } },
      ],
    },
  ],
};

export interface SoloLevel {
  code: string;
  name: Bilingual;
  body: Bilingual;
}

export const miSolo: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual; levels: SoloLevel[] } = {
  eyebrow: { zh: "四级成长量表", en: "The four growth levels" },
  title: { zh: "每个维度，看到四个成长阶段。", en: "Each dimension, across four stages of growth." },
  sub: {
    zh: "这四个阶段来自 SOLO 分类学,衡量的是理解的深浅——从零散的单点,一直到能融会贯通、迁移应用,和「对错」无关。",
    en: "The four stages come from the SOLO taxonomy. They measure how deep a student's understanding goes — from scattered single points to connected, transferable mastery — not right or wrong.",
  },
  levels: [
    { code: "L1", name: { zh: "萌芽", en: "Emerging" }, body: { zh: "只做到一个孤立的点,还没有联系起来。", en: "One isolated point, not yet connected to anything else." } },
    { code: "L2", name: { zh: "发展中", en: "Developing" }, body: { zh: "做到了几个点,但还没有连成一片。", en: "Several points, but not yet joined up." } },
    { code: "L3", name: { zh: "熟练", en: "Proficient" }, body: { zh: "把这些要素连成了结构,能主动运用。", en: "The pieces are linked into a working structure the student applies actively." } },
    { code: "L4", name: { zh: "卓越", en: "Advanced" }, body: { zh: "能迁移、能反思,超出了当下这一次任务。", en: "The student transfers and reflects on it, going beyond this one task." } },
  ],
};

export interface Principle {
  title: Bilingual;
  body: Bilingual;
}

export const miPrinciples: { eyebrow: Bilingual; title: Bilingual; items: Principle[] } = {
  eyebrow: { zh: "评估的原则", en: "Principles behind the evaluation" },
  title: { zh: "评估本身,也守着几件坚持。", en: "The evaluation keeps its own set of principles." },
  items: [
    {
      title: { zh: "只给学生本人看", en: "Private to the student" },
      body: {
        zh: "思维印记是一面私密的成长镜子,只照给学生自己看,不是拿来排名或比较的报告。",
        en: "The mind imprint is a private mirror for growth, seen only by the student — never a report card for ranking or comparison.",
      },
    },
    {
      title: { zh: "评估永远用最强的模型", en: "Always the strongest model" },
      body: {
        zh: "日常陪练可以用更轻量的模型,但一到评估这一步,我们始终用最强的那一个,保证判断可靠。",
        en: "Everyday coaching can run on a lighter model, but for the evaluation itself, we always use the very best one, so the judgment holds up.",
      },
    },
    {
      title: { zh: "安静地提供,不会推着学生看", en: "Offered, never pushed" },
      body: {
        zh: "结果安静地出现——没有徽章、不会自动弹开、也没有排行榜,什么时候点开来看,由学生自己决定。",
        en: "Results appear quietly — no badges, no auto-open, no leaderboard. The student decides if and when to look.",
      },
    },
  ],
};

export const miWhySchool: Bilingual = {
  zh: "对一所学校来说,这套评估补上了一块一直很难看到的东西:学生在 AI 时代,思考能力到底是不是在真实地成长。它把过程变成可以看见、可以长期追踪的数据,给老师的判断提供更多依据,也帮学校更早发现哪些孩子已经在善用 AI、哪些孩子还需要多带一带。",
  en: "For a school, this evaluation fills in something that's always been hard to see: whether a student's thinking ability is genuinely growing in the AI era. It turns the process into something visible and trackable over time, giving teachers more to go on and helping a school notice earlier which students are already using AI well, and which ones need more guidance.",
};

/* ---- Closing CTA ---------------------------------------------------------- */
export const closingCta: { title: Bilingual; sub: Bilingual; ctaLabel: Bilingual; ctaHref: string } = {
  title: { zh: "想了解合作，留下联系方式。", en: "Interested in partnering? Leave your contact." },
  sub: {
    zh: "告诉我们学校的大致情况和想合作的方向,我们会尽快联系你。",
    en: "Tell us a bit about your school and what kind of partnership you have in mind — we'll get back to you soon.",
  },
  ctaLabel: { zh: "留下联系方式", en: "Leave your contact" },
  ctaHref: "/contact",
};
