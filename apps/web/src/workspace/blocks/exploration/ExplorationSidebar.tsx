import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import type { DigCandidate, ExplorationLead, LeadOrigin, Reference } from "@mind-imprint/contracts";
import { RabbitHoleLoader } from "@/ui";

// GVd · ONE stateful right sidebar for the 探索 view — the ONLY right panel here
// (the separately-docked 印记·找资料 coach is gone; it now IS this sidebar's
// default 'ai' state). Three states:
//   'ai'      (DEFAULT) — the coach, passed in as a slot. Shown at Level-1 always
//             and at Level-2 until a node is clicked.
//   'node'    — a selected node's metadata + find-actions, with a "← 印记" back.
//   'results' — the dig candidates (suggestions), 采纳/丢弃, with backs.
// Dig results are suggestions; a candidate only joins the map when the student
// taps 采纳 (铁律①). Plain copy only — no rabbit metaphor in any rendered string.

export function candidateKey(c: DigCandidate): string {
  return c.url || c.doi || c.title;
}

export type DigMode = "similar" | "citation" | "cited";

// A question node's descendant paper, projected for the 论文列表 (clickable →
// selects that paper node).
export type PaperInList = { id: string; title: string };

const READING_STATUS_LABEL: Record<string, string> = {
  to_read: "待读",
  reading: "在读",
  done: "读完",
};

// 来源 in human-friendly words — where this question came from (铁律④: provenance
// is data, shown plainly, never jargon).
const ORIGIN_LABEL: Record<LeadOrigin, string> = {
  takeaway: "从阅读归纳而来",
  manual: "手动记下",
  guide: "印记引导",
  note: "从阅读笔记提出",
};

// createdAt is RFC3339 (e.g. 2026-08-01T00:00:00Z) → a plain YYYY-MM-DD 记于 date.
function formatCreated(iso: string): string {
  const m = /^(\d{4}-\d{2}-\d{2})/.exec(iso);
  return m ? m[1]! : iso;
}

export type ExplorationSidebarProps = {
  state: "ai" | "node" | "results";
  // 'ai' slot — the 印记·找资料 coach, owned by the parent (ReadingBlock).
  coach: ReactNode;
  // The selected node (node/results states), plus its joined reference when it's
  // a paper node (connectedReferenceId → references list).
  node: ExplorationLead | null;
  reference: Reference | null;
  // A question node's descendant papers, clickable to select that paper.
  papers: PaperInList[];
  onSelectPaper: (leadId: string) => void;
  // Dig state (the dig launched FROM this node — see ExplorationView).
  digging: boolean;
  digError: boolean;
  candidates: DigCandidate[];
  adopting: Set<string>;
  // A node's find-actions. onDig(mode) digs by the selected node's id; the
  // keyword box digs by a fresh term (always 'similar').
  onDig: (mode: DigMode) => void;
  onKeywordSearch: (keyword: string) => void;
  onAdopt: (c: DigCandidate) => void;
  onDiscard: (c: DigCandidate) => void;
  // "← 印记" (both node + results) returns to the coach; "← 返回" (results only)
  // goes back to the node whose search produced these results.
  onBackToAi: () => void;
  onBackToNode: () => void;
  // A paper node can jump into the reading room (reuses ReadingBlock's slot).
  onEnterReading?: () => void;
  entering?: boolean;
  // When enter-reading 422s (full text unfetchable), the paper shows an inline
  // paste box instead of dead-clicking. `pastePrompt` carries the friendly
  // message + any recovered DOI metadata; `onPaste` submits the pasted body.
  pastePrompt?: { msg: string; meta?: PasteMeta };
  pasteBusy?: boolean;
  onPaste?: (text: string) => void;
};

export type PasteMeta = { title?: string; author?: string; year?: string; journal?: string; abstract?: string };

const SHELL = "flex w-[340px] flex-none flex-col border-l border-mk-border bg-mk-surface";

