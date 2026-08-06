# Part 1 · Design-System Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the token layer, accent theming, and a reusable `ui/` primitive library (incl. the 豆豆 Pebble character + loaders) so every later page derives from the locked design system instead of hardcoded hexes.

**Architecture:** All design tokens become CSS custom properties on `:root` (accent scale is swappable per student); `tailwind.config.ts` maps the `mk-*` namespace onto those variables and remaps the legacy blue/terracotta aliases to the new warm palette so existing screens instantly warm up. A new `apps/web/src/ui/` directory holds framework-agnostic React primitives (Button, Card, forms, feedback, overlays, skeletons), the Pebble family, and an Illustration/Empty component. A `?ds` gallery route renders every primitive for visual review. **Frontend-only — no backend, no migration.**

**Tech Stack:** React 18 + Vite + TypeScript + Tailwind; Vitest + jsdom + @testing-library/react (tests in `apps/web/test/**`, mirror of `src/`); `lucide-react` for icons.

## Global Constraints

- **Single source of truth:** every value comes verbatim from `docs/superpowers/specs/2026-08-06-design-system-design.md`. Never invent a color/size — if it's not in the spec, ask.
- **Token namespace:** keep the existing `mk-*` Tailwind prefix. Colors resolve through CSS variables (`var(--mk-*)`), never inline hex, in all NEW code.
- **Accent swappable:** the 8 accent presets each define a full 50→800 scale; switching accent = swapping the `--mk-accent-*` variables only. Neutrals, macaron, and semantic colors are FIXED (never derive from accent — a student may set accent to red, so 危险 must stay independent).
- **豆豆 = React components, not static `.svg`** — inline SVG markup + CSS `@keyframes`, body fill `var(--mk-accent-500)`, copied verbatim from the mockups in `docs/design/design-system-mockups/pebble-*.html`. Motion must match the mockups exactly.
- **Motion:** easing `cubic-bezier(.2,0,0,1)`; durations fast 120 / base 200 / slow 320 ms; every animation respects `prefers-reduced-motion` (degrade to instant).
- **Icons:** Lucide, 1.75 stroke, `currentColor`. No emoji as functional icons.
- **No new user-facing page wiring** beyond the dev-only `?ds` gallery. Existing pages are NOT migrated in Part 1 (that's Parts 2/3); they only inherit the warmed `mk-*` aliases automatically.
- **Tests** live in `apps/web/test/ui/*.test.tsx`; run `npm --prefix apps/web test` and `npm --prefix apps/web run typecheck`. Component behavior/render tests only (jsdom can't compute stylesheet CSS — assert on classNames, inline styles, ARIA, and rendered structure, not `getComputedStyle` of a stylesheet rule).
- **Accessibility:** interactive primitives keyboard-operable, visible focus ring, correct ARIA roles.

---

## File Structure

- `apps/web/src/index.css` — **Modify:** add `:root` token variables + keyframes + scrollbar + reduced-motion.
- `apps/web/tailwind.config.ts` — **Modify:** remap `mk-*` to variables, add scales, font, radii, shadows, type sizes.
- `apps/web/src/ui/tokens.ts` — **Create:** TS mirror of token constants + the 8 accent presets (scales) + macaron/semantic maps (for use where inline styles are unavoidable, e.g. SVG fills).
- `apps/web/src/ui/accent.tsx` — **Create:** `AccentProvider`, `useAccent`, `applyAccent`, preset list.
- `apps/web/src/ui/Icon.tsx` — **Create:** Lucide wrapper with default stroke/size.
- `apps/web/src/ui/Button.tsx` — **Create.**
- `apps/web/src/ui/Card.tsx` — **Create:** `Surface`, `Card`, `CompactRow`.
- `apps/web/src/ui/forms.tsx` — **Create:** `Input`, `Textarea`, `Select`, `Toggle`, `Radio`, `Checkbox`, `Chip`, `Rating`.
- `apps/web/src/ui/feedback.tsx` — **Create:** `Badge`, `Tabs`, `Segmented`, `Progress`, `Stepper`, `Tooltip`.
- `apps/web/src/ui/overlays.tsx` — **Create:** `Modal`, `Drawer`, `Menu`, `toast()`.
- `apps/web/src/ui/Skeleton.tsx` — **Create:** `Skeleton`, `SkeletonText`, `SkeletonCard`, `SkeletonRow`.
- `apps/web/src/ui/Pebble.tsx` — **Create:** `Pebble` (4 states).
- `apps/web/src/ui/loaders.tsx` — **Create:** `PebbleProgress`, `PebbleInlineSpinner`, `RabbitHoleLoader`.
- `apps/web/src/ui/Illustration.tsx` — **Create:** `Illustration`, `EmptyState`.
- `apps/web/src/ui/index.ts` — **Create:** barrel export.
- `apps/web/src/dev/DesignSystemGallery.tsx` — **Create:** `?ds` gallery.
- `apps/web/src/Root.tsx` — **Modify:** add `?ds` route.
- `apps/web/test/ui/*.test.tsx` — **Create:** one test file per module.

---

### Task 1: Token layer — CSS variables + Tailwind remap

**Files:**
- Modify: `apps/web/src/index.css`
- Modify: `apps/web/tailwind.config.ts`
- Create: `apps/web/src/ui/tokens.ts`
- Test: `apps/web/test/ui/tokens.test.ts`

**Interfaces:**
- Produces: CSS variables `--mk-*` on `:root`; Tailwind color tokens `mk-*`; TS constants in `ui/tokens.ts`:
  - `MACARONS: Record<MacaronName,{base:string;bg:string;fg:string}>` where `MacaronName = 'peach'|'butter'|'matcha'|'lake'|'mist'|'taro'|'berry'`
  - `SEMANTIC: Record<'success'|'warning'|'danger'|'info',{base:string;bg:string}>`
  - `NEUTRAL: { paper; surface; border; ink; muted; secondary; faint; inputBorder }`
  - `RADIUS: { xs:6; sm:8; md:10; lg:12; full:999 }`

- [ ] **Step 1: Write the failing test** — `apps/web/test/ui/tokens.test.ts`

```ts
import { describe, it, expect } from "vitest";
import { MACARONS, SEMANTIC, NEUTRAL, RADIUS } from "@/ui/tokens";

describe("design tokens", () => {
  it("has all 7 macaron colors with tint pairs", () => {
    expect(Object.keys(MACARONS)).toEqual(
      ["peach", "butter", "matcha", "lake", "mist", "taro", "berry"],
    );
    expect(MACARONS.peach).toEqual({ base: "#F6B26B", bg: "#FDE7D3", fg: "#9A5A22" });
    expect(MACARONS.berry).toEqual({ base: "#E896A6", bg: "#FCE7EB", fg: "#A63A50" });
  });
  it("keeps semantic colors independent (danger is not accent)", () => {
    expect(SEMANTIC.danger.base).toBe("#D64541");
    expect(SEMANTIC.success.base).toBe("#5FA97E");
  });
  it("uses warm neutrals", () => {
    expect(NEUTRAL.paper).toBe("#FBF8F4");
    expect(NEUTRAL.ink).toBe("#33302E");
    expect(NEUTRAL.border).toBe("#EFE7DD");
  });
  it("tightens radius (sm workhorse = 8)", () => {
    expect(RADIUS.sm).toBe(8);
    expect(RADIUS.lg).toBe(12);
  });
});
```

- [ ] **Step 2: Run test to verify it fails** — `npm --prefix apps/web test -- tokens` → FAIL (module missing).

- [ ] **Step 3: Create `apps/web/src/ui/tokens.ts`** (values verbatim from spec §1):

```ts
export const NEUTRAL = {
  paper: "#FBF8F4", surface: "#FFFFFF", border: "#EFE7DD",
  muted: "#8A827A", secondary: "#5A5049", faint: "#B0A599",
  ink: "#33302E", inputBorder: "#E4DACF",
} as const;

export type MacaronName = "peach" | "butter" | "matcha" | "lake" | "mist" | "taro" | "berry";
export const MACARONS: Record<MacaronName, { base: string; bg: string; fg: string }> = {
  peach:  { base: "#F6B26B", bg: "#FDE7D3", fg: "#9A5A22" },
  butter: { base: "#F2CE63", bg: "#FAF3DE", fg: "#8A6320" },
  matcha: { base: "#A6C88F", bg: "#E7F1DD", fg: "#4D6B3A" },
  lake:   { base: "#6FBFB0", bg: "#E0F0EC", fg: "#177368" },
  mist:   { base: "#7FA8D8", bg: "#E6EEF9", fg: "#3C5A86" },
  taro:   { base: "#B49BD8", bg: "#F0EAF6", fg: "#5B4A80" },
  berry:  { base: "#E896A6", bg: "#FCE7EB", fg: "#A63A50" },
};

export const SEMANTIC = {
  success: { base: "#5FA97E", bg: "#E7F1DD" },
  warning: { base: "#E0A63A", bg: "#FBEFD6" },
  danger:  { base: "#D64541", bg: "#FBE3E1" },
  info:    { base: "#4C7AB0", bg: "#EAF0F6" },
} as const;

export const RADIUS = { xs: 6, sm: 8, md: 10, lg: 12, full: 999 } as const;

export const SHADOW = {
  xs: "0 1px 2px rgba(70,45,45,.06)",
  sm: "0 2px 8px rgba(70,45,45,.08)",
  md: "0 6px 18px rgba(70,45,45,.11)",
  lg: "0 16px 40px rgba(45,28,28,.18)",
} as const;

export const MOTION = { ease: "cubic-bezier(.2,0,0,1)", fast: 120, base: 200, slow: 320 } as const;
```

- [ ] **Step 4: Rewrite `apps/web/src/index.css`** — prepend the token `:root` block (keep the existing `html,body,#root{height:100%}` rule and the `.mk-think-dot` animation). Add, after the `@tailwind` lines:

```css
:root {
  /* neutrals (fixed) */
  --mk-paper:#FBF8F4; --mk-surface:#FFFFFF; --mk-border:#EFE7DD;
  --mk-muted:#8A827A; --mk-secondary:#5A5049; --mk-faint:#B0A599;
  --mk-ink:#33302E; --mk-input-border:#E4DACF;
  /* accent scale (default 朱砂红; AccentProvider overrides these 9) */
  --mk-accent-50:#FEF0ED; --mk-accent-100:#FBDAD3; --mk-accent-200:#F6A99D;
  --mk-accent-300:#F2897A; --mk-accent-400:#EE6B58; --mk-accent-500:#EA5140;
  --mk-accent-600:#CE3A2B; --mk-accent-700:#A72E22; --mk-accent-800:#7E231A;
  --mk-accent:var(--mk-accent-500);
  /* macaron (fixed) */
  --mk-peach:#F6B26B;  --mk-peach-bg:#FDE7D3;  --mk-peach-fg:#9A5A22;
  --mk-butter:#F2CE63; --mk-butter-bg:#FAF3DE; --mk-butter-fg:#8A6320;
  --mk-matcha:#A6C88F; --mk-matcha-bg:#E7F1DD; --mk-matcha-fg:#4D6B3A;
  --mk-lake:#6FBFB0;   --mk-lake-bg:#E0F0EC;   --mk-lake-fg:#177368;
  --mk-mist:#7FA8D8;   --mk-mist-bg:#E6EEF9;   --mk-mist-fg:#3C5A86;
  --mk-taro:#B49BD8;   --mk-taro-bg:#F0EAF6;   --mk-taro-fg:#5B4A80;
  --mk-berry:#E896A6;  --mk-berry-bg:#FCE7EB;  --mk-berry-fg:#A63A50;
  /* semantic (fixed) */
  --mk-success:#5FA97E; --mk-success-bg:#E7F1DD;
  --mk-warning:#E0A63A; --mk-warning-bg:#FBEFD6;
  --mk-danger:#D64541;  --mk-danger-bg:#FBE3E1;
  --mk-info:#4C7AB0;    --mk-info-bg:#EAF0F6;
  /* radius / shadow / motion */
  --mk-radius-xs:6px; --mk-radius-sm:8px; --mk-radius-md:10px; --mk-radius-lg:12px; --mk-radius-full:999px;
  --mk-shadow-xs:0 1px 2px rgba(70,45,45,.06);
  --mk-shadow-sm:0 2px 8px rgba(70,45,45,.08);
  --mk-shadow-md:0 6px 18px rgba(70,45,45,.11);
  --mk-shadow-lg:0 16px 40px rgba(45,28,28,.18);
  --mk-ease:cubic-bezier(.2,0,0,1);
  --mk-fast:120ms; --mk-base:200ms; --mk-slow:320ms;
}
body { background: var(--mk-paper); color: var(--mk-ink);
  font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Segoe UI",system-ui,sans-serif; }

/* scrollbar: hidden until .scrolling on a scroll container (spec §6) */
.mk-scroll { scrollbar-width: none; }
.mk-scroll::-webkit-scrollbar { width:8px; height:8px; }
.mk-scroll::-webkit-scrollbar-thumb { background:transparent; border-radius:999px; }
.mk-scroll.scrolling::-webkit-scrollbar-thumb { background: var(--mk-accent-200); }

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation-duration:.001ms !important; animation-iteration-count:1 !important; transition-duration:.001ms !important; }
}
```

- [ ] **Step 5: Rewrite `apps/web/tailwind.config.ts`** — map `mk-*` to variables, remap legacy aliases, tighten radii, set font/type/shadow:

```ts
import type { Config } from "tailwindcss";
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        mk: {
          // accent (swappable)
          accent: "var(--mk-accent)",
          "accent-50":"var(--mk-accent-50)","accent-100":"var(--mk-accent-100)",
          "accent-200":"var(--mk-accent-200)","accent-300":"var(--mk-accent-300)",
          "accent-400":"var(--mk-accent-400)","accent-500":"var(--mk-accent-500)",
          "accent-600":"var(--mk-accent-600)","accent-700":"var(--mk-accent-700)",
          "accent-800":"var(--mk-accent-800)",
          // neutrals
          paper:"var(--mk-paper)", surface:"var(--mk-surface)", border:"var(--mk-border)",
          ink:"var(--mk-ink)", secondary:"var(--mk-secondary)", muted:"var(--mk-muted)",
          faint:"var(--mk-faint)", "input-border":"var(--mk-input-border)",
          // macaron
          peach:"var(--mk-peach)","peach-bg":"var(--mk-peach-bg)","peach-fg":"var(--mk-peach-fg)",
          butter:"var(--mk-butter)","butter-bg":"var(--mk-butter-bg)","butter-fg":"var(--mk-butter-fg)",
          matcha:"var(--mk-matcha)","matcha-bg":"var(--mk-matcha-bg)","matcha-fg":"var(--mk-matcha-fg)",
          lake:"var(--mk-lake)","lake-bg":"var(--mk-lake-bg)","lake-fg":"var(--mk-lake-fg)",
          mist:"var(--mk-mist)","mist-bg":"var(--mk-mist-bg)","mist-fg":"var(--mk-mist-fg)",
          taro:"var(--mk-taro)","taro-bg":"var(--mk-taro-bg)","taro-fg":"var(--mk-taro-fg)",
          berry:"var(--mk-berry)","berry-bg":"var(--mk-berry-bg)","berry-fg":"var(--mk-berry-fg)",
          // semantic
          success:"var(--mk-success)","success-bg":"var(--mk-success-bg)",
          warning:"var(--mk-warning)","warning-bg":"var(--mk-warning-bg)",
          danger:"var(--mk-danger)","danger-bg":"var(--mk-danger-bg)",
          info:"var(--mk-info)","info-bg":"var(--mk-info-bg)",
          // LEGACY ALIASES (warm the 40 old-token files automatically; Parts 2/3 replace with intent)
          primary:"var(--mk-accent)","primary-hover":"var(--mk-accent-600)","primary-tint":"var(--mk-accent-50)",
          "accent-hover":"var(--mk-peach-fg)","accent-tint":"var(--mk-peach-bg)",
          green:"var(--mk-success)","green-tint":"var(--mk-success-bg)",
          amber:"var(--mk-warning)",
          bg:"var(--mk-paper)","border-2":"var(--mk-border)",
          input:"var(--mk-input-border)","input-bg":"var(--mk-surface)",
          "muted-2":"var(--mk-faint)",
        },
      },
      borderRadius: {
        "mk-xs":"var(--mk-radius-xs)","mk-sm":"var(--mk-radius-sm)",
        "mk-md":"var(--mk-radius-md)","mk-lg":"var(--mk-radius-lg)","mk-full":"var(--mk-radius-full)",
        mk:"10px", "mk-sheet":"12px", // legacy: mk 14→10, mk-sheet 22→12
      },
      boxShadow: {
        "mk-xs":"var(--mk-shadow-xs)","mk-sm":"var(--mk-shadow-sm)",
        "mk-md":"var(--mk-shadow-md)","mk-lg":"var(--mk-shadow-lg)",
      },
      fontFamily: {
        sans: ["-apple-system","BlinkMacSystemFont","PingFang SC","Segoe UI","system-ui","sans-serif"],
        mono: ["ui-monospace","SF Mono","PingFang SC","monospace"],
      },
      fontSize: {
        "mk-display":["32px",{lineHeight:"1.2",fontWeight:"700"}],
        "mk-h1":["24px",{lineHeight:"1.25",fontWeight:"700"}],
        "mk-h2":["18px",{lineHeight:"1.3",fontWeight:"600"}],
        "mk-h3":["16px",{lineHeight:"1.35",fontWeight:"600"}],
        "mk-body-lg":["16px",{lineHeight:"1.75"}],
        "mk-body":["14px",{lineHeight:"1.6"}],
        "mk-small":["12px",{lineHeight:"1.5"}],
        "mk-caption":["12px",{lineHeight:"1.45",fontWeight:"500"}],
        "mk-label":["11px",{lineHeight:"1.4",fontWeight:"700",letterSpacing:"0.1em"}],
      },
      transitionTimingFunction: { mk: "cubic-bezier(.2,0,0,1)" },
    },
  },
  plugins: [],
} satisfies Config;
```

- [ ] **Step 6: Run tests + typecheck** — `npm --prefix apps/web test -- tokens` → PASS; `npm --prefix apps/web run typecheck` → clean; `npm --prefix apps/web run build` → succeeds.

- [ ] **Step 7: Commit** — `git add apps/web/src/index.css apps/web/tailwind.config.ts apps/web/src/ui/tokens.ts apps/web/test/ui/tokens.test.ts && git commit -m "feat(ds): token layer — CSS vars + tailwind remap to warm palette"`

---

### Task 2: Accent theming (8 presets, provider, seam)

**Files:**
- Create: `apps/web/src/ui/accent.tsx`
- Test: `apps/web/test/ui/accent.test.tsx`

**Interfaces:**
- Consumes: `--mk-accent-*` CSS variables (Task 1).
- Produces:
  - `ACCENT_PRESETS: AccentPreset[]` where `AccentPreset = { id: AccentId; name: string; scale: Record<50|100|200|300|400|500|600|700|800, string> }`, `AccentId = 'vermilion'|'clay'|'tangerine'|'bamboo'|'teal'|'indigo'|'violet'|'rose'` (order per spec §1.1; `vermilion` = 朱砂红 default).
  - `applyAccent(el: HTMLElement, id: AccentId): void` — sets the 9 `--mk-accent-*` vars (500 also aliased to `--mk-accent`).
  - `AccentProvider({ children })` — on mount reads `localStorage["mk-accent"]` (fallback `'vermilion'`), applies to `document.documentElement`; provides context.
  - `useAccent(): { id: AccentId; setAccent(id: AccentId): void }` — `setAccent` applies + persists to localStorage.

**Note for later parts:** persistence is localStorage-only in Part 1. The backend field decision (overload `card_theme` vs. add a new `accent` column) is deferred to Part 2's settings work — `setAccent` is the seam.

- [ ] **Step 1: Write the failing test** — `apps/web/test/ui/accent.test.tsx`

```tsx
import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { ACCENT_PRESETS, applyAccent, AccentProvider, useAccent } from "@/ui/accent";

beforeEach(() => localStorage.clear());

describe("accent theming", () => {
  it("ships 8 presets with vermilion default first", () => {
    expect(ACCENT_PRESETS).toHaveLength(8);
    expect(ACCENT_PRESETS[0].id).toBe("vermilion");
    expect(ACCENT_PRESETS[0].scale[500]).toBe("#EA5140");
  });
  it("applyAccent sets the accent-500 variable", () => {
    const el = document.createElement("div");
    applyAccent(el, "teal");
    expect(el.style.getPropertyValue("--mk-accent-500")).toBe("#1F9488");
    expect(el.style.getPropertyValue("--mk-accent")).toBe("var(--mk-accent-500)");
  });
  it("useAccent persists selection", () => {
    function Probe() {
      const { id, setAccent } = useAccent();
      return <button data-id={id} onClick={() => setAccent("rose")}>go</button>;
    }
    render(<AccentProvider><Probe /></AccentProvider>);
    expect(screen.getByRole("button").dataset.id).toBe("vermilion");
    act(() => screen.getByRole("button").click());
    expect(screen.getByRole("button").dataset.id).toBe("rose");
    expect(localStorage.getItem("mk-accent")).toBe("rose");
  });
});
```

- [ ] **Step 2: Run → FAIL** (`npm --prefix apps/web test -- accent`).

- [ ] **Step 3: Implement `apps/web/src/ui/accent.tsx`.** Include all 8 preset scales. The default `vermilion` scale is the spec §1.2 table. Generate each other preset's scale from its base (spec §1.1 bases: clay `#D66652`, tangerine `#D98A2B`, bamboo `#4C8C5A`, teal `#1F9488`, indigo `#3A63A8`, violet `#7E52A3`, rose `#C24D7E`) using a documented tint/shade ramp so 500 = base. Provide exact hex literals (compute once, hard-code — do NOT compute at runtime). Use this ramp when deriving: 50/100/200/300/400 are progressively lighter mixes toward white; 600/700/800 progressively darker toward black; 500 = base. Structure:

```tsx
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export type AccentId = "vermilion"|"clay"|"tangerine"|"bamboo"|"teal"|"indigo"|"violet"|"rose";
type Scale = Record<50|100|200|300|400|500|600|700|800, string>;
export interface AccentPreset { id: AccentId; name: string; scale: Scale; }

export const ACCENT_PRESETS: AccentPreset[] = [
  { id:"vermilion", name:"朱砂红", scale:{50:"#FEF0ED",100:"#FBDAD3",200:"#F6A99D",300:"#F2897A",400:"#EE6B58",500:"#EA5140",600:"#CE3A2B",700:"#A72E22",800:"#7E231A"} },
  // clay/tangerine/bamboo/teal/indigo/violet/rose — 500 = spec base, ramp derived as above.
  // (Implementer: produce the 8 hex literals per preset with the documented ramp; 500 MUST equal the spec base.)
];

const ORDER = [50,100,200,300,400,500,600,700,800] as const;
export function applyAccent(el: HTMLElement, id: AccentId): void {
  const p = ACCENT_PRESETS.find(x => x.id === id) ?? ACCENT_PRESETS[0];
  for (const k of ORDER) el.style.setProperty(`--mk-accent-${k}`, p.scale[k]);
  el.style.setProperty("--mk-accent", "var(--mk-accent-500)");
}

const Ctx = createContext<{ id: AccentId; setAccent(id: AccentId): void }>({ id:"vermilion", setAccent(){} });
export function AccentProvider({ children }: { children: ReactNode }) {
  const [id, setId] = useState<AccentId>("vermilion");
  useEffect(() => {
    const saved = (localStorage.getItem("mk-accent") as AccentId | null) ?? "vermilion";
    setId(saved); applyAccent(document.documentElement, saved);
  }, []);
  const setAccent = (next: AccentId) => {
    setId(next); applyAccent(document.documentElement, next); localStorage.setItem("mk-accent", next);
  };
  return <Ctx.Provider value={{ id, setAccent }}>{children}</Ctx.Provider>;
}
export const useAccent = () => useContext(Ctx);
```

- [ ] **Step 4: Run → PASS; typecheck clean.**
- [ ] **Step 5: Commit** — `git add apps/web/src/ui/accent.tsx apps/web/test/ui/accent.test.tsx && git commit -m "feat(ds): accent theming — 8 presets, provider, localStorage seam"`

---

### Task 3: Lucide icon wrapper

**Files:**
- Modify: `apps/web/package.json` (add `lucide-react`)
- Create: `apps/web/src/ui/Icon.tsx`
- Test: `apps/web/test/ui/Icon.test.tsx`

**Interfaces:**
- Produces: `Icon({ name, size?, className, ...svgProps })` — `name` is a `lucide-react` icon component; default `strokeWidth=1.75`, `size=20`, `color="currentColor"`. Re-export commonly used icons.

- [ ] **Step 1: Install** — `npm --prefix apps/web install lucide-react` (confirm it resolves; `lucide-react` is self-contained, tree-shakeable).
- [ ] **Step 2: Write failing test** — `apps/web/test/ui/Icon.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Icon } from "@/ui/Icon";
import { Search } from "lucide-react";

describe("Icon", () => {
  it("renders an svg with 1.75 stroke and given size", () => {
    const { container } = render(<Icon icon={Search} size={16} />);
    const svg = container.querySelector("svg")!;
    expect(svg).toBeInTheDocument();
    expect(svg.getAttribute("stroke-width")).toBe("1.75");
    expect(svg.getAttribute("width")).toBe("16");
  });
});
```

- [ ] **Step 3: Implement `apps/web/src/ui/Icon.tsx`:**

```tsx
import type { LucideIcon } from "lucide-react";
export function Icon({ icon: I, size = 20, ...rest }: { icon: LucideIcon; size?: number } & React.SVGProps<SVGSVGElement>) {
  return <I size={size} strokeWidth={1.75} {...rest} />;
}
```

- [ ] **Step 4: Run → PASS; typecheck clean.**
- [ ] **Step 5: Commit** — stage `package.json`, `package-lock.json`/workspace lock, `Icon.tsx`, test. `git commit -m "feat(ds): Lucide icon wrapper"`

---

### Task 4: Button

**Files:**
- Create: `apps/web/src/ui/Button.tsx`
- Test: `apps/web/test/ui/Button.test.tsx`

**Interfaces:**
- Produces: `Button({ variant?, size?, iconStart?, iconEnd?, loading?, ...button })`.
  - `variant: 'primary'|'secondary'|'ghost'|'link'|'danger'` (default `'primary'`).
  - `size: 'md'|'sm'` (default `'md'`).
  - `IconButton({ icon, label, size?, variant? })` — square, `aria-label` required.

**Spec (§7):** primary = solid accent, white text; secondary = white bg + accent border + `accent-700` text; ghost = transparent, hover `accent-50` bg; link = `accent-700`; danger = solid `danger`. Focus ring `0 0 0 3px rgba(accent,.15)`. md padding 10×18 radius 8–10; sm padding 6×12 radius 8. Disabled = `#F0E9E1` bg `#B8ADA2` text. `loading` shows inline spinner (Task 11 `PebbleInlineSpinner` — until then, a simple spinner; wire in Task 11) and disables.

- [ ] **Step 1: Write failing test** — `apps/web/test/ui/Button.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Button } from "@/ui/Button";

describe("Button", () => {
  it("renders children and fires onClick", async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>保存</Button>);
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(onClick).toHaveBeenCalledOnce();
  });
  it("does not fire when loading", async () => {
    const onClick = vi.fn();
    render(<Button loading onClick={onClick}>保存</Button>);
    await userEvent.click(screen.getByRole("button"));
    expect(onClick).not.toHaveBeenCalled();
    expect(screen.getByRole("button")).toBeDisabled();
  });
  it("applies variant class", () => {
    render(<Button variant="danger">删除</Button>);
    expect(screen.getByRole("button").className).toContain("bg-mk-danger");
  });
});
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement `Button.tsx`** with a `variant→className` map using Tailwind `mk-*` tokens (no inline hex). Example variant map:

```tsx
const VARIANTS: Record<string,string> = {
  primary:  "bg-mk-accent text-white hover:bg-mk-accent-600 active:bg-mk-accent-700",
  secondary:"bg-mk-surface text-mk-accent-700 border border-mk-accent hover:bg-mk-accent-50",
  ghost:    "bg-transparent text-mk-ink hover:bg-mk-accent-50",
  link:     "bg-transparent text-mk-accent-700 hover:underline px-0",
  danger:   "bg-mk-danger text-white hover:brightness-95",
};
```

Base classes: `inline-flex items-center justify-center gap-2 font-medium rounded-mk-sm transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15 disabled:bg-[#F0E9E1] disabled:text-[#B8ADA2] disabled:cursor-not-allowed`. Size: md `text-mk-body px-[18px] py-[10px]`, sm `text-mk-small px-3 py-1.5`. `loading` → `disabled` + spinner slot. Render `iconStart`/`iconEnd`.

- [ ] **Step 4: Run → PASS; typecheck clean.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): Button + IconButton"`

---

### Task 5: Card / Surface / CompactRow

**Files:**
- Create: `apps/web/src/ui/Card.tsx`
- Test: `apps/web/test/ui/Card.test.tsx`

**Interfaces (§3, §10):**
- `Surface({ level?, as?, children, className })` — `level: 'hairline'|'sm'|'md'|'lg'` → shadow-xs(+border)/sm/md/lg. Default `hairline` = `shadow-mk-xs border border-mk-border rounded-mk-sm bg-mk-surface`.
- `Card({ children })` — `Surface level="md" rounded-mk-md` object card.
- `CompactRow({ thumb?, title, meta?, trailing? })` — hairline 40px thumb row for long lists.

- [ ] **Step 1: Write failing test** asserting `Surface` renders children + border class at hairline; `CompactRow` renders title/meta/trailing regions.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Level→class map:

```tsx
const LEVELS = {
  hairline:"shadow-mk-xs border border-mk-border",
  sm:"shadow-mk-sm", md:"shadow-mk-md", lg:"shadow-mk-lg",
};
```

- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): Surface/Card/CompactRow"`

---

### Task 6: Form controls (card-runtime primitives)

**Files:**
- Create: `apps/web/src/ui/forms.tsx`
- Test: `apps/web/test/ui/forms.test.tsx`

**Interfaces (§8):** `Input`, `Textarea`, `Select`, `Toggle`, `Radio`, `Checkbox`, `Chip` (multi-select pill), `Rating` (5-star, fill `warning` gold — NOT accent). Each controlled. Field styles: border `mk-input-border`, radius 8, placeholder `#B8ADA2`, focus = accent border + focus ring; error state = danger border + `mk-danger-bg` + error line.

- [ ] **Step 1: Failing test** — Input onChange fires; Toggle toggles + `role="switch"` `aria-checked`; Rating click sets value + calls onChange; Chip toggles `aria-pressed`; error prop renders the error message text.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** all controls; Rating stars fill `text-mk-warning`; selected Chip = `bg-mk-accent-50 text-mk-accent-700`. Inputs accept `error?: string`.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): form controls (Input/Textarea/Select/Toggle/Radio/Checkbox/Chip/Rating)"`

