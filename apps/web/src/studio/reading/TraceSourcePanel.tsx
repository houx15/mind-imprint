import { useState } from "react";
import type { DigCandidate } from "@mind-imprint/contracts";

// TraceSourcePanel — 追来源. The agent-supported "trace a datum to its origin"
// flow, opened from inside the reading room where the student actually notices a
// suspicious number or claim. Two ways upstream, reusing the exploration dig +
// adopt machinery:
//   · 看这篇引用了哪些文献 — the works THIS paper cites (dig "citation" via the
//     source's DOI). Empty when the source has no resolvable DOI (a pasted
//     body), which is exactly when the search path below matters.
//   · 按这条数据搜原始出处 — an OpenAlex keyword search seeded from the passage
//     she just referenced, so even a DOI-less source can be traced by the claim
//     itself.
// Each candidate can be 采纳'd into her own 来源 list (adoptCandidate) so she
// jumps from "paper X claims this, citing [12]" to actually reading [12] — one
// hop at a time, until she reaches a source she can check directly. It never
// decides for her (铁律①): it only surfaces upstream candidates and lets her
// choose which to pull in and read.
export type TraceSourcePanelProps = {
  // The referenced passage that prompted the trace — seeds the search box so she
  // isn't retyping the claim. "" when she opened the panel without a reference.
  seedQuote: string;
  // The source's URL/DOI, if any — drives the citation path. undefined/"" hides
  // nothing but yields an empty citation list (→ the guidance to use search).
  sourceUrl?: string;
  // Reuse of exploration's dig/adopt, threaded from the workspace (which holds
  // the real api client) so the reading room stays testable with a plain fake.
  onTraceCitation: (doi: string) => Promise<DigCandidate[]>;
  onTraceSearch: (keyword: string) => Promise<DigCandidate[]>;
  onAdoptSource: (candidate: DigCandidate) => Promise<void>;
  onClose: () => void;
};

function candKey(c: DigCandidate): string {
  return c.doi || c.url || c.title;
}

