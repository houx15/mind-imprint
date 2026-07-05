// Per Aspera — home page (/) content. zh is the source of truth; en mirrors it.
// Bilingual scope for this page: Hero, the belief cards (shown zh + en together),
// and the footer disclaimer. Pillars, FAQ and the closing CTA are language-
// switched (one language per page) via `t()`.
//
// Copy is drawn verbatim from docs/astranova/PerAspera官网文案v2-多页版.md §一,
// with two deliberate edits:
//   1. Belief 4's headline was "驾驭 AI，而不是被 AI 驯化。" (an X-not-Y antithesis).
//      Rewritten to a positive declarative per the site copy rule.
//   2. The closing CTA is reframed from the doc's "2-min video + parent letter"
//      application onto our funnel: 留资 → 说明会 → 一对一面谈.

import type { Bilingual } from "./site";

/* ---- Hero ------------------------------------------------------------------ */
export const hero: {
  motto: string;
  headline: Bilingual;
  /** the accented fragment inside the headline, wrapped in .em */
  headlineEm: Bilingual;
  sub: Bilingual;
  ctaPrimary: { label: Bilingual; href: string };
  ctaSecondary: { label: Bilingual; href: string };
} = {
  motto: "Per Aspera",
  headline: {
    zh: "培养 AI 时代的真正驾驭者。",
    en: "Raising the true masters of the AI era.",
  },
  headlineEm: { zh: "驾驭者", en: "masters" },
  sub: {
    zh: "当 AI 能秒答一切，我们训练它替代不了的东西：思辨、协作、伦理判断，和把问题变成作品的能力。",
    en: "When AI can answer everything, we train what it can't replace: reasoning, collaboration, ethical judgment — and the ability to turn problems into products.",
  },
  ctaPrimary: {
    label: { zh: "查看课程", en: "Explore programs" },
    href: "/programs",
  },
  ctaSecondary: {
    label: { zh: "走进研究院", en: "Enter the institute" },
    href: "/institute",
  },
};

/* ---- Three pillars --------------------------------------------------------- */
export const pillarsIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "概览", en: "Overview" },
  title: {
    zh: "一所学院，一个研究院，一套公开的理念。",
    en: "An academy, an institute, and a point of view we publish in full.",
  },
};

export interface Pillar {
  title: Bilingual;
  body: Bilingual;
  cta: Bilingual;
  href: string;
}

export const pillars: Pillar[] = [
  {
    title: { zh: "课程 · 学院", en: "Programs · Academy" },
    body: {
      zh: "两个产品：面向 10.15 Astra Nova 高中申请的 14 周冲刺营；面向 9–14 岁的长线学院——亚洲时区的问题解决者教育。",
      en: "Two products: a 14-week sprint for the Oct 15 Astra Nova high-school application, and a long-term academy for ages 9–14 — problem-solver education built for Asian time zones.",
    },
    cta: { zh: "查看课程", en: "Explore programs" },
    href: "/programs",
  },
  {
    title: { zh: "研究院 · 思维印记", en: "Institute · Mind Imprint" },
    body: {
      zh: "我们不只教，还在造工具：AI 时代的思维能力评估体系——用孩子与 AI 的真实对话，量化“怎么想”。Demo 已上线。",
      en: "We teach, and we build the tools: an assessment system for thinking in the AI era, reading how a child reasons from their real conversations with AI. The demo is live.",
    },
    cta: { zh: "走进研究院", en: "Enter the institute" },
    href: "/institute",
  },
  {
    title: { zh: "理念 · 为什么是我们", en: "Beliefs · Why us" },
    body: {
      zh: "两位创始人：多年 IB/国际课堂批判性思维教学 × 连续创业者的 AI/产品/计算思维。我们把研究全部公开。",
      en: "Two founders: years of teaching critical thinking in IB and international classrooms, crossed with a serial founder's fluency in AI, product, and computational thinking. We publish all of our research.",
    },
    cta: { zh: "了解我们", en: "Meet the founders" },
    href: "/about",
  },
];

