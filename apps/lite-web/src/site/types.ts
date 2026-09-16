/**
 * site/types.ts — 她的主页的内容模型。
 *
 * 🚨 这些形状直接照着 apps/api/internal/pbl/site.go 的 SiteContent 抄，不是猜
 * 的。服务端负责把「她写的字」和「她真做过的事」合成一页；这一层只负责画。
 *
 * 🚨 `site/` 里任何文件都不许 import 这个 app 的 UI kit（`Btn`、`Panel`、任何
 * `mk-*` token）。一旦用了，这一页就开始长得像做出它的那个产品，而那正是一个
 * 个人网站唯一不能像的东西。颜色只从版式给的四个 `--st-*` 变量来。
 *
 * 与原型相比少了两个字段，都是故意的：
 *
 * - **没有 `handle` / `domain`。** 原型给每个人发了一个 `zhiyao.me`，印在页头和
 *   页脚上。那是这一页上最像真的一处假话。真实地址是发布后拿到的那条 /p/ 链接。
 * - **`email` 默认是空的**，不从账号邮箱带出来。她是未成年人，把登录邮箱印在
 *   公开页面上比让页面被搜到更糟。
 */

export interface SiteTheme {
  paper: string;
  ink: string;
  accent: string;
  font: string;
}

export type SiteLayout = "essay" | "ledger" | "magazine";

/**
 * 她第三关定下的配色。
 *
 * 三个颜色，不是五个：SiteTheme 只认 paper / ink / accent 三格。字体跟着版式走
 * ——生成的字体栈只会挑出一个中文缺字的字体。
 *
 * 三个颜色都空 = 她还没定，渲染端退回版式自带的那一套。
 */
export interface SitePalette {
  label: string;
  /** 它为什么配她那几个关键词。她挑的时候读到的就是这一句。 */
  why: string;
  paper: string;
  ink: string;
  accent: string;
}

export interface SiteStat {
  label: string;
  value: string;
}

export interface SiteProject {
  id: string;
  year: string;
  kind: string;
  title: string;
  /** 她自己写的那一两句。空着就是空着——截取正文再加省略号是爬虫干的事。 */
  blurb: string;
  /** 代替照片的双色版。它不假装是一张照片。 */
  plate: [string, string];
}

export interface SitePost {
  id: string;
  date: string;
  title: string;
  blurb: string;
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

export interface SiteSection { key: string; title: string; depth: number; body: string; imageKey?: string; imageUrl?: string; }

export interface SiteContent {
  sections?: SiteSection[];
  name: string;
  role: string;
  headline: string;
  lead: string;
  now: string;
  motto: string[];
  stats: SiteStat[];
  tags: string[];
  projects: SiteProject[];
  posts: SitePost[];
  reads: SiteRead[];
  about: string[];
  nowList: string[];
  email: string;
  updated: string;
  /** 画头图用的种子，由她的名字派生。见 parts.tsx 的 Banner。 */
  seed: number;
}

/** 她写的那份草稿——存进 pbl_site.content 的东西。 */
export interface SiteDraft {
  sections?: SiteSection[];
  role: string;
  headline: string;
  lead: string;
  now: string;
  motto: string[];
  tags: string[];
  about: string[];
  nowList: string[];
  contact: string;
  blurbs: Record<string, string>;
}

export const EMPTY_DRAFT: SiteDraft = {
  role: "",
  headline: "",
  lead: "",
  now: "",
  motto: [],
  tags: [],
  about: [],
  nowList: [],
  contact: "",
  blurbs: {},
};

/** 一个什么都没有的页面。她还没写字的时候预览就是这个样子——空的，不是假的。 */
export const EMPTY_CONTENT: SiteContent = {
  name: "",
  role: "",
  headline: "",
  lead: "",
  now: "",
  motto: [],
  stats: [],
  tags: [],
  projects: [],
  posts: [],
  reads: [],
  about: [],
  nowList: [],
  email: "",
  updated: "",
  seed: 0,
};

/**
 * 把服务端来的草稿收成一份「每个列表都真的是列表」的草稿。
 *
 * 🚨 Go 的 nil 切片 marshal 成 `null`。服务端现在会补成 `[]`（normalizeDraft），
 * 但这一层也补一次：一条 `draft.motto.filter(...)` 就能把整个工作面打成白屏，
 * 而这种崩溃在 jsdom 里看不见——2026-09-03 是浏览器 walk 抓到的。旧数据、别的
 * 客户端、以后某次改动，都可能再送来一个 null。
 */
export function normalizeDraft(d: Partial<SiteDraft> | null | undefined): SiteDraft {
  const list = (xs: unknown): string[] => (Array.isArray(xs) ? (xs as string[]) : []);
  return {
    role: d?.role ?? "",
    headline: d?.headline ?? "",
    lead: d?.lead ?? "",
    now: d?.now ?? "",
    motto: list(d?.motto),
    tags: list(d?.tags),
    about: list(d?.about),
    nowList: list(d?.nowList),
    contact: d?.contact ?? "",
    blurbs: d?.blurbs ?? {},
  };
}
