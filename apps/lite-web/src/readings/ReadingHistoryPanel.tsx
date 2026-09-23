import { useEffect, useState } from "react";
import { BookOpen, ChevronRight, Check, Archive } from "lucide-react";
import { Drawer, Icon, Illustration } from "@/ui";
import type { Reading } from "../api/readings";

/**
 * ReadingHistoryPanel — 我的阅读, the drawer behind the landing page's
 * upper-right entry and behind the 「你有 N 篇还没读完」 notice.
 *
 * ## One list, sorted by time, filtered by chip (2026-08-28)
 *
 * It used to be two fixed sections (还没读完 above 已完成) under
 * `mk-label`-sized headers — 11px, 0.1em tracking, `--mk-faint`, which is the
 * smallest and palest type in the system carrying the one number that tells
 * her what to do next:
 *
 *   > 还没读完 · 8 is too small to be noticed and absolutely not follow our
 *   > design tokens. […] we should have a time sorting and filter. like the
 *   > unread ones have a red/orange dot, and generally sort by time. only load
 *   > the most recent xxx ones.
 *
 * So: **one list in time order**, newest first, because "what was I last
 * doing" is the question a history answers. The two groups did not disappear
 * — they became chips at a size she can hit, which is also what the notice
 * hands her (`initialFilter="open"`, so 「你有 8 篇还没读完」 opens onto those
 * eight rather than onto everything).
 *
 * An unfinished reading carries an accent dot before its title: at a glance,
 * without reading a word, she can see which rows still want her.
 *
 * Both filters route to the SAME place, `/readings/:id`; what she gets there
 * depends on the reading's own state, not on which chip was selected. An
 * unfinished one reopens the room where she left it; a finished one opens the
 * read-only 已完成 panel (ReadingRoomHost), which is why finishing a reading
 * is not something she can accidentally undo by revisiting it.
 *
 * This is history, not a feed: HER OWN readings, shown only when she opens the
 * drawer, capped at the most recent `limit` — no counts to grow, nothing to
 * scroll toward.
 */

export type ReadingFilter = "all" | "open" | "done";

export interface ReadingHistoryPanelProps {
  open: boolean;
  onClose: () => void;
  /** null while loading. */
  readings: Reading[] | null;
  error: string | null;
  onSelect: (reading: Reading) => void;
  /** Which chip the drawer opens on. The 还没读完 notice passes "open" so the
   *  list it pops is the list the notice was counting. */
  initialFilter?: ReadingFilter;
  /** How many rows to render. The server already caps what it sends; this is
   *  the second half of the same promise — a drawer is for picking up where
   *  she left off, not for scrolling a year of history. */
  limit?: number;
  /**
   * 把这一篇从列表里收起来。没传就不摆那个按钮。
   *
   * 🚨 2026-09-23 产品负责人第 3 条：「阅读列表里旧的也没办法删除。」
   * 粘错一次就多一条永远去不掉的记录 —— 而「再开一个新的」正是产品让她做的事。
   *
   * 叫 archive 不叫 delete，因为服务端做的就是收起来：她的段落、批注、
   * 和印记说过的话一条都没删（铁律④），老师那一侧照旧看得见。
   */
  onArchive?: (reading: Reading) => Promise<void> | void;
}

const DEFAULT_LIMIT = 40;

/** A reading is finished when the server says so. `finishedAt` is the stamp
 *  `POST /finish` writes alongside `status='finished'`; either alone is
 *  enough, and checking both means a legacy row missing one still lands in
 *  the right group. */
export function isFinished(r: Reading): boolean {
  return r.status === "finished" || Boolean(r.finishedAt);
}

/**
 * When this reading last mattered.
 *
 * For work still open that is `lastActivityAt` (`atom.last_activity_at`) —
 * NOT `updatedAt`, which only rename and finish ever write, so ordering by it
 * put the reading she spent the last hour in wherever it happened to have
 * been created. For work she finished it is the moment she finished it.
 */
export function sortKeyOf(r: Reading): string {
  return isFinished(r) ? (r.finishedAt ?? r.updatedAt) : r.lastActivityAt;
}

/** The whole history in one line, newest first. */
export function sortReadings(readings: Reading[]): Reading[] {
  return [...readings].sort((a, b) => sortKeyOf(b).localeCompare(sortKeyOf(a)));
}