---

### Task 7: Feedback (Badge/Tabs/Segmented/Progress/Stepper/Tooltip)

**Files:**
- Create: `apps/web/src/ui/feedback.tsx`
- Test: `apps/web/test/ui/feedback.test.tsx`

**Interfaces (§9):**
- `Badge({ tone })` — `tone: 'progress'|'done'|'draft'|'pending'` → accent-50 / success / neutral / warning pill; `CountBadge({ n })` = accent solid circle.
- `Tabs({ tabs, value, onChange })` — underline style, active = accent underline.
- `Segmented({ options, value, onChange })` — pill container + white slider.
- `Progress({ value })` — linear accent fill (0–100).
- `Stepper({ steps, current })` — done=accent✓ / current=accent ring / todo=grey.
- `Tooltip({ label, children })` — dark `#33302E` bubble on hover/focus.

- [ ] **Step 1: Failing test** — Tabs onChange on click + `aria-selected`; Segmented onChange; Progress clamps 0–100 and sets fill width; Stepper marks done/current; Badge tone class.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): feedback primitives"`

---

### Task 8: Overlays (Modal/Drawer/Menu/toast)

**Files:**
- Create: `apps/web/src/ui/overlays.tsx`
- Test: `apps/web/test/ui/overlays.test.tsx`

