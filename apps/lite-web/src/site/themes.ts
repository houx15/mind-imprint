import type { SiteLayout, SiteTheme } from "./types";

/**
 * 三个版式，三个真正不一样的页面。
 *
 * 🚨 spec §15：「Three genuinely different layouts — not one layout in three
 * colourways. Her justified choice has to be visible.」第一版做成了一个版式换
 * 三套配色，结果是她写了理由的那个决定，在页面上看不出来。
 *
 * 所以版式和调色是一起选的：`essay` 是暖纸衬线的长页，`ledger` 是深底等宽的
 * 密表，`magazine` 是白底无衬线的经典博客。三个页面从骨架到字体都不一样。
 */
export const SITE_LAYOUTS: {
  id: SiteLayout;
  name: string;
  tag: string;
  /** 她在挑的时候读到的三句话。描述的是页面本身，不是形容词。 */
  bullets: string[];
  theme: SiteTheme;
}[] = [
  {
    id: "essay",
    name: "一句话开场",
    tag: "长页 · 无导航 · 很空",
    bullets: [
      "第一屏只有一句话：你是谁、你在想什么。",
      "往下滚是你做过的事，每件配一句你自己写的话。",
      "没有导航栏，因为只有一页。",
    ],
    theme: {
      paper: "#F4F0E6",
      ink: "#23201C",
      accent: "#9C3B26",
      font: '"Noto Serif SC","Songti SC",Georgia,serif',
    },
  },
  {
    id: "ledger",
    name: "索引式",
    tag: "密 · 像一份档案柜",
    bullets: [
      "首页是一张列表：日期 + 标题 + 一句话，一屏看到十几条。",
      "顶部一行小字说明这里是什么。",
      "密度优先，没有图。",
    ],
    theme: {
      paper: "#101112",
      ink: "#D9D6CF",
      accent: "#7FD1A6",
      font: 'ui-monospace,"SF Mono","PingFang SC",monospace',
    },
  },
  {
    id: "magazine",
    name: "一个作品打头",
    tag: "图先行 · 强对比",
    bullets: [
      "顶上是这一页自己的头图，底下是导航条。",
      "左边是文章卡片，右边是侧栏：你是谁、标签、站点信息。",
      "作品做成小图排在文章后面。",
    ],
    theme: {
      paper: "#FFFFFF",
      ink: "#111111",
      accent: "#1B44D8",
      font: '-apple-system,"PingFang SC","Helvetica Neue",sans-serif',
    },
  },
];

export function themeFor(layout: SiteLayout): SiteTheme {
  return (SITE_LAYOUTS.find((l) => l.id === layout) ?? SITE_LAYOUTS[0]!).theme;
}

export function layoutName(layout: SiteLayout): string {
  return (SITE_LAYOUTS.find((l) => l.id === layout) ?? SITE_LAYOUTS[0]!).name;
}
