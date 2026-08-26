import { BookOpen, ChevronRight, Check } from "lucide-react";
import { Drawer, Icon } from "@/ui";
import type { Reading } from "../api/readings";

/**
 * ReadingHistoryPanel — 我的阅读, the drawer behind the landing page's
 * upper-right entry.
 *
 * THE ORDER IS THE POINT. 还没读完 comes first, newest touched at the top,
 * because an unfinished reading is the only thing on this page with a claim
 * on her attention — it is work she already started and can pick straight
 * back up. 已完成 sits underneath as a record: nothing to resume, only
 * something to look back at.
 *
 * Both sections route to the SAME place, `/readings/:id`; what she gets there
 * depends on the reading's own state, not on which section she clicked from.
 * An unfinished one reopens the room where she left it; a finished one opens
 * the read-only 已完成 panel (ReadingRoomHost), which is why finishing a
 * reading is not something she can accidentally undo by revisiting it.
 *
 * This is history, not a feed: it is a fixed list of HER OWN readings, shown
 * only when she opens the drawer, with no counts to grow and nothing to
 * scroll toward.
 */

export interface ReadingHistoryPanelProps {
  open: boolean;
  onClose: () => void;
  /** null while loading. */
  readings: Reading[] | null;
  error: string | null;
  onSelect: (reading: Reading) => void;
}

/** A reading is finished when the server says so. `finishedAt` is the stamp
 *  `POST /finish` writes alongside `status='finished'`; either alone is
 *  enough, and checking both means a legacy row missing one still lands in
 *  the right section. */
export function isFinished(r: Reading): boolean {
  return r.status === "finished" || Boolean(r.finishedAt);
}

/** Unfinished first, each section newest-first: unfinished by last touch
 *  (that is where she left off), finished by when she finished. */
export function splitReadings(readings: Reading[]): { unfinished: Reading[]; finished: Reading[] } {
  const unfinished = readings
    .filter((r) => !isFinished(r))
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
  const finished = readings
    .filter(isFinished)
    .sort((a, b) => (b.finishedAt ?? b.updatedAt).localeCompare(a.finishedAt ?? a.updatedAt));
  return { unfinished, finished };
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

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div className="px-1 pb-2 text-mk-label text-mk-faint">{children}</div>;
}

function HistoryRow({
  reading,
  meta,
  action,
  tone,
  onSelect,
}: {
  reading: Reading;
  meta: string;
  action: string;
  tone: "open" | "done";
  onSelect: (r: Reading) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(reading)}
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
          icon={tone === "done" ? Check : BookOpen}
          size={16}
          className={tone === "done" ? "text-mk-success" : "text-mk-accent-600"}
        />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-mk-h3 text-mk-ink">{reading.title}</span>
        <span className="block truncate text-mk-small text-mk-muted">{meta}</span>
      </span>
      <span className="flex shrink-0 items-center gap-0.5 text-mk-small text-mk-accent-700">
        {action}
        <Icon icon={ChevronRight} size={14} />
      </span>
    </button>
  );
}

export function ReadingHistoryPanel({
  open,
  onClose,
  readings,
  error,
  onSelect,
}: ReadingHistoryPanelProps) {
  const { unfinished, finished } = splitReadings(readings ?? []);

  return (
    <Drawer open={open} onClose={onClose} side="right">
      <div className="flex flex-col gap-6">
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

        {!error && readings !== null && unfinished.length === 0 && finished.length === 0 && (
          <p className="text-mk-body text-mk-muted">还没有开始过阅读。回到首页，贴一篇进来就开始了。</p>
        )}

        {unfinished.length > 0 && (
          <section>
            <SectionLabel>还没读完 · {unfinished.length}</SectionLabel>
            <div className="flex flex-col gap-2">
              {unfinished.map((r) => (
                <HistoryRow
                  key={r.id}
                  reading={r}
                  tone="open"
                  meta={r.hasSource ? `上次读到 ${shortDay(r.updatedAt)}` : "还没放正文进来"}
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
              {finished.map((r) => (
                <HistoryRow
                  key={r.id}
                  reading={r}
                  tone="done"
                  meta={`完成于 ${shortDay(r.finishedAt ?? r.updatedAt)}`}
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
