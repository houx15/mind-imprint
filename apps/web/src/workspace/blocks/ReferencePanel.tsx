import { useEffect, useState } from "react";
import type { Proposal, Reference, ReferenceRef, Snippet, StudioStage } from "@mind-imprint/contracts";
import { EmptyState } from "@/ui/Illustration";
import { getLibrary, getSnippets } from "../api/workspace";
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
 *   批注      — always a calm "coming later" line; no annotation entity
 *              exists yet (deferred), so this NEVER fakes content
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

export function ReferencePanel({
  projectId,
  reference,
  stage,
  proposal,
}: {
  projectId: string;
  reference: ReferenceRef[];
  stage: StudioStage;
  proposal: Proposal;
}) {
  const [lib, setLib] = useState<Reference[] | null>(null);
  const [snippets, setSnippets] = useState<Snippet[] | null>(null);

  // Lazily fetch library + snippets once — the panel is context, not the
  // main event, but it needs both to resolve 印记's curated ids at all, so
  // (unlike the old tab-per-fetch panel) both load together up front.
  useEffect(() => {
    let cancelled = false;
    Promise.all([getLibrary(projectId), getSnippets(projectId)])
      .then(([libRes, snips]) => {
        if (cancelled) return;
        setLib(libRes.references);
        setSnippets(snips);
      })
      .catch(() => {
        if (cancelled) return;
        setLib([]);
        setSnippets([]);
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const loading = lib == null || snippets == null;
  const resolved = loading ? [] : resolveReferences(reference, lib, snippets);
  const materials = resolved.filter((r): r is MaterialRef => r.kind === "material" && !r.missing);
  const notes = resolved.filter((r): r is NoteRef => r.kind === "note" && !r.missing);
  const showProposal = PROPOSAL_VISIBLE_STAGES.includes(stage);
  const isEmpty = !loading && !showProposal && materials.length === 0 && notes.length === 0;

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
            <AnnotationGroup />
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

function AnnotationGroup() {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[14px] font-semibold text-mk-ink">批注</p>
      <p className="text-[14px] leading-relaxed text-mk-faint">批注会在印记体检你的写作后出现。</p>
    </div>
  );
}
