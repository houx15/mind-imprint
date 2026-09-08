import { useEffect, useMemo, useRef, useState } from "react";
import { Info, Languages } from "lucide-react";
import { fieldById } from "../tree/geometry";
import { KeywordDrawer } from "../tree/KeywordDrawer";
import { Sys, cx } from "../tree/ui";
import type { FieldId } from "../tree/types";
import type { LiveTree } from "../tree/useInterestTree";
import { useFitScale } from "../tree/useFitScale";
import { Planet, type Lang } from "./Planet";
import { NewsSheet } from "./NewsSheet";
import { PLANET_SLOTS, STAGE, type SkyPlanet, pickWords, placeWords, planetThreads } from "./skyLayout";
import { useExploreToday } from "./useExploreToday";
import "./explore.css";

/**
 * 探索 · 今日新闻星图。
 *
 * ## 这一屏建立在五条规则上
 *
 * 1. **正好五颗，每颗是一条新闻。** 不是一个分类，不是一条信息流。五件今天
 *    值得知道的事，有排序。一个刷不完的列表会把「每天来看一眼」变成「每天在
 *    这里待着」。
 * 2. **一颗泡泡自己说出它是什么。** 标题就在玻璃里面。五个没有标签的圆圈是在
 *    要求学生盲点，而它「揭晓」的东西恰恰是本来就该先告诉她的。
 * 3. **被留在后面的是钩子。** 悬停一颗星球，升起的是这条新闻**向她提出的
 *    问题**。问题才是让一个十五岁的人凑近的东西。
 * 4. **省略要被说出来。** 政治与冲突在**抓取那一层**就被过滤掉（见
 *    `internal/news/filter.go`），右上角的 ⓘ 把这件事说全。一个被过滤过的集合
 *    如果摆成「全部」，那是用版面撒谎。
 * 5. **一颗星球的颜色 = 它属于树的哪根枝。** 两屏共用同一套七色，所以一颗蓝色
 *    的星球和树上那根蓝色的枝是同一件事。
 * 6. **外圈是她自己的词，不是我们的猜测**（2026-09-07）。上一版那圈是推荐词，
 *    和她的词长得一模一样、摆在同一个位置上，于是这张图同时在说两件事却没有
 *    任何东西把它们分开。现在外圈只有她树上已经有的词，五颗新闻星向它们连线：
 *    一条线的意思是「今天这条和你已经在意的这个词扎在同一门学问上」。点开一个
 *    词，打开的是树上那一屏的同一个抽屉。
 *
 * ## 从原型搬过来时改掉的三件事（2026-09-03）
 *
 * - **数据是真的。** 十二个源、抓取、去重、政治过滤、一次模型调用选五条 ——
 *   全在服务端（`internal/news` + `internal/api/explore.go`）。
 * - **按八个「领域」上色改成按七根主枝上色。** 原来那套分类只活在这一屏里。
 * - **日期轴去掉了。** 原型可以左右翻五天的 mock 数据；真数据现在只有今天。
 *   翻看历史是后面的事，摆一个只能停在今天的轴是一个骗人的控件。
 */

const SELECTION_NOTE =
  "这五条是从十二个科学期刊与科普源（Nature、Quanta、arXiv、Phys.org、ScienceDaily 等）当天的" +
  "六十余条里挑出来的。挑选标准是「能不能引出一个你可以自己追问的问题」，不是热度。";

const POLITICS_NOTE =
  "选举、战争、制裁一类的新闻在抓取那一层就被过滤掉了，不会出现在这里。" +
  "这不是说它们不重要，是说这个产品没有接住它们的语境。";

