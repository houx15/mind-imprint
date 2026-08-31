import { useMemo, useRef, useState } from "react";
import { ChevronDown, Info, Languages } from "lucide-react";
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
import { useFitScale } from "../fit";
import { Planet } from "./Planet";
import { NewsSheet } from "./NewsSheet";

/**
 * 世界 · 今日探索地图.
 *
 * ## The five rules this screen is built on
 * 1. **Exactly five, and each node is ONE story.** Not a category, not a
 *    feed. Five things worth knowing, ranked.
 * 2. **Undiscovered until opened.** A veiled planet shows only its field
 *    glyph; opening it lights it. Discovery is the verb, not consumption.
 * 3. **The hook comes before the headline.** Hovering whispers the QUESTION;
 *    the title is secondary. A question is what makes a 13-year-old lean in.
 * 4. **The omission is stated.** Politics and conflict are filtered at the
 *    data level, and the chip that says so opens a real explanation. A
 *    filtered set presented as "everything" is a lie by layout.
 * 5. **No streaks, no leaderboards, no badges.** The counter reads
 *    「已浏览 2 / 5」and resets with the day. Nothing here rewards coming
 *    back tomorrow (铁律②).
 *
 * ## Why the page scrolls now (2026-08-31)
 * It used to be a locked stage — `h-full overflow-hidden` — which meant the
 * map had to hold everything, and everything it could not hold was simply
 * gone. It is now a **document with a stage at the top**: the map fills the
 * first screen, and below it sit the day's five as a readable ledger, the
 * timeline, and the transparency notes. Scrolling is the second gear: look
 * first, then read the list. Nothing important lives only in the stage.
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
  const fieldRef = useRef<HTMLDivElement>(null);
  // Planets are fixed pixel sizes on a stage that shrinks. Without this, a
  // 700px-tall window overlaps planet 1's title with planet 5.
  const scale = useFitScale(fieldRef, 520, 0.66);

  const items = useMemo(() => newsForDate(state.date), [state.date]);
  const lit = items.filter((n) => state.discovered.includes(n.id)).length;
  const open = openId ? (items.find((n) => n.id === openId) ?? null) : null;

  function onOpen(item: NewsItem) {
    discover(item.id);
    setOpenId(item.id);
  }

  const matching = state.domainFilter
    ? items.filter((n) => n.domain === state.domainFilter)
    : items;

  return (
    <div className="eco-sky eco-stars relative min-h-full">
      {/* ── top bar ─────────────────────────────────────────────────────── */}
      <header className="relative z-20 flex flex-wrap items-center justify-between gap-4 px-7 pt-6">
        <div className="min-w-0">
          <Sys tone="dark">今日探索地图 · EXPLORATION MAP</Sys>
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
            已浏览{" "}
            <span className="font-mono font-bold text-[#F5EFE7]">
              {lit} / {items.length}
            </span>
          </div>
        </div>
      </header>

      {/* ── the stage ───────────────────────────────────────────────────── */}
      {/* Height-capped rather than flex-filled: the page scrolls now, so the
          map takes the first screen and hands the rest to the ledger below. */}
      <div
        ref={fieldRef}
        className="relative z-10"
        style={{ height: "min(620px, calc(100vh - 210px))", minHeight: 380 }}
      >
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
              slot={{ ...slot, size: Math.round(slot.size * scale) }}
              discovered={state.discovered.includes(item.id)}
              kept={state.kept.includes(item.id)}
              dimmed={dimmed}
              onOpen={() => onOpen(item)}
            />
          );
        })}

        {matching.length === 0 ? (
          <div className="absolute inset-0 z-20 flex items-center justify-center">
            <Panel tone="dark" className="eco-in max-w-[42ch] p-6 text-center">
              <Sys tone="dark">这一天没有这个领域</Sys>
              <p className="mt-2 text-mk-body-lg leading-[1.85] text-[#E6DDD2]">
                {state.lang === "zh"
                  ? "每天只有五条，所以不是每个领域每天都会出现。换一天，或者看全部。"
                  : "Only five a day, so not every field appears every day. Try another day, or view all."}
              </p>
              <div className="mt-4 flex justify-center gap-2">
                <Chip tone="dark" active onClick={() => setDomainFilter(null)}>
                  看全部领域
                </Chip>
              </div>
            </Panel>
          </div>
        ) : lit === 0 && scale > 0.9 ? (
          <p
            className="pointer-events-none absolute bottom-1 left-1/2 -translate-x-1/2 text-center text-mk-small"
            style={{ color: "#8E8175" }}
          >
            五颗行星在等你。把光标移上去，它会先给你一个问题。
          </p>
        ) : null}
      </div>

      {/* ── scroll cue ──────────────────────────────────────────────────── */}
      <div className="relative z-20 flex items-center gap-3 px-7 pt-2">
        <div className="eco-scanline flex-1" />
        <span className="flex items-center gap-1.5 text-mk-small" style={{ color: "#8E8175" }}>
          往下是今天这五条的清单
          <ChevronDown size={14} strokeWidth={1.8} />
        </span>
        <div className="eco-scanline flex-1" />
      </div>

      {/* ── the ledger ──────────────────────────────────────────────────── */}
      {/* The same five, as text. Not a duplicate: the map is for LOOKING (what
          is big, what field, what have I opened), the ledger is for READING
          (导读 first, then the headline). A student who does not enjoy hunting
          on a starfield still gets the day. */}
      <section className="relative z-10 px-7 pt-6">
        <Sys tone="dark">今天这五条 · TODAY&apos;S FIVE</Sys>
        <ul className="mt-3 space-y-2">
          {items.map((item, i) => {
            const meta = DOMAIN_META[item.domain];
            const seen = state.discovered.includes(item.id);
            const dimmed = state.domainFilter !== null && state.domainFilter !== item.domain;
            return (
              <li key={item.id} style={{ opacity: dimmed ? 0.35 : 1, transition: "opacity 220ms" }}>
                <button
                  type="button"
                  onClick={() => onOpen(item)}
                  className="eco-in group flex w-full items-start gap-4 rounded-mk-lg p-4 text-left
                             transition-colors duration-[140ms] ease-mk hover:bg-[rgba(240,233,224,.07)]
                             focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                  style={{
                    border: "1px solid rgba(240,233,224,.12)",
                    background: "rgba(240,233,224,.03)",
                    ["--i" as string]: i,
                  }}
                >
                  <span
                    className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-mk-full
                               font-mono text-[15px] font-bold"
                    style={{
                      background: `color-mix(in srgb, ${meta.hue} 26%, transparent)`,
                      color: "#F0E9E0",
                      border: `1px solid color-mix(in srgb, ${meta.hue} 46%, transparent)`,
                    }}
                  >
                    {item.rank}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                      <span className="eco-mono" style={{ color: meta.hue }}>
                        {state.lang === "zh" ? meta.zh : meta.en}
                      </span>
                      <span className="eco-mono" style={{ color: "#7C7166" }}>
                        {item.source}
                      </span>
                      {seen ? (
                        <span className="eco-mono" style={{ color: "#6FBFB0", letterSpacing: 0 }}>
                          已浏览
                        </span>
                      ) : null}
                    </span>
                    {/* 导读 leads — it is the line written for HER. */}
                    <span className="mt-1.5 block text-mk-body-lg font-semibold leading-[1.7] text-[#F2EBE1]">
                      {item.lead[state.lang]}
                    </span>
                    <span className="mt-1 block text-mk-small leading-[1.75] text-[#9A8E80]">
                      {item.title[state.lang]}
                    </span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </section>

      {/* ── bottom console ──────────────────────────────────────────────── */}
      <footer className="relative z-20 px-7 pb-10 pt-8">
        <div className="eco-scanline mb-5" />

        <div className="flex flex-wrap items-end justify-between gap-5">
          {/* date rail */}
          <div>
            <Sys tone="dark" className="mb-2 block">
              时间轴
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
          <Panel tone="dark" className="eco-in mt-4 max-w-[70ch] p-5">
            <div className="flex items-start justify-between gap-4">
              <Sys tone="dark">{note === "politics" ? "为什么这里没有政治" : "五条是怎么挑的"}</Sys>
              <button
                type="button"
                onClick={() => setNote(null)}
                className="eco-mono shrink-0 text-[#8E8175] hover:text-[#E6DDD2] focus-visible:outline-none"
              >
                关闭
              </button>
            </div>
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