export function ExplorationSidebar(props: ExplorationSidebarProps) {
  const { state, coach, node } = props;

  // 'ai' → the coach slot IS the sidebar (its own fixed-width aside). A missing
  // slot (never in the app, only bare unit renders) degrades to an empty column.
  if (state === "ai") {
    return coach ? <>{coach}</> : <aside className={SHELL} />;
  }

  // Guard: node/results always ride a selected node; if it vanished, fall back
  // to the coach rather than a blank panel.
  if (!node) {
    return coach ? <>{coach}</> : <aside className={SHELL} />;
  }

  if (state === "results") {
    return <ResultsPanel {...props} node={node} />;
  }
  return <NodePanel {...props} node={node} />;
}

/* ---------- 'node' · a selected node's metadata + find-actions ---------- */

function NodePanel(props: ExplorationSidebarProps & { node: ExplorationLead }) {
  const {
    node,
    reference,
    papers,
    onSelectPaper,
    digging,
    onDig,
    onKeywordSearch,
    onBackToAi,
    onEnterReading,
    entering,
    pastePrompt,
    pasteBusy,
    onPaste,
  } = props;
  const [keyword, setKeyword] = useState("");
  useEffect(() => {
    setKeyword("");
  }, [node.id]);

  const isPaper = Boolean(node.connectedReferenceId);

  function submitKeyword() {
    const kw = keyword.trim();
    if (!kw) return;
    onKeywordSearch(kw);
  }

  return (
    <aside className={SHELL}>
      <BackBar label="← 印记" onClick={onBackToAi} />

      {isPaper ? (
        <PaperMeta
          reference={reference}
          node={node}
          onEnterReading={onEnterReading}
          entering={entering}
          pastePrompt={pastePrompt}
          pasteBusy={pasteBusy}
          onPaste={onPaste}
        />
      ) : (
        <QuestionMeta node={node} papers={papers} onSelectPaper={onSelectPaper} />
      )}

      {/* ---- find-actions ---- */}
      <div className="flex-none border-b border-mk-border px-4 py-3">
        {isPaper ? (
          <div className="flex flex-col gap-2">
            <FindButton disabled={digging} onClick={() => onDig("similar")}>
              {digging ? "印记在找…" : "找相似文献"}
            </FindButton>
            <FindButton disabled={digging} onClick={() => onDig("citation")}>
              找它引用的文献
            </FindButton>
            <FindButton disabled={digging} onClick={() => onDig("cited")}>
              找引用它的文献
            </FindButton>
          </div>
        ) : (
          <>
            <FindButton disabled={digging} onClick={() => onDig("similar")}>
              {digging ? "印记在找…" : "找相似文献"}
            </FindButton>
            <div className="mt-2 flex items-center gap-1.5">
              <input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") submitKeyword();
                }}
                placeholder="换个词找…"
                className="min-w-0 flex-1 rounded-mk border border-mk-border bg-mk-paper px-2.5 py-1.5 text-[12px] text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
              />
              <button
                type="button"
                onClick={submitKeyword}
                disabled={digging || !keyword.trim()}
                className="flex-none rounded-mk border border-mk-accent/40 bg-mk-surface px-2.5 py-1.5 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
              >
                找
              </button>
            </div>
          </>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3 text-[12.5px] leading-relaxed text-mk-faint">
        {isPaper
          ? "顺着这篇论文找——相似的、它引用的、引用它的。挑有用的采纳，会挂到这条线下面。"
          : "点上面的「找相似文献」，或换个关键词，印记会顺着这个问题给你几篇论文。"}
      </div>
    </aside>
  );
}