**Interfaces (§12):**
- `Modal({ open, onClose, title, children, footer })` — radius 12 + scrim + header/body/footer (footer right-aligned). Esc + scrim-click close. Only for destructive/confirm; danger button uses `danger` token.
- `Drawer({ open, onClose, side?, children })` — right drawer, radius 12 on inner corners; for sub-tasks/isolated context. `side: 'left'|'right'` (default right).
- `Menu({ trigger, items })` — popover, radius 10, shadow-lg; items may be `tone:'danger'` (red text).
- `toast(message, opts?)` — dark bubble `#33302E`, radius 11; imperative; auto-dismiss. Provide `<ToastHost/>` mounted once.

- [ ] **Step 1: Failing test** — Modal hidden when `open=false`, visible + focus-trapped + Esc calls onClose when open; Menu opens on trigger click and fires item onSelect; `toast()` renders then removes after timeout (use `vi.useFakeTimers`).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** using a portal to `document.body`. Respect `prefers-reduced-motion`.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): overlays (Modal/Drawer/Menu/toast)"`

---

### Task 9: Skeletons

**Files:**
- Create: `apps/web/src/ui/Skeleton.tsx`
- Test: `apps/web/test/ui/Skeleton.test.tsx`
- Modify: `apps/web/src/index.css` (add `@keyframes mk-shim` + `.mk-skeleton`)