export function ExploreView({ tree }: { tree: LiveTree }) {
  const [openId, setOpenId] = useState<string | null>(null);
  const [openWord, setOpenWord] = useState<string | null>(null);
  const [hotPlanet, setHotPlanet] = useState<string | null>(null);
  const [hotWord, setHotWord] = useState<string | null>(null);
  const [seen, setSeen] = useState<string[]>([]);
  const [lang, setLang] = useState<Lang>("zh");
  const [note, setNote] = useState(false);
  const fieldRef = useRef<HTMLDivElement>(null);
  // 星球是固定像素，舞台会缩。没有这个，700px 高的窗口上 1 号星球的标题会压到
  // 5 号身上。
  const scale = useFitScale(fieldRef, 640, 0.58);

  const live = useExploreToday();

  // 外圈是**她树上已经有的词**。挑哪几个、摆在哪，全在 skyLayout 里算 ——
  // 这一层的条数取决于她有多少词，今天三个明天九个，位置不能手摆。
  const skyPlanets: SkyPlanet[] = useMemo(
    () =>
      live.planets.map((p) => ({
        id: p.id,
        rank: p.rank,
        disciplineId: p.discipline?.id ?? "",
        interestId: p.interestId,
        field: p.field as FieldId,
      })),
    [live.planets],
  );

  const stars = useMemo(() => {
    const mine = tree.keywords.map((k) => ({
      id: k.id,
      zh: k.text,
      field: k.field,
      interestId: k.interestId,
      disciplineIds: k.disciplineIds,
      strength: k.strength,
    }));
    return placeWords(pickWords(mine, skyPlanets));
  }, [tree.keywords, skyPlanets]);

  const threads = useMemo(() => planetThreads(skyPlanets, stars), [skyPlanets, stars]);
  const openKw = openWord ? (tree.keywords.find((k) => k.id === openWord) ?? null) : null;

  // 悬停一颗星球，它连着的那几个词一起亮，其余压暗；悬停一个词，反过来。
  // 静止时所有连线都很淡 —— 这一屏第一眼要读到的是「今天有五条」，不是一张网。
  const focus = hotPlanet ?? hotWord;
  const litWords = new Set<string>();
  const litThreads = new Set<string>();
  if (focus) {
    for (const t of threads) {
      if (t.planetId !== focus && t.to.id !== focus) continue;
      litWords.add(t.to.id);
      litThreads.add(`${t.planetId}-${t.to.id}`);
    }
  }
  const open = openId ? (live.planets.find((p) => p.id === openId) ?? null) : null;
  const lit = live.planets.filter((p) => seen.includes(p.id)).length;
  const known = live.status === "ready" || live.status === "empty";

  function onOpen(id: string) {
    setSeen((s) => (s.includes(id) ? s : [...s, id]));
    setOpenId(id);
  }

  return (
    <div className="exp-sky exp-stars relative flex min-h-full flex-col overflow-hidden">
      {/* ── 顶栏 ─────────────────────────────────────────────────────────── */}
      <header className="relative z-20 flex flex-wrap items-start justify-between gap-4 px-7 pt-5">
        <div className="min-w-0">
          <Sys tone="dark">今日探索地图 · EXPLORATION MAP</Sys>
          <p className="mt-1 text-mk-h2 text-[#F5EFE7]">
            {live.day ? formatDay(live.day) : "今天"}
          </p>
          <p className="mt-1.5 max-w-[52ch] text-mk-small leading-[1.8] text-[#9A8E80]">
            {/* 只在真的有线的时候才说有线。一句说明配一张没有线的图，比不写
                这句更糟。 */}
            {threads.length > 0
              ? "五条今天值得知道的事。外圈是你树上的词，连线是它们和今天这五条的关系。"
              : stars.length > 0
                ? "五条今天值得知道的事。外圈是你树上的词。"
                : "五条今天值得知道的事。把光标移上去，它会先问你一个问题。"}
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => setLang((l) => (l === "zh" ? "en" : "zh"))}
            className="inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small
                       text-[#C0B4A6] transition-colors hover:bg-[rgba(240,233,224,.1)]"
            style={{ borderColor: "rgba(240,233,224,.22)" }}
          >
            <Languages size={14} strokeWidth={1.8} />
            {lang === "zh" ? "中 / EN" : "EN / 中"}
          </button>

          <span className="text-right">
            <Sys tone="dark">已浏览</Sys>
            <span className="block font-mono text-mk-h3 tabular-nums text-[#F5EFE7]">
              {known ? `${lit} / ${live.planets.length}` : "—"}
            </span>
          </span>

          {/* 省略要被说出来。一个被过滤过的集合摆成「全部」是用版面撒谎。 */}
          <button
            type="button"
            onClick={() => setNote((v) => !v)}
            aria-expanded={note}
            className="rounded-mk-full border p-2 text-[#C0B4A6] transition-colors hover:bg-[rgba(240,233,224,.1)]"
            style={{ borderColor: "rgba(240,233,224,.22)" }}
            aria-label="这五条是怎么来的"
          >
            <Info size={15} strokeWidth={1.8} />
          </button>
        </div>
      </header>

      {note ? (
        <div className="relative z-20 mx-7 mt-3 rounded-mk-md p-4"
             style={{ background: "rgba(23,19,15,.9)", border: "1px solid rgba(240,233,224,.18)" }}>
          <Sys tone="dark">这五条是怎么来的</Sys>
          <p className="mt-1.5 text-mk-small leading-[1.85] text-[#C0B4A6]">{SELECTION_NOTE}</p>
          <p className="mt-2 text-mk-small leading-[1.85] text-[#C0B4A6]">{POLITICS_NOTE}</p>
        </div>
      ) : null}

      {/* ── 星图 ─────────────────────────────────────────────────────────── */}
      <div className="relative z-10 flex-1 px-7 pb-10 pt-4">
        <div
          ref={fieldRef}
          className="relative mx-auto h-full w-full"
          style={{ minHeight: "min(640px, calc(100vh - 260px))", maxWidth: 1180 }}
        >
          {/* 连线：一颗新闻星到她的一个词。坐标是百分比，所以 viewBox 也用百分比，
              端点永远和星对齐。

              🚨 `--hue` 直接用 `fieldById(...).hue`，**不要再包一层 `var()`** ——
              那个值本身就是 `"var(--mk-lake)"`，包成 `var(var(--mk-lake))` 是无效
              的，而无效的后果是整条线一声不响地不见了。 */}
          {threads.length > 0 ? (
            <svg
              className="pointer-events-none absolute inset-0 h-full w-full"
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
              aria-hidden
            >
              {threads.map((t) => {
                const key = `${t.planetId}-${t.to.id}`;
                return (
                  <line
                    key={key}
                    x1={(t.from.x / STAGE.w) * 100}
                    y1={(t.from.y / STAGE.h) * 100}
                    x2={t.to.xPct}
                    y2={t.to.yPct}
                    vectorEffect="non-scaling-stroke"
                    strokeWidth={t.strength >= 2 ? 1.4 : 1}
                    className={cx("exp-thread", litThreads.has(key) && "exp-thread-lit")}
                    style={{ ["--hue" as string]: fieldById(t.to.field).hue }}
                  />
                );
              })}
            </svg>
          ) : null}

          {stars.map((star) => (
            <button
              key={star.id}
              type="button"
              onClick={() => setOpenWord(star.id)}
              onMouseEnter={() => setHotWord(star.id)}
              onMouseLeave={() => setHotWord(null)}
              onFocus={() => setHotWord(star.id)}
              onBlur={() => setHotWord(null)}
              className={cx(
                "exp-star",
                star.drift,
                focus && litWords.has(star.id) && "exp-star-lit",
                focus && !litWords.has(star.id) && star.id !== focus && "exp-star-dim",
              )}
              style={
                {
                  left: `${star.xPct}%`,
                  top: `${star.yPct}%`,
                  "--hue": fieldById(star.field).hue,
                } as React.CSSProperties
              }
            >
              <span className="exp-star-dot" />
              <span className="exp-star-name">{star.zh}</span>
            </button>
          ))}

          <ExploreState live={live} />

          {live.planets.map((p) => {
            const slot = PLANET_SLOTS[p.rank] ?? PLANET_SLOTS[5]!;
            return (
              <Planet
                key={p.id}
                item={p}
                lang={lang}
                slot={{
                  x: `${slot.xPct}%`,
                  y: `${slot.yPct}%`,
                  drift: slot.drift,
                  size: Math.round(slot.size * scale),
                }}
                discovered={seen.includes(p.id)}
                kept={p.saved}
                finished={p.finished}
                dimmed={false}
                onOpen={() => onOpen(p.id)}
                onHover={(on) => setHotPlanet(on ? p.id : null)}
              />
            );
          })}
        </div>
      </div>

      <NewsSheet
        item={open}
        lang={lang}
        onClose={() => setOpenId(null)}
        onSaved={live.applySaved}
      />

      {/* 点开一个词，看到的是**树上那一屏同一个抽屉** —— 它是什么、什么时候
          第一次出现、由哪几件事长出来、接下来能挖什么。地图上再写一份「这个词
          是什么」的面板，等于同一个东西有两个说法，而其中一个迟早会说错。 */}
      <KeywordDrawer kw={openKw} onClose={() => setOpenWord(null)} />
    </div>
  );
}

