import { useEffect, useMemo, useState } from "react";
import type {
  ExplorationLead,
  ExplorationView as ExplorationViewData,
  GuideDirection,
  MaterialSource,
  PhaseTag,
  Reference,
} from "@mind-imprint/contracts";
import { enterReading, NoReadableContentError, postRabbitHoleCard } from "../../api/workspace";
import { createLead, digDeeper, getExploration, patchLead } from "../../../api/exploration";
import { StudioCardSheet } from "../../../studio/StudioCardSheet";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

// S3 rabbit-hole exploration surface (Task 9): the branch view over the
// leads Task 8's endpoints track. No graph/tree library — this is a
// structured branch layout (flex columns + light connector rules), not a
// physics graph, per the brief. Visual language (chips/badges) is borrowed
// verbatim from ReadingBlock's list view so 列表 ⇄ 探索图谱 reads as one room,
// not two products bolted together.

const PHASE_CHIP = "flex-none rounded bg-mk-bg px-1.5 py-0.5 text-[10px] font-bold text-mk-muted";
const READ_BADGE_DONE = "flex-none rounded px-1.5 py-0.5 text-[10px] font-bold bg-mk-green-tint text-mk-green";
const READ_BADGE_ACTIVE = "flex-none rounded px-1.5 py-0.5 text-[10px] font-bold bg-mk-primary-tint text-mk-primary";

export type ExplorationViewProps = {
  projectId: string;
  references: Reference[];
  // Matches ReadingBlock's setReadingSource signature exactly — pass it
  // straight through so 悬空来源's "进入阅读室" reuses the same swap slot.
  onEnterReading?: (
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
  ) => void;
};

function readingBadge(r: Reference): { label: string; cls: string } | null {
  if (r.takeaway != null) return { label: "已归纳", cls: READ_BADGE_DONE };
  if (r.materialId != null) return { label: "在读", cls: READ_BADGE_ACTIVE };
  return null;
}

