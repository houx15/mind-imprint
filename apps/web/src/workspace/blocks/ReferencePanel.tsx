import { useEffect, useState } from "react";
import type { Annotation, DraftAnnotation, Proposal, Reference, ReferenceRef, Snippet, StudioStage } from "@mind-imprint/contracts";
import { EmptyState } from "@/ui/Illustration";
import { getAnnotations, getLibrary, getSnippets } from "../api/workspace";
import { getProposalAnnotations } from "../../api/proposalAnnotations";
import { resolveReferences, type ResolvedRef } from "./referenceResolve";

/**
 * ReferencePanel — the writing stage's left "reference" sub-pane (agentic
 * studio, P3 Task 3). Replaces the static three-tab WritingReferencePanel:
 * instead of always showing everything, it renders what 印记 actually
 * CURATED this turn (`studio_state.reference`, resolved via
 * `resolveReferences` against the student's own library/snippets), grouped
 * by kind:
 *   材料      — curated Reference rows (title + note fragments)
 *   阅读笔记  — curated snippet/reading-note entries (quote → finding)
 *   批注      — curated 整稿体检 review items (criterion·band + combined
 *              missing/fix text); the calm "coming later" line only shows
 *              while nothing has been curated yet — never faked content
 *   提案要点  — the four-dim proposal, but ONLY while the proposal is still
 *              live work (proposal_writing/proposal_review). Once the
 *              student has moved on to 写正文, forcing 提案要点 back into
 *              view isn't useful (spec §5) — it's omitted.
 *
 * 铁律: this panel never WRITES the student's thinking — it only surfaces
 * what 印记 chose to keep in view. `missing` resolved refs (dangling ids,
 * or the always-missing annotation kind) are silently skipped — never
 * rendered as broken entries.
 *
 * GOTCHA (design-system convention): exactly ONE Tailwind class per
 * competing CSS property, never `bg-mk-<token>/<opacity>` (mk tokens are
 * hex → invalid alpha), and body/section text stays ≥14px — 12px is the
 * floor, reserved for meta/captions/fragment previews.
 */

const PROPOSAL_SECTIONS: { key: keyof Proposal; label: string }[] = [
  { key: "objective", label: "研究问题 / 目标" },
  { key: "reason", label: "动机与意义" },
  { key: "activities", label: "活动计划" },
  { key: "resources", label: "资源与文献" },
  { key: "counterpoints", label: "可能的反例 / 张力" },
];

const PROPOSAL_VISIBLE_STAGES: StudioStage[] = ["proposal_writing", "proposal_review"];

type MaterialRef = Extract<ResolvedRef, { kind: "material" }>;
type NoteRef = Extract<ResolvedRef, { kind: "note" }>;
type AnnotationRef = Extract<ResolvedRef, { kind: "annotation" }>;

