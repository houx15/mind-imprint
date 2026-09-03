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

export interface SiteContent {
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
