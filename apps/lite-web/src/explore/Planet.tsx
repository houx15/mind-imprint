import { useState } from "react";
import type { ExplorePlanet } from "../api/explore";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import { cx } from "../tree/ui";

export type Lang = "zh" | "en";

/**
 * A news bubble.
 *
 * ## Why the headline is INSIDE it (2026-08-31)
 * v1 was an opaque planet with its title set underneath, and the title only
 * appeared once she had opened it — discovery as a reveal. Two things were
 * wrong with that. A label under a drifting sphere is a second object that has
 * to be kept clear of the next sphere's label, which is why the stage could
 * only hold five small planets and needed a scrolling list underneath to be
 * readable at all. And a sky of five unlabelled circles asks a student to
 * click blind: the "reveal" she is being offered is the answer to *what is
 * this*, which is not a question worth making her ask.
 *
 * So a bubble carries its own headline now, and the stage stands on its own.
 * What is still held back is the thing worth holding back — the HOOK, the
 * question the story puts to her, which surfaces on hover.
 *
 * ## Three states
 *  - **unread** — glass: translucent, the field's colour in the rim, the
 *    headline legible but quiet.
 *  - **hovered** — the hook question rises above it.
 *  - **read** — lit from inside, and marked. It stays that way for the day.
 *
 * `dimmed` is the field filter: non-matching bubbles recede rather than
 * disappearing, so the filter never makes the sky feel broken.
 *
 * ## 颜色 = 它会长在树的哪根枝上（2026-09-03）
 * 原型按八个「领域」（科技 / 科学 / 环境 …）上色，那套分类只活在这一屏里。
 * 现在按**七根主枝**上色，和兴趣树用同一套颜色 —— 于是一颗蓝色的星球和树上
 * 那根蓝色的枝是同一件事，收藏它就是往那根枝上加一个词。一屏一套配色是装饰，
 * 两屏一套配色是语言。
 */
