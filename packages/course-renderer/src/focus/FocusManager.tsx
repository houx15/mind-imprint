import { createContext, useContext, useEffect, useRef } from "react";
import type { ReactNode } from "react";
import type { TargetRef } from "@mind-imprint/course-contract";

/**
 * §17 focus actions — the current focus target for the active slice, or `null`
 * when nothing is focused. SlicePlayer sets this from `focus` / `clearFocus`
 * effects; block/item wrappers read it and highlight themselves when they match.
 */
const FocusContext = createContext<TargetRef | null>(null);

export const FocusProvider = FocusContext.Provider;

export function useCurrentFocus(): TargetRef | null {
  return useContext(FocusContext);
}

/** True when a block-LEVEL focus (no `itemId`) targets `blockId`. */
export function isBlockFocused(focus: TargetRef | null, blockId: string): boolean {
  return focus !== null && focus.blockId === blockId && !("itemId" in focus);
}

/**
 * The item id focused inside `blockId`, or `undefined`. Used by SlicePlayer to
 * derive a block renderer's `focusedItemId` prop from the current focus.
 */
export function focusedItemIdFor(focus: TargetRef | null, blockId: string): string | undefined {
  if (focus !== null && focus.blockId === blockId && "itemId" in focus) return focus.itemId;
  return undefined;
}

export interface FocusTargetProps {
  blockId: string;
  /** When given, this wrapper represents an item inside the block, not the block. */
  itemId?: string;
  className?: string;
  children: ReactNode;
}

/** True when the environment asks for no non-essential motion (§P2-06 scroll behavior). */
function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * §17 / §P2-06 — the visible + assistive-tech emphasis for a `.course-focus-ring`
 * match: an outline plus enough offset to read clearly against any block content.
 * Minimal and inline for now; Slice 5 folds this into the course stylesheet proper.
 */
const FOCUS_RING_CSS = `
.course-focus-ring {
  outline: 3px solid #2f6feb;
  outline-offset: 3px;
  border-radius: 4px;
}
`;

let focusRingStyleInjected = false;

/**
 * Injects `.course-focus-ring` once per document (idempotent — safe to call from
 * every `SlicePlayer` mount without piling up duplicate `<style>` tags).
 */
function ensureFocusRingStyle(): void {
  if (focusRingStyleInjected || typeof document === "undefined") return;
  const style = document.createElement("style");
  style.setAttribute("data-course-focus-ring", "");
  style.textContent = FOCUS_RING_CSS;
  document.head.appendChild(style);
  focusRingStyleInjected = true;
}

/**
 * Wraps a block (or an item within it) and, when the current focus matches:
 * sets `data-focused="true"` + the `.course-focus-ring` class, moves real DOM
 * (keyboard/assistive-tech) focus onto the wrapper (§P2-06 — focus was
 * previously visual metadata only), and scrolls it into view. The wrapper
 * carries `tabIndex={-1}` so it's a valid programmatic focus target even when
 * the block it wraps has no natively focusable element, and an `aria-label`
 * so a screen reader has something to announce on the focus move. Losing focus
 * (the match no longer holds — e.g. a `clearFocus` effect) blurs the wrapper
 * if it still holds DOM focus, so emphasis doesn't linger past its target.
 */
export function FocusTarget({ blockId, itemId, className, children }: FocusTargetProps) {
  const focus = useCurrentFocus();
  const focused =
    itemId === undefined
      ? isBlockFocused(focus, blockId)
      : focus !== null && focus.blockId === blockId && "itemId" in focus && focus.itemId === itemId;

  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    ensureFocusRingStyle();
  }, []);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (focused) {
      el.focus({ preventScroll: true });
      el.scrollIntoView({ block: "nearest", behavior: prefersReducedMotion() ? "auto" : "smooth" });
    } else if (document.activeElement === el) {
      el.blur();
    }
  }, [focused]);

  return (
    <div
      ref={ref}
      data-focus-block={blockId}
      data-focus-item={itemId}
      data-focused={focused ? "true" : undefined}
      tabIndex={-1}
      aria-label={itemId ? `${blockId}-${itemId}` : blockId}
      className={[className, focused ? "course-focus-ring" : null].filter(Boolean).join(" ") || undefined}
    >
      {children}
    </div>
  );
}
