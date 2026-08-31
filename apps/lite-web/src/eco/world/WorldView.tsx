import { useMemo, useRef, useState } from "react";
import { Info, Languages } from "lucide-react";
import { useEco } from "../store";
import {
  NEWS_DATES,
  POLITICS_NOTE,
  SELECTION_NOTE,
  labelForDate,
  newsForDate,
} from "../data/news";
import type { NewsItem } from "../data/types";
import { ViewSwitch } from "../home/ViewSwitch";
import { Panel, Sys, cx } from "../ui";
import { useFitScale } from "../fit";
import { Planet } from "./Planet";
import { NewsSheet } from "./NewsSheet";

/**
 * 世界 · 今日探索地图.
 *
 * ## The five rules this screen is built on
 * 1. **Exactly five, and each node is ONE story.** Not a category, not a
 *    feed. Five things worth knowing, ranked.
 * 2. **A bubble says what it is.** The headline sits inside the glass. A
 *    field of five unlabelled circles asks a student to click blind, and the
 *    thing it "reveals" is only what she should have been told up front.
 * 3. **The hook is what stays held back.** Hovering a bubble raises the
 *    QUESTION the story puts to her. A question is what makes a 13-year-old
 *    lean in; the headline is only what stops her guessing.
 * 4. **The omission is stated.** Politics and conflict are filtered at the
 *    data level, and the ⓘ in the header says so in full. A filtered set
 *    presented as "everything" is a lie by layout.
 * 5. **No streaks, no leaderboards, no badges.** The counter reads
 *    「已浏览 2 / 5」and resets with the day. Nothing here rewards coming
 *    back tomorrow (铁律②).
 *
 * ## One screen (2026-08-31, second pass)
 * Everything below the stage is gone: the 「今天这五条」 ledger (a second copy
 * of the map, and a screen that says everything twice teaches that neither
 * copy is the real one), the eight domain filter chips, and the row of
 * honesty chips.
 *
 * The map is the screen now. **A field of five things does not need a
 * filter** — filtering five items into two is a control that costs more
 * attention than it saves, and it existed because the console had room, not
 * because anyone needed it.
 *
 * The date moved up: a short axis under the title, top-left, where it reads
 * as *which day am I looking at* rather than as a row of buttons.
 *
 * 🚨 The transparency did NOT go with the clutter. Both notes and the
 * prototype-data disclosure moved into one ⓘ beside the counter. The rule is
 * that the omission is stated somewhere a student can find, not that it
 * occupies a quarter of the screen.
 *
 * Planet positions are hand-placed per rank rather than laid out by an
 * algorithm: five objects on a stage is a composition, and a composition
 * beats a distribution every time.
 */

/** Hand-placed stage positions, keyed by rank (1 = most important).
 *
 *  Sizes roughly doubled when the headline moved inside the glass: a bubble is
 *  now a place to READ, and 88px of circle cannot hold a 20-character Chinese
 *  headline at any type size a 13-year-old should be asked to read. The slots
 *  were re-spread to match — the closest pair (1 and 4) clears by ~60px at
 *  1100px wide, which is enough for the ±40px drift underneath them. */
const SLOTS: Record<number, { x: string; y: string; size: number; drift: string }> = {
  1: { x: "27%", y: "41%", size: 236, drift: "eco-drift" },
  2: { x: "61%", y: "24%", size: 200, drift: "eco-drift-1" },
  3: { x: "79%", y: "63%", size: 178, drift: "eco-drift-2" },
  4: { x: "11%", y: "80%", size: 158, drift: "eco-drift-3" },
  5: { x: "46%", y: "77%", size: 150, drift: "eco-drift-4" },
};

