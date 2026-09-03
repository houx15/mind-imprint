import { useRef, useState } from "react";
import { Info, Languages } from "lucide-react";
import { Sys, cx } from "../tree/ui";
import { useFitScale } from "../tree/useFitScale";
import { Planet, type Lang } from "./Planet";
import { NewsSheet } from "./NewsSheet";
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
 * 5. **一颗星球的颜色 = 它会长在树的哪根枝上。** 两屏共用同一套七色，所以
 *    收藏一颗蓝色的星球，就是往树上那根蓝色的枝加一个词。
 *
 * ## 从原型搬过来时改掉的三件事（2026-09-03）
 *
 * - **数据是真的。** 十二个源、抓取、去重、政治过滤、一次模型调用选五条 ——
 *   全在服务端（`internal/news` + `internal/api/explore.go`）。
 * - **按八个「领域」上色改成按七根主枝上色。** 原来那套分类只活在这一屏里。
 * - **日期轴去掉了。** 原型可以左右翻五天的 mock 数据；真数据现在只有今天。
 *   翻看历史是后面的事，摆一个只能停在今天的轴是一个骗人的控件。
 */

/** 手摆的舞台位置，按 rank（1 = 今天最重要的那条）。
 *
 *  五个物体在一块画布上是一次**构图**，而构图永远赢过分布 —— 所以不用算法排。
 *  最近的一对（1 和 4）在 1100px 宽时留出约 60px，够它们各自 ±40px 的漂移。 */
const SLOTS: Record<number, { x: string; y: string; size: number; drift: string }> = {
  1: { x: "27%", y: "41%", size: 236, drift: "exp-drift" },
  2: { x: "61%", y: "24%", size: 200, drift: "exp-drift-1" },
  3: { x: "79%", y: "63%", size: 178, drift: "exp-drift-2" },
  4: { x: "11%", y: "80%", size: 158, drift: "exp-drift-3" },
  5: { x: "46%", y: "77%", size: 150, drift: "exp-drift-4" },
};

const SELECTION_NOTE =
  "这五条是从十二个科学期刊与科普源（Nature、Quanta、arXiv、Phys.org、ScienceDaily 等）当天的" +
  "六十余条里挑出来的。挑选标准是「能不能引出一个你可以自己追问的问题」，不是热度。";

const POLITICS_NOTE =
  "选举、战争、制裁一类的新闻在抓取那一层就被过滤掉了，不会出现在这里。" +
  "这不是说它们不重要，是说这个产品没有接住它们的语境。";

export function ExploreView() {
  const [openId, setOpenId] = useState<string | null>(null);
  const [seen, setSeen] = useState<string[]>([]);
  const [lang, setLang] = useState<Lang>("zh");
  const [note, setNote] = useState(false);
  const fieldRef = useRef<HTMLDivElement>(null);
  // 星球是固定像素，舞台会缩。没有这个，700px 高的窗口上 1 号星球的标题会压到
  // 5 号身上。
  const scale = useFitScale(fieldRef, 640, 0.58);

  const live = useExploreToday();
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
            五条今天值得知道的事。把光标移上去，它会先问你一个问题。
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
          {/* 轨道环：给这片场地一个中心，又不抢注意力。 */}
          <span className="exp-orbit absolute left-1/2 top-1/2 h-[62%] w-[62%] -translate-x-1/2 -translate-y-1/2" />
          <span className="exp-orbit exp-orbit-2 absolute left-1/2 top-1/2 h-[86%] w-[86%] -translate-x-1/2 -translate-y-1/2" />
          <span className="exp-orbit exp-orbit-3 absolute left-1/2 top-1/2 h-[110%] w-[110%] -translate-x-1/2 -translate-y-1/2" />

          <ExploreState live={live} />

          {live.planets.map((p) => {
            const slot = SLOTS[p.rank] ?? SLOTS[5]!;
            return (
              <Planet
                key={p.id}
                item={p}
                lang={lang}
                slot={{ ...slot, size: Math.round(slot.size * scale) }}
                discovered={seen.includes(p.id)}
                kept={p.saved}
                dimmed={false}
                onOpen={() => onOpen(p.id)}
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
      </div>
    </div>
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
