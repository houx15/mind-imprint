import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { Icon, Pebble } from "@/ui";
import type { ReadingBlockTool } from "../api/readingRoom";

/**
 * BlockToolbar — 点一段，工具就浮在你手边。
 *
 * REPLACES the 详细带读 button that used to hang under every paragraph. That
 * button had two problems and the ruling named both:
 *
 *   > the 详细带读 button is still not like a button that can click and never
 *   > get noticed. think about those, when click some texts, there appears a
 *   > 横着的一栏，有一些按钮，在我的鼠标附近, so that I know those are
 *   > possible operations.
 *
 * So the gesture is now the paragraph itself, and the answer arrives where she
 * is already looking — a single horizontal row of operations, pinned near the
 * point she clicked, with 豆豆 on the left so it reads as 印记 offering them
 * rather than as chrome that was always there.
 *
 * ## Positioning
 *
 * `position: fixed` at her pointer's x, vertically clamped to the paragraph:
 * above it when there is room, below it when there isn't. It re-measures on
 * scroll and resize from the paragraph's own rect, so it stays glued to the
 * paragraph she opened instead of floating away the moment the article moves
 * — a bar that detaches on the first scroll is worse than no bar.
 *
 * ## Dismissal
 *
 * Escape, a click outside it, or picking the paragraph again. It does NOT
 * close when a tool runs: reading one explanation and then wanting the next
 * one is the common case, and closing under her would make her click twice
 * for every tool after the first.
 */

const BAR_H = 44;
const GAP = 10;

export function BlockToolbar({
  anchorEl,
  pointerX,
  tools,
  openedTools,
  busyTool,
  activeTool,
  onPick,
  onClose,
}: {
  /** The paragraph this bar belongs to; it is measured, never mutated. */
  anchorEl: HTMLElement;
  /** Where her pointer went down, in viewport coordinates. */
  pointerX: number;
  tools: ReadingBlockTool[];
  /** Tools already fetched for this paragraph — shown as hers, not as new. */
  openedTools: Set<string>;
  busyTool: string | null;
  activeTool: string | null;
  onPick: (tool: ReadingBlockTool) => void;
  onClose: () => void;
}) {
  const barRef = useRef<HTMLDivElement | null>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  // Measured after paint (the bar's own width decides the horizontal clamp),
  // then kept in sync with whatever moves the article underneath it.
  useLayoutEffect(() => {
    function place() {
      const rect = anchorEl.getBoundingClientRect();
      const width = barRef.current?.offsetWidth ?? 320;
      const left = Math.min(Math.max(12, pointerX - width / 2), window.innerWidth - width - 12);
      const above = rect.top - BAR_H - GAP;
      const top = above >= 12 ? above : Math.min(rect.bottom + GAP, window.innerHeight - BAR_H - 12);
      setPos({ left, top });
    }
    place();
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [anchorEl, pointerX, tools.length]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    function onDown(e: PointerEvent) {
      const bar = barRef.current;
      if (!bar) return;
      const target = e.target as Node | null;
      // A click back on the same paragraph is the paragraph's own gesture to
      // handle (it toggles), so it must not ALSO be read as "outside".
      if (target && (bar.contains(target) || anchorEl.contains(target))) return;
      onClose();
    }
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onDown, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onDown, true);
    };
  }, [anchorEl, onClose]);

  return (
    <div
      ref={barRef}
      role="toolbar"
      aria-label="这一段可以怎么拆"
      className="mk-blockbar fixed z-50 flex items-center gap-1 rounded-mk-full border border-mk-border bg-mk-surface py-1 pl-2 pr-1 shadow-mk-md"
      style={{
        left: pos?.left ?? -9999,
        top: pos?.top ?? -9999,
        visibility: pos ? "visible" : "hidden",
      }}
    >
      <span className="flex shrink-0 items-center pr-0.5">
        <Pebble state={busyTool ? "thinking" : "idle"} size={26} />
      </span>

      <span aria-hidden="true" className="mr-0.5 h-5 w-px shrink-0 bg-mk-border" />

      {tools.map((tool) => {
        const isActive = activeTool === tool.id;
        const opened = openedTools.has(tool.id);
        return (
          <button
            key={tool.id}
            type="button"
            onClick={() => onPick(tool)}
            disabled={busyTool !== null}
            aria-pressed={isActive}
            className={[
              "shrink-0 whitespace-nowrap rounded-mk-full px-2.5 py-1 text-mk-small transition-colors duration-[120ms] ease-mk",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
              "disabled:cursor-not-allowed disabled:opacity-60",
              isActive
                ? "text-white"
                : opened
                  ? "text-mk-accent-700 hover:bg-mk-accent-50"
                  : "text-mk-secondary hover:bg-mk-accent-50 hover:text-mk-accent-700",
            ].join(" ")}
            style={isActive ? { background: "var(--mk-accent-500)" } : undefined}
          >
            {busyTool === tool.id ? "…" : tool.label}
          </button>
        );
      })}

      <span aria-hidden="true" className="ml-0.5 h-5 w-px shrink-0 bg-mk-border" />

      <button
        type="button"
        aria-label="关掉这一栏"
        onClick={onClose}
        className="shrink-0 rounded-mk-full p-1.5 text-mk-faint transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <Icon icon={X} size={14} />
      </button>
    </div>
  );
}
