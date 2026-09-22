import type { ShowcaseConfig, ShowcaseIllustration, ShowcaseStyle } from "./showcaseTypes";

export interface ShowcasePreset {
  id: ShowcaseStyle;
  name: string;
  description: string;
  config: Partial<ShowcaseConfig>;
}

export const SHOWCASE_PRESETS: readonly ShowcasePreset[] = [
  { id: "cute", name: "云朵手帐", description: "圆润字形、柔和卡片与云朵插画", config: { style: "cute", palette: "rose", font: "rounded", layout: "studio", illustration: "clouds" } },
  { id: "dark", name: "月夜档案", description: "深色界面、微光线条与月亮插画", config: { style: "dark", palette: "night", font: "serif", layout: "journal", illustration: "moon" } },
  { id: "anime", name: "晴空漫画", description: "鲜明描边、网点层次与天空插画", config: { style: "anime", palette: "ocean", font: "handwritten", layout: "folio", illustration: "sky" } },
  { id: "mecha", name: "机械控制台", description: "技术标记、切角面板与机器人插画", config: { style: "mecha", palette: "night", font: "display", layout: "studio", illustration: "robot" } },
];

export const SHOWCASE_ILLUSTRATIONS: ReadonlyArray<{ id: ShowcaseIllustration; name: string; src?: string }> = [
  { id: "none", name: "无插画" },
  { id: "clouds", name: "云朵", src: "/images/showcase/clouds.png" },
  { id: "moon", name: "月亮", src: "/images/showcase/moon.png" },
  { id: "sky", name: "天空", src: "/images/showcase/sky.png" },
  { id: "robot", name: "机器人", src: "/images/showcase/robot.png" },
];

