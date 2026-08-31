import { styleById } from "./homepage";
import { fieldById } from "./tree";
import { READINGS, STUDENT, WRITINGS } from "./library";
import { motiveOf } from "./cards";
import { ARTIFACTS } from "./artifacts";
import type { HomepageSection, Project, StyleId } from "./types";

/**
 * 她做出来的那个网站 — content.
 *
 * ## Two rules this file exists to hold
 *
 * **① It is HER site, not our product.** Nothing on this page may be written in
 * the app's voice. No 「我做过的」/「我写的」 rubric over every block, no
 * 「为什么做它：」 field label, no footer sentence explaining our pedagogy. Real
 * personal sites are written by a person to strangers, and a person does not
 * label her own paragraphs. The section names are the ones people actually use
 * — 项目 · 文章 · 关于 · 在读 — and each appears **once**.
 *
 * **② Every post gets a written line, never a truncated one.** Slicing 44
 * characters off the body and adding 「…」 is how a scraper presents someone
 * else's writing. A blurb is a sentence the author chose.
 *
 * ## Where the content comes from
 * Precedence, most-hers first: what she rewrote in 内容草稿 (`hp-content`
 * blocks — 印记 promised those ship 一个字不动) → what she typed in the studio
 * → the mock below. Projects, posts and reading come from real state.
 */

export interface SiteTheme {
  paper: string;
  ink: string;
  accent: string;
  font: string;
}

export interface SiteProject {
  id: string;
  year: string;
  kind: string;
  title: string;
  /** One or two sentences, in her voice, with the reason folded into the
   *  prose. On a real site the reason IS the description. */
  blurb: string;
  /** The two-colour wash that stands in for a photograph. */
  plate: [string, string];
}

export interface SitePost {
  id: string;
  date: string;
  title: string;
  blurb: string;
  /** 分类 — the meta row every blog post list has. */
  kind: string;
  words: number;
  tags: string[];
}

export interface SiteRead {
  id: string;
  title: string;
  source: string;
  takeaway: string;
}

export interface SiteContent {
  name: string;
  handle: string;
  domain: string;
  /** The one-line status. A real convention (the /now page), not a widget. */
  now: string;
  /** The sentence the whole page is built around. */
  headline: string;
  lead: string;
  /** 一句话身份, under the name. Every reference site has this line. */
  role: string;
  /** The spaced-out line under a masthead: 算法 · 工程 · 产品 · 日常. */
  motto: string[];
  /** 站点信息 / colophon rows — label, value. */
  stats: { label: string; value: string }[];
  /** 标签云. */
  tags: string[];
  projects: SiteProject[];
  posts: SitePost[];
  reads: SiteRead[];
  about: string[];
  detail: string;
  nowList: string[];
  email: string;
  updated: string;
}

/* ── the mock ─────────────────────────────────────────────────────────── */

