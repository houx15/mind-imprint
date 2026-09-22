import type { AwakeningReportRow, AwakeningThread } from "../api/awakening";
import { HISTORY, LIBRARY, TERMINAL } from "./content";

/**
 * 线索库那一屏显示什么。
 *
 * 摘成纯函数的理由和 hub.ts 一样：**一行说错的字不会报错**，它只会让她认错
 * 线索、点进一条她以为是别的那条。而这里有三件容易悄悄写反的事：
 *
 *   没起名的线索要退回显示她的原话（否则库里是一排空行）
 *   已经总结过的线索仍然可以接着问（不是只读的历史）
 *   「上次动过」要按她的日子算，不是打印一个时间戳
 */

export interface LibraryRow {
  id: string;
  /** 显示出来的名字：她挑定的那个，没有就用她自己写下的第一句。 */
  name: string;
  /** 这一行没有名字，显示的是她的原话。界面据此加一个「未命名」的标记。 */
  unnamed: boolean;
  /** 「已答 3 / 8」「已总结」这类状态字。 */
  state: string;
  /** 「今天」「3 天前」。 */
  when: string;
  summarized: boolean;
  /** 总结过不止一次时那一句；否则空串。 */
  extra: string;
}

/** 线索库一行放得下的字数。和服务端起名那边的上限是同一个数。 */
const NAME_MAX = 14;

/**
 * 她那一条能显示出来的名字。没起名就退回她自己写的第一句。
 *
 * 退回来的那一句按标签的宽度裁 —— 库是一张表，一行两行地铺开就认不快了。
 * 🚨 裁的只是**这一行显示的字**：她写的那句话原样留在对话里，一个字都没少
 * （memory: observation-tool-is-the-bug-2026-09-12）。点进去看到的还是全文。
 */
export function threadName(t: Pick<AwakeningThread, "title" | "firstText">): string {
  const title = t.title.trim();
  if (title) return title;
  return trimLabel(t.firstText);
}

/** 按第一个句读断句，再按宽度裁。和服务端 FallbackTitle 同一条规矩。 */
export function trimLabel(text: string): string {
  let s = text.trim().replace(/\s+/g, " ");
  if (s === "") return "";
  const cut = s.search(/[。！？，、；!?,;]/);
  if (cut > 0 && [...s.slice(0, cut)].length >= 4) s = s.slice(0, cut);
  const chars = [...s];
  return (chars.length > NAME_MAX ? chars.slice(0, NAME_MAX).join("") : s).trim();
}

/**
 * 上次动过是什么时候，按她的日子说。
 *
 * 🚨 比的是**日期**，不是过了多少小时：昨天夜里十一点和今天早上八点隔了九个
 * 小时，但对她来说那是「昨天」。
 */
export function whenLabel(iso: string, now: Date): string {
  const t = new Date(iso);
  if (!iso || Number.isNaN(t.getTime())) return "";
  const days = Math.round(
    (new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() -
      new Date(t.getFullYear(), t.getMonth(), t.getDate()).getTime()) /
      86_400_000,
  );
  if (days <= 0) return LIBRARY.today;
  if (days === 1) return LIBRARY.yesterday;
  return LIBRARY.daysAgo.replace("{n}", String(days));
}

export function libraryRows(threads: AwakeningThread[], now: Date): LibraryRow[] {
  return threads.map((t) => {
    const name = threadName(t);
    return {
      id: t.id,
      // 一条刚开、一个字都还没写的线索：这两样都没有，界面仍然要有东西可显示。
      name: name || LIBRARY.unnamed,
      unnamed: t.title.trim() === "",
      state: t.summarized
        ? LIBRARY.summarized
        : LIBRARY.progress.replace("{n}", String(Math.min(t.turnCount, TERMINAL.steps.length))),
      when: whenLabel(t.lastTurnAt || t.updatedAt, now),
      summarized: t.summarized,
      extra: t.reportCount > 1 ? LIBRARY.reports.replace("{n}", String(t.reportCount)) : "",
    };
  });
}

/* ── 兴趣印记那张表 ─────────────────────────────────────────────────────── */

export interface HistoryRow {
  id: string;
  runId: string;
  /** 这条线索的名字；没起名就退回她自己写的第一句。 */
  name: string;
  /** 「今天 · 3 个词」。 */
  meta: string;
}

/**
 * 印记那张表怎么摆。
 *
 * 和线索库同一条命名规矩（threadName），所以同一条线索在两处叫同一个名字 ——
 * 两处各写一遍就会在她起名之后只改了一边。
 */
export function historyRows(reports: AwakeningReportRow[], now: Date): HistoryRow[] {
  return reports.map((p) => ({
    id: p.id,
    runId: p.runId,
    name: threadName(p) || HISTORY.title,
    meta: [
      whenLabel(p.createdAt, now),
      // 🚨 一个词都没长出来的那一份照实说。留空会让那一行看起来像是没加载出来，
      // 而「这一趟没长出词」本身就是结果（memory: ai-errors-must-surface-never-fake）。
      p.wordCount > 0 ? HISTORY.words.replace("{n}", String(p.wordCount)) : HISTORY.noWords,
    ]
      .filter(Boolean)
      .join(" · "),
  }));
}