**Interfaces (§15):** `Skeleton({ w?, h?, radius? })`, `SkeletonText({ lines })`, `SkeletonCard()`, `SkeletonRow()`. Shimmer keyframe:

```css
.mk-skeleton { background:linear-gradient(90deg,#F1EBE3 25%,#F8F4EE 50%,#F1EBE3 75%); background-size:200px 100%; animation:mk-shim 1.3s linear infinite; }
@keyframes mk-shim { 0%{background-position:-200px 0} 100%{background-position:200px 0} }
```

- [ ] **Step 1: Failing test** — `SkeletonText lines={3}` renders 3 bars, each with `.mk-skeleton`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): skeleton loaders"`

---

### Task 10: 豆豆 Pebble — four states

**Files:**
- Create: `apps/web/src/ui/Pebble.tsx`
- Test: `apps/web/test/ui/Pebble.test.tsx`
- Modify: `apps/web/src/index.css` (add pebble keyframes: `mk-blink`, `mk-pulse`, `mk-dart`, `mk-spin`, `mk-pop`)

**Interfaces (§14):** `Pebble({ state?, size? })` — `state: 'idle'|'thinking'|'generating'|'processing'|'done'` (default `'idle'`), `size` px (default 28). Body path + eyes copied verbatim from `docs/design/design-system-mockups/pebble-states.html`. Body `fill: var(--mk-accent-500)` (follows student accent). At `size<=28`, eyes enlarge to `rx2.1 ry2.7` and drop catchlights (spec §14). Each state toggles the mockup's keyframes; `prefers-reduced-motion` → static idle.

