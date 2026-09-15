import { DiscoveryDesk } from "./DiscoveryDesk";
import { useEffect, useMemo, useState } from "react";
import { Info, Languages } from "lucide-react";
import { KeywordDrawer } from "../tree/KeywordDrawer";
import { Sys } from "../tree/ui";
import type { FieldId } from "../tree/types";
import type { LiveTree } from "../tree/useInterestTree";
import { type Lang } from "./Planet";
import { NewsSheet } from "./NewsSheet";
import { type SkyPlanet, pickWords, placeWords, planetThreads } from "./skyLayout";
import { useExploreToday } from "./useExploreToday";
import "./explore.css";

/** Today's edition keeps server ranking, read/save state and keyword associations.
 * The discovery desk previews stories; only opening details records "seen".
 * Loading, failure and regeneration retain the existing API behavior. */

const SELECTION_NOTE =
  "这五条是从十二个科学期刊与科普源（Nature、Quanta、arXiv、Phys.org、ScienceDaily 等）当天的" +
  "六十余条里挑出来的。挑选标准是「能不能引出一个你可以自己追问的问题」，不是热度。";

const POLITICS_NOTE =
  "选举、战争、制裁一类的新闻在抓取那一层就被过滤掉了，不会出现在这里。" +
  "这不是说它们不重要，是说这个产品没有接住它们的语境。";

export function ExploreView({ tree }: { tree: LiveTree }) {
  const [openId, setOpenId] = useState<string | null>(null);
  const [openWord, setOpenWord] = useState<string | null>(null);
  const [seen, setSeen] = useState<string[]>([]);
  const [lang, setLang] = useState<Lang>("zh");
  const [note, setNote] = useState(false);
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

  const open = openId ? (live.planets.find((p) => p.id === openId) ?? null) : null;
  const lit = live.planets.filter((p) => seen.includes(p.id)).length;
  const known = live.status === "ready" || live.status === "empty";

  function onOpen(id: string) {
    setSeen((s) => (s.includes(id) ? s : [...s, id]));
    setOpenId(id);
  }

  return (
    <div className="exp-sky discovery-page relative flex min-h-full flex-col overflow-hidden">
      {/* ── 顶栏 ─────────────────────────────────────────────────────────── */}
      <header className="relative z-20 flex flex-wrap items-start justify-between gap-4 px-7 pt-5">
        <div className="min-w-0">
          <Sys tone="dark">今日发现 · DAILY DISCOVERY</Sys>
          <p className="mt-1 text-mk-h2 text-[var(--mk-explore-ink)]">
            {live.day ? formatDay(live.day) : "今天"}
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => setLang((l) => (l === "zh" ? "en" : "zh"))}
            className="inline-flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-small
                       text-[var(--mk-explore-muted)] transition-colors hover:bg-[rgba(240,233,224,.1)]"
            style={{ borderColor: "var(--mk-explore-line)" }}
          >
            <Languages size={14} strokeWidth={1.8} />
            {lang === "zh" ? "中 / EN" : "EN / 中"}
          </button>

          <span className="text-right">
            <Sys tone="dark">已浏览</Sys>
            <span className="block font-mono text-mk-h3 tabular-nums text-[var(--mk-explore-ink)]">
              {known ? `${lit} / ${live.planets.length}` : "—"}
            </span>
          </span>

          {/* 省略要被说出来。一个被过滤过的集合摆成「全部」是用版面撒谎。 */}
          <button
            type="button"
            onClick={() => setNote((v) => !v)}
            aria-expanded={note}
            className="rounded-mk-full border p-2 text-[var(--mk-explore-muted)] transition-colors hover:bg-[rgba(240,233,224,.1)]"
            style={{ borderColor: "var(--mk-explore-line)" }}
            aria-label="这五条是怎么来的"
          >
            <Info size={15} strokeWidth={1.8} />
          </button>
        </div>
      </header>

      {note ? (
        <div className="relative z-20 mx-7 mt-3 rounded-mk-md p-4"
             style={{ background: "var(--mk-explore-surface)", border: "1px solid var(--mk-explore-line)" }}>
          <Sys tone="dark">这五条是怎么来的</Sys>
          <p className="mt-1.5 text-mk-small leading-[1.85] text-[var(--mk-explore-muted)]">{SELECTION_NOTE}</p>
          <p className="mt-2 text-mk-small leading-[1.85] text-[var(--mk-explore-muted)]">{POLITICS_NOTE}</p>
        </div>
      ) : null}

      <div className="discovery-stage relative">
        <ExploreState live={live} />
        <DiscoveryDesk planets={live.planets} lang={lang} seen={seen} onOpen={onOpen} connections={threads} onWord={setOpenWord} />
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
        style={{ borderColor: "var(--mk-explore-line)", background: "var(--mk-explore-surface)" }}
      >
        {live.status === "loading" && (
          <>
            <Sys tone="dark">处理中 · BUILDING</Sys>
            <p className="mt-2 text-mk-body text-[var(--mk-explore-ink)]">正在生成今日发现</p>
            {/* 第一个打开的人触发一次抓取（十二个源）加一次模型调用。说出来，
                别让她以为卡住了。 */}
            <p className="mt-1 text-mk-small leading-[1.8] text-[var(--mk-explore-muted)]">
              正在从十二个科学源里挑今天的五条，需要几秒。
            </p>
          </>
        )}

        {live.status === "error" && (
          <>
            <Sys tone="dark">读取失败 · ERROR</Sys>
            <p className="mt-2 text-mk-body text-[var(--mk-explore-ink)]">今日发现加载失败</p>
            <p className="mt-1 break-words text-mk-small text-[var(--mk-explore-muted)]">{live.error}</p>
            <button
              type="button"
              onClick={live.reload}
              className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-[var(--mk-explore-ink)] transition hover:opacity-80"
              style={{ borderColor: "var(--mk-explore-line)" }}
            >
              重试
            </button>
          </>
        )}

        {live.status === "empty" && (
          <>
            <Sys tone="dark">空 · NO STARMAP TODAY</Sys>
            <p className="mt-2 text-mk-body text-[var(--mk-explore-ink)]">今天暂无发现内容</p>
            {/* 后台原话原样给出。空着比摆一张昨天的诚实。 */}
            <p className="mt-1 break-words text-mk-small leading-[1.8] text-[var(--mk-explore-muted)]">
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
      <p className="mt-3 text-mk-small text-[var(--mk-explore-muted)]">今日重试次数已用完，请明天再试。</p>
    );
  }
  if (left > 0) {
    return <p className="mt-3 text-mk-small text-[var(--mk-explore-muted)]">{left} 秒后可以再试一次。</p>;
  }
  return (
    <button
      type="button"
      onClick={onRetry}
      className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-[var(--mk-explore-ink)] transition hover:opacity-80"
      style={{ borderColor: "var(--mk-explore-line)" }}
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

export { cx } from "../tree/ui";
