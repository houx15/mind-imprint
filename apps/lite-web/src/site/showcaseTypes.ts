export type ShowcaseLayout = "folio" | "journal" | "studio";
export type ShowcasePalette = "paper" | "forest" | "ocean" | "rose" | "night" | "sunshine";
export type ShowcaseFont = "sans" | "serif" | "mono" | "rounded" | "handwritten" | "display";
export type ShowcaseStyle = "classic" | "minimal" | "cute" | "dark" | "anime" | "mecha";
export type ShowcaseIllustration = "none" | "clouds" | "moon" | "sky" | "robot";
export type ShowcaseAboutLayout = "classic" | "orbit";
export type ShowcasePortfolioLayout = "sections" | "flow" | "timeline" | "film" | "planets" | "cloud" | "calendar" | "list";
export type ShowcaseKind = "writing" | "reading" | "project";

export interface ShowcaseAboutMessage { role: "user" | "assistant"; content: string; }

export interface ShowcaseComponent {
  id: string;
  title: string;
  format: "svg" | "html";
  source: string;
  height: number;
  placement: "after-about" | "after-works";
  enabled: boolean;
}
export interface ShowcaseCustomWork { id: string; title: string; summary: string; url: string; date?: string; }

export interface ShowcaseConfig {
  components?: ShowcaseComponent[];
  customWorks?: ShowcaseCustomWork[];
  homeWorkLimit?: number;
  aboutConversation?: ShowcaseAboutMessage[];
  guideConversation?: ShowcaseAboutMessage[];
  componentPrompt?: string;
  interestTreeMode?: "none" | "tree" | "keywords";
  name: string;
  bio: string;
  tagline: string;
  interests: string[];
  layout: ShowcaseLayout;
  palette: ShowcasePalette;
  font: ShowcaseFont;
  style?: ShowcaseStyle;
  illustration?: ShowcaseIllustration;
  heroTitle?: string;
  aboutLayout?: ShowcaseAboutLayout;
  portfolioLayout?: ShowcasePortfolioLayout;
  /** Persisted object keys. Renderers receive their signed presentation URLs separately. */
  avatarKey?: string;
  heroImageKey?: string;
  /** Student-authored image prompts belong to the private draft only. */
  heroImagePrompt?: string;
  avatarImagePrompt?: string;
  writingStyle: "cards" | "list";
  readingStyle: "shelf" | "list";
  sectionOrder: ShowcaseKind[];
  selectedWorkIds: string[];
  /** Up to two selected works receive an expanded introduction on the homepage. */
  featuredWorkIds?: string[];
}

export interface ShowcaseWork {
  id: string;
  kind: ShowcaseKind;
  title: string;
  summary: string;
  publicPath?: string;
  externalUrl?: string;
  /** Private editor link, supplied only by the authenticated showcase state. */
  managePath?: string;
  date?: string;
}

export const DEFAULT_SHOWCASE: ShowcaseConfig = {
  name: "",
  bio: "",
  tagline: "",
  interests: [],
  layout: "folio",
  palette: "paper",
  font: "sans",
  style: "classic",
  illustration: "none",
  heroTitle: "",
  aboutLayout: "classic",
  portfolioLayout: "sections",
  avatarKey: "",
  heroImageKey: "",
  writingStyle: "cards",
  readingStyle: "shelf",
  sectionOrder: ["writing", "reading", "project"],
  selectedWorkIds: [],
  featuredWorkIds: [],
};

export const SHOWCASE_LAYOUTS: ReadonlyArray<{ value: ShowcaseLayout; label: string }> = [
  { value: "folio", label: "作品集" },
  { value: "journal", label: "刊物" },
  { value: "studio", label: "工作室" },
];

export const SHOWCASE_PALETTES: ReadonlyArray<{ value: ShowcasePalette; label: string }> = [
  { value: "paper", label: "纸张" },
  { value: "forest", label: "森林" },
  { value: "ocean", label: "海洋" },
  { value: "rose", label: "玫瑰" },
  { value: "night", label: "夜色" },
  { value: "sunshine", label: "阳光" },
];

export const SHOWCASE_FONTS: ReadonlyArray<{ value: ShowcaseFont; label: string }> = [
  { value: "sans", label: "现代无衬线" },
  { value: "serif", label: "编辑衬线" },
  { value: "mono", label: "等宽字体" },
  { value: "rounded", label: "可爱圆体" },
  { value: "handwritten", label: "中文手写" },
  { value: "display", label: "机械标题" },
];
