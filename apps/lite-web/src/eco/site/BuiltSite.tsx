import { Essay } from "./Essay";
import { Ledger } from "./Ledger";
import { Magazine } from "./Magazine";
import type { SiteContent, SiteTheme } from "../data/site";

/**
 * 她的网站 — the thing the 个人主页 project actually builds.
 *
 * ## Three layouts, because she chose between three pages
 * `hp-style` offers 一句话开场 / 索引式 / 一个作品打头 and describes each in three
 * bullets — no navigation, or a dense index, or the work first. The first
 * version of this component rendered ONE layout and swapped the palette, which
 * made the decision she had to write a reason for invisible. Now the choice
 * picks the page:
 *
 *   A → `Essay`     长页、无导航、很大的一句话（参考 lixiaolai.com）
 *   B → `Ledger`    一张密表，等宽字
 *   C → `Magazine`  头图 + 卡片 + 侧栏（参考 keyork / terrifyzhao 那类经典博客）
 *
 * ## What a personal site has that a product screen does not
 * Six real ones, read side by side. Every one carries: an identity block —
 * avatar or wordmark, name, ONE line of who this is, a contact link; section
 * names people actually use (文章 / 项目 / 标签 / 归档 / 关于); a post list with
 * a real meta row (date · 分类 · 字数); a 站点信息 block; and a footer that says
 * © name · built with X and nothing more. None of them label their own
 * paragraphs, and none of them look like the tool that made them.
 *
 * ## One component, three places
 * The build step's preview, `/eco/p/:handle`, and the 我的主页 tab all render
 * this. A preview that is a different component is a lie waiting to happen.
 *
 * `narrow` is a prop rather than a media query: the preview renders this in a
 * 390px frame inside a 1400px window, and a `md:` breakpoint would read the
 * real viewport and lay the phone preview out as a desktop — wrong in exactly
 * the moment she is checking it.
 */
export function BuiltSite({
  site,
  theme,
  layout,
  stage = 3,
  narrow = false,
}: {
  site: SiteContent;
  theme: SiteTheme;
  layout: "essay" | "ledger" | "magazine";
  /** 印记's build rounds, made visible: at 1 the headline really is too big on
   *  a phone, and the project photographs really are missing. */
  stage?: 1 | 2 | 3;
  narrow?: boolean;
}) {
  const props = { site, theme, stage, narrow };
  if (layout === "ledger") return <Ledger {...props} />;
  if (layout === "magazine") return <Magazine {...props} />;
  return <Essay {...props} />;
}