export function Planet({
  item,
  lang,
  slot,
  discovered,
  kept,
  finished,
  dimmed,
  onOpen,
  onHover,
}: {
  item: ExplorePlanet;
  lang: Lang;
  slot: { x: string; y: string; size: number; drift: string };
  discovered: boolean;
  /** 这一篇在她的阅读室里。**不等于她读完了** —— 见下面那个标记。 */
  kept: boolean;
  /** 这一篇她真的读完了（reading.status = 'finished'）。 */
  finished: boolean;
  dimmed: boolean;
  onOpen: () => void;
  /** 悬停上来了没有。地图靠它决定把哪几条连线点亮（2026-09-07）。 */
  onHover?: (hovering: boolean) => void;
}) {
  const [hover, setHover] = useState(false);
  function enter() {
    setHover(true);
    onHover?.(true);
  }
  function leave() {
    setHover(false);
    onHover?.(false);
  }
  const meta = fieldById(item.field as FieldId);
  // 一个标记，按强度取一个：读完了 > 在阅读室 > 只是打开看过。
  const mark = finished
    ? { label: "已读完", color: "var(--mk-explore-accent)" }
    : kept
      ? { label: "在阅读室", color: "var(--mk-explore-accent)" }
      : discovered
        ? { label: "已浏览", color: "var(--mk-explore-muted)" }
        : null;
  const showHook = hover && !dimmed;
  // The headline has to survive a 150px bubble on a short window as well as a
  // 240px one. Scaling the type with the sphere keeps the text block the same
  // fraction of the circle at every size, which is what stops it spilling past
  // the rim when `useFitScale` shrinks the stage.
  const titleSize = Math.max(11, Math.round(slot.size * 0.062));

  return (
    <div
      className={cx("absolute", slot.drift)}
      style={{
        left: slot.x,
        top: slot.y,
        transform: "translate(-50%,-50%)",
        opacity: dimmed ? 0.22 : 1,
        transition: "opacity 260ms cubic-bezier(.2,0,0,1)",
        zIndex: hover ? 15 : 10,
      }}
    >
      <button
        type="button"
        onClick={onOpen}
        onMouseEnter={enter}
        onMouseLeave={leave}
        onFocus={enter}
        onBlur={leave}
        className="exp-planet group relative block cursor-pointer focus-visible:outline-none"
        style={{ width: slot.size, height: slot.size, ["--planet-hue" as string]: meta.hue }}
        aria-label={`${item.titleZh} — ${meta.label}${discovered ? "，已浏览" : ""}`}
      >
        {/* the bubble */}
        <span
          className={cx("exp-bubble block", discovered && "exp-bubble-lit")}
          style={{
            width: slot.size,
            height: slot.size,
            // Glass, not paint: a wash of the field's hue strongest at the rim,
            // so the middle stays clear enough to set text on.
            background: `radial-gradient(circle at 50% 52%,
                 color-mix(in srgb, ${meta.hue} ${discovered ? 24 : 13}%, transparent) 0%,
                 color-mix(in srgb, ${meta.hue} ${discovered ? 32 : 19}%, transparent) 58%,
                 color-mix(in srgb, ${meta.hue} ${discovered ? 60 : 38}%, transparent) 100%)`,
            borderColor: `color-mix(in srgb, ${meta.hue} ${discovered ? 78 : 46}%, transparent)`,
            boxShadow: discovered
              ? `inset 0 0 40px color-mix(in srgb, ${meta.hue} 28%, transparent),
                 0 18px 54px color-mix(in srgb, ${meta.hue} 30%, transparent),
                 0 0 78px color-mix(in srgb, ${meta.hue} 22%, transparent)`
              : "inset 0 0 34px rgba(255,255,255,.06), 0 14px 40px rgba(0,0,0,.44)",
          }}
        />

        {/* the headline, inside the glass */}
        <span
          className="pointer-events-none absolute left-1/2 top-1/2 flex -translate-x-1/2 -translate-y-1/2
                     flex-col items-center text-center"
          style={{ width: slot.size * 0.74 }}
        >
          <span
            className="exp-mono mb-1 flex items-center gap-1.5"
            style={{ color: `color-mix(in srgb, ${meta.hue} 45%, var(--mk-explore-ink))` }}
          >
            {lang === "zh" ? meta.label : meta.en}
          </span>
          <span
            className="block"
            style={{
              fontSize: titleSize,
              lineHeight: 1.45,
              fontWeight: discovered ? 700 : 600,
              color: "var(--mk-explore-ink)",
              textShadow: "none",
              display: "-webkit-box",
              WebkitLineClamp: 4,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {lang === "zh" ? item.titleZh : item.titleEn || item.titleZh}
          </span>
        </span>

        {/* 这里原来有一个 1–5 的角标。删掉了（2026-09-07）：一个圆角框里的小
            数字挂在球的右上角，读出来是「三条未读消息」，不是「今天第三条」。
            排序仍然存在 —— 它就是球的大小和位置，不需要再写一个数字。 */}

        {/* 🚨 一颗星球上只挂一个标记，挂最强的那个真话。
            2026-09-08：原来「在阅读室里」是一个绿色对勾，产品负责人点了
            「现在读」就退出来，读到的是「这条我已经读完了」—— 对勾在任何界面上
            都是「完成」。三种状态现在分开说，而且「已读完」只在服务端说这一篇
            status='finished' 的时候才出现。 */}
        {mark ? (
          <span
            className="exp-mono absolute bottom-[6%] left-1/2 -translate-x-1/2 rounded-mk-full px-2 py-0.5"
            style={{ background: "var(--mk-explore-surface)", color: mark.color, letterSpacing: 0 }}
          >
            {mark.label}
          </span>
        ) : null}

        {/* the hook — appears on hover, above everything. It is the one thing
            still held back: the question the story puts to her. */}
        {showHook ? (
          <span
            className="exp-in pointer-events-none absolute left-1/2 z-30 w-[300px] -translate-x-1/2 rounded-mk-lg p-4 text-left"
            style={{
              bottom: "calc(100% + 12px)",
              background: "var(--mk-explore-surface)",
              border: "1px solid var(--mk-explore-line)",
              boxShadow: "var(--mk-shadow-md)",
              backdropFilter: "blur(12px)",
            }}
          >
            <span className="exp-mono block" style={{ color: "var(--mk-explore-muted)" }}>
              它想问你
            </span>
            <span
              className="mt-1.5 block text-mk-body-lg font-semibold leading-[1.6]"
              style={{ color: "var(--mk-explore-ink)" }}
            >
              {item.hook}
            </span>
            {item.discipline ? (
              <span className="mt-2.5 block text-mk-small leading-[1.7]" style={{ color: "var(--mk-explore-muted)" }}>
                {item.discipline.zh} · {item.discipline.asks}
              </span>
            ) : null}
          </span>
        ) : null}
      </button>
    </div>
  );
}
