import { useMemo, useState } from "react";
import { Info, Languages } from "lucide-react";
import { useEco } from "../store";
import {
  DOMAIN_META,
  DOMAIN_ORDER,
  NEWS_DATES,
  POLITICS_NOTE,
  SELECTION_NOTE,
  labelForDate,
  newsForDate,
} from "../data/news";
import type { NewsItem } from "../data/types";
import { ViewSwitch } from "../home/ViewSwitch";
import { Chip, Panel, Sys, cx } from "../ui";
import { Planet } from "./Planet";
import { NewsSheet } from "./NewsSheet";

/**
 * 世界 · the news star map.
 *
 * ## The five rules this screen is built on
 * 1. **Exactly five.** Not a feed. Five things worth knowing, ranked.
 * 2. **Undiscovered until opened.** A veiled planet shows only its field
 *    glyph; opening it lights it. Discovery is the verb, not consumption.
 * 3. **The hook comes before the headline.** Hovering whispers the QUESTION;
 *    the title is secondary. A question is what makes a 13-year-old lean in.
 * 4. **The omission is stated.** Politics and conflict are filtered at the
 *    data level, and the chip that says so opens a real explanation. A
 *    filtered set presented as "everything" is a lie by layout.
 * 5. **No streaks, no leaderboards, no badges.** The counter reads
 *    「你点亮了 2 / 5」and resets with the day. Nothing here rewards coming
 *    back tomorrow (铁律②).
 *
 * Planet positions are hand-placed per rank rather than laid out by an
 * algorithm: five objects on a stage is a composition, and a composition
 * beats a distribution every time.
 */

/** Hand-placed stage positions, keyed by rank (1 = most important). */
const SLOTS: Record<number, { x: string; y: string; size: number; drift: string }> = {
  1: { x: "31%", y: "44%", size: 172, drift: "eco-drift" },
  2: { x: "63%", y: "26%", size: 132, drift: "eco-drift-1" },
  3: { x: "73%", y: "64%", size: 116, drift: "eco-drift-2" },
  4: { x: "14%", y: "72%", size: 100, drift: "eco-drift-3" },
  5: { x: "50%", y: "72%", size: 88, drift: "eco-drift-4" },
};

