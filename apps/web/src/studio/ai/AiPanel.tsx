import type { ReactNode } from "react";
import { ArrowLeftRight, ChevronLeft, ChevronRight } from "lucide-react";
import { Icon } from "@/ui/Icon";
import { Pebble } from "@/ui/Pebble";

/**
 * AiPanel (studio agentic rebuild, spec §17).
 *
 * The CONSTANT AI panel chrome that sits beside the studio work area —
 * WorkspaceContainer (Task 4) renders this on whichever edge `side` says,
 * and each room fills `children` with its coach content (shared
 * `ChatLog`/`Composer` from Task 1). Purely controlled: flip/collapse
 * intents are reported via `onFlip`/`onToggleCollapse`, the actual state
 * (side + collapsed) lives in the parent.
 *
 * GOTCHA (same as Card.tsx/ChatLog.tsx/Composer.tsx): exactly ONE class per
 * competing CSS property — Tailwind's compiled stylesheet order is
 * alphabetical, not className order.
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export type AiPanelSide = "left" | "right";

export interface AiPanelProps {
  side: AiPanelSide;
  onFlip: () => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
  title?: string;
  children?: ReactNode;
}

const PANEL_BASE = "flex h-full shrink-0 flex-col bg-mk-surface transition-[width] duration-[var(--mk-base)] ease-mk";

const ICON_BUTTON_BASE =
  "flex shrink-0 items-center justify-center rounded-mk-sm p-1 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-paper hover:text-mk-ink";

export function AiPanel({ side, onFlip, collapsed, onToggleCollapse, title = "印记", children }: AiPanelProps) {
  const borderClass = side === "right" ? "border-l" : "border-r";

  if (collapsed) {
    // Chevron points inward — toward the work area — inviting expansion.
    const ExpandIcon = side === "right" ? ChevronLeft : ChevronRight;
    return (
      <div className={cx(PANEL_BASE, borderClass, "border-mk-border", "w-[48px] items-center gap-2 py-3")}>
        <Pebble size={24} />
        <button type="button" aria-label="展开 AI 面板" onClick={onToggleCollapse} className={ICON_BUTTON_BASE}>
          <Icon icon={ExpandIcon} size={16} />
        </button>
      </div>
    );
  }

  // Chevron points outward — toward the panel's own edge — inviting collapse.
  const CollapseIcon = side === "right" ? ChevronRight : ChevronLeft;
  return (
    <div className={cx(PANEL_BASE, borderClass, "border-mk-border", "w-[320px]")}>
      <div className="flex shrink-0 items-center gap-2 border-b border-mk-border px-4 py-3">
        <Pebble size={24} />
        <span className="flex-1 truncate text-mk-h3">{title}</span>
        <button type="button" aria-label="切换 AI 面板左右" onClick={onFlip} className={ICON_BUTTON_BASE}>
          <Icon icon={ArrowLeftRight} size={16} />
        </button>
        <button type="button" aria-label="折叠 AI 面板" onClick={onToggleCollapse} className={ICON_BUTTON_BASE}>
          <Icon icon={CollapseIcon} size={16} />
        </button>
      </div>
      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto bg-mk-paper">{children}</div>
    </div>
  );
}
