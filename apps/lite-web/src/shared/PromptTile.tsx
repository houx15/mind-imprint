import { ArrowUpRight } from "lucide-react";

/**
 * PromptTile — the tile on both landings' 「不知道读什么？」/「不知道写什么？」
 * shelves.
 *
 * ONE component for both, rather than the two near-identical module-private
 * copies the landings used to carry. They were always the same object with a
 * different data shape behind it, and keeping two of them is how the two
 * pages quietly stop looking like the same product.
 *
 * The 2026-08-27 restyle, in the product's own words: *more technical-feeling,
 * instead of a bad left-border line box.* What went, and why:
 *
 *   - **The 3px coloured left stripe.** It read as a highlighter mark on a
 *     document — a stationery gesture, not an instrument one. Colour now
 *     survives as a single small square in the meta row: enough to tell the
 *     tiles apart at a glance, not enough to decorate them.
 *   - **Lift-and-shadow on hover.** A card that floats toward you is a
 *     brochure. Hover now moves one hairline: the top rule lights up in the
 *     accent and the corner arrow resolves. The tile stays exactly where it
 *     is.
 *   - **The pill-shaped tag.** Replaced by a monospaced, letter-spaced label
 *     next to a zero-padded index. Mono numerals are what make a surface read
 *     as an instrument rather than a poster, and they cost nothing.
 *
 * Everything here is CSS on tokens — no new asset, no image, nothing that can
 * fail to load.
 */

export type PromptTone = "peach" | "matcha" | "lake" | "taro";

export function PromptTile({
  index,
  tag,
  title,
  reason,
  tone,
  disabled,
  onPick,
}: {
  /** 1-based position on the shelf; rendered zero-padded as 01, 02, … */
  index: number;
  tag: string;
  title: string;
  reason: string;
  tone: PromptTone;
  disabled: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onPick}
      disabled={disabled}
      className="group relative flex flex-col items-start gap-2 overflow-hidden rounded-mk-sm border border-mk-border bg-mk-surface px-4 pb-4 pt-3.5 text-left transition-colors duration-200 ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed disabled:opacity-60 motion-reduce:transition-none"
    >
      {/* The one moving part: a hairline along the top edge, drawn from the
          left on hover. `scaleX` rather than a width change so it never
          reflows anything, and `motion-reduce` drops the animation but keeps
          the colour change so the affordance survives either way. */}
      <span
        aria-hidden="true"
        className="absolute inset-x-0 top-0 h-px origin-left scale-x-0 transition-transform duration-300 ease-mk group-hover:scale-x-100 motion-reduce:transition-none motion-reduce:group-hover:scale-x-100"
        style={{ background: "var(--mk-accent-500)" }}
      />

      <span className="flex w-full items-center gap-2">
        <span
          aria-hidden="true"
          className="h-[7px] w-[7px] shrink-0 rounded-[1px]"
          style={{ background: `var(--mk-${tone})` }}
        />
        <span
          className="font-mono text-[11px] leading-none text-mk-faint"
          style={{ fontVariantNumeric: "tabular-nums", letterSpacing: "0.08em" }}
        >
          {String(index).padStart(2, "0")}
        </span>
        <span aria-hidden="true" className="h-px w-3 bg-mk-border" />
        <span
          className="font-mono text-[11px] leading-none text-mk-muted"
          style={{ letterSpacing: "0.12em" }}
        >
          {tag}
        </span>
        <ArrowUpRight
          aria-hidden="true"
          size={14}
          className="ml-auto shrink-0 text-mk-faint opacity-0 transition-opacity duration-200 ease-mk group-hover:opacity-100 motion-reduce:transition-none"
        />
      </span>

      <span className="text-mk-h3 leading-snug text-mk-ink">{title}</span>
      <span className="text-mk-small leading-relaxed text-mk-muted">{reason}</span>
    </button>
  );
}
