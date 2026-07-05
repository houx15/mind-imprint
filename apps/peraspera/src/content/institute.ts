// Per Aspera — institute page (/institute, /en/institute) content.
//
// Two source docs, both authoritative in different ways:
//   1. docs/astranova/PerAspera官网文案v2-多页版.md §三「研究院页 /institute」
//      — the marketing copy (page hero, Mind Imprint one-liner, four-step
//      how-it-works, three design red lines, demo CTA, origin story; public
//      research intro + two long-read previews + short stance).
//   2. apps/site/src/components/pages/EvaluationPage.astro — the real
//      cognitive model behind Mind Imprint (packages/contracts). Where the v2
//      copy is imprecise, this file defers to the real product:
//        - "9 维 rubric" in v2 is corrected to the real ten dimensions
//          (D1–D10), grouped under two faces (生成式驾驭 / 批判式防护) and
//          four categories.
//        - SOLO has four levels, L1–L4, each anchored by a real student
//          answer — matches v2.
//        - Evaluation always runs on the strongest model available (never
//          downgraded), and results are private to the student — offered
//          quietly, never pushed (no badges/streaks/auto-open). v2's red
//          line #3 ("评估对家长透明") is kept, but reworded so it doesn't
//          contradict student-only privacy: what's transparent to parents is
//          the *rubric* (real answer anchors per level), not any individual
//          student's own results.
//
// zh is the source of truth; en is an idiomatic (not literal) translation.
// No "不是…而是" / "not X but Y" antithesis anywhere below.

import type { Bilingual } from "./site";

/* ---- Page hero -------------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "研究院", en: "Institute" },
  title: {
    zh: "教育的下半场，是测量「怎么想」。",
    en: "The second half of education is measuring how you think.",
  },
  sub: {
    zh: "我们在探索 AI 时代的两件底层工具：思维能力的评估，和思维能力的提升。所有研究公开。",
    en: "We're building two foundational tools for the AI era: a way to assess thinking ability, and a way to grow it. All of our research is public.",
  },
};

/* ---- Mind Imprint (板块 1) --------------------------------------------------- */
export const mindImprintIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "旗舰项目 · 已有 Demo", en: "Flagship project · demo live" },
  title: { zh: "思维印记", en: "Mind Imprint" },
};

export const oneLiner: { headline: Bilingual; body: Bilingual } = {
  headline: {
    zh: "世界上第一份「怎么想」的成绩单。",
    en: "The world's first report card for how you think.",
  },
  body: {
    zh: "学校测「知不知道」，思维印记测「怎么想」——用孩子与 AI 协作的真实过程，把思维质量变成一份看得见、也会成长的记录。",
    en: "Schools measure what you know. Mind Imprint measures how you think — turning the real, lived process of a student working with AI into a record of thinking quality, one that a student can actually watch grow.",
  },
};

export interface HowItWorksStep {
  step: string;
  title: Bilingual;
  desc: Bilingual;
}

export const howItWorksIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "它如何工作", en: "How it works" },
  title: { zh: "四步，从一次对话到一枚印记。", en: "Four steps, from one conversation to one imprint." },
};

export const howItWorks: HowItWorksStep[] = [
  {
    step: "01",
    title: { zh: "真实对话", en: "Real dialogue" },
    desc: {
      zh: "学生与 AI 陪练围绕一个真实问题展开对话——AI 只负责追问，绝不替学生下结论。",
      en: "The student and an AI coach talk through a real problem. The AI's job is only to ask follow-up questions — it never concludes anything on the student's behalf.",
    },
  },
  {
    step: "02",
    title: { zh: "过程抓取", en: "Process capture" },
    desc: {
      zh: "提问质量、信源意识、视角切换、被追问后的反应……整场协作都被记录成一条过程轨迹。",
      en: "Prompt quality, source awareness, how perspectives shift, how the student responds to pushback — the whole collaboration becomes a recorded process trail.",
    },
  },
  {
    step: "03",
    title: { zh: "多维评估", en: "Multi-dimensional evaluation" },
    desc: {
      zh: "评估永远交给最强的模型，沿着「生成式驾驭」与「批判式防护」两面、四个类别、十个维度打分；每个维度再对应 SOLO 四个成长阶段（L1 萌芽到 L4 卓越），每一级都配一份真实学生答案作参照。",
      en: "The evaluation always runs on the strongest model available. It scores across two faces — generative drive and critical guard — four categories, and ten dimensions. Each dimension maps onto the four SOLO growth stages, from L1 Emerging to L4 Advanced, and every level is anchored by a real student answer.",
    },
  },
  {
    step: "04",
    title: { zh: "思维印记", en: "The Mind Imprint" },
    desc: {
      zh: "结果呈现为这一程「怎么想」的完整印记——十个维度各自的成长阶段，配上一段具体的过程叙述；它安静地生成，由学生自己决定什么时候打开。",
      en: "The result takes shape as a complete imprint of how that session went — a growth stage for each of the ten dimensions, plus a written narrative of the process. It's generated quietly and stays for the student to open whenever they choose.",
    },
  },
];

export interface RedLine {
  title: Bilingual;
  desc: Bilingual;
}

export const redLinesIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "设计红线", en: "Design red lines" },
  title: { zh: "这是信任的核心。", en: "This is where the trust lives." },
};

