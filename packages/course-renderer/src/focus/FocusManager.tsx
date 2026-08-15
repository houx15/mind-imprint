import { createContext, useContext } from "react";
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

/**
 * Wraps a block (or an item within it) and sets `data-focused="true"` + a visible
 * focus-ring class when the current focus matches. A block-level wrapper matches
 * only a block-level focus; an item wrapper matches only its own item.
 */
export function FocusTarget({ blockId, itemId, className, children }: FocusTargetProps) {
  const focus = useCurrentFocus();
  const focused =
    itemId === undefined
      ? isBlockFocused(focus, blockId)
      : focus !== null && focus.blockId === blockId && "itemId" in focus && focus.itemId === itemId;
  return (
    <div
      data-focus-block={blockId}
      data-focus-item={itemId}
      data-focused={focused ? "true" : undefined}
      className={[className, focused ? "course-focus-ring" : null].filter(Boolean).join(" ") || undefined}
    >
      {children}
    </div>
  );
}