**Geometry (verbatim, viewBox `0 0 40 40`):**
- Body: `M20 6 C 30 5 36.5 11 35.5 21 C 34.5 30 27 35.5 18.5 34.5 C 9.5 33.5 4.5 27 5.5 18 C 6.5 10 12 7 20 6 Z`
- Eyes (≤28px): left `ellipse cx16 cy21 rx2 ry2.6`, right `cx25 cy20.4 rx2 ry2.6`, `fill #2A2724`.
- Keyframes: copy `blink 3s`, `pulse 1.8s`, `dart 1.4s`, `spin .9s`, `pop 2.4s` from the mockups (`pebble-states.html`); use `transform-box:fill-box; transform-origin:center` for eye transforms and `transform-box:view-box; transform-origin:20px 20px` for the processing ring.

- [ ] **Step 1: Failing test** — `Pebble` renders an svg with the body path `d` starting `M20 6`; state `done` applies the `mk-pop`-bearing class; body fill references `var(--mk-accent-500)`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — inline SVG + state→className; add keyframes to index.css. Copy motion values verbatim from the mockup; do not re-derive.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): 豆豆 Pebble character — four states"`

---

### Task 11: Pebble loaders (progress rider / inline / rabbit-hole)

**Files:**
- Create: `apps/web/src/ui/loaders.tsx`
- Test: `apps/web/test/ui/loaders.test.tsx`
- Modify: `apps/web/src/index.css` (add `mk-fill`, `mk-ride`, `mk-hop` keyframes)
- Modify: `apps/web/src/ui/Button.tsx` (use `PebbleInlineSpinner` for `loading`)