/** Unfinished first, each group newest-first. Kept because the two groups are
 *  still what the chips select, and what the landing's notice counts. */
export function splitReadings(readings: Reading[]): { unfinished: Reading[]; finished: Reading[] } {
  const sorted = sortReadings(readings);
  return { unfinished: sorted.filter((r) => !isFinished(r)), finished: sorted.filter(isFinished) };
}

/** 今天 / 昨天 / 8月26日 — a date a student reads at a glance, not a
 *  timestamp. Anything unparseable degrades to an empty string rather than
 *  rendering "Invalid Date". */
export function shortDay(iso: string | null): string {
  if (!iso) return "";
  const then = new Date(iso);
  if (Number.isNaN(then.getTime())) return "";
  const startOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((startOf(new Date()) - startOf(then)) / 86_400_000);
  if (days <= 0) return "今天";
  if (days === 1) return "昨天";
  return `${then.getMonth() + 1}月${then.getDate()}日`;
}

function FilterChip({
  active,
  count,
  onClick,
  children,
}: {
  active: boolean;
  count: number;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={[
        "flex items-center gap-1.5 rounded-mk-full border px-3 py-1.5 text-mk-body transition-colors duration-[120ms] ease-mk",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
        active
          ? "border-transparent text-white"
          : "border-mk-border text-mk-secondary hover:border-mk-accent-200 hover:text-mk-accent-700",
      ].join(" ")}
      style={active ? { background: "var(--mk-accent-500)" } : undefined}
    >
      {children}
      <span className="tabular-nums opacity-70">{count}</span>
    </button>
  );
}

function HistoryRow({
  reading,
  meta,
  action,
  tone,
  onSelect,
  onArchive,
}: {
  reading: Reading;
  meta: string;
  action: string;
  tone: "open" | "done";
  onSelect: (r: Reading) => void;
  onArchive?: (r: Reading) => Promise<void> | void;
}) {
  // 收起来要按两次。第二次那一下是「确认」，不是弹窗 ——
  // 🚨 window.confirm 会把整个页面挡住，走查和自动化都过不去。
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);

  return (
    // 🚨 外层从 <button> 换成 <div>：按钮不能套按钮（HTML 不允许，而且
    // 里面那一下会被外面那一下吃掉 —— memory control-that-is-not-wired-2026-09-22
    // 里三个假控件之一就是这个形状）。整行可点的那一块自己是一个 button。
    <div className="group flex w-full items-center gap-1 rounded-mk-md border border-mk-border bg-mk-surface transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:bg-mk-accent-50">
    <button
      type="button"
      onClick={() => onSelect(reading)}
      className="flex min-w-0 flex-1 items-center gap-3 rounded-mk-md px-3 py-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full"
        style={{
          background:
            tone === "done"
              ? "color-mix(in srgb, var(--mk-success) 16%, var(--mk-surface))"
              : "color-mix(in srgb, var(--mk-accent-500) 14%, var(--mk-surface))",
        }}
      >
        <Icon
          icon={tone === "done" ? Check : BookOpen}
          size={16}
          className={tone === "done" ? "text-mk-success" : "text-mk-accent-600"}
        />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          {/* 还没读完 wears a dot. It is the same signal a mail client spends
              on an unread message, and it survives being read at a glance —
              which the meta line underneath does not. */}
          {tone === "open" && (
            <span
              aria-label="还没读完"
              className="h-[7px] w-[7px] shrink-0 rounded-mk-full"
              style={{ background: "var(--mk-accent-500)" }}
            />
          )}
          <span className="min-w-0 truncate text-mk-h3 text-mk-ink">{reading.title}</span>
        </span>
        <span className="block truncate text-mk-small text-mk-muted">{meta}</span>
      </span>
      <span className="flex shrink-0 items-center gap-0.5 text-mk-small text-mk-accent-700">
        {action}
        <Icon icon={ChevronRight} size={14} />
      </span>
    </button>
    {onArchive &&
      (confirming ? (
        <span className="flex shrink-0 items-center gap-1 pr-2">
          <button
            type="button"
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await onArchive(reading);
              } finally {
                setBusy(false);
                setConfirming(false);
              }
            }}
            className="rounded-mk-sm px-2 py-1 text-mk-small text-mk-danger underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            {busy ? "收起中…" : "确认收起"}
          </button>
          <button
            type="button"
            onClick={() => setConfirming(false)}
            className="rounded-mk-sm px-2 py-1 text-mk-small text-mk-muted underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            取消
          </button>
        </span>
      ) : (
        <button
          type="button"
          aria-label={`收起《${reading.title}》`}
          onClick={() => setConfirming(true)}
          // 🚨 **一直看得见**，只是淡。
          //
          // 第一版写的是 `opacity-0 group-hover:opacity-100` —— 在真浏览器里
          // 看着挺干净，可触摸屏上**没有 hover**：一个用 iPad 的学生永远
          // 看不见这颗按钮，而初中生用平板的不少。
          // 「悬停才出现」在这个产品里等于「一半的人没有这个功能」。
          className="mr-2 flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-mk-muted opacity-50 transition-opacity duration-[120ms] ease-mk hover:text-mk-danger focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 group-hover:opacity-100"
        >
          <Icon icon={Archive} size={15} />
        </button>
      ))}
    </div>
  );
}

