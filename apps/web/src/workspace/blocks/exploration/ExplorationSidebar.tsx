import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import type { DigCandidate, ExplorationLead, LeadOrigin, Reference } from "@mind-imprint/contracts";
import { RabbitHoleLoader } from "@/ui";
import { PaperDetail, referenceToPaperView } from "./PaperDetail";

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

  // 'ai' → the coach slot IS the sidebar (its own fixed-width aside). The coach
  // now lives in the constant 印记 rail, not here — with no slot passed in,
  // collapse entirely so the graph canvas gets the full width (no dead column).
  if (state === "ai") {
    return coach ? <>{coach}</> : null;
  }

  // Guard: node/results always ride a selected node; if it vanished, collapse
  // back to nothing (same as the 'ai' default) rather than a blank panel.
  if (!node) {
    return coach ? <>{coach}</> : null;
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
      {/* The back bar stays pinned; everything below it scrolls as ONE column —
          so a tall paper card / find-actions never overflow the fixed-height
          panel and read as "stuck" (the old bug: flex-none sections clipped
          because the panel itself never scrolled). */}
      <BackBar label="← 印记" onClick={onBackToAi} />

      <div className="min-h-0 flex-1 overflow-y-auto">
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
        <div className="border-b border-mk-border px-4 py-3">
          {isPaper ? (
            <div data-tour="explore-find" className="flex flex-col gap-2">
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
              <div data-tour="explore-keyword" className="mt-2 flex items-center gap-1.5">
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

        <div className="px-4 py-3 text-[14px] leading-relaxed text-mk-faint">
          {isPaper
            ? "顺着这篇论文找——相似的、它引用的、引用它的。挑有用的采纳，会挂到这条线下面。"
            : "点上面的「找相似文献」，或换个关键词，印记会顺着这个问题给你几篇论文。"}
        </div>
      </div>
    </aside>
  );
}

// EvidenceNote — a paper's 证据笔记 facets: nature (支持/反驳) + a structured
// note (key argument / key evidence / where it can appear) + triage
// (must-read/to-decide) + archive. It now lives in the READING ROOM (rendered
// beside 我的笔记, where the student actually reads the source), not the warren
// map sidebar. The caller owns the outer container; this renders just the
// heading + controls.
export function EvidenceNote({
  reference,
  onSetEvidence,
  onSetTriage,
  onArchive,
  hideHeading,
}: {
  reference: Reference;
  onSetEvidence: (ev: { nature: "" | "support" | "challenge"; argument: string; finding: string; placement: string }) => void;
  onSetTriage: (triage: "" | "red" | "yellow") => void;
  onArchive: () => void;
  // The reading room provides its own collapsible "证据笔记" header, so it hides
  // this component's built-in one to avoid a duplicate title.
  hideHeading?: boolean;
}) {
  const [argument, setArgument] = useState(reference.evidenceArgument ?? "");
  const [finding, setFinding] = useState(reference.evidenceFinding ?? "");
  const [placement, setPlacement] = useState(reference.evidencePlacement ?? "");
  const nature = (reference.evidenceNature ?? "") as "" | "support" | "challenge";
  const triage = (reference.triage ?? "") as "" | "red" | "yellow";
  useEffect(() => {
    setArgument(reference.evidenceArgument ?? "");
    setFinding(reference.evidenceFinding ?? "");
    setPlacement(reference.evidencePlacement ?? "");
  }, [reference.id, reference.evidenceArgument, reference.evidenceFinding, reference.evidencePlacement]);

  function saveNote(next: Partial<{ nature: "" | "support" | "challenge"; argument: string; finding: string; placement: string }>) {
    onSetEvidence({ nature, argument, finding, placement, ...next });
  }

  return (
    <div>
      {!hideHeading && <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">证据笔记</p>}

      {/* triage — 必读 / 待定 */}
      <div className="mt-2 flex items-center gap-1.5">
        <span className="text-[12px] font-bold text-mk-faint">优先级</span>
        <TriageDot label="必读" active={triage === "red"} color="bg-mk-danger" onClick={() => onSetTriage(triage === "red" ? "" : "red")} />
        <TriageDot label="待定" active={triage === "yellow"} color="bg-mk-warning" onClick={() => onSetTriage(triage === "yellow" ? "" : "yellow")} />
      </div>

      {/* nature — 支持 / 反驳 */}
      <div className="mt-2.5 flex items-center gap-1.5">
        <span className="text-[12px] font-bold text-mk-faint">对这个子问题</span>
        <NatureChip label="支持" active={nature === "support"} tone="text-mk-success border-mk-success" onClick={() => saveNote({ nature: nature === "support" ? "" : "support" })} />
        <NatureChip label="反驳/张力" active={nature === "challenge"} tone="text-mk-danger border-mk-danger" onClick={() => saveNote({ nature: nature === "challenge" ? "" : "challenge" })} />
      </div>

      <NoteField label="关键论点" value={argument} onChange={setArgument} onBlur={() => saveNote({ argument })} placeholder="这篇在说什么？" />
      <NoteField label="关键证据" value={finding} onChange={setFinding} onBlur={() => saveNote({ finding })} placeholder="它拿出的数据/发现？" />
      <NoteField label="可以用在" value={placement} onChange={setPlacement} onBlur={() => saveNote({ placement })} placeholder="论文里哪个部分？" />

      <button
        type="button"
        onClick={onArchive}
        className="mt-2.5 text-[12px] font-semibold text-mk-faint underline decoration-dotted hover:text-mk-accent"
      >
        {reference.archived ? "已归档 · 移回" : "归档（有点意思，但不太相关）"}
      </button>
    </div>
  );
}

function TriageDot({ label, active, color, onClick }: { label: string; active: boolean; color: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center gap-1 rounded-full border px-2 py-0.5 text-[12px] font-bold ${active ? "border-mk-accent bg-mk-accent-50 text-mk-accent" : "border-mk-border text-mk-faint"}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${color}`} />
      {label}
    </button>
  );
}

function NatureChip({ label, active, tone, onClick }: { label: string; active: boolean; tone: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full border px-2 py-0.5 text-[12px] font-bold ${active ? tone : "border-mk-border text-mk-faint"}`}
    >
      {label}
    </button>
  );
}

function NoteField({ label, value, onChange, onBlur, placeholder }: { label: string; value: string; onChange: (v: string) => void; onBlur: () => void; placeholder: string }) {
  return (
    <div className="mt-2.5">
      <p className="text-[12px] font-bold text-mk-faint">{label}</p>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        rows={2}
        placeholder={placeholder}
        className="mt-1 w-full resize-none rounded-mk border border-mk-border bg-mk-paper px-2 py-1.5 text-[12px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
      />
    </div>
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
  // Pasting the body is an important, deliberate action — so it opens in a
  // MODAL (a roomy textarea), not a cramped inline box that used to overflow
  // the sidebar. Closed whenever a different paper is selected or the prompt
  // clears.
  const [pasteOpen, setPasteOpen] = useState(false);
  useEffect(() => {
    setPasteOpen(false);
  }, [node.id, Boolean(pastePrompt)]);

  // One shared layout with the search-result single-paper view (PaperDetail):
  // an added node is always "已在图谱". A node with no joined reference (rare)
  // still renders from its own text.
  const paper = reference
    ? referenceToPaperView(reference)
    : { title: node.text, authors: "", year: "", journal: "", abstract: "", link: "", doi: "", added: true };

  return (
    <div className="flex-none border-b border-mk-border px-4 py-3">
      <PaperDetail
        paper={paper}
        flush
        primaryAction={
          onEnterReading && !pastePrompt
            ? {
                label: entering ? "打开中…" : "进入阅读室",
                onClick: onEnterReading,
                busy: entering,
                // Guided tour · the "enter reading room" affordance on an
                // adopted node/paper's own card (the ONLY per-node
                // enter-reading control in this view — the map/library route
                // via a different surface).
                dataTour: "explore-enter-reading",
              }
            : undefined
        }
      >
        {/* 422 fallback: the full text couldn't be fetched. Instead of a dead
            click, offer a prominent button that opens the paste MODAL →
            pasteContent → straight into the reading room. */}
        {pastePrompt && onPaste && (
          <button
            type="button"
            onClick={() => setPasteOpen(true)}
            className="mt-3 w-full rounded-mk border border-mk-accent-200 bg-mk-accent-50 px-3 py-2 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-100"
          >
            粘贴正文，开始共读
          </button>
        )}
      </PaperDetail>

      {pasteOpen && pastePrompt && onPaste && (
        <PasteReadingModal
          msg={pastePrompt.msg}
          busy={pasteBusy}
          onSubmit={(text) => onPaste(text)}
          onClose={() => setPasteOpen(false)}
        />
      )}
    </div>
  );
}

// PasteReadingModal — the deliberate "paste the article body" action, in a
// roomy modal (the old inline sidebar box overflowed and read as "stuck").
// Clicking the backdrop closes it; the card stops propagation.
function PasteReadingModal({
  msg,
  busy,
  onSubmit,
  onClose,
}: {
  msg: string;
  busy?: boolean;
  onSubmit: (text: string) => void;
  onClose: () => void;
}) {
  const [text, setText] = useState("");
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="flex w-full max-w-xl flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-mk-lg"
      >
        <h3 className="text-mk-h3 text-mk-ink">粘贴正文，开始共读</h3>
        <p className="mt-1.5 text-mk-small leading-relaxed text-mk-muted">{msg}</p>
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={12}
          autoFocus
          placeholder="把文章正文粘到这里，直接进阅读室和印记逐句共读……"
          className="mt-3 w-full resize-y rounded-mk border border-mk-border bg-mk-paper px-3 py-2.5 text-[13px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
        />
        <div className="mt-4 flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-mk border border-mk-input-border bg-mk-surface px-4 py-2 text-[13px] font-semibold text-mk-secondary hover:bg-mk-paper"
          >
            取消
          </button>
          <button
            type="button"
            onClick={() => onSubmit(text.trim())}
            disabled={busy || !text.trim()}
            className="rounded-mk bg-mk-accent px-5 py-2 text-[13px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
          >
            {busy ? "开始中…" : "开始共读"}
          </button>
        </div>
      </div>
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
      <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">问题</span>
      <h3 className="mt-2 text-[14px] font-bold leading-snug text-mk-ink">{node.text}</h3>

      <dl className="mt-2.5 space-y-1 text-[12px]">
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
        <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">论文列表</p>
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
    <aside data-tour="explore-suggestions" className={SHELL}>
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
          // Task 7 (review fix round 1) · this dig is the "reading-list search"
          // HIDDEN subagent — but it was ALREADY a non-chat loading scene
          // (RabbitHoleLoader), which already satisfies "a loading status,
          // never a chat". Keep the richer animation; just frame the caption
          // as a subagent doing the searching.
          <div className="flex justify-center py-4">
            <RabbitHoleLoader caption="subagent 正在检索来源……" />
          </div>
        ) : digError ? (
          <p className="py-2 text-[14px] font-semibold text-mk-accent">刚才没接上，再试一次？</p>
        ) : candidates.length === 0 ? (
          <p className="py-2 text-[14px] leading-relaxed text-mk-faint">
            没有更多论文了。挑有用的采纳了、其余丢弃就好，或者换个词再找。
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-[12px] font-bold text-mk-faint">挖到这些论文</p>
            {candidates.map((c) => {
              const key = candidateKey(c);
              const cMeta = [c.authors, c.year, c.journal].map((s) => s?.trim()).filter(Boolean).join(" · ");
              const busy = adopting.has(key);
              return (
                <div key={key} className="rounded-mk border border-mk-border bg-mk-paper p-2.5">
                  <p className="text-[14px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                  {cMeta && <p className="mt-0.5 text-[12px] text-mk-faint">{cMeta}</p>}
                  {c.abstract?.trim() && (
                    <p className="mt-1 max-h-20 overflow-y-auto text-[12px] leading-relaxed text-mk-muted">
                      {c.abstract}
                    </p>
                  )}
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => onAdopt(c)}
                      disabled={busy}
                      className="rounded-mk bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
                    >
                      {busy ? "采纳中…" : "采纳"}
                    </button>
                    <button
                      type="button"
                      onClick={() => onDiscard(c)}
                      disabled={busy}
                      className="rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-50"
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
      className="w-full rounded-mk bg-mk-accent px-3 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
    >
      {children}
    </button>
  );
}
