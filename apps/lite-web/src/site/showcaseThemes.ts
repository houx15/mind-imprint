import type { ShowcaseFont, ShowcasePalette } from "./showcaseTypes";

export interface ShowcaseTheme {
  paper: string;
  ink: string;
  muted: string;
  accent: string;
  accentInk: string;
  wash: string;
  line: string;
}

export const SHOWCASE_THEMES: Record<ShowcasePalette, ShowcaseTheme> = {
  paper: { paper: "#f3eee3", ink: "#26231f", muted: "#665f55", accent: "#ca4a2b", accentInk: "#fff9ef", wash: "#e7ddcb", line: "#c9bcaa" },
  forest: { paper: "#e8ede3", ink: "#17352b", muted: "#486057", accent: "#e46f45", accentInk: "#fff8ef", wash: "#cddbc7", line: "#adc0ad" },
  ocean: { paper: "#e6f1f1", ink: "#123640", muted: "#45636c", accent: "#ef6b4d", accentInk: "#fff8f2", wash: "#c7e0e1", line: "#a5c7ca" },
  rose: { paper: "#f6e9e9", ink: "#442830", muted: "#74535c", accent: "#a72d52", accentInk: "#fff7f8", wash: "#ead0d3", line: "#d6b4ba" },
  night: { paper: "#141c2d", ink: "#f2ead8", muted: "#a9aec0", accent: "#f4bd52", accentInk: "#1c2030", wash: "#202c43", line: "#3d4960" },
  sunshine: { paper: "#ffe861", ink: "#252113", muted: "#62572f", accent: "#2753d7", accentInk: "#ffffff", wash: "#fff3a0", line: "#bcae54" },
};

export const SHOWCASE_FONT_STACKS: Record<ShowcaseFont, string> = {
  sans: '"Avenir Next","PingFang SC","Microsoft YaHei",sans-serif',
  serif: '"Iowan Old Style","Songti SC","Noto Serif SC",Georgia,serif',
  mono: '"IBM Plex Mono","SFMono-Regular","PingFang SC",monospace',
  rounded: '"Showcase Happy","PingFang SC","Microsoft YaHei",sans-serif',
  handwritten: '"Showcase Hand","Kaiti SC","STKaiti",cursive',
  display: '"Showcase Tech","PingFang SC","Microsoft YaHei",sans-serif',
};