/* ---- Beliefs (always shown zh + en together) ------------------------------- */
export const beliefsIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "我们相信什么", en: "What we believe" },
  title: {
    zh: "四条不肯让步的信念。",
    en: "Four convictions we won't trade away.",
  },
};

export interface Belief {
  /** short thematic tag beside the star mark */
  tag: Bilingual;
  headZh: string;
  headEn: string;
  bodyZh?: string;
  bodyEn?: string;
}

export const beliefs: Belief[] = [
  {
    tag: { zh: "思考", en: "Thinking" },
    headZh: "在 AI 能秒答一切的时代，“怎么想”是唯一值钱的能力。",
    headEn: "In an age where AI answers everything, how you think is the only thing left worth learning.",
  },
  {
    tag: { zh: "能力", en: "Ability" },
    headZh: "能力无法代办。",
    headEn: "Ability cannot be outsourced.",
    bodyZh: "没有人能替孩子完成一次真实的思考——所以我们不背题、不代写、不写脚本。",
    bodyEn: "No one can think on your child's behalf — so we don't drill answers, ghostwrite letters, or script videos.",
  },
  {
    tag: { zh: "家庭", en: "Family" },
    headZh: "家长是教育的一半。",
    headEn: "Parents are half of the education.",
    bodyZh: "好学校面试的是整个家庭；好教育发生在饭桌上。",
    bodyEn: "Great schools interview the whole family; great education happens at the dinner table.",
  },
  {
    tag: { zh: "主动权", en: "Agency" },
    headZh: "驾驭 AI，握住主动权。",
    headEn: "Master AI, and keep the upper hand.",
    bodyZh: "会用 AI 的孩子很多，敢对 AI 说“你错了”的孩子很少——我们培养后者。",
    bodyEn: "Many kids can use AI; few dare tell it “you're wrong.” We raise the few.",
  },
];

/* ---- Featured FAQ (full set lives on /about) ------------------------------- */
export const faqIntro: { eyebrow: Bilingual; title: Bilingual; more: Bilingual } = {
  eyebrow: { zh: "常见问题", en: "FAQ" },
  title: { zh: "先答几个高频问题。", en: "A few questions, up front." },
  more: { zh: "完整问答见关于页", en: "See the full FAQ on About" },
};

export interface Faq {
  q: Bilingual;
  a: Bilingual;
}

export const faqs: Faq[] = [
  {
    q: { zh: "Per Aspera 与 Astra Nova 是什么关系？", en: "How is Per Aspera related to Astra Nova?" },
    a: {
      zh: "没有任何关系，我们是独立机构（详见关于页）。",
      en: "In no way. We are an independent organization (see the About page for the full statement).",
    },
  },
  {
    q: { zh: "你们能保证录取吗？", en: "Can you guarantee admission?" },
    a: {
      zh: "不能，任何人都不能。我们承诺能力提升，成功费仅录取后收取。",
      en: "No — and no one can. What we promise is real growth in ability; the success fee is charged only after an offer.",
    },
  },
  {
    q: { zh: "孩子几岁可以来？", en: "What ages do you work with?" },
    a: {
      zh: "冲刺营 13–17 岁；长线学院 9–14 岁；家长工作坊不限。",
      en: "The sprint is for ages 13–17; the long-term academy for 9–14; the parent workshops have no age limit.",
    },
  },
];

/* ---- Closing CTA (reframed onto 留资 → 说明会 → 一对一面谈) ------------------- */
export const closing: {
  motto: string;
  title: Bilingual;
  body: Bilingual;
  cta: Bilingual;
  href: string;
} = {
  motto: "Per Aspera",
  title: { zh: "先来一次说明会。", en: "Start with an info session." },
  body: {
    zh: "留下联系方式，我们约你参加线上说明会，再做一对一面谈——看看我们是否合适同行。",
    en: "Leave your contact and we'll invite you to an online info session, then a one-on-one conversation — to see if we're a good fit.",
  },
  cta: { zh: "预约说明会", en: "Book an info session" },
  href: "/apply",
};
