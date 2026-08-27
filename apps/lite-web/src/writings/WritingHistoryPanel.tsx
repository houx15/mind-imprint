import { PenLine, ChevronRight, Check } from "lucide-react";
import { Drawer, Icon } from "@/ui";
import type { Writing } from "../api/writings";
import { isWritingFinished } from "../api/writings";

/**
 * WritingHistoryPanel — 我的写作, the drawer behind the landing page's
 * upper-right entry. Same order-is-the-point structure as
 * readings/ReadingHistoryPanel — 还没写完 (newest touched) above 已完成 — and
 * the same "both sections route to the same place, the writing's own state
 * decides what she gets there" rule.
 */

export interface WritingHistoryPanelProps {
  open: boolean;
  onClose: () => void;
  /** null while loading. */
  writings: Writing[] | null;
  error: string | null;
  onSelect: (writing: Writing) => void;
}

/** Unfinished first, each section newest-first: unfinished by last update
 *  (there is no separate last-activity stamp on `writing`, unlike reading —
 *  every write path that touches the atom also bumps `writing.updated_at`),
 *  finished by when she finished. */
export function splitWritings(writings: Writing[]): { unfinished: Writing[]; finished: Writing[] } {
  const unfinished = writings
    .filter((w) => !isWritingFinished(w))
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
  const finished = writings
    .filter(isWritingFinished)
    .sort((a, b) => (b.finishedAt ?? b.updatedAt).localeCompare(a.finishedAt ?? a.updatedAt));
  return { unfinished, finished };
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

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div className="px-1 pb-2 text-mk-label text-mk-faint">{children}</div>;
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
        <span className="block truncate text-mk-h3 text-mk-ink">{writing.title || "还没起名字的写作"}</span>
        <span className="block truncate text-mk-small text-mk-muted">{meta}</span>
      </span>
      <span className="flex shrink-0 items-center gap-0.5 text-mk-small text-mk-accent-700">
        {action}
        <Icon icon={ChevronRight} size={14} />
      </span>
    </button>
  );
}

export function WritingHistoryPanel({ open, onClose, writings, error, onSelect }: WritingHistoryPanelProps) {
  const { unfinished, finished } = splitWritings(writings ?? []);

  return (
    <Drawer open={open} onClose={onClose} side="right">
      <div className="flex flex-col gap-6">
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

        {!error && writings !== null && unfinished.length === 0 && finished.length === 0 && (
          <p className="text-mk-body text-mk-muted">还没有开始过写作。回到首页，说一句你想写什么就开始了。</p>
        )}

        {unfinished.length > 0 && (
          <section>
            <SectionLabel>还没写完 · {unfinished.length}</SectionLabel>
            <div className="flex flex-col gap-2">
              {unfinished.map((w) => (
                <HistoryRow
                  key={w.id}
                  writing={w}
                  tone="open"
                  meta={`${stageLabel(w.stage)} · 上次改到 ${shortDay(w.updatedAt)}`}
                  action="继续"
                  onSelect={onSelect}
                />
              ))}
            </div>
          </section>
        )}

        {finished.length > 0 && (
          <section>
            <SectionLabel>已完成 · {finished.length}</SectionLabel>
            <div className="flex flex-col gap-2">
              {finished.map((w) => (
                <HistoryRow
                  key={w.id}
                  writing={w}
                  tone="done"
                  meta={`完成于 ${shortDay(w.finishedAt ?? w.updatedAt)}`}
                  action="看报告"
                  onSelect={onSelect}
                />
              ))}
            </div>
          </section>
        )}
      </div>
    </Drawer>
  );
}
