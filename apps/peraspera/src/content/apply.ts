// Per Aspera — apply page (/apply, /en/apply) content.
//
// IMPORTANT framing: this is a lead-capture / info-session booking page, NOT
// Astra Nova's own application. Our funnel is 留资 → 说明会 → 一对一面谈
// (leave contact info → info session → one-on-one interview). We do not ask
// for a video, a parent letter, essays, or any upload here — that belongs to
// the family's eventual Astra Nova application, described on /programs, not
// to us. This page only collects contact info + interest so we can follow up
// and schedule a session.
//
// zh is the source of truth; en is an idiomatic (not literal) translation.
// No "不是…而是" / "not X but Y" antithesis anywhere below.

import type { Bilingual } from "./site";

/* ---- Page hero -------------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "预约说明会", en: "Book an info session" },
  title: {
    zh: "先聊一次，看看合不合适。",
    en: "Let's talk first, and see if we're a good fit.",
  },
  sub: {
    zh: "留下联系方式，我们会安排一次线上说明会，讲清楚冲刺营和学院的内容、时间与筛选方式；如果双方都觉得合适，再约一次一对一面谈。",
    en: "Leave your contact info and we'll set up an online info session to walk you through the Sprint and the Academy — the content, the schedule, how we screen families. If it feels like a fit on both sides, we'll follow up with a one-on-one interview.",
  },
};

/* ---- Funnel: 留资 → 说明会 → 一对一面谈 -------------------------------------- */
export interface FunnelStep {
  title: Bilingual;
  body: Bilingual;
}

export const funnelIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "流程", en: "How it works" },
  title: { zh: "三步，从留资到面谈。", en: "Three steps, from a form to a conversation." },
};

export const funnel: FunnelStep[] = [
  {
    title: { zh: "留资", en: "Leave your contact" },
    body: {
      zh: "填写下面这张表，告诉我们孩子的年龄和你最想了解的方向。",
      en: "Fill out the form below and tell us your child's age and what you'd most like to know.",
    },
  },
  {
    title: { zh: "说明会", en: "Info session" },
    body: {
      zh: "我们安排一次线上说明会，讲清楚课程内容、时间安排与筛选方式，你可以随时提问。",
      en: "We'll set up an online info session covering the program content, schedule, and how we screen families — with room for your questions throughout.",
    },
  },
  {
    title: { zh: "一对一面谈", en: "One-on-one interview" },
    body: {
      zh: "说明会后如果双方都觉得合适，我们再约一次一对一面谈，认识孩子和家庭，确认入营。",
      en: "If it still feels right after the info session, we'll schedule a one-on-one interview to get to know your child and your family, and confirm enrollment.",
    },
  },
];

/* ---- Form ------------------------------------------------------------------- */
export const formIntro: { eyebrow: Bilingual; title: Bilingual } = {
  eyebrow: { zh: "第一步", en: "Step one" },
  title: { zh: "填一张表，剩下的交给我们。", en: "Fill in one form. We'll take it from there." },
};

export interface InterestOption {
  value: "sprint" | "academy" | "explore";
  label: Bilingual;
}

export const interestOptions: InterestOption[] = [
  { value: "sprint", label: { zh: "冲刺营", en: "The Sprint" } },
  { value: "academy", label: { zh: "长线学院", en: "The Academy" } },
  { value: "explore", label: { zh: "先了解一下", en: "Just exploring" } },
];

export const form: {
  contactName: { label: Bilingual; placeholder: Bilingual };
  contact: { label: Bilingual; placeholder: Bilingual };
  childAge: { label: Bilingual; placeholder: Bilingual };
  interest: { label: Bilingual; placeholder: Bilingual };
  message: { label: Bilingual; placeholder: Bilingual };
  optional: Bilingual;
  required: Bilingual;
  submit: Bilingual;
  submitting: Bilingual;
  success: Bilingual;
  error: Bilingual;
} = {
  contactName: {
    label: { zh: "家长/孩子称呼", en: "Parent or child's name" },
    placeholder: { zh: "怎么称呼你", en: "What should we call you?" },
  },
  contact: {
    label: { zh: "联系方式（微信 / 手机 / 邮箱）", en: "Contact info (WeChat, phone, or email)" },
    placeholder: { zh: "微信号 / 手机号 / 邮箱", en: "WeChat ID, phone number, or email" },
  },
  childAge: {
    label: { zh: "孩子年龄", en: "Child's age" },
    placeholder: { zh: "例如 14", en: "e.g. 14" },
  },
  interest: {
    label: { zh: "你最想了解哪个方向？", en: "Which program interests you most?" },
    placeholder: { zh: "请选择", en: "Please choose" },
  },
  message: {
    label: { zh: "想问的问题", en: "Anything you'd like to ask" },
    placeholder: {
      zh: "关于时间、费用、筛选……都可以先写在这里",
      en: "Schedule, fees, screening — anything is fair game here",
    },
  },
  optional: { zh: "选填", en: "Optional" },
  required: { zh: "必填", en: "Required" },
  submit: { zh: "预约说明会 →", en: "Book an info session →" },
  submitting: { zh: "提交中…", en: "Submitting…" },
  success: {
    zh: "我们收到啦，会尽快联系你安排说明会。",
    en: "We've got it — we'll reach out soon to arrange your info session.",
  },
  error: {
    zh: "提交出了点问题，请稍后再试，或直接联系我们。",
    en: "Something went wrong — please try again shortly, or contact us directly.",
  },
};
