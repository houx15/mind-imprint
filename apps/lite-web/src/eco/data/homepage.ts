import type { HomepageSection, StyleId } from "./types";

/**
 * 我的主页 — the styles and the section menu.
 *
 * ## What used to live here
 * This file was the config behind a six-step 「建一个属于你的主页」 wizard:
 * three AI working modes, a `compileInstruction` compiler, per-mode scripts,
 * a step list. All of it moved into `data/cards.ts` on 2026-08-31, because
 * the teaching in it — *明确的指令 = 一个直接的例子 + 内容的结构 + 你想怎么和
 * AI 配合* — is not about homepages. It is how you instruct an AI to build
 * anything, so it became the 明确指令 card and every track can now reach it.
 *
 * What stayed is what the published page actually needs: four looks, and the
 * menu of blocks she can put on it.
 */

export const PAGE_STYLES: {
  id: StyleId;
  label: string;
  en: string;
  blurb: string;
  paper: string;
  ink: string;
  accent: string;
  /** CSS font stack for the published page. */
  font: string;
  /** Heading treatment on the published page. */
  headline: "serif-xl" | "mono-caps" | "sans-tight" | "serif-italic";
}[] = [
  {
    id: "morning",
    label: "清晨纸感",
    en: "Morning Paper",
    blurb: "暖白纸、衬线标题、一条细线。像一封写好的信。",
    paper: "#FBF8F4",
    ink: "#33302E",
    accent: "#EA5140",
    font: '"Noto Serif SC","Songti SC",Georgia,serif',
    headline: "serif-xl",
  },
  {
    id: "terminal",
    label: "深夜终端",
    en: "Night Terminal",
    blurb: "近黑底、等宽字、绿色光标。安静、精确、有点酷。",
    paper: "#14120F",
    ink: "#E8E2D8",
    accent: "#6FBFB0",
    font: 'ui-monospace,"SF Mono","PingFang SC",monospace',
    headline: "mono-caps",
  },
  {
    id: "magazine",
    label: "杂志切页",
    en: "Magazine",
    blurb: "大标题、粗色块、强对比。适合作品多、想被一眼看到的人。",
    paper: "#FFFFFF",
    ink: "#1A1A1A",
    accent: "#E0A63A",
    font: '-apple-system,"PingFang SC","Helvetica Neue",sans-serif',
    headline: "sans-tight",
  },
  {
    id: "garden",
    label: "植物园",
    en: "Garden",
    blurb: "淡绿底、圆角、手写感的小标注。适合还在生长中的东西。",
    paper: "#F4F7F0",
    ink: "#2F3B2C",
    accent: "#5FA97E",
    font: '"Noto Serif SC","Songti SC",Georgia,serif',
    headline: "serif-italic",
  },
];

export function styleById(id: StyleId) {
  return PAGE_STYLES.find((s) => s.id === id) ?? PAGE_STYLES[0]!;
}

/** The section menu. `enabled` is her structural choice in step 2 — the
 *  ORDER and the SELECTION are the "structure of content" half of a clear
 *  instruction, so they are chosen before any writing happens. */
export const DEFAULT_SECTIONS: HomepageSection[] = [
  {
    id: "intro",
    label: "一句话介绍",
    hint: "你是谁 + 你着迷于什么。不是头衔。",
    value: "",
    enabled: true,
  },
  {
    id: "question",
    label: "我在乎的问题",
    hint: "一个你反复回来的问题。写下来它就有了地址。",
    value: "",
    enabled: true,
  },
  {
    id: "projects",
    label: "我做过的",
    hint: "从你的项目里挑。做完的和在做的都可以，标清楚就行。",
    value: "",
    picker: "projects",
    picked: [],
    enabled: true,
  },
  {
    id: "writings",
    label: "我写的",
    hint: "挑你自己最喜欢的，不一定是分最高的那篇。",
    value: "",
    picker: "writings",
    picked: [],
    enabled: true,
  },
  {
    id: "readings",
    label: "我读过的",
    hint: "读过什么，比说自己喜欢什么更可信。",
    value: "",
    picker: "readings",
    picked: [],
    enabled: true,
  },
  {
    id: "detail",
    label: "一个关于我的怪细节",
    hint: "让页面变成「你的页面」的那一件小事。",
    value: "",
    enabled: true,
  },
  {
    id: "contact",
    label: "怎么找到我",
    hint: "一个真的能收到消息的地方就够了。",
    value: "",
    enabled: false,
  },
];
