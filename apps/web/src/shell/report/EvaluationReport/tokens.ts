import type { RiskEntry } from "@mind-imprint/contracts";
import { MACARONS } from "@/ui";

/**
 * EvaluationReport shared design tokens (Task 8).
 *
 * D/A axis level is shown by COLOR ONLY (never a digit) — see the mockup's
 * `.axis-legend`/`.lvbar`. Ramps are index-addressed: `DEPTH_RAMP[level-1]`
 * for `level` 1..4, `AUTONOMY_RAMP[band]` for `band` 0..5.
 */
export const DEPTH_RAMP = ["#CDE7E1", "#8FCEC1", "#4FB0A0", "#177368"] as const;
export const AUTONOMY_RAMP = ["#EFEAF6", "#DACFEC", "#C3B0E1", "#A98FD3", "#8A6AC0", "#5B4A80"] as const;

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
