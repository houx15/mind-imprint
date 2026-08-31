import { useState } from "react";
import { Check, ImageIcon } from "lucide-react";
import { useEco } from "../store";
import { COVER_ARTS, COVER_GLYPHS } from "../data/projects";
import type { Cover, Project } from "../data/types";
import { Sys, cx } from "../ui";

/**
 * 封面.
 *
 * ## Why a gradient and a glyph rather than an image upload
 * A student who has to go and find a picture before her project looks like a
 * project has been handed a chore, and the shelf stops being uniform the
 * moment one card is a blurry photo taken at an angle. Two taps — a ground and
 * a mark — get the thing that actually matters: **the shelf reads as hers**,
 * and three projects side by side look like a body of work rather than three
 * rows in a table.
 *
 * It is also the only purely aesthetic choice in the whole project flow, which
 * is worth having on purpose. Everything else asks her to justify herself;
 * this one is allowed to just be a colour she likes.
 */
export function CoverArt({
  cover,
  size = "md",
  className,
}: {
  cover: Cover;
  size?: "sm" | "md" | "lg";
  className?: string;
}) {
  const art = COVER_ARTS[cover.art] ?? COVER_ARTS[0]!;
  const px = size === "sm" ? 40 : size === "lg" ? 96 : 64;
  return (
    <span
      className={cx("flex shrink-0 items-center justify-center rounded-mk-md", className)}
      style={{
        width: px,
        height: px,
        background: `linear-gradient(145deg, ${art.from}, ${art.to})`,
        color: art.ink,
        fontSize: Math.round(px * 0.42),
        lineHeight: 1,
        boxShadow: "inset 0 1px 0 rgba(255,255,255,.24)",
      }}
      aria-hidden
    >
      {cover.glyph}
    </span>
  );
}

/** The picker. Opens in place; there is no save button because there is
 *  nothing to get wrong — every tap is already the new cover. */
export function CoverPicker({ project }: { project: Project }) {
  const { setCover } = useEco();
  const [open, setOpen] = useState(false);

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-mk-sm px-2 py-1 text-mk-small text-mk-muted
                   transition-colors duration-[120ms] hover:bg-mk-paper hover:text-mk-ink
                   focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <ImageIcon size={13} strokeWidth={1.9} />
        换封面
      </button>

      {open ? (
        <>
          {/* Click-away. A picker that only closes via its own button strands
              people who tapped it by accident. */}
          <button
            type="button"
            aria-label="关闭"
            onClick={() => setOpen(false)}
            className="fixed inset-0 z-40 cursor-default"
          />
          <div
            className="eco-in absolute right-0 top-full z-50 mt-2 w-[280px] rounded-mk-lg border
                       border-mk-border bg-mk-surface p-4 shadow-mk-lg"
          >
            <Sys>底色</Sys>
            <div className="mt-2 grid grid-cols-8 gap-1.5">
              {COVER_ARTS.map((a, i) => (
                <button
                  key={a.from}
                  type="button"
                  aria-label={`底色 ${i + 1}`}
                  onClick={() => setCover(project.id, { ...project.cover, art: i })}
                  className="relative h-6 w-6 rounded-mk-sm transition-transform duration-[120ms]
                             hover:scale-110 focus-visible:outline-none focus-visible:ring-2
                             focus-visible:ring-mk-accent-200"
                  style={{ background: `linear-gradient(145deg, ${a.from}, ${a.to})` }}
                >
                  {project.cover.art === i ? (
                    <Check
                      size={12}
                      strokeWidth={3}
                      color={a.ink}
                      className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2"
                    />
                  ) : null}
                </button>
              ))}
            </div>

            <Sys className="mt-4 block">图形</Sys>
            <div className="mt-2 grid grid-cols-8 gap-1.5">
              {COVER_GLYPHS.map((g) => (
                <button
                  key={g}
                  type="button"
                  aria-label={`图形 ${g}`}
                  onClick={() => setCover(project.id, { ...project.cover, glyph: g })}
                  className={cx(
                    "flex h-6 w-6 items-center justify-center rounded-mk-sm text-[13px]",
                    "transition-colors duration-[120ms] focus-visible:outline-none",
                    "focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                    project.cover.glyph === g
                      ? "bg-mk-accent-50 font-bold text-mk-accent-700"
                      : "text-mk-secondary hover:bg-mk-paper",
                  )}
                >
                  {g}
                </button>
              ))}
            </div>

            <div className="mt-4 flex items-center gap-3 border-t border-mk-border pt-3">
              <CoverArt cover={project.cover} />
              <p className="text-mk-small leading-[1.7] text-mk-muted">
                它会出现在项目列表和你的主页上。
              </p>
            </div>
          </div>
        </>
      ) : null}
    </div>
  );
}
