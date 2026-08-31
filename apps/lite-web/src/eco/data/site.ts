import { styleById } from "./homepage";
import { READINGS, STUDENT, WRITINGS } from "./library";
import { motiveOf } from "./cards";
import { trackById } from "./projects";
import { ARTIFACTS } from "./artifacts";
import type { Cover, HomepageSection, Project, StyleId } from "./types";

/**
 * 她做出来的那个网站.
 *
 * ## Why this file exists
 * The 个人主页 project spends seven steps building a personal website and, up
 * to now, the thing it built was four lines of text inside a fake browser
 * frame. A project whose deliverable is never actually seen teaches that the
 * deliverable does not matter — and this one is the artifact she sends to a
 * person. So the site is modelled properly here and rendered by ONE component
 * (`site/BuiltSite.tsx`) in all three places it shows up: the build step's
 * preview, the published URL `/eco/p/:handle`, and the 我的主页 tab.
 *
 * ## Where the content comes from
 * Precedence, most-hers first:
 *   ① what she rewrote in 内容草稿 (`hp-content` blocks) — 印记 promised those
 *      would ship 一个字不动, and this is where that promise is kept;
 *   ② what she typed in the 我的主页 studio sections, if she used it;
 *   ③ the mock below.
 * Projects, posts and the reading list come from real state, so a site with
 * three finished projects on it is a site she actually filled.
 *
 * 🚨 Nothing here is lorem ipsum and nothing is a placeholder in disguise. A
 * mock that reads as filler cannot tell anyone whether the layout works.
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
  line: string;
  /** 为什么做它 — the line that makes this a person's site instead of a CV. */
  why: string;
  cover: Cover;
}

export interface SitePost {
  id: string;
  date: string;
  title: string;
  line: string;
  spine: string;
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
  /** The status line at the top — a personal site's most-read sentence. */
  now: string;
  headline: string;
  lead: string;
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
  now: "在给小区花园做一张不会让人迷路的地图",
  headline: "一件还能修的东西，是谁决定它该被扔的？",
  lead: "我在拆家里所有还能拆的东西。这一页放我做过的、写过的，还有我最近在想的问题。",
  projects: [
    {
      id: "s-pill",
      year: "2026",
      kind: "产品设计",
      title: "让爷爷看得清的药盒",
      line: "做了三版。最后一版最丑，但他现在自己会拿。",
      why: "他上周吃错了一次药。",
      cover: { art: 4, glyph: "◇" },
    },
    {
      id: "s-garden",
      year: "2026",
      kind: "在做",
      title: "让人在花园里不迷路",
      line: "十四个路口、一张手画的地图、一批贴在树上的编号。",
      why: "外婆在自己住了九年的小区里迷过一次路。",
      cover: { art: 1, glyph: "◎" },
    },
    {
      id: "s-lamp",
      year: "2025",
      kind: "拆解",
      title: "一盏修不好的台灯",
      line: "拆到第七颗螺丝才发现那块板子是焊死的。整只换，或者扔。",
      why: "我想知道「修不好」是坏了，还是被设计成这样。",
      cover: { art: 7, glyph: "✳" },
    },
  ],
  posts: [],
  reads: [],
  about: [
    "我十四岁，读初二。三年前家里一盏台灯坏了，我拆开看，里面是一块焊死的电路板，售后说只能整只换。那天我第一次意识到，「修不好」有时候不是坏得太狠，是被设计成这样的。",
    "从那以后我做的东西基本都在回答同一个问题：一件还能用的东西，是谁决定它该被扔的。我做过一个我爷爷能看清的药盒，写过一篇关于修理权的短文，现在在给我们小区的花园画一张地图。",
  ],
  detail: "我留着每一个我拆开过的东西的螺丝，装在一个铁饼干盒里，现在有 213 颗。",
  nowList: [
    "在数花园里的路口，数到第十四个。",
    "在读《修了十一年，只补了 3%》，关于一条河。",
    "在试第三次修那盏台灯，这次买了热风枪。",
  ],
  email: "hi@zhiyao.me",
  updated: "2026-08-31",
};

/* ── builders ─────────────────────────────────────────────────────────── */

function postsFrom(): SitePost[] {
  return WRITINGS.slice(0, 4).map((w) => ({
    id: w.id,
    date: w.date,
    title: w.title,
    line: (w.body[0] ?? "").slice(0, 44),
    spine: w.spine,
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

function projectsFrom(projects: Project[]): SiteProject[] {
  // 🚨 The site does not list itself. The 个人主页 project's deliverable IS this
  // page, so putting it in 作品 gives a visitor a card that links to the page
  // they are already reading.
  const done = projects.filter((p) => p.phase === "published" && p.track !== "website");
  if (done.length === 0) return BASE.projects;
  return done.map((p) => {
    const first = (p.summary ?? "").split("。")[0];
    return {
      id: p.id,
      year: p.startedAt.slice(0, 4),
      kind: trackById(p.track).label,
      title: p.title,
      line: first ? `${first}。` : p.intent,
      why: motiveOf(p)?.cost ?? motiveOf(p)?.who ?? "",
      cover: p.cover,
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

  return {
    ...BASE,
    headline: pick("question", "question", BASE.headline),
    lead: pick("intro", "intro", BASE.lead),
    detail: pick("detail", "detail", BASE.detail),
    email: studio("contact") || BASE.email,
    projects: projectsFrom(projects),
    posts: postsFrom(),
    reads: readsFrom(),
  };
}

/* ── theme ────────────────────────────────────────────────────────────── */

const FALLBACK: SiteTheme = {
  paper: "#FBF8F4",
  ink: "#33302E",
  accent: "#EA5140",
  font: '"Noto Serif SC","Songti SC",Georgia,serif',
};

/**
 * The look is HERS: whichever 方案 she took in 三个方案 wins, because that
 * choice cost her a written reason and the site is where the reason pays off.
 * The studio's style is the fallback for a page built the old way.
 */
export function siteTheme(projects: Project[], style: StyleId | null): SiteTheme {
  for (const p of projects) {
    const st = p.artifacts["hp-style"];
    const opt = ARTIFACTS["hp-style"]?.options?.find((o) => o.id === st?.choice);
    if (opt) {
      return {
        paper: opt.paper,
        ink: opt.ink,
        accent: opt.accent,
        font: opt.font ?? FALLBACK.font,
      };
    }
  }
  if (style) {
    const s = styleById(style);
    return { paper: s.paper, ink: s.ink, accent: s.accent, font: s.font };
  }
  return FALLBACK;
}