export function WorldView() {
  const { state, setDate, setLang, discover } = useEco();
  const [openId, setOpenId] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const fieldRef = useRef<HTMLDivElement>(null);
  // Planets are fixed pixel sizes on a stage that shrinks. Without this, a
  // 700px-tall window overlaps planet 1's title with planet 5.
  const scale = useFitScale(fieldRef, 640, 0.58);

  const items = useMemo(() => newsForDate(state.date), [state.date]);
  const lit = items.filter((n) => state.discovered.includes(n.id)).length;
  const open = openId ? (items.find((n) => n.id === openId) ?? null) : null;

  function onOpen(item: NewsItem) {
    discover(item.id);
    setOpenId(item.id);
  }

  return (
    <div className="eco-sky eco-stars relative flex h-full flex-col overflow-hidden">
      {/* ── top bar ─────────────────────────────────────────────────────── */}
      <header className="relative z-20 flex flex-wrap items-start justify-between gap-4 px-7 pt-5">
        <div className="min-w-0">
          <Sys tone="dark">今日探索地图 · EXPLORATION MAP</Sys>
          <p className="mt-1 text-mk-h2 text-[#F5EFE7]">
            {labelForDate(state.date, state.lang)}
          </p>
          {/* The date axis. Top-left, under the title, because the question it
              answers is 「我在看哪一天」 — which belongs beside the day, not in
              a console at the far end of the page. */}
          <div className="mt-3 flex items-center">
            {NEWS_DATES.map((d, i) => {
              const active = d === state.date;
              return (
                <div key={d} className="flex items-center">
                  <button
                    type="button"
                    onClick={() => setDate(d)}
                    aria-pressed={active}
                    className="group flex flex-col items-center gap-1.5 px-2.5 py-1 focus-visible:outline-none"
                    title={d}
                  >
                    <span
                      className="h-2 w-2 transition-all duration-[200ms] ease-mk"
                      style={{
                        background: active ? "var(--mk-accent-400)" : "rgba(240,233,224,.32)",
                        transform: active ? "rotate(45deg) scale(1.5)" : "rotate(45deg)",
                        boxShadow: active ? "0 0 12px var(--mk-accent-400)" : "none",
                      }}
                    />
                    <span
                      className={cx(
                        "whitespace-nowrap text-mk-small transition-colors duration-[160ms]",
                        active ? "font-semibold text-[#F5EFE7]" : "text-[#8E8175]",
                      )}
                    >
                      {i === 0 ? "今天" : labelForDate(d, state.lang)}
                    </span>
                  </button>
                  {i < NEWS_DATES.length - 1 ? (
                    <span
                      className="mb-5 h-px w-6 shrink-0"
                      style={{ background: "rgba(240,233,224,.18)" }}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
        </div>

        <ViewSwitch view="world" />

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setLang(state.lang === "zh" ? "en" : "zh")}
            className="flex items-center gap-1.5 rounded-mk-full px-3 py-1.5 text-mk-small transition-colors
                       duration-[120ms] ease-mk hover:bg-[rgba(240,233,224,.1)] focus-visible:outline-none
                       focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
            style={{ border: "1px solid rgba(240,233,224,.18)", color: "#D8CCBD" }}
            aria-label="切换语言"
          >
            <Languages size={14} strokeWidth={1.8} />
            {state.lang === "zh" ? "中 / EN" : "EN / 中"}
          </button>
          <div
            className="rounded-mk-full px-3 py-1.5 text-mk-small"
            style={{ border: "1px solid rgba(240,233,224,.18)", color: "#D8CCBD" }}
          >
            已浏览{" "}
            <span className="font-mono font-bold text-[#F5EFE7]">
              {lit} / {items.length}
            </span>
          </div>
          {/* One button for everything this screen owes the student about how
              the five were chosen and what is missing. */}
          <button
            type="button"
            onClick={() => setNote(note ? null : "selection")}
            aria-label="这五条是怎么来的"
            title="这五条是怎么来的"
            className="flex h-[34px] w-[34px] items-center justify-center rounded-mk-full transition-colors
                       duration-[120ms] ease-mk hover:bg-[rgba(240,233,224,.1)] focus-visible:outline-none
                       focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
            style={{ border: "1px solid rgba(240,233,224,.18)", color: "#D8CCBD" }}
          >
            <Info size={15} strokeWidth={1.8} />
          </button>
        </div>
      </header>

      {/* ── the stage ───────────────────────────────────────────────────── */}
      {/* Flex-filled: nothing sits below it any more, so the map takes the
          whole remaining screen. */}
      <div ref={fieldRef} className="relative z-10 min-h-0 flex-1">
        {/* Orbit rings. Purely atmospheric: they give the field a centre. */}
        <div className="pointer-events-none absolute inset-0 overflow-hidden">
          <div
            className="eco-orbit"
            style={{ left: "50%", top: "50%", width: 660, height: 660, marginLeft: -330, marginTop: -330 }}
          />
          <div
            className="eco-orbit eco-orbit-2"
            style={{ left: "50%", top: "50%", width: 940, height: 940, marginLeft: -470, marginTop: -470 }}
          />
          <div
            className="eco-orbit eco-orbit-3"
            style={{ left: "50%", top: "50%", width: 1240, height: 1240, marginLeft: -620, marginTop: -620 }}
          />
        </div>

        {items.map((item) => {
          // Every item has rank 1..5 by construction; fall back to the
          // smallest slot rather than crashing on unexpected data.
          const slot = SLOTS[item.rank] ?? SLOTS[5]!;
          return (
            <Planet
              key={item.id}
              item={item}
              lang={state.lang}
              slot={{ ...slot, size: Math.round(slot.size * scale) }}
              discovered={state.discovered.includes(item.id)}
              kept={state.kept.includes(item.id)}
              dimmed={false}
              onOpen={() => onOpen(item)}
            />
          );
        })}

        {lit === 0 && scale > 0.9 ? (
          <p
            className="pointer-events-none absolute bottom-2 left-1/2 -translate-x-1/2 text-center text-mk-small"
            style={{ color: "#8E8175" }}
          >
            五个泡泡，五条今天值得知道的事。把光标移上去，它会先问你一个问题。
          </p>
        ) : null}
      </div>

      {/* ── how the five were chosen ───────────────────────────────────── */}
      {note ? (
        <div className="pointer-events-none absolute inset-0 z-30 flex items-end justify-center px-7 pb-7">
          <Panel tone="dark" className="eco-in pointer-events-auto max-w-[74ch] p-5">
            <div className="flex items-start justify-between gap-4">
              <Sys tone="dark">这五条是怎么来的</Sys>
              <button
                type="button"
                onClick={() => setNote(null)}
                className="eco-mono shrink-0 text-[#8E8175] hover:text-[#E6DDD2] focus-visible:outline-none"
              >
                关闭
              </button>
            </div>
            <p className="mt-2 text-mk-body leading-[1.85] text-[#E6DDD2]">
              {SELECTION_NOTE[state.lang]}
            </p>
            <p className="mt-3 border-t pt-3 text-mk-body leading-[1.85] text-[#E6DDD2]"
               style={{ borderColor: "rgba(240,233,224,.14)" }}>
              {POLITICS_NOTE[state.lang]}
            </p>
            {/* Honesty about the data itself. Remove ONLY when the items become
                real reporting — see data/news.ts. */}
            <p className="mt-3 border-t pt-3 text-mk-small leading-[1.8] text-[#9A8E80]"
               style={{ borderColor: "rgba(240,233,224,.14)" }}>
              原型说明：这些新闻是为原型写的示例内容，不是真实报道。
            </p>
          </Panel>
        </div>
      ) : null}

      <NewsSheet item={open} onClose={() => setOpenId(null)} />
    </div>
  );
}