const BASE: SiteContent = {
  name: STUDENT.name,
  handle: STUDENT.handle,
  domain: "zhiyao.me",
  now: "在给小区花园画一张不会迷路的地图",
  role: "初二学生 · 在拆东西 · 在写为什么它们修不好",
  motto: ["拆解", "修理", "设计", "日常"],
  stats: [
    { label: "建站", value: "2026 年 3 月" },
    { label: "写了", value: "4 篇" },
    { label: "做了", value: "3 件" },
    { label: "更新", value: "大约两周一次" },
  ],
  tags: ["修理权", "拆解", "设计", "例外与代表性", "让步段", "螺丝"],
  headline: "一件还能修的东西，是谁决定它该被扔的？",
  lead: "我十四岁，在拆家里所有还能拆的东西。这一页放我做过的、写过的，和我最近在想的问题。",
  projects: [
    {
      id: "s-pill",
      year: "2026",
      kind: "做的东西",
      title: "让爷爷看得清的药盒",
      blurb:
        "他上周吃错了一次药。我做了三版：第一版好看但他看不清，第二版装不下他的药，第三版用红胶带分格，最丑，但他现在自己会拿。",
      plate: ["#6E4F8E", "#33224A"],
    },
    {
      id: "s-garden",
      year: "2026",
      kind: "在做",
      title: "让人在花园里不迷路",
      blurb:
        "外婆在自己住了九年的小区里迷过一次路。十四个路口、一张手画的地图、一批贴在树上的编号，还差物业那一关。",
      plate: ["#3F7E5C", "#1C4432"],
    },
    {
      id: "s-lamp",
      year: "2025",
      kind: "拆解",
      title: "一盏修不好的台灯",
      blurb:
        "拆到第七颗螺丝，才看见那块板子是焊死的。售后说整只换，或者扔。这一页上所有东西都是从那天开始的。",
      plate: ["#4A4E60", "#22242E"],
    },
  ],
  posts: [],
  reads: [],
  about: [
    "我十四岁，读初二。三年前家里一盏台灯坏了，我拆开看，里面是一块焊死的电路板，售后说只能整只换。那天我第一次意识到，「修不好」有时候不是坏得太狠，是被设计成这样的。",
    "从那以后我做的东西基本都在回答同一个问题：一件还能用的东西，是谁决定它该被扔的。我做过一个我爷爷能看清的药盒，写过一篇关于修理权的短文，现在在给我们小区的花园画一张地图。",
    "我留着每一个我拆开过的东西的螺丝，装在一个铁饼干盒里，现在有 213 颗。",
  ],
  detail: "我留着每一个我拆开过的东西的螺丝，装在一个铁饼干盒里，现在有 213 颗。",
  nowList: [
    "在数花园里的路口，数到第十四个。",
    "在读一篇关于一条河的报道，治理了十一年，只补回 3%。",
    "在试第三次修那盏台灯，这次买了热风枪。",
  ],
  email: "hi@zhiyao.me",
  updated: "2026-08-31",
};

/** 🚨 Written lines, one per post. See rule ② at the top of this file. */
const POST_BLURBS: Record<string, string> = {
  "w-coral": "红海那片珊瑚没有白化，是好消息。但它只有 4 平方公里。",
  "w-three-seconds": "我试着在别人说完之后数三下再开口，试了一周。",
  "w-repair": "修灯的师傅看了一眼说：这个胶封死了，拆开就废了。",
  "w-letters": "十万封家书里出现最多的三个词：钱、天气、你吃了吗。",
};

/* ── builders ─────────────────────────────────────────────────────────── */

function postsFrom(): SitePost[] {
  return [...WRITINGS]
    // 🚨 Newest first. A blog listing out of date order reads as broken before
    // a visitor has read a single word of it.
    .sort((a, b) => (a.date < b.date ? 1 : -1))
    .slice(0, 4)
    .map((w) => ({
      id: w.id,
      date: w.date,
      title: w.title,
      blurb: POST_BLURBS[w.id] ?? "",
      kind: fieldById(w.field).label,
      words: w.words,
      tags: w.keywords,
    }));
}

function readsFrom(): SiteRead[] {
  return READINGS.filter((r) => r.takeaway)
    .slice(0, 3)
    .map((r) => ({
      id: r.id,
      title: r.title,
      source: r.source,
      takeaway: r.takeaway ?? "",
    }));
}

/** Muted two-colour washes. Deliberately NOT the app's cover gradients with a
 *  glyph on them — a tile with an icon in the middle is app furniture, and it
 *  is the single thing that made this page read as a product screen. */
const PLATES: [string, string][] = [
  ["#6E4F8E", "#33224A"],
  ["#3F7E5C", "#1C4432"],
  ["#4A4E60", "#22242E"],
  ["#A2603C", "#4E2A18"],
  ["#3C6480", "#1B3242"],
  ["#8A5566", "#42222E"],
];

