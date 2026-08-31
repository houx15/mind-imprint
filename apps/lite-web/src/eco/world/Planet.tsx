import { useState } from "react";
import { DOMAIN_META } from "../data/news";
import type { Lang, NewsItem } from "../data/types";
import { cx } from "../ui";

/**
 * A news planet.
 *
 * Three states, and the order they reveal in is the whole design:
 *  - **veiled** — she has not opened it. Only the field glyph and a colour.
 *    No title. It is a thing seen from far away.
 *  - **hovered** — the HOOK QUESTION appears. Not the headline. The question
 *    is the invitation; a headline would let her decide she already knows.
 *  - **discovered** — lit, titled, and marked. It stays that way for the day.
 *
 * `dimmed` is the field filter: non-matching planets recede rather than
 * disappearing, so the filter never makes the sky feel broken.
 */
export function Planet({
  item,
  lang,
  slot,
  discovered,
  kept,
  dimmed,
  onOpen,
}: {
  item: NewsItem;
  lang: Lang;
  slot: { x: string; y: string; size: number; drift: string };
  discovered: boolean;
  kept: boolean;
  dimmed: boolean;
  onOpen: () => void;
}) {
  const [hover, setHover] = useState(false);
  const meta = DOMAIN_META[item.domain];
  const showHook = hover && !dimmed;

  return (
    <div
      className={cx("absolute", slot.drift)}
      style={{
        left: slot.x,
        top: slot.y,
        transform: "translate(-50%,-50%)",
        opacity: dimmed ? 0.24 : 1,
        transition: "opacity 260ms cubic-bezier(.2,0,0,1)",
        zIndex: hover ? 15 : 10,
      }}
    >
      {/* The button's box is EXACTLY the sphere. The rank tick and the label
          are positioned against it, so they can never drift the way they do
          when a wide label block is part of the flow inside the button. */}
      <button
        type="button"
        onClick={onOpen}
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
        onFocus={() => setHover(true)}
        onBlur={() => setHover(false)}
        className="eco-planet group relative block cursor-pointer focus-visible:outline-none"
        style={{ width: slot.size, height: slot.size }}
        aria-label={
          discovered
            ? `${item.title[lang]} — ${meta[lang === "zh" ? "zh" : "en"]}`
            : `还没看过的 ${meta.zh} 行星，点击打开`
        }
      >
        {/* the sphere */}
        <span
          className={cx(
            "eco-planet-body eco-planet-spin block overflow-hidden",
            !discovered && "eco-veil",
            discovered && "eco-lit",
          )}
          style={{
            width: slot.size,
            height: slot.size,
            background: `radial-gradient(circle at 34% 30%, color-mix(in srgb, ${meta.hue} 88%, #FFFFFF), ${meta.hue} 58%, color-mix(in srgb, ${meta.hue} 62%, #17130F))`,
            boxShadow: discovered
              ? `0 0 0 1px rgba(255,255,255,.16), 0 18px 46px color-mix(in srgb, ${meta.hue} 34%, transparent), 0 0 70px color-mix(in srgb, ${meta.hue} 26%, transparent)`
              : "0 0 0 1px rgba(255,255,255,.08), 0 12px 30px rgba(0,0,0,.4)",
          }}
        >
          <span
            className="absolute inset-0 flex items-center justify-center font-mono font-bold"
            style={{
              fontSize: Math.round(slot.size * 0.26),
              color: "rgba(23,19,15,.5)",
              textShadow: "0 1px 0 rgba(255,255,255,.28)",
            }}
            aria-hidden
          >
            {meta.glyph}
          </span>
        </span>

        {/* rank tick — a tiny instrument reading on the rim */}
        <span
          className="eco-mono absolute -right-1 -top-1 flex h-6 w-6 items-center justify-center rounded-mk-full"
          style={{
            background: "#17130F",
            border: "1px solid rgba(240,233,224,.28)",
            color: "#D8CCBD",
            letterSpacing: 0,
          }}
        >
          {item.rank}
        </span>

        {kept ? (
          <span
            className="absolute -left-1 -top-1 flex h-6 w-6 items-center justify-center rounded-mk-full text-[12px]"
            style={{ background: "var(--mk-matcha)", color: "#17130F" }}
            title="已收进你的树"
          >
            ✓
          </span>
        ) : null}

        {/* label under the sphere */}
        <span
          className="absolute left-1/2 block w-[220px] -translate-x-1/2 text-center"
          style={{ top: "calc(100% + 10px)" }}
        >
          {/* The editorial weight (0.85) used to print here. It told a student
              nothing she could act on — a bare decimal with no scale and no
              units. The field name is what actually helps her choose. */}
          <span className="eco-mono block" style={{ color: "#8E8175" }}>
            {lang === "zh" ? meta.zh : meta.en}
          </span>
          <span
            className="mt-1 block text-mk-small leading-snug"
            style={{
              color: discovered ? "#F0E9E0" : "#6E6459",
              fontWeight: discovered ? 600 : 400,
            }}
          >
            {discovered ? item.title[lang] : "还没看过"}
          </span>
        </span>

        {/* the hook — appears on hover, above everything */}
        {showHook ? (
          <span
            className="eco-in pointer-events-none absolute left-1/2 z-30 w-[290px] -translate-x-1/2 rounded-mk-lg p-4 text-left"
            style={{
              bottom: `calc(100% + 14px)`,
              background: "rgba(28,23,19,.94)",
              border: "1px solid rgba(240,233,224,.18)",
              boxShadow: "0 26px 60px rgba(0,0,0,.5)",
              backdropFilter: "blur(12px)",
            }}
          >
            <span className="eco-mono block" style={{ color: "#8E8175" }}>
              它想问你
            </span>
            <span
              className="mt-1.5 block text-mk-body-lg font-semibold leading-[1.6]"
              style={{ color: "#F5EFE7" }}
            >
              {item.hook[lang]}
            </span>
            <span className="mt-2.5 block text-mk-small" style={{ color: "#9A8E80" }}>
              {discovered ? item.title[lang] : "点开看看它是什么"}
            </span>
          </span>
        ) : null}
      </button>
    </div>
  );
}