**Interfaces (§15):**
- `PebbleProgress({ value?, slim? })` — 豆豆 rides ON TOP of an accent fill bar (rail `--mk-accent-100`). No `value` → indeterminate (looping `mk-fill`/`mk-ride`); with `value` (0–100) → fill width = value, 豆豆 parked at the fill head. Verbatim from `pebble-progress-v2.html` (rider `bottom:9px`, ground shadow, `ride` rotate 0→660).
- `PebbleInlineSpinner({ size? })` — 豆豆 rolls in place, `mk-spin 1s linear`, size 20–26.
- `RabbitHoleLoader()` — ground hole (rim/dark/豆豆/lip z-order 1/2/3/4) + carrot; `mk-hop 2.9s`. Verbatim from `pebble-rabbithole-v2.html`.

- [ ] **Step 1: Failing test** — `PebbleProgress value={40}` sets fill element width to `40%`; `PebbleInlineSpinner` renders the pebble body path; `RabbitHoleLoader` renders the carrot path + hole layers.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement**, copying SVG + keyframes verbatim from the two mockups (`pebble-progress-v2.html`, `pebble-rabbithole-v2.html`). Wire `Button loading` to `PebbleInlineSpinner`.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): Pebble loaders (progress rider / inline / rabbit-hole)"`