export function WorldView() {
  const { state, setDate, setLang, setDomainFilter, discover } = useEco();
  const [openId, setOpenId] = useState<string | null>(null);
  const [note, setNote] = useState<"politics" | "selection" | null>(null);

  const items = useMemo(() => newsForDate(state.date), [state.date]);
  const lit = items.filter((n) => state.discovered.includes(n.id)).length;
  const open = openId ? items.find((n) => n.id === openId) ?? null : null;

  function onOpen(item: NewsItem) {
    discover(item.id);
    setOpenId(item.id);
  }

  return (
    <div className="eco-sky eco-stars relative flex min-h-full flex-col">
      {/* ── top bar ─────────────────────────────────────────────────────── */}
      <header className="relative z-20 flex flex-wrap items-center justify-between gap-4 px-7 pt-6">
        <div className="min-w-0">
          <Sys tone="dark">今日星图 · SIGNAL MAP</Sys>
          <p className="mt-1 text-mk-h2 text-[#F5EFE7]">
            {labelForDate(state.date, state.lang)}
            <span className="ml-3 font-mono text-mk-small font-normal text-[#8E8175]">
              {state.date}
            </span>
          </p>
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
            你点亮了{" "}
            <span className="font-mono font-bold text-[#F5EFE7]">
              {lit} / {items.length}
            </span>
          </div>
        </div>
      </header>

      {/* ── the field ───────────────────────────────────────────────────── */}
      <div className="relative z-10 min-h-[560px] flex-1">
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
          const dimmed = state.domainFilter !== null && state.domainFilter !== item.domain;
          return (
            <Planet
              key={item.id}
              item={item}
              lang={state.lang}
              slot={slot}
              discovered={state.discovered.includes(item.id)}
              kept={state.kept.includes(item.id)}
              dimmed={dimmed}
              onOpen={() => onOpen(item)}
            />
          );
        })}

        {/* An invitation, not an instruction — it fades once she has opened
            one, because by then she knows. */}
        {lit === 0 ? (
          <p
            className="pointer-events-none absolute bottom-1 left-1/2 -translate-x-1/2 text-center text-mk-small"
            style={{ color: "#8E8175" }}
          >
            五颗行星在等你点亮。把光标移上去，它会先给你一个问题。
          </p>
        ) : null}
      </div>

      {/* ── bottom console ──────────────────────────────────────────────── */}
      <footer className="relative z-20 px-7 pb-6">
        <div className="eco-scanline mb-4" />

        <div className="flex flex-wrap items-end justify-between gap-5">
          {/* date rail */}
          <div>
            <Sys tone="dark" className="mb-2 block">
              换一天，换一批
            </Sys>
            <div className="flex items-center gap-1.5">
              {NEWS_DATES.map((d, i) => {
                const active = d === state.date;
                return (
                  <button
                    key={d}
                    type="button"
                    onClick={() => setDate(d)}
                    className={cx(
                      "group relative rounded-mk-md px-3 py-2 text-left transition-all duration-[160ms] ease-mk",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]",
                    )}
                    style={{
                      background: active ? "rgba(240,233,224,.94)" : "rgba(240,233,224,.05)",
                      border: active
                        ? "1px solid transparent"
                        : "1px solid rgba(240,233,224,.14)",
                      color: active ? "#17130F" : "#B6A99A",
                    }}
                  >
                    <span className="eco-mono block opacity-70">{i === 0 ? "TODAY" : `D-${i}`}</span>
                    <span className="mt-0.5 block whitespace-nowrap text-mk-small font-semibold">
                      {labelForDate(d, state.lang)}
                    </span>
                    {/* tick marks under the rail — a scale, not just buttons */}
                    <span
                      aria-hidden
                      className="absolute -bottom-2 left-1/2 h-1.5 w-px -translate-x-1/2"
                      style={{ background: active ? "var(--mk-accent-400)" : "rgba(240,233,224,.2)" }}
                    />
                  </button>
                );
              })}
            </div>
          </div>

          {/* filters + honesty chips */}
          <div className="flex max-w-full flex-col items-start gap-2.5">
            <div className="flex flex-wrap items-center gap-1.5">
              <Chip
                tone="dark"
                active={state.domainFilter === null}
                onClick={() => setDomainFilter(null)}
              >
                全部领域
              </Chip>
              {DOMAIN_ORDER.map((d) => (
                <Chip
                  key={d}
                  tone="dark"
                  hue={DOMAIN_META[d].hue}
                  active={state.domainFilter === d}
                  onClick={() => setDomainFilter(state.domainFilter === d ? null : d)}
                >
                  {state.lang === "zh" ? DOMAIN_META[d].zh : DOMAIN_META[d].en}
                </Chip>
              ))}
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
              <button
                type="button"
                onClick={() => setNote(note === "politics" ? null : "politics")}
                className="inline-flex items-center gap-1.5 rounded-mk-full px-3 py-1.5 text-mk-small
                           transition-colors duration-[120ms] ease-mk hover:bg-[rgba(240,233,224,.1)]
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                style={{ border: "1px dashed rgba(240,233,224,.26)", color: "#B6A99A" }}
              >
                <Info size={13} strokeWidth={1.8} />
                已过滤：政治与冲突
              </button>
              <button
                type="button"
                onClick={() => setNote(note === "selection" ? null : "selection")}
                className="inline-flex items-center gap-1.5 rounded-mk-full px-3 py-1.5 text-mk-small
                           transition-colors duration-[120ms] ease-mk hover:bg-[rgba(240,233,224,.1)]
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                style={{ border: "1px dashed rgba(240,233,224,.26)", color: "#B6A99A" }}
              >
                <Info size={13} strokeWidth={1.8} />
                为什么是这五条
              </button>
              {/* Honesty about the data itself. Remove ONLY when the items
                  become real reporting — see data/news.ts. */}
              <span
                className="eco-mono rounded-mk-full px-2.5 py-1.5"
                style={{ border: "1px solid rgba(240,233,224,.14)", color: "#7C7166" }}
                title="这些新闻是为原型写的示例内容，不是真实报道"
              >
                原型数据
              </span>
            </div>
          </div>
        </div>

        {note ? (
          <Panel tone="dark" className="eco-in mt-4 max-w-[76ch] p-5">
            <Sys tone="dark">{note === "politics" ? "为什么这里没有政治" : "五条是怎么挑的"}</Sys>
            <p className="mt-2 text-mk-body-lg leading-[1.85] text-[#E6DDD2]">
              {(note === "politics" ? POLITICS_NOTE : SELECTION_NOTE)[state.lang]}
            </p>
          </Panel>
        ) : null}
      </footer>

      <NewsSheet item={open} onClose={() => setOpenId(null)} />
    </div>
  );
}