function PaperMeta({
  reference,
  node,
  onEnterReading,
  entering,
  pastePrompt,
  pasteBusy,
  onPaste,
}: {
  reference: Reference | null;
  node: ExplorationLead;
  onEnterReading?: () => void;
  entering?: boolean;
  pastePrompt?: { msg: string; meta?: PasteMeta };
  pasteBusy?: boolean;
  onPaste?: (text: string) => void;
}) {
  const [pasteText, setPasteText] = useState("");
  useEffect(() => {
    // Reset the draft whenever a different paper is selected or the prompt clears.
    setPasteText("");
  }, [node.id, Boolean(pastePrompt)]);
  const title = reference?.title ?? node.text;
  const authors = reference?.author?.trim() || "";
  const year = reference?.year?.trim() || "";
  const journal = reference?.journal?.trim() || "";
  const statusLabel = reference ? READING_STATUS_LABEL[reference.readingStatus] : undefined;
  const link = reference?.url?.trim() || "";
  const abstract = reference?.abstract?.trim() || "";

  return (
    <div className="flex-none border-b border-mk-border px-4 py-3">
      <div className="flex items-center gap-2">
        <span className="rounded-full bg-mk-success-bg px-2 py-0.5 text-[10px] font-bold text-mk-success">论文</span>
        {statusLabel && (
          <span className="rounded-full bg-mk-paper px-2 py-0.5 text-[10px] font-bold text-mk-faint">{statusLabel}</span>
        )}
      </div>
      <h3 className="mt-2 text-[14.5px] font-bold leading-snug text-mk-ink">{title}</h3>

      {/* Labeled metadata — each field on its own row with a clear label, not one
          run-on line of grey text. Empty fields still show their label + 「—」so
          the structure reads clearly. */}
      <dl className="mt-3 space-y-1.5 text-[12px]">
        <MetaRow label="作者" value={authors} />
        <MetaRow label="年份" value={year} />
        <MetaRow label="期刊" value={journal} />
        <div className="flex gap-2">
          <dt className="w-9 flex-none font-bold text-mk-faint">链接</dt>
          <dd className="min-w-0 flex-1">
            {link ? (
              <a
                href={link}
                target="_blank"
                rel="noopener noreferrer"
                className="break-all font-semibold text-mk-accent underline-offset-2 hover:underline"
              >
                {link}
              </a>
            ) : (
              <span className="text-mk-faint">—</span>
            )}
          </dd>
        </div>
      </dl>

      {/* 摘要 — labeled block, always present so its absence is explicit. */}
      <div className="mt-3">
        <p className="text-[11px] font-bold uppercase tracking-wider text-mk-faint">摘要</p>
        {abstract ? (
          <p className="mt-1 max-h-56 overflow-y-auto whitespace-pre-line text-[12px] leading-relaxed text-mk-muted">
            {abstract}
          </p>
        ) : (
          <p className="mt-1 text-[12px] text-mk-faint">这篇还没有摘要。</p>
        )}
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        {link && (
          <a
            href={link}
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
          >
            打开原文
          </a>
        )}
        {onEnterReading && !pastePrompt && (
          <button
            type="button"
            onClick={onEnterReading}
            disabled={entering}
            className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[11.5px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
          >
            {entering ? "打开中…" : "进入阅读室"}
          </button>
        )}
      </div>

      {/* 422 fallback: the full text couldn't be fetched, so instead of a dead
          click we ask her to paste the body → pasteContent → straight into the
          reading room. Mirrors the Library preview's paste path. */}
      {pastePrompt && onPaste && (
        <div className="mt-3 rounded-mk border border-mk-accent-200 bg-mk-accent-50 p-2.5">
          <p className="text-[12px] font-semibold leading-relaxed text-mk-ink">{pastePrompt.msg}</p>
          <textarea
            value={pasteText}
            onChange={(e) => setPasteText(e.target.value)}
            rows={5}
            placeholder="把文章正文粘到这里，直接进阅读室和印记逐句共读……"
            className="mt-2 w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-2.5 py-2 text-[12px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
          />
          <button
            type="button"
            onClick={() => onPaste(pasteText.trim())}
            disabled={pasteBusy || !pasteText.trim()}
            className="mt-2 w-full rounded-mk bg-mk-accent px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
          >
            {pasteBusy ? "开始中…" : "开始共读"}
          </button>
        </div>
      )}
    </div>
  );
}

// A single labeled metadata row (作者/年份/期刊). Empty → 「—」so the label stays
// visible and the panel reads as a clear structure, not a grey run-on line.
function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-2">
      <dt className="w-9 flex-none font-bold text-mk-faint">{label}</dt>
      <dd className={`min-w-0 flex-1 ${value ? "text-mk-ink" : "text-mk-faint"}`}>{value || "—"}</dd>
    </div>
  );
}

