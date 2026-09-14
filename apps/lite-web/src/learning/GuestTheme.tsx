import type { CSSProperties, ReactNode } from "react";
import { AccentProvider } from "@/ui/accent";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import { CompanionAppearanceProvider } from "@/ui/CompanionAppearance";
import bookmark from "../home/assets/yinji-bookmark.webp";
import "@/ui/themes/lite.css";
import "./student-surfaces.css";

/** Theme only: guest pages keep their existing authentication and share logic. */
export function GuestTheme({ children }: { children: ReactNode }) {
  const style = Object.fromEntries(Object.entries(LITE_ACCENT_PRESETS[0]!.scale).map(([step, value]) => [`--mk-theme-accent-${step}`, value])) as CSSProperties;
  return <AccentProvider initialAccent="teal" presets={LITE_ACCENT_PRESETS}><CompanionAppearanceProvider image={bookmark}><div className="lite-student student-guest h-full min-h-full bg-mk-paper text-mk-ink" style={style}>{children}</div></CompanionAppearanceProvider></AccentProvider>;
}
