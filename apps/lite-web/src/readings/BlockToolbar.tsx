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
 * `position: fixed` at her pointer's x, dropped into the BLANK BAND between
 * the paragraph and its neighbour — above it when there is room, below it when
 * there isn't. It re-measures on scroll and resize from the paragraph's own
 * rect, so it stays glued to the paragraph she opened instead of floating away
 * the moment the article moves — a bar that detaches on the first scroll is
 * worse than no bar.
 *
 * 🚨 **It must never sit on top of the words.** The first version clamped the
 * below-branch into the viewport (`Math.min(rect.bottom + GAP, innerHeight -
 * BAR_H - 12)`), which, once a paragraph's bottom edge had scrolled past the
 * fold, dragged the bar back UP into the middle of that paragraph — covering
 * 「光伏板的寿命通[bar]常有二十五年。」, i.e. the text, not the gutter
 * (`.deploy-local/card-eyeball-2026-08-29/_toolbar-overlap.png`). And even
 * unclamped, hugging the paragraph's top edge covers the LAST LINE OF THE
 * PARAGRAPH ABOVE, because the bar is taller than the margin between them.
 * `placeBar` below solves both by centring the bar on the blank band; see its
 * own comment for why glyphs, not boxes, are the thing being avoided.
 *
 * ## Dismissal
 *
 * Escape, a click outside it, or picking the paragraph again. It does NOT
 * close when a tool runs: reading one explanation and then wanting the next
 * one is the common case, and closing under her would make her click twice
 * for every tool after the first.
 */

const BAR_H = 44;
/** Keep the bar this far off the viewport edges. */
const EDGE = 12;
/** Assumed blank band when a paragraph has no measurable neighbour (first or
 *  last in the article). Matches lite's own `margin-bottom` for article
 *  paragraphs — see `.mk-lite-room … p[data-block-id]` in `index.css`. */
const FALLBACK_GUTTER = 28;

export type BarRect = { top: number; bottom: number };
/** The blank bands either side of the paragraph: the bottom of whatever is
 *  above it, and the top of whatever is below it. */
export type BarGutters = { above: number; below: number };

/**
 * Where the bar goes, given the paragraph it belongs to and the viewport.
 *
 * Exported (and pure) so a test can assert the rule that matters without a
 * layout engine.
 *
 * ## Why "outside the paragraph rect" is not the rule
 *
 * It was, for about an hour, and it is not enough. Article paragraphs sit
 * 20–28px apart while the bar is ~34px tall, so a bar hugging the paragraph's
 * top edge lands squarely on the LAST LINE OF THE PARAGRAPH ABOVE — a
 * different paragraph, the same complaint. What the eye actually cares about
 * is glyphs, and `line-height: 1.95` puts ~8px of blank leading inside each
 * paragraph's box at both ends. So the real blank band between two rows of
 * type is `8 + margin + 8` ≈ 44px, and the bar is CENTRED IN IT: it may
 * intrude a few px into either box, but only into leading, never onto a
 * letter.
 *
 * The order of preference:
 *
 *  1. Centred in the blank band above the paragraph, if that lands on screen.
 *  2. Centred in the band below it.
 *  3. Neither band is on screen (she has scrolled the paragraph across the
 *     fold): fall back to strictly outside the paragraph's own box, giving up
 *     the centring rather than the invariant.
 *  4. Degenerate only: the paragraph spans the viewport top to bottom, so no
 *     band outside it is on screen at all. Dock to the edge with more room and
 *     accept the overlap — scrolling one line reinstates (1)/(2).
 */
export function placeBar(
  rect: BarRect,
  gutters: BarGutters,
  viewportH: number,
  height: number,
): number {
  const minTop = EDGE;
  const maxTop = viewportH - height - EDGE;
  if (maxTop < minTop) return minTop;

  const fits = (top: number) => top >= minTop && top <= maxTop;

  const above = (gutters.above + rect.top - height) / 2;
  if (fits(above)) return above;
  const below = (rect.bottom + gutters.below - height) / 2;
  if (fits(below)) return below;

  if (rect.top - height >= minTop) return Math.min(rect.top - height, maxTop);
  if (rect.bottom <= maxTop) return Math.max(rect.bottom, minTop);

  return rect.top - minTop >= maxTop - rect.bottom ? minTop : maxTop;
}

/**
 * The blank band either side of `el`, measured off its real neighbours.
 *
 * `previousElementSibling` is not always a paragraph — `renderAfterBlock`
 * hangs the lens card and the tools panel between them — so a neighbour is
 * only trusted when it is genuinely on the far side; otherwise the paragraph's
 * own margin stands in.
 */
function guttersAround(el: HTMLElement, rect: BarRect): BarGutters {
  const prev = el.previousElementSibling?.getBoundingClientRect();
  const next = el.nextElementSibling?.getBoundingClientRect();
  return {
    above: prev && prev.bottom <= rect.top ? prev.bottom : rect.top - FALLBACK_GUTTER,
    below: next && next.top >= rect.bottom ? next.top : rect.bottom + FALLBACK_GUTTER,
  };
}

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
      const height = barRef.current?.offsetHeight || BAR_H;
      // On a phone the bar can be as wide as the screen; `Math.max` of the
      // two clamps would otherwise push it off the left edge.
      const left = Math.max(EDGE, Math.min(pointerX - width / 2, window.innerWidth - width - EDGE));
      setPos({ left, top: placeBar(rect, guttersAround(anchorEl, rect), window.innerHeight, height) });
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
        // Six chips + 豆豆 + 关闭 is wider than a phone. Capping it here (and
        // scrolling the chips, see `.mk-blockbar` in index.css) is what keeps
        // the horizontal clamp above honest: a bar wider than the screen has
        // no left position that fits.
        maxWidth: `calc(100vw - ${EDGE * 2}px)`,
      }}
    >
      <span className="flex shrink-0 items-center pr-0.5">
        <Pebble state={busyTool ? "thinking" : "idle"} size={26} />
      </span>

      <span aria-hidden="true" className="mr-0.5 h-5 w-px shrink-0 bg-mk-border" />

      {/* Only the chips scroll. 豆豆 and 关掉这一栏 stay pinned to the two
          ends — on a 375px screen the pill is capped at the viewport width,
          and with the whole bar as the scroller the close button ended up
          past the right edge, reachable only by dragging the pill sideways. */}
      <div className="mk-blockbar__tools">
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
      </div>

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
