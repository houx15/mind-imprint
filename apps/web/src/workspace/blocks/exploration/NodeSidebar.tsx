import { useEffect, useState } from "react";
import type { DigCandidate, ExplorationLead, Reference } from "@mind-imprint/contracts";

// GVb · the Level-2 right sidebar (ResearchRabbit-style detail panel). It shows
// the SELECTED mindmap node's metadata, the two search actions, and the dig
// results — replacing the old bottom DigTray. Dig results live here as
// suggestions; a candidate only joins the mindmap when the student taps 采纳
// (铁律①). Plain copy only — no rabbit metaphor in any rendered string.

export function candidateKey(c: DigCandidate): string {
  return c.url || c.doi || c.title;
}

const READING_STATUS_LABEL: Record<string, string> = {
  to_read: "待读",
  reading: "在读",
  done: "读完",
};

export type NodeSidebarProps = {
  // The selected node's lead (null before any node is picked), plus its joined
  // reference when it's a paper node (connectedReferenceId → references list).
  node: ExplorationLead | null;
  reference: Reference | null;
  // Dig state (the dig launched FROM this node — see ExplorationView).
  digging: boolean;
  digError: boolean;
  candidates: DigCandidate[];
  adopting: Set<string>;
  onFindSimilar: () => void;
  onKeywordSearch: (keyword: string) => void;
  onAdopt: (c: DigCandidate) => void;
  onDiscard: (c: DigCandidate) => void;
  // A paper node can jump into the reading room (reuses ReadingBlock's slot).
  onEnterReading?: () => void;
  entering?: boolean;
};

export function NodeSidebar({
  node,
  reference,
  digging,
  digError,
  candidates,
  adopting,
  onFindSimilar,
  onKeywordSearch,
  onAdopt,
  onDiscard,
  onEnterReading,
  entering,
}: NodeSidebarProps) {
  const [keyword, setKeyword] = useState("");
  // Reset the keyword box when the selection changes (each node's search is its
  // own).
  useEffect(() => {
    setKeyword("");
  }, [node?.id]);

  if (!node) {
    return (
      <aside className="flex w-80 flex-none flex-col border-l border-mk-border bg-mk-surface">
        <div className="flex flex-1 items-center justify-center px-6 text-center text-[12.5px] leading-relaxed text-mk-muted-2">
          点图上的一个节点，这里会显示它的信息，还能顺着它找相关论文。
        </div>
      </aside>
    );
  }

  const isPaper = Boolean(node.connectedReferenceId);
  const title = reference?.title ?? node.text;
  const meta = reference
    ? [reference.author, reference.year, reference.journal].map((s) => s?.trim()).filter(Boolean).join(" · ")
    : "";
  const link = reference?.url?.trim() || "";
  const statusLabel = reference ? READING_STATUS_LABEL[reference.readingStatus] : undefined;

  function submitKeyword() {
    const kw = keyword.trim();
    if (!kw) return;
    onKeywordSearch(kw);
  }

  return (
    <aside className="flex w-80 flex-none flex-col border-l border-mk-border bg-mk-surface">
      {/* ---- selected node metadata ---- */}
      <div className="flex-none border-b border-mk-border px-4 py-3">
        <div className="flex items-center gap-2">
          <span
            className={`rounded-full px-2 py-0.5 text-[10px] font-bold ${
              isPaper ? "bg-mk-green-tint text-mk-green" : "bg-mk-primary-tint text-mk-primary"
            }`}
          >
            {isPaper ? "论文" : "问题"}
          </span>
          {statusLabel && (
            <span className="rounded-full bg-mk-bg px-2 py-0.5 text-[10px] font-bold text-mk-muted-2">{statusLabel}</span>
          )}
        </div>
        <h3 className="mt-2 text-[13.5px] font-bold leading-snug text-mk-ink">{title}</h3>
        {meta && <p className="mt-1 text-[11.5px] text-mk-muted-2">{meta}</p>}
        {reference?.abstract?.trim() && (
          <p className="mt-2 max-h-40 overflow-y-auto text-[12px] leading-relaxed text-mk-muted">{reference.abstract}</p>
        )}
        {isPaper && (
          <div className="mt-2.5 flex flex-wrap items-center gap-2">
            {link && (
              <a
                href={link}
                target="_blank"
                rel="noopener noreferrer"
                className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] font-bold text-mk-primary hover:border-mk-primary hover:bg-mk-primary/10"
              >
                打开原文
              </a>
            )}
            {onEnterReading && (
              <button
                type="button"
                onClick={onEnterReading}
                disabled={entering}
                className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] font-bold text-mk-primary hover:border-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
              >
                {entering ? "打开中…" : "进入阅读室"}
              </button>
            )}
          </div>
        )}
      </div>

      {/* ---- search actions ---- */}
      <div className="flex-none border-b border-mk-border px-4 py-3">
        <button
          type="button"
          onClick={onFindSimilar}
          disabled={digging}
          className="w-full rounded-mk bg-mk-primary px-3 py-1.5 text-[12.5px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
        >
          {digging ? "印记在找…" : "找相似文献"}
        </button>
        <div className="mt-2 flex items-center gap-1.5">
          <input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") submitKeyword();
            }}
            placeholder="换个词找…"
            className="min-w-0 flex-1 rounded-mk border border-mk-border bg-mk-bg/60 px-2.5 py-1.5 text-[12px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
          <button
            type="button"
            onClick={submitKeyword}
            disabled={digging || !keyword.trim()}
            className="flex-none rounded-mk border border-mk-primary/40 bg-mk-surface px-2.5 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
          >
            找
          </button>
        </div>
      </div>

      {/* ---- dig results (suggestions; 采纳 to land one on the map) ---- */}
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {digging ? (
          <p className="py-2 text-[12.5px] text-mk-muted-2">印记在找相关论文……</p>
        ) : digError ? (
          <p className="py-2 text-[12.5px] font-semibold text-mk-accent">刚才没接上，再试一次？</p>
        ) : candidates.length === 0 ? (
          <p className="py-2 text-[12.5px] leading-relaxed text-mk-muted-2">
            点「找相似文献」，印记会顺着这个节点给你几篇相关论文。挑有用的采纳，其余丢弃就好。
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-[11.5px] font-bold text-mk-muted-2">挖到这些论文</p>
            {candidates.map((c) => {
              const key = candidateKey(c);
              const cMeta = [c.authors, c.year, c.journal].map((s) => s?.trim()).filter(Boolean).join(" · ");
              const busy = adopting.has(key);
              return (
                <div key={key} className="rounded-mk border border-mk-border bg-mk-bg/50 p-2.5">
                  <p className="text-[12.5px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                  {cMeta && <p className="mt-0.5 text-[11px] text-mk-muted-2">{cMeta}</p>}
                  {c.abstract?.trim() && (
                    <p className="mt-1 max-h-20 overflow-y-auto text-[11.5px] leading-relaxed text-mk-muted">{c.abstract}</p>
                  )}
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => onAdopt(c)}
                      disabled={busy}
                      className="rounded-mk bg-mk-primary px-2.5 py-1 text-[11.5px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
                    >
                      {busy ? "采纳中…" : "采纳"}
                    </button>
                    <button
                      type="button"
                      onClick={() => onDiscard(c)}
                      disabled={busy}
                      className="rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-50"
                    >
                      丢弃
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </aside>
  );
}