export function ExplorationView({ projectId, references, onEnterReading }: ExplorationViewProps) {
  const [view, setView] = useState<ExplorationViewData>({ leads: [], danglingSourceIds: [] });
  const [loading, setLoading] = useState(true);
  const [busyLeadIds, setBusyLeadIds] = useState<Set<string>>(new Set());
  const [pickerFor, setPickerFor] = useState<string | null>(null);
  const [newLeadText, setNewLeadText] = useState("");
  const [creatingLead, setCreatingLead] = useState(false);
  const [directions, setDirections] = useState<GuideDirection[] | null>(null);
  // S4 · the 兔子洞 card. Opening is the student's tap; on submit the reflection
  // persists to the process tree (过程即数据) — no longer the S3 silent discard.
  const [rabbitOpen, setRabbitOpen] = useState(false);
  const [digging, setDigging] = useState(false);
  const [diggingError, setDiggingError] = useState(false);
  const [adoptedDirections, setAdoptedDirections] = useState<Set<number>>(new Set());
  // #12/#13 · dig deeper from ONE lead, carrying the student's own thinking.
  // null = the whole-graph 深挖一层.
  const [focusLead, setFocusLead] = useState<ExplorationLead | null>(null);
  const [thought, setThought] = useState("");
  const [enteringRefId, setEnteringRefId] = useState<string | null>(null);
  const [leadActionError, setLeadActionError] = useState(false);

  const refresh = useMemo(
    () => async () => {
      try {
        setView(await getExploration(projectId));
      } catch {
        /* keep the last-good view; a failed refresh must never blank the graph */
      }
    },
    [projectId],
  );

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const v = await getExploration(projectId);
        if (!cancelled) setView(v);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  async function withBusy(lid: string, fn: () => Promise<unknown>) {
    setBusyLeadIds((s) => new Set(s).add(lid));
    try {
      await fn();
      await refresh();
    } finally {
      setBusyLeadIds((s) => {
        const n = new Set(s);
        n.delete(lid);
        return n;
      });
    }
  }

  function connectLead(lid: string, connectedReferenceId: string) {
    setPickerFor(null);
    setLeadActionError(false);
    void withBusy(lid, () => patchLead(projectId, lid, { status: "connected", connectedReferenceId })).catch(() => {
      setLeadActionError(true);
    });
  }
  function pruneLead(lid: string) {
    setPickerFor(null);
    setLeadActionError(false);
    void withBusy(lid, () => patchLead(projectId, lid, { status: "pruned" })).catch(() => {
      setLeadActionError(true);
    });
  }

  async function addManualLead() {
    const text = newLeadText.trim();
    if (!text || creatingLead) return;
    setCreatingLead(true);
    try {
      await createLead(projectId, text);
      setNewLeadText("");
      await refresh();
    } catch {
      /* keep her draft text so she can just retry */
    } finally {
      setCreatingLead(false);
    }
  }

  async function runDigDeeper() {
    if (digging) return;
    setDigging(true);
    setDiggingError(false);
    setDirections(null);
    setAdoptedDirections(new Set());
    try {
      const guide = await digDeeper(projectId, focusLead ? { leadId: focusLead.id, thought: thought.trim() } : undefined);
      setDirections(guide.directions);
    } catch {
      setDiggingError(true);
    } finally {
      setDigging(false);
    }
  }

  async function adoptDirection(i: number, direction: string) {
    if (adoptedDirections.has(i)) return;
    setAdoptedDirections((s) => new Set(s).add(i));
    try {
      await createLead(projectId, direction);
      await refresh();
    } catch {
      setAdoptedDirections((s) => {
        const n = new Set(s);
        n.delete(i);
        return n;
      });
    }
  }

  async function enterDangling(ref: Reference) {
    if (!onEnterReading || enteringRefId) return;
    setEnteringRefId(ref.id);
    try {
      const { source, suggestedReason } = await enterReading(projectId, ref.id);
      onEnterReading(source, ref.id, suggestedReason, ref.phaseTag, ref.readingReason, ref.readingFocus);
    } catch (e) {
      // NoReadableContentError has a paste fallback in the Library preview —
      // this tray just nudges toward the Library rather than duplicating it.
      void (e instanceof NoReadableContentError);
    } finally {
      setEnteringRefId(null);
    }
  }

  const leadsBySource = useMemo(() => {
    const m = new Map<string, ExplorationLead[]>();
    for (const l of view.leads) {
      if (l.sourceReferenceId) {
        const arr = m.get(l.sourceReferenceId) ?? [];
        arr.push(l);
        m.set(l.sourceReferenceId, arr);
      }
    }
    return m;
  }, [view.leads]);

  // Only counts as "branched" with at least one non-pruned source-lead — a
  // ref whose only leads are pruned has no visible open branch to show here,
  // and the server's danglingSourceIds correctly re-includes it (a source
  // whose leads are ALL pruned falls back to dangling). Without this guard
  // that ref would render in both sections at once.
  const branchedRefs = references.filter((r) => (leadsBySource.get(r.id) ?? []).some((l) => l.status !== "pruned"));
  const danglingRefs = references.filter((r) => view.danglingSourceIds.includes(r.id));
  // Every lead with no source reference lands here — including leads that
  // were connected/pruned whose origin reference was later deleted (DB sets
  // sourceReferenceId to NULL on delete). Those must still render (with their
  // resolved marker) instead of silently vanishing from the whole view.
  const looseLeads = view.leads.filter((l) => l.sourceReferenceId == null);

  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载探索图谱中…</div>;
  }

  const empty = branchedRefs.length === 0 && danglingRefs.length === 0 && looseLeads.length === 0;

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-y-auto bg-mk-bg/40 px-6 py-5">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h2 className="font-sans text-[16px] font-bold text-mk-ink">探索图谱</h2>
          <p className="mt-0.5 text-[12px] text-mk-muted-2">来源怎么分出新的线索、还有什么线索没追完——一眼看全</p>
        </div>
        {CARD_REGISTRY["rabbit-hole"] && (
          <button
            type="button"
            onClick={() => setRabbitOpen(true)}
            className="flex-none rounded-lg border border-mk-border bg-mk-surface px-3 py-1.5 text-[12px] font-semibold text-mk-ink hover:bg-mk-bg"
          >
            ＋ 兔子洞
          </button>
        )}
      </div>

      {rabbitOpen && CARD_REGISTRY["rabbit-hole"] && (
        <div className="mb-4">
          <StudioCardSheet
            spec={CARD_REGISTRY["rabbit-hole"]}
            onSubmit={async (env) => {
              setRabbitOpen(false);
              setLeadActionError(false);
              try {
                await postRabbitHoleCard(projectId, env.field_values, env.event_trace);
                // #13 · the card used to close with no visible change. Now the
                // student's "next exploration direction" (or, failing that, her
                // interest anchor) becomes a real 线索 — it shows up in 待追的线索
                // and can be connected to a source or dug into.
                const fv = env.field_values as Record<string, unknown>;
                const seed = [fv.explorable_direction, fv.anchor_note]
                  .map((v) => (typeof v === "string" ? v.trim() : ""))
                  .find((s) => s.length > 0);
                if (seed) await createLead(projectId, seed);
              } catch {
                setLeadActionError(true);
              } finally {
                // Always re-fetch: even if createLead failed after the card
                // persisted, the graph should reflect reality (review Low).
                await refresh();
              }
            }}
            onSkip={() => setRabbitOpen(false)}
          />
        </div>
      )}

      {leadActionError && (
        <p className="mb-3 text-[12px] font-semibold text-mk-accent">刚才那步没接上，再试一次？</p>
      )}

      {empty ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-mk-lg border border-dashed border-mk-border bg-mk-surface px-6 py-10 text-center">
          <p className="text-[13.5px] font-bold text-mk-ink">这里还是空的</p>
          <p className="mt-1.5 max-w-sm text-[12.5px] leading-relaxed text-mk-muted">
            读完一篇来源、并在「归纳」时写下你发现的新线索，它们就会长到这里（比如从 NASA 报告牵到《自然·可持续发展》那篇论文）。也随时可以自己记一条待追的方向——下面就能加。
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-6">
          {branchedRefs.length > 0 && (
            <section>
              <h3 className="mb-2 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">有分支的来源</h3>
              <div className="flex flex-col gap-4">
                {branchedRefs.map((ref) => (
                  <SourceBranch
                    key={ref.id}
                    ref_={ref}
                    leads={leadsBySource.get(ref.id) ?? []}
                    references={references}
                    busyLeadIds={busyLeadIds}
                    pickerFor={pickerFor}
                    onOpenPicker={setPickerFor}
                    onConnect={connectLead}
                    onPrune={pruneLead}
                    onDig={(l) => { setFocusLead(l); setThought(""); setDirections(null); }}
                  />
                ))}
              </div>
            </section>
          )}

          {danglingRefs.length > 0 && (
            <section>
              <h3 className="mb-2 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">悬空来源 · 读完了但还没牵出线索</h3>
              <div className="flex flex-col gap-2">
                {danglingRefs.map((ref) => {
                  const badge = readingBadge(ref);
                  return (
                    <div key={ref.id} className="flex items-center justify-between gap-3 rounded-mk border border-mk-border bg-mk-surface px-3.5 py-2.5">
                      <div className="min-w-0">
                        <div className="flex items-center gap-1.5">
                          <span className="truncate text-[13px] font-semibold text-mk-ink">{ref.title}</span>
                          {badge && <span className={badge.cls}>{badge.label}</span>}
                          {ref.phaseTag && <span className={PHASE_CHIP}>{ref.phaseTag}</span>}
                        </div>
                        <p className="mt-0.5 text-[12px] leading-relaxed text-mk-muted-2">
                          「{ref.title}」读完了，但还没牵出新的线索——里面有没有什么点让你想继续往下挖？
                        </p>
                      </div>
                      {onEnterReading && (
                        <button
                          type="button"
                          onClick={() => enterDangling(ref)}
                          disabled={enteringRefId === ref.id}
                          className="flex-none rounded-mk bg-mk-primary px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
                        >
                          {enteringRefId === ref.id ? "打开中…" : "进入阅读室"}
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
            </section>
          )}

          <section>
            <h3 className="mb-2 text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">待追的线索</h3>
            {looseLeads.length === 0 ? (
              <p className="mb-2 text-[12px] text-mk-muted-2">还没有手动记的线索——读完一篇来源会自动生成，或者自己记一条。</p>
            ) : (
              <div className="mb-2 flex flex-wrap gap-2">
                {looseLeads.map((l) => (
                  <BranchLeadChip
                    key={l.id}
                    lead={l}
                    references={references}
                    busy={busyLeadIds.has(l.id)}
                    pickerOpen={pickerFor === l.id}
                    onOpenPicker={() => setPickerFor(pickerFor === l.id ? null : l.id)}
                    onConnect={(refId) => connectLead(l.id, refId)}
                    onPrune={() => pruneLead(l.id)}
                    onDig={() => { setFocusLead(l); setThought(""); setDirections(null); }}
                  />
                ))}
              </div>
            )}
            <div className="flex items-center gap-2 rounded-mk border border-mk-border bg-mk-surface px-2.5 py-2">
              <input
                value={newLeadText}
                onChange={(e) => setNewLeadText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") addManualLead();
                }}
                placeholder="+ 手动添加线索……（比如「中国碳排放全球第一，这跟可持续矛盾吗？」）"
                className="flex-1 bg-transparent text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2"
              />
              <button
                type="button"
                onClick={addManualLead}
                disabled={creatingLead || !newLeadText.trim()}
                className="flex-none rounded-mk bg-mk-primary px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
              >
                {creatingLead ? "记录中…" : "添加"}
              </button>
            </div>
          </section>
        </div>
      )}

      <section className="mt-6 rounded-mk-lg border border-mk-primary/25 bg-mk-primary-tint/30 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-[13.5px] font-bold text-mk-ink">{focusLead ? "深挖这条线索" : "深挖一层"}</h3>
            {focusLead ? (
              <p className="mt-0.5 truncate text-[12px] font-semibold text-mk-primary">
                「{focusLead.text}」
                <button type="button" onClick={() => { setFocusLead(null); setThought(""); }} className="ml-1 font-bold text-mk-muted-2 hover:text-mk-ink">改回整体 ×</button>
              </p>
            ) : (
              <p className="mt-0.5 text-[12px] text-mk-muted-2">让印记根据你已经读过的来源，提几个可以继续挖的方向——不会自动帮你记下</p>
            )}
          </div>
          <button
            type="button"
            onClick={runDigDeeper}
            disabled={digging}
            className="flex-none rounded-mk bg-mk-primary px-3.5 py-2 text-[12.5px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
          >
            {digging ? "在想……" : focusLead ? "让印记给方向" : "深挖一层"}
          </button>
        </div>

        {focusLead && (
          <textarea
            value={thought}
            onChange={(e) => setThought(e.target.value)}
            rows={2}
            placeholder="说说你现在的想法（可选）——印记会顺着你的思路给方向"
            className="mt-3 w-full resize-none rounded-mk border border-mk-border bg-mk-surface px-3 py-2 text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
        )}

        {digging && <p className="mt-3 text-[12.5px] text-mk-muted-2">印记在想新的方向……</p>}
        {!digging && diggingError && <p className="mt-3 text-[12.5px] font-semibold text-mk-accent">刚才没接上，再试一次？</p>}
        {!digging && directions && directions.length === 0 && (
          <p className="mt-3 text-[12.5px] text-mk-muted-2">印记暂时没找到新的方向——继续往下读，新的方向会自然冒出来。</p>
        )}
        {!digging && directions && directions.length > 0 && (
          <div className="mt-3 flex flex-col gap-2">
            {directions.map((d, i) => (
              <div key={i} className="rounded-mk border border-mk-border bg-mk-surface p-3">
                <p className="text-[13px] font-semibold leading-relaxed text-mk-ink">{d.direction}</p>
                <p className="mt-1 text-[12px] leading-relaxed text-mk-muted-2">{d.why}</p>
                <button
                  type="button"
                  onClick={() => adoptDirection(i, d.direction)}
                  disabled={adoptedDirections.has(i)}
                  className="mt-2 rounded-mk border border-mk-primary/40 bg-mk-primary-tint px-2.5 py-1 text-[11.5px] font-bold text-mk-primary hover:bg-mk-primary/20 disabled:opacity-50"
                >
                  {adoptedDirections.has(i) ? "已记为线索" : "记为线索"}
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

/* ---------- a source with leads hanging beneath it ---------- */

function SourceBranch({
  ref_,
  leads,
  references,
  busyLeadIds,
  pickerFor,
  onOpenPicker,
  onConnect,
  onPrune,
  onDig,
}: {
  ref_: Reference;
  leads: ExplorationLead[];
  references: Reference[];
  busyLeadIds: Set<string>;
  pickerFor: string | null;
  onOpenPicker: (lid: string | null) => void;
  onConnect: (lid: string, connectedReferenceId: string) => void;
  onPrune: (lid: string) => void;
  onDig: (lead: ExplorationLead) => void;
}) {
  const badge = readingBadge(ref_);
  return (
    <div>
      <div className="flex items-center gap-1.5 rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
        <span className="h-1.5 w-1.5 flex-none rounded-full bg-mk-green" />
        <span className="truncate text-[13px] font-semibold text-mk-ink">{ref_.title}</span>
        {badge && <span className={badge.cls}>{badge.label}</span>}
        {ref_.phaseTag && <span className={PHASE_CHIP}>{ref_.phaseTag}</span>}
      </div>
      <div className="ml-3 flex flex-col gap-1.5 border-l-2 border-mk-border pl-4 pt-2">
        {leads.map((l) => (
          <BranchLeadChip
            key={l.id}
            lead={l}
            references={references}
            busy={busyLeadIds.has(l.id)}
            pickerOpen={pickerFor === l.id}
            onOpenPicker={() => onOpenPicker(pickerFor === l.id ? null : l.id)}
            onConnect={(refId) => onConnect(l.id, refId)}
            onPrune={() => onPrune(l.id)}
            onDig={() => onDig(l)}
          />
        ))}
      </div>
    </div>
  );
}

function BranchLeadChip({
  lead,
  references,
  busy,
  pickerOpen,
  onOpenPicker,
  onConnect,
  onPrune,
  onDig,
}: {
  lead: ExplorationLead;
  references: Reference[];
  busy: boolean;
  pickerOpen: boolean;
  onOpenPicker: () => void;
  onConnect: (refId: string) => void;
  onPrune: () => void;
  onDig?: () => void;
}) {
  if (lead.status !== "open") {
    return <ResolvedLeadChip lead={lead} references={references} />;
  }
  return (
    <div className="relative flex items-center gap-2 rounded-full bg-mk-primary-tint/60 px-3 py-1.5">
      <span className="text-[12.5px] font-semibold text-mk-ink">{lead.text}</span>
      <div className="ml-auto flex flex-none items-center gap-1.5">
        {onDig && (
          <button
            type="button"
            onClick={onDig}
            disabled={busy}
            title="让印记顺着这条线索给你下一步方向"
            className="rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[11px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
          >
            深挖这条
          </button>
        )}
        <button
          type="button"
          onClick={onOpenPicker}
          disabled={busy}
          className="rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[11px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
        >
          连接来源
        </button>
        <button
          type="button"
          onClick={onPrune}
          disabled={busy}
          className="rounded-full border border-mk-border bg-mk-surface px-2 py-0.5 text-[11px] font-bold text-mk-muted hover:text-mk-accent disabled:opacity-50"
        >
          剪枝
        </button>
      </div>
      {pickerOpen && <ConnectPicker references={references} onPick={onConnect} onClose={onOpenPicker} />}
    </div>
  );
}

// Connected/pruned leads are resolved — no more actions, just a marker so the
// branch still reads honestly (nothing silently vanishes once acted on).
function ResolvedLeadChip({ lead, references }: { lead: ExplorationLead; references: Reference[] }) {
  if (lead.status === "connected") {
    const target = references.find((r) => r.id === lead.connectedReferenceId);
    return (
      <div className="flex items-center gap-1.5 rounded-full bg-mk-green-tint/60 px-3 py-1.5 text-[12.5px]">
        <span className="font-semibold text-mk-ink line-through decoration-mk-green/40">{lead.text}</span>
        <span className="flex-none rounded-full bg-mk-green px-2 py-0.5 text-[10.5px] font-bold text-white">
          → {target?.title ?? "已连接来源"}
        </span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5 rounded-full bg-mk-bg px-3 py-1.5 text-[12.5px]">
      <span className="font-semibold text-mk-muted-2 line-through">{lead.text}</span>
      <span className="flex-none rounded-full bg-mk-muted px-2 py-0.5 text-[10.5px] font-bold text-white">已剪枝</span>
    </div>
  );
}

function ConnectPicker({
  references,
  onPick,
  onClose,
}: {
  references: Reference[];
  onPick: (refId: string) => void;
  onClose: () => void;
}) {
  return (
    <div className="absolute left-0 top-full z-20 mt-1 max-h-56 w-64 overflow-y-auto rounded-mk border border-mk-border bg-mk-surface p-1.5 shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
      <div className="flex items-center justify-between px-1.5 pb-1">
        <span className="text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">连接到哪篇来源</span>
        <button type="button" onClick={onClose} className="text-[13px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
      </div>
      {references.length === 0 ? (
        <p className="px-1.5 py-2 text-[12px] text-mk-muted-2">文献库还没有来源可连接。</p>
      ) : (
        references.map((r) => (
          <button
            key={r.id}
            type="button"
            onClick={() => onPick(r.id)}
            className="block w-full truncate rounded px-2 py-1.5 text-left text-[12.5px] text-mk-ink hover:bg-mk-primary-tint"
          >
            {r.title}
          </button>
        ))
      )}
    </div>
  );
}