---

### Task 12: Illustration + EmptyState

**Files:**
- Create: `apps/web/src/ui/Illustration.tsx`
- Test: `apps/web/test/ui/Illustration.test.tsx`

**Interfaces (§16):**
- `Illustration({ name, tone?, className })` — `name` maps to a bundled SVG in `apps/web/src/assets/illustrations/`; imported as URL and rendered in `<img>`. `tone` (a macaron/accent name) documents intended recolor (recolor pipeline is a follow-up; for now render as-is with `tone` stored as data-attr).
- `EmptyState({ illustration, title, body, action })` — illustration + title + one line + one primary action (spec §16 empty-state pattern). Copy tone per §19 (lively, e.g. "快来创建你的第一个写作项目吧！").

Map the 10 undraw + 6 Humaaans files to `name` keys (e.g. `bookLover`, `emptyProjects`, `reading`, `warren`, `writing`, `focus`, `completed`, `loading`, `sent`, `questions`; humaaans `standing20`, `sitting5`…).

- [ ] **Step 1: Failing test** — `EmptyState` renders title, body, and the action button; clicking action fires callback; `Illustration name="reading"` renders an `<img>`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Use `import url from "../assets/illustrations/....svg"` (Vite asset URL).
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): Illustration + EmptyState"`

---

