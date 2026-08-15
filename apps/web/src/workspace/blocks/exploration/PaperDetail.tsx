import type { ReactNode } from "react";
import type { DigCandidate, Reference } from "@mind-imprint/contracts";

// PaperDetail — the ONE per-paper detail layout, shared by the research-map
// search flow (a not-yet-added DigCandidate) and the graph-node sidebar (an
// added Reference). Same labeled metadata + abstract + actions; the only
// difference is the action bar and the "已在图谱 / 待加入" state, plus optional
// children (evidence note, paste box) the node view slots in.

const READING_STATUS_LABEL: Record<string, string> = {
  to_read: "待读",
  reading: "在读",
  done: "读完",
};

export type PaperView = {
  title: string;
  authors: string;
  year: string;
  journal: string;
  abstract: string;
  link: string; // resolved original-source url (doi → doi.org/…)
  doi: string;
  added: boolean;
  statusLabel?: string; // reading status, only when added
};

/** Normalize a search-result candidate (not yet on the map) into a PaperView. */
export function candidateToPaperView(c: DigCandidate, added: boolean): PaperView {
  return {
    title: c.title,
    authors: c.authors?.trim() || "",
    year: c.year?.trim() || "",
    journal: c.journal?.trim() || "",
    abstract: c.abstract?.trim() || "",
    link: c.url || (c.doi ? "https://doi.org/" + c.doi : ""),
    doi: c.doi?.trim() || "",
    added,
  };
}

/** Normalize an added reference (already on the map) into a PaperView. */
export function referenceToPaperView(r: Reference): PaperView {
  return {
    title: r.title,
    authors: r.author?.trim() || "",
    year: r.year?.trim() || "",
    journal: r.journal?.trim() || "",
    abstract: r.abstract?.trim() || "",
    link: r.url?.trim() || "",
    doi: "",
    added: true,
    statusLabel: READING_STATUS_LABEL[r.readingStatus],
  };
}

export type PaperAction = { label: string; onClick: () => void; busy?: boolean; disabled?: boolean };

export function PaperDetail({
  paper,
  primaryAction,
  secondaryAction,
  onFind,
  finding,
  flush,
  children,
}: {
  paper: PaperView;
  /** The main action: 采纳/收进未归类 (candidate) or 进入阅读室 (added reference). */
  primaryAction?: PaperAction;
  /** An optional lighter action next to it (e.g. 丢弃). */
  secondaryAction?: PaperAction;
  /** Related/cited lookups — 找相似 / 找它引用的 / 找引用它的. Absent → not shown. */
  onFind?: (mode: "similar" | "citation" | "cited") => void;
  /** A find lookup is in flight (disables the find row). */
  finding?: boolean;
  /** Render without the outer card chrome — for embedding in the node sidebar's
      own bordered panel (so it isn't a card-inside-a-card). */
  flush?: boolean;
  /** Extra panels the node view slots in (evidence note, paste box). */
  children?: ReactNode;
}) {
  const inner = (
    <>
      <div className="flex items-center gap-2">
        <span className="rounded-full bg-mk-success-bg px-2 py-0.5 text-[12px] font-bold text-mk-success">论文</span>
        {paper.statusLabel && (
          <span className="rounded-full bg-mk-paper px-2 py-0.5 text-[12px] font-bold text-mk-faint">{paper.statusLabel}</span>
        )}
        <span
          className={`rounded-full px-2 py-0.5 text-[12px] font-bold ${
            paper.added ? "bg-mk-accent-50 text-mk-accent" : "bg-mk-paper text-mk-faint"
          }`}
        >
          {paper.added ? "已在图谱" : "待加入"}
        </span>
      </div>

      <h3 className="mt-2 text-[14.5px] font-bold leading-snug text-mk-ink">{paper.title}</h3>

      <dl className="mt-3 space-y-1.5 text-[12px]">
        <MetaRow label="作者" value={paper.authors} />
        <MetaRow label="年份" value={paper.year} />
        <MetaRow label="期刊" value={paper.journal} />
        <div className="flex gap-2">
          <dt className="w-9 flex-none font-bold text-mk-faint">链接</dt>
          <dd className="min-w-0 flex-1">
            {paper.link ? (
              <a href={paper.link} target="_blank" rel="noopener noreferrer" className="break-all font-semibold text-mk-accent underline-offset-2 hover:underline">
                {paper.link}
              </a>
            ) : (
              <span className="text-mk-faint">—</span>
            )}
          </dd>
        </div>
      </dl>

      <div className="mt-3">
        <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">摘要</p>
        {paper.abstract ? (
          <p className="mt-1 max-h-56 overflow-y-auto whitespace-pre-line text-[12px] leading-relaxed text-mk-muted">{paper.abstract}</p>
        ) : (
          <p className="mt-1 text-[12px] text-mk-faint">这篇还没有摘要。</p>
        )}
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        {paper.link && (
          <a href={paper.link} target="_blank" rel="noopener noreferrer" className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50">
            打开原文
          </a>
        )}
        {primaryAction && (
          <button
            type="button"
            onClick={primaryAction.onClick}
            disabled={primaryAction.busy || primaryAction.disabled}
            className="rounded-mk bg-mk-accent px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
          >
            {primaryAction.busy ? "处理中…" : primaryAction.label}
          </button>
        )}
        {secondaryAction && (
          <button
            type="button"
            onClick={secondaryAction.onClick}
            disabled={secondaryAction.busy || secondaryAction.disabled}
            className="rounded-mk border border-mk-border px-3 py-1.5 text-[12px] font-bold text-mk-faint hover:text-mk-accent disabled:opacity-60"
          >
            {secondaryAction.label}
          </button>
        )}
      </div>

      {/* Related/cited — search MORE papers from this one (feeds back into the
          results list → single-paper loop). Available whether or not it's on the
          map yet: a not-yet-added candidate searches off its own DOI. */}
      {onFind && (
        <div className="mt-3 border-t border-mk-border pt-3">
          <p className="text-[12px] font-bold text-mk-faint">从这篇继续找</p>
          <div className="mt-1.5 flex flex-wrap gap-2">
            <FindButton label="找相似" onClick={() => onFind("similar")} disabled={finding} />
            <FindButton label="找它引用的" onClick={() => onFind("citation")} disabled={finding || !paper.doi} />
            <FindButton label="找引用它的" onClick={() => onFind("cited")} disabled={finding || !paper.doi} />
          </div>
        </div>
      )}

      {children}
    </>
  );

  if (flush) return inner;
  return <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">{inner}</div>;
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <dt className="w-9 flex-none font-bold text-mk-faint">{label}</dt>
      <dd className={`min-w-0 flex-1 ${value ? "text-mk-ink" : "text-mk-faint"}`}>{value || "—"}</dd>
    </div>
  );
}

function FindButton({ label, onClick, disabled }: { label: string; onClick: () => void; disabled?: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50 disabled:opacity-40"
    >
      {label}
    </button>
  );
}
