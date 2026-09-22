import type { ShowcaseConfig, ShowcaseIllustration, ShowcaseStyle } from "./showcaseTypes";

export interface ShowcasePreset {
  id: ShowcaseStyle;
  name: string;
  description: string;
  config: Partial<ShowcaseConfig>;
}

export const SHOWCASE_PRESETS: readonly ShowcasePreset[] = [
  { id: "minimal", name: "留白展厅", description: "大字欢迎页、克制线条与清晰作品列表", config: { style: "minimal", palette: "paper", font: "sans", layout: "folio", illustration: "none", aboutLayout: "classic", portfolioLayout: "list" } },
  { id: "cute", name: "云朵手帐", description: "云朵背景、柔和色调与清晰文字", config: { style: "cute", palette: "rose", font: "sans", layout: "studio", illustration: "clouds" } },
  { id: "dark", name: "月夜档案", description: "深色界面、微光线条与月亮插画", config: { style: "dark", palette: "night", font: "sans", layout: "journal", illustration: "moon" } },
  { id: "anime", name: "晴空漫画", description: "全屏晴空、轻盈文字与漫画配色", config: { style: "anime", palette: "ocean", font: "sans", layout: "folio", illustration: "sky" } },
  { id: "mecha", name: "机械控制台", description: "技术标记、切角面板与机器人插画", config: { style: "mecha", palette: "night", font: "sans", layout: "studio", illustration: "robot" } },
];

export const SHOWCASE_ILLUSTRATIONS: ReadonlyArray<{ id: ShowcaseIllustration; name: string; src?: string }> = [
  { id: "none", name: "无插画" },
  { id: "clouds", name: "云朵", src: "/images/showcase/clouds.webp" },
  { id: "moon", name: "月亮", src: "/images/showcase/moon.webp" },
  { id: "sky", name: "天空", src: "/images/showcase/sky.webp" },
  { id: "robot", name: "机器人", src: "/images/showcase/robot.webp" },
];