function QuestionMeta({
  node,
  papers,
  onSelectPaper,
}: {
  node: ExplorationLead;
  papers: PaperInList[];
  onSelectPaper: (leadId: string) => void;
}) {
  return (
    <div className="flex-none border-b border-mk-border px-4 py-3">
      <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[10px] font-bold text-mk-accent">问题</span>
      <h3 className="mt-2 text-[14px] font-bold leading-snug text-mk-ink">{node.text}</h3>

      <dl className="mt-2.5 space-y-1 text-[11.5px]">
        <div className="flex gap-1.5">
          <dt className="flex-none font-bold text-mk-faint">来源</dt>
          <dd className="text-mk-muted">{ORIGIN_LABEL[node.origin]}</dd>
        </div>
        <div className="flex gap-1.5">
          <dt className="flex-none font-bold text-mk-faint">记于</dt>
          <dd className="text-mk-muted">{formatCreated(node.createdAt)}</dd>
        </div>
      </dl>

      <div className="mt-3">
        <p className="text-[11px] font-bold uppercase tracking-wider text-mk-faint">论文列表</p>
        {papers.length === 0 ? (
          <p className="mt-1 text-[12px] leading-relaxed text-mk-faint">还没有论文挂在这个问题下面，往下找几篇吧。</p>
        ) : (
          <ul className="mt-1.5 flex flex-col gap-1">
            {papers.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  onClick={() => onSelectPaper(p.id)}
                  className="w-full rounded-mk border border-mk-border bg-mk-paper px-2.5 py-1.5 text-left text-[12px] font-semibold leading-snug text-mk-ink hover:border-mk-accent hover:bg-mk-accent-50"
                >
                  {p.title}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

/* ---------- 'results' · dig candidates (suggestions; 采纳 to land one) ---------- */

function ResultsPanel(props: ExplorationSidebarProps & { node: ExplorationLead }) {
  const { digging, digError, candidates, adopting, onAdopt, onDiscard, onBackToAi, onBackToNode } = props;
  return (
    <aside className={SHELL}>
      <div className="flex flex-none items-center gap-2 border-b border-mk-border px-3 py-2">
        <button
          type="button"
          onClick={onBackToAi}
          className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
        >
          ← 印记
        </button>
        <button
          type="button"
          onClick={onBackToNode}
          className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-muted hover:text-mk-accent"
        >
          ← 返回
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {digging ? (
          <div className="flex justify-center py-4">
            <RabbitHoleLoader caption="印记在找相关论文……" />
          </div>
        ) : digError ? (
          <p className="py-2 text-[12.5px] font-semibold text-mk-accent">刚才没接上，再试一次？</p>
        ) : candidates.length === 0 ? (
          <p className="py-2 text-[12.5px] leading-relaxed text-mk-faint">
            没有更多论文了。挑有用的采纳了、其余丢弃就好，或者换个词再找。
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-[11.5px] font-bold text-mk-faint">挖到这些论文</p>
            {candidates.map((c) => {
              const key = candidateKey(c);
              const cMeta = [c.authors, c.year, c.journal].map((s) => s?.trim()).filter(Boolean).join(" · ");
              const busy = adopting.has(key);
              return (
                <div key={key} className="rounded-mk border border-mk-border bg-mk-paper p-2.5">
                  <p className="text-[12.5px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                  {cMeta && <p className="mt-0.5 text-[11px] text-mk-faint">{cMeta}</p>}
                  {c.abstract?.trim() && (
                    <p className="mt-1 max-h-20 overflow-y-auto text-[11.5px] leading-relaxed text-mk-muted">
                      {c.abstract}
                    </p>
                  )}
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => onAdopt(c)}
                      disabled={busy}
                      className="rounded-mk bg-mk-accent px-2.5 py-1 text-[11.5px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
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

/* ---------- small shared bits ---------- */

function BackBar({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <div className="flex flex-none items-center border-b border-mk-border px-3 py-2">
      <button
        type="button"
        onClick={onClick}
        className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
      >
        {label}
      </button>
    </div>
  );
}

function FindButton({
  children,
  disabled,
  onClick,
}: {
  children: ReactNode;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="w-full rounded-mk bg-mk-accent px-3 py-1.5 text-[12.5px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
    >
      {children}
    </button>
  );
}
