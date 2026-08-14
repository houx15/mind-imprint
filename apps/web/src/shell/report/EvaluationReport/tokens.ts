import type { RiskEntry } from "@mind-imprint/contracts";
import { MACARONS } from "@/ui";

/**
 * EvaluationReport shared design tokens.
 *
 * D/A position is shown by SEMANTIC COLOR ONLY — never a level digit, never an
 * L1–L4 ladder or a 0–5 band strip. The scale is a plain warn→good signal:
 * orange/yellow = needs attention, light/deep green = good. Depth `level` 1..4
 * and autonomy `band` 0..5 each fold into one of four tiers via the helpers
 * below, which return the bar/pill colors plus a plain-language verdict word
 * (never "L2" / "band 3").
 */
export interface AxisTier {
  /** Left accent bar / dot color. */
  bar: string;
  /** Verdict-pill tinted background. */
  bg: string;
  /** Verdict-pill / on-tint text color. */
  fg: string;
  /** Plain-language verdict word (e.g. 需加深 / 良好) — NOT a level number. */
  word: string;
}

// Four warn→good color tiers, shared by both axes. Orange → yellow = warn;
// light green → deep green = good. Chosen for AA-legible fg-on-bg pills.
const TIER_COLORS = [
  { bar: "#E07A3F", bg: "#FBE6DA", fg: "#AE531B" }, // 0 · orange — needs attention
  { bar: "#D19A1E", bg: "#F8EFD3", fg: "#856110" }, // 1 · yellow — developing
  { bar: "#6FAE57", bg: "#E7F1DF", fg: "#477433" }, // 2 · light green — good
  { bar: "#2E8B57", bg: "#DCEEE4", fg: "#1E6A40" }, // 3 · deep green — strong
] as const;

const DEPTH_WORDS = ["需加深", "发展中", "良好", "扎实"] as const;
const AUTONOMY_WORDS = ["偏依赖", "渐自主", "较自主", "高自主"] as const;

/** Depth level 1..4 → tier (1→orange … 4→deep green). Clamped. */
export function depthTier(level: number): AxisTier {
  const i = Math.min(3, Math.max(0, level - 1));
  return { ...TIER_COLORS[i]!, word: DEPTH_WORDS[i]! };
}

/** Autonomy band 0..5 → tier (0–1 warn, 2 developing, 3 good, 4–5 strong). */
export function autonomyTier(band: number): AxisTier {
  const i = band <= 1 ? 0 : band === 2 ? 1 : band === 3 ? 2 : 3;
  return { ...TIER_COLORS[i]!, word: AUTONOMY_WORDS[i]! };
}

export type EventKindName = "chat" | "reading" | "graph" | "writing" | "review" | "milestone";

export interface EventKindStyle {
  /** Timeline dot / accent color. */
  dot: string;
  /** Foreground (text-on-bg) color. */
  fg: string;
  /** Tinted background color. */
  bg: string;
  /** 中文 kind label for the chip. */
  label: string;
}

// milestone reuses the live per-user accent CSS custom properties (not a
// fixed macaron) — same pattern as the mockup's `.chip.type`/tabbar accent
// usage, so this stays correct under AccentProvider theming.
export const EVENT_KIND: Record<EventKindName, EventKindStyle> = {
  chat: { dot: MACARONS.mist.base, fg: MACARONS.mist.fg, bg: MACARONS.mist.bg, label: "对话" },
  reading: { dot: MACARONS.lake.base, fg: MACARONS.lake.fg, bg: MACARONS.lake.bg, label: "阅读" },
  graph: { dot: MACARONS.taro.base, fg: MACARONS.taro.fg, bg: MACARONS.taro.bg, label: "探索" },
  writing: { dot: MACARONS.matcha.base, fg: MACARONS.matcha.fg, bg: MACARONS.matcha.bg, label: "写作" },
  review: { dot: MACARONS.berry.base, fg: MACARONS.berry.fg, bg: MACARONS.berry.bg, label: "复盘" },
  milestone: { dot: "var(--mk-accent)", fg: "var(--mk-accent-600)", bg: "var(--mk-accent-50)", label: "里程碑" },
};

export const RISK_LABEL: Record<RiskEntry["type"], string> = {
  "ai-ghostwrite": "AI 代劳",
  "missing-source": "缺少信源",
  "argument-logic": "论证逻辑",
  "data-scope": "数据口径",
  "rabbit-hole-offtopic": "兔子洞跑题",
};