export function ReadingHistoryPanel({
  open,
  onClose,
  readings,
  error,
  onSelect,
  initialFilter = "all",
  limit = DEFAULT_LIMIT,
  onArchive,
}: ReadingHistoryPanelProps) {
  const [filter, setFilter] = useState<ReadingFilter>(initialFilter);
  // Re-armed on every OPEN, not on every render: 我的阅读 and the 还没读完
  // notice are two doors into the same drawer and they want different chips,
  // so which one she came through has to win over whatever she last picked.
  useEffect(() => {
    if (open) setFilter(initialFilter);
  }, [open, initialFilter]);

  const all = sortReadings(readings ?? []);
  const openCount = all.filter((r) => !isFinished(r)).length;
  const doneCount = all.length - openCount;
  const matching =
    filter === "all" ? all : filter === "open" ? all.filter((r) => !isFinished(r)) : all.filter(isFinished);
  const shown = matching.slice(0, limit);
  const hidden = matching.length - shown.length;

  return (
    <Drawer open={open} onClose={onClose} side="right">
      <div className="flex flex-col gap-5">
        <div className="flex items-baseline justify-between">
          <h2 className="text-mk-h1 text-mk-ink">我的阅读</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-mk-small text-mk-muted underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            关闭
          </button>
        </div>

        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
        {!error && readings === null && <p className="text-mk-body text-mk-muted">加载中…</p>}

        {!error && readings !== null && all.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <Illustration name="bookLover" className="h-[120px] w-[120px]" />
            <p className="text-mk-body text-mk-muted">还没有开始过阅读。回到首页，贴一篇进来就开始了。</p>
          </div>
        )}

        {!error && all.length > 0 && (
          <>
            <div className="flex flex-wrap gap-2">
              <FilterChip active={filter === "all"} count={all.length} onClick={() => setFilter("all")}>
                全部
              </FilterChip>
              <FilterChip active={filter === "open"} count={openCount} onClick={() => setFilter("open")}>
                还没读完
              </FilterChip>
              <FilterChip active={filter === "done"} count={doneCount} onClick={() => setFilter("done")}>
                已完成
              </FilterChip>
            </div>

            {shown.length === 0 ? (
              <p className="py-6 text-center text-mk-body text-mk-muted">
                {filter === "open" ? "都读完了，这里空着。" : "还没有读完的。"}
              </p>
            ) : (
              <div className="flex flex-col gap-2">
                {shown.map((r) => {
                  const done = isFinished(r);
                  return (
                    <HistoryRow
                      key={r.id}
                      reading={r}
                      tone={done ? "done" : "open"}
                      meta={
                        done
                          ? `完成于 ${shortDay(r.finishedAt ?? r.updatedAt)}`
                          : r.hasSource
                            ? `上次读到 ${shortDay(r.lastActivityAt)}`
                            : "还没放正文进来"
                      }
                      onArchive={onArchive}
                      action={done ? "看报告" : "继续"}
                      onSelect={onSelect}
                    />
                  );
                })}
              </div>
            )}

            {hidden > 0 && (
              <p className="text-center text-mk-small text-mk-faint">只显示最近 {shown.length} 篇</p>
            )}
          </>
        )}
      </div>
    </Drawer>
  );
}