### Task 13: Barrel export + `?ds` gallery + wire route

**Files:**
- Create: `apps/web/src/ui/index.ts` (barrel)
- Create: `apps/web/src/dev/DesignSystemGallery.tsx`
- Modify: `apps/web/src/Root.tsx` (add `?ds` branch)
- Test: `apps/web/test/ui/gallery.test.tsx`

**Interfaces:**
- `ui/index.ts` re-exports every primitive + `tokens` + `accent`.
- `DesignSystemGallery` renders, in labeled sections wrapped in `<AccentProvider>` with an accent-picker row: color swatches (accent scale, macaron, semantic), type scale, buttons (all variants×sizes×states), cards/surfaces, every form control, feedback set, overlays (open-on-click), skeletons, Pebble ×4 states, all 3 loaders, EmptyState samples, illustrations grid.
- `Root.tsx`: `if (params.has("ds")) return <DesignSystemGallery/>;`

- [ ] **Step 1: Failing test** — `DesignSystemGallery` renders without throwing and shows an "设计系统" heading + at least one `Pebble` svg + the accent-picker (8 swatches).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** gallery + barrel + Root route.
- [ ] **Step 4: Run → PASS; typecheck; build.** Then `npm --prefix apps/web run dev` and manually confirm `?ds` renders (the human/controller does the visual check).
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): ui barrel + ?ds design-system gallery"`

---

## Self-Review Notes

- **Spec coverage:** §1 colors→T1/T2; §2 type→T1; §3 spacing/radius/shadow→T1; §4 motion→T1 (tokens) + per-component; §5 icons→T3; §6 scrollbar→T1; §7 buttons→T4; §8 forms→T6; §9 feedback→T7; §10 cards→T5; §12 overlays→T8; §13 chat composer chips → **deferred to Part 3** (studio-specific, not a generic primitive); §14 Pebble→T10; §15 loaders→T9+T11; §16 illustrations→T12; §17/§18 layout/graph → **Parts 2/3**; §19 voice → applied in EmptyState copy, full sweep in Part 4.
- **Type consistency:** `AccentId` used identically in T2 + T13; `MacaronName` in T1 + T12; `Pebble` state enum shared T10/T13.
- **Deferred by design (not gaps):** chat/composer/summon-chips (Part 3), graph tokens (Part 3), accent backend persistence (Part 2), undraw automated recolor (Part 4 / open item), 「还没有项目」bespoke illustration (spec open item).
