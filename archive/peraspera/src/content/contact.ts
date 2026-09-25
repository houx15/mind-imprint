// Per Aspera — /contact page (联系我们) content. Also the single source for
// LeadForm's field labels + interest options, since the lead form lives here
// (the old apply-page funnel is gone — this page just says: leave your
// contact info, and we'll get in touch).
//
// zh is the source of truth; en is an idiomatic (not literal) translation.
// We do not run info sessions, so that wording is banned here; likewise no
// jargon for "leaving contact info" — say it plainly. No "不是…而是" /
// "not X but Y" antithesis.

import type { Bilingual } from "./site";

/* ---- Page hero -------------------------------------------------------------- */
export const hero: { eyebrow: Bilingual; title: Bilingual; sub: Bilingual } = {
  eyebrow: { zh: "联系我们", en: "Contact us" },
  title: { zh: "留下联系方式，我们会联系你。", en: "Leave your contact info, and we'll reach out." },
  sub: {
    zh: "不管你是想了解申请辅导、课程项目，还是想聊学校合作，写几句话告诉我们，我们看到后会尽快联系你。",
    en: "Whether you're curious about application coaching, our courses, or a partnership for your school, tell us a bit below — we'll get back to you soon.",
  },
};

/* ---- Form ------------------------------------------------------------------- */
export interface InterestOption {
  value: "coaching" | "academy" | "partnership" | "explore";
  label: Bilingual;
}

export const interestOptions: InterestOption[] = [
  { value: "coaching", label: { zh: "申请辅导", en: "Application coaching" } },
  { value: "academy", label: { zh: "课程项目", en: "Courses" } },
  { value: "partnership", label: { zh: "学校合作", en: "School partnership" } },
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
    placeholder: { zh: "例如 14（如果不涉及可以不填）", en: "e.g. 14 (skip if not relevant)" },
  },
  interest: {
    label: { zh: "你想聊聊哪方面？", en: "What would you like to talk about?" },
    placeholder: { zh: "请选择", en: "Please choose" },
  },
  message: {
    label: { zh: "想说的话", en: "Anything you'd like to tell us" },
    placeholder: {
      zh: "孩子情况、想问的问题、方便联系的时间……都可以写在这里",
      en: "Your child, your questions, a good time to reach you — anything is fine here",
    },
  },
  optional: { zh: "选填", en: "Optional" },
  required: { zh: "必填", en: "Required" },
  submit: { zh: "联系我们", en: "Contact us" },
  submitting: { zh: "提交中…", en: "Submitting…" },
  success: {
    zh: "我们收到了，会尽快与你联系。",
    en: "Thanks — we'll be in touch soon.",
  },
  error: {
    zh: "提交遇到点问题，请稍后再试，或直接通过下方邮箱联系我们。",
    en: "Something went wrong — please try again, or email us directly.",
  },
};

/* ---- Direct contact fallback ------------------------------------------------- */
export const directContact: { title: Bilingual; email: string; wechat: Bilingual } = {
  title: { zh: "或直接联系我们", en: "Or reach us directly" },
  email: "hello@peraspera.org",
  wechat: { zh: "微信：peraspera_edu", en: "WeChat: peraspera_edu" },
};