/**
 * 四个状态，说清楚是哪一个。
 *
 * 🚨 **绝不用昨天的星图冒充今天的。** 生成失败时这一屏是空的，并且把服务端
 * 那句原话原样显示出来 —— 学生和我们看到同一句（界面文案 §8）。
 */
function ExploreState({ live }: { live: ReturnType<typeof useExploreToday> }) {
  if (live.status === "ready") return null;

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center px-6">
      <div
        className="max-w-[440px] rounded-[18px] border px-6 py-5 text-center backdrop-blur-sm"
        style={{ borderColor: "rgba(245,239,231,0.14)", background: "rgba(16,13,10,0.78)" }}
      >
        {live.status === "loading" && (
          <>
            <Sys tone="dark">处理中 · BUILDING</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">正在生成今天的星图</p>
            {/* 第一个打开的人触发一次抓取（十二个源）加一次模型调用。说出来，
                别让她以为卡住了。 */}
            <p className="mt-1 text-mk-small leading-[1.8] text-[#9A8E80]">
              正在从十二个科学源里挑今天的五条，需要几秒。
            </p>
          </>
        )}

        {live.status === "error" && (
          <>
            <Sys tone="dark">读取失败 · ERROR</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">星图读取失败</p>
            <p className="mt-1 break-words text-mk-small text-[#9A8E80]">{live.error}</p>
            <button
              type="button"
              onClick={live.reload}
              className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-[#F5EFE7] transition hover:opacity-80"
              style={{ borderColor: "rgba(245,239,231,0.3)" }}
            >
              重试
            </button>
          </>
        )}

        {live.status === "empty" && (
          <>
            <Sys tone="dark">空 · NO STARMAP TODAY</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">今天没有星图</p>
            {/* 后台原话原样给出。空着比摆一张昨天的诚实。 */}
            <p className="mt-1 break-words text-mk-small leading-[1.8] text-[#9A8E80]">
              {live.note || "今天还没有生成。"}
            </p>
            {/* 🚨 这个按钮以前是死的：服务端失败一次就把这一天封了，按下去请求
                照发、结果一模一样。现在按不按得动由服务端的重试次数决定，按不动
                的时候就不摆按钮，改说还要等多久。 */}
            <RetryLine retryAfter={live.retryAfter} onRetry={live.reload} />
          </>
        )}
      </div>
    </div>
  );
}