export const redLines: RedLine[] = [
  {
    title: { zh: "AI 绝不替孩子下结论", en: "The AI never concludes for the child" },
    desc: {
      zh: "它的角色是陪练：只追问、只反馈，思考的结论永远由学生自己给出。",
      en: "Its role is a coach — it only asks questions and reflects back what it sees. The conclusion always comes from the student.",
    },
  },
  {
    title: { zh: "奖励思考本身，不奖励停留时长", en: "It rewards thinking itself, not time on screen" },
    desc: {
      zh: "没有连胜、没有排行榜、没有推送提醒——我们不做任何让人上瘾的机制。",
      en: "No streaks, no leaderboards, no push notifications — nothing here is built to keep a student hooked.",
    },
  },
  {
    title: { zh: "评估标准对家长透明", en: "The rubric is transparent to parents" },
    desc: {
      zh: "每个等级都配一份真实学生答案样例，家长能看懂「L3 熟练」具体是什么样子；每一份具体的评估结果，仍然只留给学生自己查看。",
      en: "Every level comes with a real student-answer example, so parents can see exactly what \"L3 Proficient\" looks like. Each student's own results stay private, visible only to that student.",
    },
  },
];

export const demoCta: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual;
  label: Bilingual;
  href: string;
} = {
  eyebrow: { zh: "体验", en: "Try it" },
  title: { zh: "看一遍思维印记怎么用。", en: "See Mind Imprint in action." },
  body: {
    zh: "目前采用预约演示——这样既能保护产品，也帮我们找到最认同这套方法的家庭。",
    en: "Demos are currently by appointment — it protects the product, and helps us find the families who believe in this approach.",
  },
  label: { zh: "预约演示", en: "Book a demo" },
  href: "/apply",
};

export const originIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "它从哪来", en: "Where it comes from" },
  title: { zh: "先有真实的课，再有评估的尺。", en: "A real classroom first, then a ruler to measure it." },
};

export const origin: Bilingual = {
  zh: "思维印记源于多年 IB / TOK 真实课堂：威尼斯过度旅游案例、化石能源资金链溯源、气候纪录片信息手法拆解……它同时是 Per Aspera 全部课程的底层引擎——冲刺营的周报、学院的学期报告，都由它生成。",
  en: "Mind Imprint grew out of years of real IB/TOK classrooms — the Venice overtourism case, tracing the money behind fossil-fuel financing, unpacking the persuasion tactics in climate documentaries, and more. It's also the engine underneath every Per Aspera program: the Sprint's weekly reports and the Academy's term reports are both generated by it.",
};

/* ---- Public research (板块 2) ------------------------------------------------ */
export const researchIntro: {
  eyebrow: Bilingual;
  title: Bilingual;
  body: Bilingual;
} = {
  eyebrow: { zh: "公开研究", en: "Public research" },
  title: {
    zh: "看得越透，越不会迷信；不迷信，才选得对。",
    en: "The clearer the view, the less room for myth — and clear judgment starts there.",
  },
  body: {
    zh: "我们把对全球创新教育的研究全部公开——包括别人不敢写的部分。",
    en: "We publish all of our research into global innovative education, including the parts other people are reluctant to write.",
  },
};

export interface ResearchCard {
  title: Bilingual;
  blurb: Bilingual;
  href: string;
}

export const researchCards: ResearchCard[] = [
  {
    title: { zh: "Ad Astra / Astra Nova 完全解读", en: "The Full Ad Astra / Astra Nova Story" },
    blurb: {
      zh: "「Musk 的学校」其实是三所学校——从 SpaceX 园区里的 Ad Astra，到面向全球招生的在线学校 Astra Nova，再到德州 Bastrop 的新校区。招生标准、真实的录取与被拒案例、宣传与现实之间的落差，我们全部摊开，都带来源。",
      en: "\"Musk's school\" is actually three schools — from Ad Astra on the SpaceX campus, to the online, globally enrolled Astra Nova, to the new campus in Bastrop, Texas. We lay out the admissions standards, real acceptance and rejection cases, and the gap between the marketing and the reality, every claim sourced.",
    },
    href: "/institute/ad-astra",
  },
  {
    title: { zh: "全球同类学校版图", en: "The Global Landscape of Similar Schools" },
    blurb: {
      zh: "Alpha School、Synthesis、Khan Lab School、Minerva、Sora、Nueva、Acton……我们把每一所学校招什么人、学费多少、怎么教、争议在哪，摆在同一张表里，方便直接对照。",
      en: "Alpha School, Synthesis, Khan Lab School, Minerva, Sora, Nueva, Acton, and more — we put who each one admits, what it costs, how it teaches, and where the controversy lies side by side, so they're easy to compare directly.",
    },
    href: "/institute/schools",
  },
];

export const stanceIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "我们的立场", en: "Our stance" },
  title: { zh: "为什么研究它们，也为什么保持清醒。", en: "Why we study them, and why we stay clear-eyed doing it." },
};

export const stance: Bilingual = {
  zh: "生源筛选不等于教学法有效——一位被 Ad Astra 拒绝的家长说得最透：“当年一起研究这所学校的家长群体，本身就保证了这些孩子会有好出路。”我们向这些学校学习的是它们的内核：真问题、真思辨、真协作；至于名人效应、招生稀缺性与被过度渲染的故事，我们如实写出来，把判断交还给你自己。",
  en: "Selective admissions is not the same thing as an effective teaching method — one parent rejected by Ad Astra put it best: “the community of parents who researched that school together was, on its own, enough to guarantee those kids a good outcome.” What we take from these schools is their core: real problems, real reasoning, real collaboration. As for the celebrity effect, the scarcity, and the stories that get overhyped, we write those down honestly too, and leave the judgment to you.",
};
