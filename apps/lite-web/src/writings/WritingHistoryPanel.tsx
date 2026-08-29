import { useEffect, useState } from "react";
import { PenLine, ChevronRight, Check } from "lucide-react";
import { Drawer, Icon, Illustration } from "@/ui";
import type { Writing } from "../api/writings";
import { isWritingFinished } from "../api/writings";

/**
 * WritingHistoryPanel — 我的写作, the drawer behind the landing page's
 * upper-right entry and behind the 「你有 N 篇还没写完」 notice.
 *
 * Exact mirror of readings/ReadingHistoryPanel (2026-08-28's redesign),
 * ported to the writing vocabulary — see that file's comment for the full
 * reasoning. In short: **one list in time order**, newest first, with chips
 * (全部 / 还没写完 / 已完成) replacing what used to be two fixed sections
 * under an 11px `mk-label` header carrying the one number that mattered —
 * too small to be noticed and not on our design tokens. An unfinished
 * writing carries an accent dot before its title; both filters route to the
 * SAME place, `/writings/:id`, and what she gets there depends on the
 * writing's own state, not on which chip was selected.
 *
 * "Last mattered" for writing is `atom.last_activity_at`
 * (`writingDTO.lastActivityAt`, writings.go) — NOT `updatedAt`, which only
 * rename/stage-change/target-words ever write, so an hour spent drafting
 * moved neither. This used to be a stale gap (writing had no equivalent
 * field at all); writings.go now bumps it through the same `loadOwnedAtom`
 * chokepoint reading does.
 */

export type WritingFilter = "all" | "open" | "done";

export interface WritingHistoryPanelProps {
  open: boolean;
  onClose: () => void;
  /** null while loading. */
  writings: Writing[] | null;
  error: string | null;
  onSelect: (writing: Writing) => void;
  /** Which chip the drawer opens on. The 还没写完 notice passes "open" so the
   *  list it pops is the list the notice was counting. */
  initialFilter?: WritingFilter;
  /** How many rows to render. The server already caps what it sends; this is
   *  the second half of the same promise — a drawer is for picking up where
   *  she left off, not for scrolling a year of history. */
  limit?: number;
}

const DEFAULT_LIMIT = 40;

/** When this writing last mattered — mirrors readings/ReadingHistoryPanel's
 *  `sortKeyOf` exactly. */
export function sortKeyOf(w: Writing): string {
  return isWritingFinished(w) ? (w.finishedAt ?? w.updatedAt) : w.lastActivityAt;
}

/** The whole history in one line, newest first. */
export function sortWritings(writings: Writing[]): Writing[] {
  return [...writings].sort((a, b) => sortKeyOf(b).localeCompare(sortKeyOf(a)));
}

/** Unfinished first, each group newest-first. Kept because the two groups are
 *  still what the chips select. */
export function splitWritings(writings: Writing[]): { unfinished: Writing[]; finished: Writing[] } {
  const sorted = sortWritings(writings);
  return {
    unfinished: sorted.filter((w) => !isWritingFinished(w)),
    finished: sorted.filter(isWritingFinished),
  };
}

/** 今天 / 昨天 / 8月26日 — mirrors readings/ReadingHistoryPanel's `shortDay`. */
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

const STAGE_LABELS: Record<string, string> = {
  ideate: "构思",
  outline: "大纲",
  snippets: "段落",
  draft: "成稿",
  finished: "成稿",
};

function stageLabel(stage: string): string {
  return STAGE_LABELS[stage] ?? "写作";
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
  writing,
  meta,
  action,
  tone,
  onSelect,
}: {
  writing: Writing;
  meta: string;
  action: string;
  tone: "open" | "done";
  onSelect: (w: Writing) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(writing)}
      className="group flex w-full items-center gap-3 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-3 text-left transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
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
          icon={tone === "done" ? Check : PenLine}
          size={16}
          className={tone === "done" ? "text-mk-success" : "text-mk-accent-600"}
        />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5">
          {/* 还没写完 wears a dot — same at-a-glance signal as reading's. */}
          {tone === "open" && (
            <span
              aria-label="还没写完"
              className="h-[7px] w-[7px] shrink-0 rounded-mk-full"
              style={{ background: "var(--mk-accent-500)" }}
            />
          )}
          <span className="min-w-0 truncate text-mk-h3 text-mk-ink">
            {writing.title || "还没起名字的写作"}
          </span>
        </span>
        <span className="block truncate text-mk-small text-mk-muted">{meta}</span>
      </span>
      <span className="flex shrink-0 items-center gap-0.5 text-mk-small text-mk-accent-700">
        {action}
        <Icon icon={ChevronRight} size={14} />
      </span>
    </button>
  );
}

export function WritingHistoryPanel({
  open,
  onClose,
  writings,
  error,
  onSelect,
  initialFilter = "all",
  limit = DEFAULT_LIMIT,
}: WritingHistoryPanelProps) {
  const [filter, setFilter] = useState<WritingFilter>(initialFilter);
  // Re-armed on every OPEN, not on every render — same reasoning as
  // ReadingHistoryPanel: 我的写作 and the 还没写完 notice are two doors into
  // the same drawer wanting different chips, so which one she came through
  // has to win over whatever she last picked.
  useEffect(() => {
    if (open) setFilter(initialFilter);
  }, [open, initialFilter]);

  const all = sortWritings(writings ?? []);
  const openCount = all.filter((w) => !isWritingFinished(w)).length;
  const doneCount = all.length - openCount;
  const matching =
    filter === "all"
      ? all
      : filter === "open"
        ? all.filter((w) => !isWritingFinished(w))
        : all.filter(isWritingFinished);
  const shown = matching.slice(0, limit);
  const hidden = matching.length - shown.length;

  return (
    <Drawer open={open} onClose={onClose} side="right">
      <div className="flex flex-col gap-5">
        <div className="flex items-baseline justify-between">
          <h2 className="text-mk-h1 text-mk-ink">我的写作</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-mk-small text-mk-muted underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            关闭
          </button>
        </div>

        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
        {!error && writings === null && <p className="text-mk-body text-mk-muted">加载中…</p>}

        {!error && writings !== null && all.length === 0 && (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <Illustration name="writing" className="h-[120px] w-[120px]" />
            <p className="text-mk-body text-mk-muted">还没有开始过写作。回到首页，说一句你想写什么就开始了。</p>
          </div>
        )}

        {!error && all.length > 0 && (
          <>
            <div className="flex flex-wrap gap-2">
              <FilterChip active={filter === "all"} count={all.length} onClick={() => setFilter("all")}>
                全部
              </FilterChip>
              <FilterChip active={filter === "open"} count={openCount} onClick={() => setFilter("open")}>
                还没写完
              </FilterChip>
              <FilterChip active={filter === "done"} count={doneCount} onClick={() => setFilter("done")}>
                已完成
              </FilterChip>
            </div>

            {shown.length === 0 ? (
              <p className="py-6 text-center text-mk-body text-mk-muted">
                {filter === "open" ? "都写完了，这里空着。" : "还没有写完的。"}
              </p>
            ) : (
              <div className="flex flex-col gap-2">
                {shown.map((w) => {
                  const done = isWritingFinished(w);
                  return (
                    <HistoryRow
                      key={w.id}
                      writing={w}
                      tone={done ? "done" : "open"}
                      meta={
                        done
                          ? `完成于 ${shortDay(w.finishedAt ?? w.updatedAt)}`
                          : `${stageLabel(w.stage)} · 上次改到 ${shortDay(w.lastActivityAt)}`
                      }
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