function projectsFrom(projects: Project[]): SiteProject[] {
  // 🚨 The site does not list itself. The 个人主页 project's deliverable IS this
  // page, so a card for it points a visitor at the page they are reading.
  const done = projects.filter((p) => p.phase === "published" && p.track !== "website");
  if (done.length === 0) return BASE.projects;
  return done.map((p, i) => {
    const why = motiveOf(p)?.cost ?? "";
    const summary = (p.summary ?? p.intent).trim();
    return {
      id: p.id,
      year: p.startedAt.slice(0, 4),
      kind: p.phase === "published" ? "做的东西" : "在做",
      title: p.title,
      // The reason leads, because that is how a person introduces their own
      // work — never as a labelled field underneath it.
      blurb: why ? `${why}${summary}` : summary,
      plate: PLATES[i % PLATES.length] ?? PLATES[0]!,
    };
  });
}

/** Her rewrites of 印记's draft, keyed by block id. */
function draftBlocks(projects: Project[]): Record<string, string> {
  const spec = ARTIFACTS["hp-content"];
  const out: Record<string, string> = {};
  if (!spec?.blocks) return out;
  const site = projects.find((p) => p.artifacts["hp-content"]);
  const edits = site?.artifacts["hp-content"]?.blocks ?? {};
  for (const b of spec.blocks) {
    const v = (edits[b.id] ?? "").trim();
    if (v) out[b.id] = v;
  }
  return out;
}

export function buildSite(args: {
  projects: Project[];
  sections?: HomepageSection[];
}): SiteContent {
  const { projects, sections = [] } = args;
  const blocks = draftBlocks(projects);
  const studio = (id: string) =>
    (sections.find((s) => s.id === id && s.enabled)?.value ?? "").trim();
  const pick = (blockId: string, sectionId: string, fallback: string) =>
    blocks[blockId] || studio(sectionId) || fallback;

  const detail = pick("detail", "detail", BASE.detail);
  return {
    ...BASE,
    headline: pick("question", "question", BASE.headline),
    lead: pick("intro", "intro", BASE.lead),
    detail,
    // The odd detail is the last thing she says about herself, so it is the
    // last paragraph of 关于 — not a boxed callout with a label on it.
    about: [...BASE.about.slice(0, 2), detail],
    email: studio("contact") || BASE.email,
    projects: projectsFrom(projects),
    posts: postsFrom(),
    reads: readsFrom(),
  };
}

/* ── theme ────────────────────────────────────────────────────────────── */

const FALLBACK: SiteTheme = {
  paper: "#F7F3EA",
  ink: "#2A2724",
  accent: "#A6402C",
  font: '"Noto Serif SC","Songti SC",Georgia,serif',
};

/**
 * Which of the three 方案 she took. The id matters as much as the colours: the
 * three options describe three genuinely different pages (a long empty essay,
 * a dense index, a work-first spread), so `layoutOf` picks the layout and the
 * palette together. Swapping only the colours would make her decision — the
 * one she had to write a reason for — change nothing anyone can see.
 */
export function siteStyle(
  projects: Project[],
  style: StyleId | null,
): { layout: "essay" | "ledger" | "magazine"; theme: SiteTheme } {
  for (const p of projects) {
    const choice = p.artifacts["hp-style"]?.choice;
    const opt = ARTIFACTS["hp-style"]?.options?.find((o) => o.id === choice);
    if (opt) {
      return {
        layout: opt.id === "B" ? "ledger" : opt.id === "C" ? "magazine" : "essay",
        theme: {
          paper: opt.paper,
          ink: opt.ink,
          accent: opt.accent,
          font: opt.font ?? FALLBACK.font,
        },
      };
    }
  }
  if (style) {
    const s = styleById(style);
    return {
      layout: s.headline === "mono-caps" ? "ledger" : s.headline === "sans-tight" ? "magazine" : "essay",
      theme: { paper: s.paper, ink: s.ink, accent: s.accent, font: s.font },
    };
  }
  return { layout: "essay", theme: FALLBACK };
}