export function ReferencePanel({
  projectId,
  reference,
  stage,
  proposal,
  onInsert,
  canInsert,
  annotationsVersion,
}: {
  projectId: string;
  reference: ReferenceRef[];
  stage: StudioStage;
  proposal: Proposal;
  /** P3 · insert a fragment into the draft at the caret (the fold of the old
   * floating 材料 box). Wired to the shared draftInsertRef in WorkspaceContainer. */
  onInsert?: (text: string) => void;
  /** Whether the draft is currently open (正文 tab) — the 「插入」 action only
   * shows when true, so it's never a dead no-op on 大纲/片段 (P3 review). */
  canInsert?: boolean;
  /** slice 3b · bumped by the container after a 批注 review so the proposal
   * 批注 group re-fetches. */
  annotationsVersion?: number;
}) {
  const [lib, setLib] = useState<Reference[] | null>(null);
  const [snippets, setSnippets] = useState<Snippet[] | null>(null);
  const [annotations, setAnnotations] = useState<Annotation[] | null>(null);
  // slice 3b · the proposal's layered colored 批注 (view-only). Only on a
  // proposal stage; re-fetched when annotationsVersion changes.
  const isProposalStage = PROPOSAL_VISIBLE_STAGES.includes(stage);
  const [proposalAnnos, setProposalAnnos] = useState<DraftAnnotation[]>([]);
  useEffect(() => {
    if (!isProposalStage) {
      setProposalAnnos([]);
      return;
    }
    let cancelled = false;
    void getProposalAnnotations(projectId)
      .then((a) => { if (!cancelled) setProposalAnnos(a); })
      .catch(() => { if (!cancelled) setProposalAnnos([]); });
    return () => { cancelled = true; };
  }, [projectId, isProposalStage, annotationsVersion]);

  // Lazily fetch library + snippets + annotations once — the panel is
  // context, not the main event, but it needs all three to resolve 印记's
  // curated ids at all, so (unlike the old tab-per-fetch panel) they load
  // together up front.
  useEffect(() => {
    let cancelled = false;
    Promise.all([getLibrary(projectId), getSnippets(projectId), getAnnotations(projectId)])
      .then(([libRes, snips, annos]) => {
        if (cancelled) return;
        setLib(libRes.references);
        setSnippets(snips);
        setAnnotations(annos);
      })
      .catch(() => {
        if (cancelled) return;
        setLib([]);
        setSnippets([]);
        setAnnotations([]);
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const loading = lib == null || snippets == null || annotations == null;
  const resolved = loading ? [] : resolveReferences(reference, lib, snippets, annotations);
  const materials = resolved.filter((r): r is MaterialRef => r.kind === "material" && !r.missing);
  const notes = resolved.filter((r): r is NoteRef => r.kind === "note" && !r.missing);
  const annotationItems = resolved.filter((r): r is AnnotationRef => r.kind === "annotation" && !r.missing);
  const showProposal = PROPOSAL_VISIBLE_STAGES.includes(stage);
  // 你的材料 = everything collected MINUS what 印记 already surfaced in the curated
  // 材料 group, so a source never shows twice (spec §5: the floating 材料 box,
  // folded into the left panel).
  const curatedIds = new Set(materials.map((m) => m.id));
  const curatedNoteIds = new Set(notes.map((n) => n.id));
  const collectedLib = loading ? [] : (lib ?? []).filter((r) => !curatedIds.has(r.id));
  const collectedSnips = loading ? [] : (snippets ?? []).filter((s) => !curatedNoteIds.has(s.id));
  const hasCollected = !loading && (collectedLib.length > 0 || collectedSnips.length > 0);
  const isEmpty =
    !loading &&
    !showProposal &&
    materials.length === 0 &&
    notes.length === 0 &&
    annotationItems.length === 0 &&
    proposalAnnos.length === 0 &&
    !hasCollected;

  return (
    <div className="flex h-full min-h-0 flex-col border-r border-mk-border bg-mk-surface">
      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <p className="text-mk-body text-mk-faint">加载中…</p>
        ) : isEmpty ? (
          <div className="flex h-full items-center justify-center">
            <EmptyState
              illustration="reading"
              title="还没有印记留下的参考"
              body="随着你们一起讨论、阅读，它挑出的材料和笔记会出现在这里。"
            />
          </div>
        ) : (
          <div className="flex flex-col gap-5">
            {showProposal && <ProposalGroup proposal={proposal} />}
            {materials.length > 0 && <MaterialGroup items={materials} />}
            {notes.length > 0 && <NoteGroup items={notes} />}
            {hasCollected && (
              <CollectedSection lib={collectedLib} snippets={collectedSnips} onInsert={onInsert} canInsert={canInsert} />
            )}
            {/* slice 3b · the proposal uses the layered colored 批注 (view-only);
                the essay keeps the flat curated review-item annotations. */}
            {isProposalStage ? <ProposalAnnotationGroup items={proposalAnnos} /> : <AnnotationGroup items={annotationItems} />}
          </div>
        )}
      </div>
    </div>
  );
}

function ProposalGroup({ proposal }: { proposal: Proposal }) {
  // Defensive `?? ""`: a proposal object may predate the counterpoints field
  // (stale payload / test mock) — never crash on a missing section.
  const allEmpty = PROPOSAL_SECTIONS.every((s) => !(proposal[s.key] ?? "").trim());
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">提案要点</p>
      {allEmpty ? (
        <p className="text-[14px] leading-relaxed text-mk-faint">提案要点还没成形。</p>
      ) : (
        <div className="flex flex-col gap-4">
          {PROPOSAL_SECTIONS.map((s) => {
            const value = (proposal[s.key] ?? "").trim();
            return (
              <div key={s.key}>
                <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">{s.label}</p>
                {value ? (
                  <p className="mt-1 whitespace-pre-line text-[14px] leading-relaxed text-mk-ink">{value}</p>
                ) : (
                  <p className="mt-1 text-[14px] text-mk-faint">还没写</p>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function MaterialGroup({ items }: { items: MaterialRef[] }) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">材料</p>
      <div className="flex flex-col gap-3">
        {items.map((item) => (
          <div key={item.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
            <p className="text-[14px] font-semibold leading-snug text-mk-ink">{item.title}</p>
            {item.fragments.length > 0 && (
              <ul className="mt-1.5 flex flex-col gap-1.5">
                {item.fragments.map((f, i) => (
                  <li key={i} className="text-[12px] leading-relaxed text-mk-muted">
                    {f}
                  </li>
                ))}
              </ul>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function NoteGroup({ items }: { items: NoteRef[] }) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">阅读笔记</p>
      <ul className="flex flex-col gap-2">
        {items.map((item) => (
          <li key={item.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5 text-[12px] leading-relaxed text-mk-muted">
            {item.quote.trim() && <span className="text-mk-faint">「{item.quote}」</span>}
            {item.quote.trim() ? " — " : ""}
            {item.finding}
          </li>
        ))}
      </ul>
    </div>
  );
}

function AnnotationGroup({ items }: { items: AnnotationRef[] }) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">批注</p>
      {items.length === 0 ? (
        <p className="text-[14px] leading-relaxed text-mk-faint">批注会在印记体检你的写作后出现。</p>
      ) : (
        <div className="flex flex-col gap-3">
          {items.map((item) => (
            <div key={item.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
              <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">
                {item.criterion}·{item.band}
              </p>
              <p className="mt-1 text-[14px] leading-relaxed text-mk-ink">{item.text}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// slice 3b · nature → solid color token (never mk-*/opacity — the transparent
// gotcha). Coloring the text makes the underline (currentColor) match, so a
// sentence 批注 is colored text + underline; §4's "green/blue/red texts and
// underlines".
const NATURE_TEXT: Record<string, string> = {
  good: "text-mk-success",
  suggest: "text-mk-info",
  problem: "text-mk-danger",
};
const NATURE_DOT: Record<string, string> = {
  good: "bg-mk-success",
  suggest: "bg-mk-info",
  problem: "bg-mk-danger",
};

// ProposalAnnotationGroup — the layered colored 批注 (view-only, §4). paper =
// overall summary (green good / blue-red enhance); paragraph = a blue/red
// comment with a locator, no underline; sentence = the quoted sentence colored
// + underlined, with the comment. It never touches the editable draft.
export function ProposalAnnotationGroup({ items }: { items: DraftAnnotation[] }) {
  const paper = items.filter((a) => a.level === "paper");
  const paragraph = items.filter((a) => a.level === "paragraph");
  const sentence = items.filter((a) => a.level === "sentence");
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">AI批注</p>
      {items.length === 0 ? (
        <p className="text-[14px] leading-relaxed text-mk-faint">批注会在印记看过你的写作后出现。</p>
      ) : (
        <div className="flex flex-col gap-4">
          {paper.length > 0 && (
            <div className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
              <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">总体</p>
              <ul className="mt-1.5 flex flex-col gap-1.5">
                {paper.map((a) => (
                  <li key={a.id} className="flex gap-1.5 text-[14px] leading-relaxed">
                    <span className={`mt-1.5 h-1.5 w-1.5 flex-none rounded-full ${NATURE_DOT[a.nature] ?? "bg-mk-faint"}`} />
                    <span className="text-mk-ink">{a.note}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {paragraph.map((a) => (
            <div key={a.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
              <p className="flex items-center gap-1.5 text-[12px] font-bold text-mk-faint">
                <span className={`h-1.5 w-1.5 rounded-full ${NATURE_DOT[a.nature] ?? "bg-mk-faint"}`} />
                段落{a.locator ? ` · ${a.locator}` : ""}
              </p>
              <p className="mt-1 text-[14px] leading-relaxed text-mk-ink">{a.note}</p>
            </div>
          ))}
          {sentence.map((a) => (
            <div key={a.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
              {a.quote && (
                <p className={`text-[14px] leading-relaxed underline ${NATURE_TEXT[a.nature] ?? "text-mk-ink"}`}>
                  「{a.quote}」{a.locator ? <span className="text-[12px] text-mk-faint no-underline"> · {a.locator}</span> : null}
                </p>
              )}
              <p className="mt-1 text-[14px] leading-relaxed text-mk-ink">{a.note}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** Flatten one collected reference into its insertable fragments (same fields
 * as resolveReferences' material flatten): my note, each quote→finding, and the
 * takeaway's proposal impact. */
function fragmentsOf(ref: Reference): string[] {
  const out: string[] = [];
  if (ref.readingNote?.trim()) out.push(ref.readingNote.trim());
  for (const n of ref.notes ?? []) {
    const q = n.quote?.trim() ? `「${n.quote.trim()}」` : "";
    const f = [q, n.finding?.trim()].filter(Boolean).join(" → ");
    if (f) out.push(f);
  }
  if (ref.takeaway?.proposalImpact?.trim()) out.push(ref.takeaway.proposalImpact.trim());
  return out;
}

/** 你的材料 — the browse of everything the student collected (library materials
 * + saved snippets), folded in from the retired floating 材料 box (spec §5).
 * Read-only reference; each fragment can be inserted into the draft at the caret
 * (`onInsert`) when the 正文 tab is open. */
function CollectedSection({
  lib,
  snippets,
  onInsert,
  canInsert,
}: {
  lib: Reference[];
  snippets: Snippet[];
  onInsert?: (text: string) => void;
  canInsert?: boolean;
}) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">你的材料</p>
      <div className="flex flex-col gap-3">
        {lib.map((ref) => {
          const frags = fragmentsOf(ref);
          return (
            <div key={ref.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
              <p className="text-[14px] font-semibold leading-snug text-mk-ink">{ref.title}</p>
              {frags.length > 0 && (
                <ul className="mt-1.5 flex flex-col gap-2">
                  {frags.map((f, i) => (
                    <li key={i} className="flex items-start gap-2">
                      <p className="min-w-0 flex-1 text-[14px] leading-relaxed text-mk-muted">{f}</p>
                      {onInsert && canInsert && (
                        <button
                          type="button"
                          onClick={() => onInsert(f)}
                          title="插入到正文光标处"
                          className="mt-0.5 flex-none rounded-mk-sm border border-mk-border px-2 py-0.5 text-[14px] font-semibold text-mk-muted transition hover:text-mk-accent"
                        >
                          插入
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          );
        })}
        {snippets.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-[12px] font-bold uppercase tracking-wider text-mk-faint">片段</p>
            {snippets.map((s) => (
              <div key={s.id} className="flex items-start gap-2 rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
                <p className="min-w-0 flex-1 text-[14px] leading-relaxed text-mk-muted">{s.text}</p>
                {onInsert && canInsert && s.text.trim() && (
                  <button
                    type="button"
                    onClick={() => onInsert(s.text)}
                    title="插入到正文光标处"
                    className="mt-0.5 flex-none rounded-mk-sm border border-mk-border px-2 py-0.5 text-[14px] font-semibold text-mk-muted transition hover:text-mk-accent"
                  >
                    插入
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
