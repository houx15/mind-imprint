import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

/**
 * Accent theming (design-system foundation, Part 1 Task 2).
 *
 * 8 accent presets, each a 50→800 step scale keyed to a fixed base hex at 500.
 * Non-default presets' scales are derived once from their spec base color via a
 * documented tint/shade ramp (see below) and hard-coded as literals — NO runtime
 * color math. `vermilion` is the exact spec §1.2 scale (also the default CSS
 * values already declared on :root in index.css).
 *
 * Ramp used to derive the 7 non-vermilion scales from their base (500 = base):
 *   - 50/100/200/300/400: linear mix of base toward white, weights
 *     0.92 / 0.79 / 0.52 / 0.34 / 0.15 respectively (blend = base + (white-base)*t).
 *   - 600/700/800: linear mix of base toward black, weights
 *     0.24 / 0.40 / 0.54 respectively (blend = base * (1 - s)).
 *   These weights were chosen to approximate the spacing of the vermilion
 *   reference scale. Values below are the computed, rounded hex literals.
 */

export type AccentId =
  | "vermilion"
  | "clay"
  | "tangerine"
  | "bamboo"
  | "teal"
  | "indigo"
  | "violet"
  | "rose";

type Scale = Record<50 | 100 | 200 | 300 | 400 | 500 | 600 | 700 | 800, string>;

export interface AccentPreset {
  id: AccentId;
  name: string;
  scale: Scale;
}

// Tuple (not AccentPreset[]) so literal-index access (ACCENT_PRESETS[0]) is
// known-defined under noUncheckedIndexedAccess; still usable as a normal
// array (.find, .map, iteration, length) by every consumer.
type PresetTuple = readonly [
  AccentPreset,
  AccentPreset,
  AccentPreset,
  AccentPreset,
  AccentPreset,
  AccentPreset,
  AccentPreset,
  AccentPreset,
];

export const ACCENT_PRESETS: PresetTuple = [
  {
    id: "vermilion",
    name: "朱砂红",
    scale: {
      50: "#FEF0ED",
      100: "#FBDAD3",
      200: "#F6A99D",
      300: "#F2897A",
      400: "#EE6B58",
      500: "#EA5140",
      600: "#CE3A2B",
      700: "#A72E22",
      800: "#7E231A",
    },
  },
  {
    id: "clay",
    name: "陶土红",
    scale: {
      50: "#FCF3F1",
      100: "#F6DFDB",
      200: "#EBB6AC",
      300: "#E49A8D",
      400: "#DC7D6C",
      500: "#D66652",
      600: "#A34E3E",
      700: "#803D31",
      800: "#622F26",
    },
  },
  {
    id: "tangerine",
    name: "蜜柑橙",
    scale: {
      50: "#FCF6EE",
      100: "#F7E6D2",
      200: "#EDC799",
      300: "#E6B273",
      400: "#DF9C4B",
      500: "#D98A2B",
      600: "#A56921",
      700: "#82531A",
      800: "#643F14",
    },
  },
  {
    id: "bamboo",
    name: "竹青绿",
    scale: {
      50: "#F1F6F2",
      100: "#D9E7DC",
      200: "#A9C8B0",
      300: "#89B392",
      400: "#679D73",
      500: "#4C8C5A",
      600: "#3A6A44",
      700: "#2E5436",
      800: "#234029",
    },
  },
  {
    id: "teal",
    name: "松石青",
    scale: {
      50: "#EDF6F5",
      100: "#D0E9E6",
      200: "#93CCC6",
      300: "#6BB8B0",
      400: "#41A49A",
      500: "#1F9488",
      600: "#187067",
      700: "#135952",
      800: "#0E443F",
    },
  },
  {
    id: "indigo",
    name: "靛蓝",
    scale: {
      50: "#EFF3F8",
      100: "#D6DEED",
      200: "#A0B4D5",
      300: "#7D98C6",
      400: "#587AB5",
      500: "#3A63A8",
      600: "#2C4B80",
      700: "#233B65",
      800: "#1B2E4D",
    },
  },
  {
    id: "violet",
    name: "紫棠",
    scale: {
      50: "#F5F1F8",
      100: "#E4DBEC",
      200: "#C1ACD3",
      300: "#AA8DC2",
      400: "#916CB1",
      500: "#7E52A3",
      600: "#603E7C",
      700: "#4C3162",
      800: "#3A264B",
    },
  },
  {
    id: "rose",
    name: "玫紫",
    scale: {
      50: "#FAF1F5",
      100: "#F2DAE4",
      200: "#E2AAC1",
      300: "#D78AAA",
      400: "#CB6891",
      500: "#C24D7E",
      600: "#933B60",
      700: "#742E4C",
      800: "#59233A",
    },
  },
];

const ORDER = [50, 100, 200, 300, 400, 500, 600, 700, 800] as const;

export function applyAccent(el: HTMLElement, id: AccentId): void {
  const preset = ACCENT_PRESETS.find((p) => p.id === id) ?? ACCENT_PRESETS[0];
  for (const step of ORDER) {
    el.style.setProperty(`--mk-accent-${step}`, preset.scale[step]);
  }
  el.style.setProperty("--mk-accent", "var(--mk-accent-500)");
}

const STORAGE_KEY = "mk-accent";

interface AccentContextValue {
  id: AccentId;
  setAccent(id: AccentId): void;
}

const AccentContext = createContext<AccentContextValue>({
  id: "vermilion",
  setAccent() {},
});

function isAccentId(value: unknown): value is AccentId {
  return typeof value === "string" && ACCENT_PRESETS.some((p) => p.id === value);
}

export interface AccentProviderProps {
  children: ReactNode;
  /**
   * Server-known accent (e.g. `MeUser.avatar_color`) to seed on mount, taking
   * priority over localStorage. When absent or not a valid preset id, falls
   * back to the old localStorage → vermilion path (this is how the `?ds`
   * gallery, which knows nothing about a signed-in user, keeps working).
   */
  initialAccent?: AccentId;
  /**
   * Called after a local `setAccent` with the new id, so a caller (e.g.
   * StudentApp) can persist it server-side. Fire-and-forget: accent.tsx never
   * awaits it and swallows any throw or rejection — the local change always
   * sticks regardless of network outcome. Keeps this module free of any
   * import from `api/`.
   */
  onPersist?: (id: AccentId) => void | Promise<void>;
}

export function AccentProvider({ children, initialAccent, onPersist }: AccentProviderProps) {
  const [id, setId] = useState<AccentId>("vermilion");

  useEffect(() => {
    const seed = isAccentId(initialAccent)
      ? initialAccent
      : ((localStorage.getItem(STORAGE_KEY) as AccentId | null) ?? "vermilion");
    setId(seed);
    applyAccent(document.documentElement, seed);
    // Only ever run on mount: initialAccent is a one-time seed, not a
    // subscription — the provider owns `id` afterward via setAccent.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const setAccent = (next: AccentId) => {
    setId(next);
    applyAccent(document.documentElement, next);
    localStorage.setItem(STORAGE_KEY, next);
    try {
      // onPersist may be sync (throw) or async (rejected Promise); swallow
      // either so the local change above always stands regardless of
      // network outcome.
      Promise.resolve(onPersist?.(next)).catch(() => {});
    } catch {
      // onPersist threw synchronously before returning a promise.
    }
  };

  return <AccentContext.Provider value={{ id, setAccent }}>{children}</AccentContext.Provider>;
}

export const useAccent = (): AccentContextValue => useContext(AccentContext);