export function TraceSourcePanel({
  seedQuote,
  sourceUrl,
  onTraceCitation,
  onTraceSearch,
  onAdoptSource,
  onClose,
}: TraceSourcePanelProps) {
  const [keyword, setKeyword] = useState(seedQuote);
  const [citation, setCitation] = useState<DigCandidate[] | null>(null);
  const [search, setSearch] = useState<DigCandidate[] | null>(null);
  const [loadingCitation, setLoadingCitation] = useState(false);
  const [loadingSearch, setLoadingSearch] = useState(false);
  const [adopted, setAdopted] = useState<Set<string>>(new Set());
  const [adopting, setAdopting] = useState<string | null>(null);

  async function runCitation() {
    if (loadingCitation) return;
    setLoadingCitation(true);
    try {
      setCitation(await onTraceCitation(sourceUrl ?? ""));
    } catch {
      setCitation([]);
    } finally {
      setLoadingCitation(false);
    }
  }

  async function runSearch() {
    const q = keyword.trim();
    if (!q || loadingSearch) return;
    setLoadingSearch(true);
    try {
      setSearch(await onTraceSearch(q));
    } catch {
      setSearch([]);
    } finally {
      setLoadingSearch(false);
    }
  }

  async function adopt(c: DigCandidate) {
    const key = candKey(c);
    if (adopted.has(key) || adopting) return;
    setAdopting(key);
    try {
      await onAdoptSource(c);
      setAdopted((prev) => new Set(prev).add(key));
    } catch {
      /* leave un-adopted so she can retry */
    } finally {
      setAdopting(null);
    }
  }

  const renderCandidate = (c: DigCandidate) => {
    const key = candKey(c);
    const isAdopted = adopted.has(key);
    return (
      <li key={key} className="rounded-mk-sm border border-mk-border bg-mk-surface p-3">
        <div className="font-sans text-[14px] font-bold leading-snug text-mk-ink">{c.title || "（无标题）"}</div>
        <div className="mt-0.5 font-sans text-[12px] text-mk-muted">
          {[c.authors, c.year, c.journal].filter(Boolean).join(" · ")}
        </div>
        {c.abstract && (
          <p className="mt-1.5 line-clamp-3 font-sans text-[12.5px] leading-relaxed text-mk-ink-soft">{c.abstract}</p>
        )}
        <div className="mt-2 flex items-center gap-3">
          <button
            type="button"
            onClick={() => void adopt(c)}
            disabled={isAdopted || adopting === key}
            className={
              isAdopted
                ? "cursor-default rounded-mk-sm border border-mk-matcha bg-transparent px-3 py-1 font-sans text-[12.5px] font-bold text-mk-matcha"
                : "cursor-pointer rounded-mk-sm border-0 bg-mk-accent px-3 py-1 font-sans text-[12.5px] font-bold text-mk-surface disabled:opacity-50"
            }
          >
            {isAdopted ? "已加入我的来源 ✓" : adopting === key ? "加入中…" : "采纳并加入我的来源"}
          </button>
          {c.url && (
            <a
              href={c.url}
              target="_blank"
              rel="noopener noreferrer"
              className="font-sans text-[12px] font-semibold text-mk-accent no-underline hover:underline"
            >
              打开 ↗
            </a>
          )}
        </div>
      </li>
    );
  };

  return (
    <div className="mk-finalize-panel__backdrop" role="presentation" onClick={onClose}>
      <div
        className="mk-finalize-panel"
        role="dialog"
        aria-modal="true"
        aria-label="追来源"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="mk-finalize-panel__head">
          <div>
            <span className="mk-finalize-panel__kicker">追来源</span>
            <h2>顺着数据往上游走</h2>
          </div>
          <button type="button" className="mk-finalize-panel__close" aria-label="关闭" onClick={onClose}>
            ×
          </button>
        </header>

        <div className="mk-finalize-panel__body">
          <p className="font-sans text-[13px] leading-relaxed text-mk-ink-soft">
            一个数字可疑时，别停在这一篇——看它引用了谁，或按这条数据去搜它的原始出处，一层层往上，直到找到你能直接核对的来源。
          </p>

          {/* Path 1 · what this paper cites */}
          <section className="mt-4">
            <h3 className="font-sans text-[13px] font-bold text-mk-ink">① 这篇引用了哪些文献</h3>
            <button
              type="button"
              onClick={() => void runCitation()}
              disabled={loadingCitation}
              className="mt-2 cursor-pointer rounded-mk-sm border border-mk-accent bg-transparent px-3 py-1.5 font-sans text-[12.5px] font-bold text-mk-accent disabled:opacity-50"
            >
              {loadingCitation ? "查找中…" : "看这篇引用的文献"}
            </button>
            {citation !== null &&
              (citation.length > 0 ? (
                <ul className="mt-3 flex flex-col gap-2">{citation.map(renderCandidate)}</ul>
              ) : (
                <p className="mt-2 font-sans text-[12.5px] text-mk-muted">
                  这条来源没有可解析的 DOI，取不到它的引用列表——用下面的搜索，按这条数据本身找原始出处。
                </p>
              ))}
          </section>

          {/* Path 2 · search for the datum's origin */}
          <section className="mt-5">
            <h3 className="font-sans text-[13px] font-bold text-mk-ink">② 按这条数据搜原始出处</h3>
            <div className="mt-2 flex gap-2">
              <input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void runSearch();
                }}
                placeholder="把可疑的那条数据或说法填这里……"
                className="min-w-0 flex-1 rounded-mk-sm border border-mk-border bg-mk-surface px-3 py-1.5 font-sans text-[13px] text-mk-ink outline-none"
              />
              <button
                type="button"
                onClick={() => void runSearch()}
                disabled={loadingSearch || !keyword.trim()}
                className="flex-none cursor-pointer rounded-mk-sm border-0 bg-mk-accent px-3 py-1.5 font-sans text-[12.5px] font-bold text-mk-surface disabled:opacity-50"
              >
                {loadingSearch ? "搜索中…" : "搜原始出处"}
              </button>
            </div>
            {search !== null &&
              (search.length > 0 ? (
                <ul className="mt-3 flex flex-col gap-2">{search.map(renderCandidate)}</ul>
              ) : (
                <p className="mt-2 font-sans text-[12.5px] text-mk-muted">
                  没搜到匹配的文献——换个更具体的关键词（比如具体的数值、地点或年份）再试。
                </p>
              ))}
          </section>
        </div>

        <footer className="mk-finalize-panel__footer">
          <span className="font-sans text-[12px] text-mk-muted">
            采纳的来源会进入「我的来源」，可在那里逐篇打开续读、再体检可信度。
          </span>
          <button type="button" className="mk-finalize-panel__confirm" onClick={onClose}>
            完成
          </button>
        </footer>
      </div>
    </div>
  );
}