/**
 * 空星图下面那一行：能重试就给按钮，不能就说清楚为什么。
 *
 * `retryAfter` 是服务端算的秒数（0 = 现在就能试，-1 = 今天不再试了）。倒计时
 * 走完自动把按钮放出来，她不用刷新页面才发现可以再试了。
 */
function RetryLine({ retryAfter, onRetry }: { retryAfter: number; onRetry: () => void }) {
  const [left, setLeft] = useState(retryAfter);

  useEffect(() => setLeft(retryAfter), [retryAfter]);
  useEffect(() => {
    if (left <= 0) return;
    const t = window.setTimeout(() => setLeft(left - 1), 1000);
    return () => window.clearTimeout(t);
  }, [left]);

  if (retryAfter < 0) {
    return (
      <p className="mt-3 text-mk-small text-[#9A8E80]">今天已经试过多次，不再重试。明天会重新生成。</p>
    );
  }
  if (left > 0) {
    return <p className="mt-3 text-mk-small text-[#9A8E80]">{left} 秒后可以再试一次。</p>;
  }
  return (
    <button
      type="button"
      onClick={onRetry}
      className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-[#F5EFE7] transition hover:opacity-80"
      style={{ borderColor: "rgba(245,239,231,0.3)" }}
    >
      重试
    </button>
  );
}

/** `2026-09-03` → `9 月 3 日 · 周三`。坏掉的日期原样显示，不猜。 */
function formatDay(day: string): string {
  const t = Date.parse(day + "T00:00:00");
  if (!Number.isFinite(t)) return day;
  const d = new Date(t);
  const week = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()];
  return `${d.getMonth() + 1} 月 ${d.getDate()} 日 · ${week}`;
}

export { cx };
