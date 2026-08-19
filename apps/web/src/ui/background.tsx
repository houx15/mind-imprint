import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

/**
 * background.tsx — the per-student page background colorway, mirroring accent.tsx
 * exactly. A preset overrides the single `--mk-paper` CSS variable on
 * `document.documentElement`; that one write retints `body`, the app shell, and
 * every `bg-mk-paper` surface at once. Card surfaces stay white (`--mk-surface`),
 * so a tinted background gives cards contrast and the white preset leans on the
 * hairline borders — a clean, calm look either way.
 *
 * Kept in sync with the backend allowlist (users_background.go
 * pageBackgroundPresets) and the migration 0074 CHECK.
 */

export type BackgroundId = "paper" | "white" | "cream" | "blue" | "green" | "slate";

export interface BackgroundPreset {
  id: BackgroundId;
  name: string;
  /** The `--mk-paper` value this preset paints the page with. */
  paper: string;
}

export const BACKGROUND_PRESETS: readonly BackgroundPreset[] = [
  { id: "paper", name: "温暖", paper: "#FBF8F4" },
  { id: "white", name: "纯白", paper: "#FFFFFF" },
  { id: "cream", name: "米纸", paper: "#F6F1E7" },
  { id: "blue", name: "微蓝", paper: "#EEF3FA" },
  { id: "green", name: "微绿", paper: "#EEF4EA" },
  { id: "slate", name: "微灰", paper: "#F3F4F6" },
];

export function applyBackground(el: HTMLElement, id: BackgroundId): void {
  const preset = BACKGROUND_PRESETS.find((p) => p.id === id);
  el.style.setProperty("--mk-paper", preset ? preset.paper : "#FBF8F4");
}

const STORAGE_KEY = "mk-background";

function isBackgroundId(value: unknown): value is BackgroundId {
  return typeof value === "string" && BACKGROUND_PRESETS.some((p) => p.id === value);
}

interface BackgroundContextValue {
  id: BackgroundId;
  setBackground(id: BackgroundId): void;
}

const BackgroundContext = createContext<BackgroundContextValue>({
  id: "paper",
  setBackground() {},
});

export interface BackgroundProviderProps {
  children: ReactNode;
  /**
   * Server-known background (e.g. `MeUser.page_background`) to seed on mount,
   * taking priority over localStorage. When absent or not a valid preset id,
   * falls back to the localStorage → 'paper' (the warm default) path.
   */
  initialBackground?: BackgroundId;
  /**
   * Called after a local `setBackground`, so a caller (e.g. StudentApp) can
   * persist it server-side. Fire-and-forget: this module never awaits it and
   * swallows any throw/rejection — the local change always sticks.
   */
  onPersist?: (id: BackgroundId) => void | Promise<void>;
}

export function BackgroundProvider({ children, initialBackground, onPersist }: BackgroundProviderProps) {
  const [id, setId] = useState<BackgroundId>("paper");

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY);
    const seed: BackgroundId = isBackgroundId(initialBackground)
      ? initialBackground
      : isBackgroundId(stored)
        ? stored
        : "paper";
    setId(seed);
    applyBackground(document.documentElement, seed);
    // One-time seed only; the provider owns `id` afterward via setBackground.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const setBackground = (next: BackgroundId) => {
    setId(next);
    applyBackground(document.documentElement, next);
    localStorage.setItem(STORAGE_KEY, next);
    try {
      Promise.resolve(onPersist?.(next)).catch(() => {});
    } catch {
      /* onPersist threw synchronously before returning a promise */
    }
  };

  return <BackgroundContext.Provider value={{ id, setBackground }}>{children}</BackgroundContext.Provider>;
}

export const useBackground = (): BackgroundContextValue => useContext(BackgroundContext);
