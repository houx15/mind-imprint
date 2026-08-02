import { useEffect, useMemo, useState } from "react";
import type {
  CardTurnRef,
  ExplorationLead,
  ExplorationView as ExplorationViewData,
  GuideDirection,
  MaterialSource,
  PhaseTag,
  Reference,
} from "@mind-imprint/contracts";
import { enterReading, NoReadableContentError, reflectProjectCard } from "../../api/workspace";
import { createLead, digDeeper, getExploration, patchLead } from "../../../api/exploration";
import { StudioCardSheet } from "../../../studio/StudioCardSheet";
import { compileCardForCoach } from "../../../studio/compileCard";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

// S3 rabbit-hole exploration surface (Task 9): the branch view over the
// leads Task 8's endpoints track. No graph/tree library — this is a
// structured branch layout (flex columns + light connector rules), not a
// physics graph, per the brief. Visual language (chips/badges) is borrowed
// verbatim from ReadingBlock's list view so 列表 ⇄ 探索图谱 reads as one room,
// not two products bolted together.
//
// Batch5 Item A: this view is now a FULL 线索→来源 workspace, not just a
// branch reader — a student can attach a source under an open lead (keeping
// the lead itself open for more sources), register a brand-new untracked
// source without leaving the graph, peek a source's metadata inline, and
// enter the reading room from any source node. No new DB shape: "a source
// under a lead" is modeled as a CHILD branch (createLead+parentLeadId) whose
// connectedReferenceId IS that source — the child resolves-as-source while
// the parent lead stays open for more.

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
  // Item A #2 · register a brand-new untracked source without leaving the
  // graph — lifted from ReadingBlock so it reuses the SAME createReference +
  // library-refresh path the 列表 add-source modal uses (no separate
  // client-side reference store here). Returns the created Reference so this
  // view can immediately attach it under a lead when one was chosen.
  onCreateReference?: (input: { title: string; url?: string }) => Promise<Reference>;
  // Followup fix (2026-08): the rabbit-hole card now goes through the shared
  // reflect turn (like every other deck card), so it gets AI feedback on what
  // the student actually wrote instead of a silent persist. This view has no
  // chat thread of its own — the parent (ReadingBlock's 找资料 coach panel)
  // renders the student turn + reply, mirroring CoachCardPanel's onReflected.
  onCardReflected?: (studentText: string, reply: string, card?: CardTurnRef) => void;
  // Followup fix 2 (2026-08): the ONLY other place that can open the
  // 兔子洞 card is the sibling FloatingCoach (印记 · 找资料) — it has no state
  // of its own here, so ReadingBlock bridges the request the same way it
  // bridges onCardReflected above: a one-shot boolean flag this view consumes
  // (opens the SAME sheet the header's ＋兔子洞 button opens, then flips the
  // flag back off) rather than forking a second rabbit-hole implementation.
  openRabbitHoleRequested?: boolean;
  onRabbitHoleOpenConsumed?: () => void;
};

function readingBadge(r: Reference): { label: string; cls: string } | null {
  if (r.takeaway != null) return { label: "已归纳", cls: READ_BADGE_DONE };
  if (r.materialId != null) return { label: "在读", cls: READ_BADGE_ACTIVE };
  return null;
}

export function ExplorationView({
  projectId,
  references,
  onEnterReading,
  onCreateReference,
  onCardReflected,
  openRabbitHoleRequested,
  onRabbitHoleOpenConsumed,
}: ExplorationViewProps) {
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
  // Item A #1/#2 · attach-a-source popover (pick existing) + add-untracked modal.
  const [sourcePickerFor, setSourcePickerFor] = useState<string | null>(null);
  const [addSourceOpen, setAddSourceOpen] = useState(false);
  const [addSourceForLead, setAddSourceForLead] = useState<string | null>(null);
  const [addSourceBusy, setAddSourceBusy] = useState(false);
  // Item B · rabbit-hole redesign: which open 线索 (if any) to dig from, and a
  // gentle notice when the card carried neither a chosen lead nor written text.
  const [rabbitLeadId, setRabbitLeadId] = useState<string | null>(null);
  const [rabbitNotice, setRabbitNotice] = useState<string | null>(null);

  // Followup fix 2 (2026-08): the sibling 找资料 coach's own ＋兔子洞 affordance
  // requests the SAME sheet via this one-shot flag — reset it right away so a
  // later remount (or another true→true edge, which React wouldn't re-fire on
  // its own anyway) can't reopen it unexpectedly.
  useEffect(() => {
    if (!openRabbitHoleRequested) return;
    setRabbitOpen(true);
    setRabbitLeadId(null);
    setRabbitNotice(null);
    onRabbitHoleOpenConsumed?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openRabbitHoleRequested]);

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

  // #12 · hang a 分支 under a lead.
  async function addBranch(parentLeadId: string, text: string) {
    const t = text.trim();
    if (!t) return;
    try {
      await createLead(projectId, t, { parentLeadId });
      await refresh();
    } catch {
      setLeadActionError(true);
    }
  }

  // Item A #1 · attach an EXISTING reference under an open lead, keeping the
  // parent open: a child branch is created then immediately connected to that
  // source. The parent lead is never resolved by this — more sources can
  // still be added under it later.
  async function attachExistingSource(leadId: string, referenceId: string) {
    setSourcePickerFor(null);
    setLeadActionError(false);
    const ref = references.find((r) => r.id === referenceId);
    await withBusy(leadId, async () => {
      const child = await createLead(projectId, ref?.title ?? "来源", { parentLeadId: leadId });
      await patchLead(projectId, child.id, { status: "connected", connectedReferenceId: referenceId });
    }).catch(() => setLeadActionError(true));
  }

  // Item A #2 · open the add-untracked-source modal, optionally scoped to a
  // lead (null = just register it in the library; it'll surface wherever an
  // unconnected read source normally would, e.g. 悬空来源 once read).
  function openAddSource(leadId: string | null) {
    setSourcePickerFor(null);
    setAddSourceForLead(leadId);
    setAddSourceOpen(true);
  }

  async function submitNewSource(input: { title: string; url: string }) {
    if (!onCreateReference || addSourceBusy) return;
    if (!input.title && !input.url) return;
    setAddSourceBusy(true);
    setLeadActionError(false);
    const leadId = addSourceForLead;
    try {
      const ref = await onCreateReference({ title: input.title, url: input.url || undefined });
      if (leadId) {
        const child = await createLead(projectId, ref.title || input.title || "来源", { parentLeadId: leadId });
        await patchLead(projectId, child.id, { status: "connected", connectedReferenceId: ref.id });
      }
      setAddSourceOpen(false);
      setAddSourceForLead(null);
      await refresh();
    } catch {
      setLeadActionError(true);
    } finally {
      setAddSourceBusy(false);
    }
  }

  // Item B · dig from a lead (or the whole graph when lead is null), carrying
  // a thought. Both the 深挖这条/深挖一层 button AND the rabbit-hole finalize
  // path call this — one code path, one place suggestions ever render.
  async function digFromLead(lead: ExplorationLead | null, thoughtText: string) {
    if (digging) return;
    setFocusLead(lead);
    setThought(thoughtText);
    setDigging(true);
    setDiggingError(false);
    setDirections(null);
    setAdoptedDirections(new Set());
    try {
      const guide = await digDeeper(projectId, lead ? { leadId: lead.id, thought: thoughtText.trim() } : undefined);
      setDirections(guide.directions);
    } catch {
      setDiggingError(true);
    } finally {
      setDigging(false);
    }
  }

  async function runDigDeeper() {
    await digFromLead(focusLead, thought);
  }

  async function adoptDirection(i: number, direction: string) {
    if (adoptedDirections.has(i)) return;
    setAdoptedDirections((s) => new Set(s).add(i));
    try {
      // #12 · when the directions came from digging ONE lead, adopt them as its
      // 分支 (nested under it); otherwise as a top-level thread.
      await createLead(projectId, direction, focusLead ? { parentLeadId: focusLead.id } : undefined);
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

  // #12 · 分支 render NESTED under their parent, not in the flat lists. Split
  // top-level threads (parentLeadId null) from children, and index children by
  // parent for the recursive render. (Parent delete cascades to children, so
  // there are no orphaned 分支.)
  const childrenByParent = useMemo(() => {
    const m = new Map<string, ExplorationLead[]>();
    for (const l of view.leads) {
      if (l.parentLeadId) {
        const arr = m.get(l.parentLeadId) ?? [];
        arr.push(l);
        m.set(l.parentLeadId, arr);
      }
    }
    return m;
  }, [view.leads]);
  const topLevel = useMemo(() => view.leads.filter((l) => l.parentLeadId == null), [view.leads]);
  // Item B · every currently-open lead (top-level or branch) is a valid dig
  // target for the rabbit-hole card.
  const openLeadsFlat = useMemo(() => view.leads.filter((l) => l.status === "open"), [view.leads]);

  const leadsBySource = useMemo(() => {
    const m = new Map<string, ExplorationLead[]>();
    for (const l of topLevel) {
      if (l.sourceReferenceId) {
        const arr = m.get(l.sourceReferenceId) ?? [];
        arr.push(l);
        m.set(l.sourceReferenceId, arr);
      }
    }
    return m;
  }, [topLevel]);

  // A ref is "branched" (renders its lead tree) when it has a top-level lead that
  // is itself non-pruned OR still has a non-pruned 分支 beneath it — otherwise an
  // open child would vanish when its parent lead is pruned (the parent falls to
  // dangling server-side, but its open children must stay reachable). Exclude
  // those from dangling so a ref never renders in both sections (review Medium).
  const hasOpenDescendant = (leadId: string): boolean =>
    (childrenByParent.get(leadId) ?? []).some((k) => k.status !== "pruned" || hasOpenDescendant(k.id));
  const branchedRefs = references.filter((r) =>
    (leadsBySource.get(r.id) ?? []).some((l) => l.status !== "pruned" || hasOpenDescendant(l.id)),
  );
  const branchedIds = new Set(branchedRefs.map((r) => r.id));
  const danglingRefs = references.filter((r) => view.danglingSourceIds.includes(r.id) && !branchedIds.has(r.id));
  // Every top-level lead with no source reference lands here — including leads
  // that were connected/pruned whose origin reference was later deleted (DB sets
  // sourceReferenceId to NULL on delete). Those must still render (with their
  // resolved marker) instead of silently vanishing from the whole view.
  const looseLeads = topLevel.filter((l) => l.sourceReferenceId == null);

  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载探索图谱中…</div>;
  }

  const empty = branchedRefs.length === 0 && danglingRefs.length === 0 && looseLeads.length === 0;

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-y-auto bg-mk-bg/40 px-6 py-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <h2 className="font-sans text-[16px] font-bold text-mk-ink">探索图谱</h2>
          <p className="mt-0.5 text-[12px] text-mk-muted-2">来源怎么分出新的线索、还有什么线索没追完——一眼看全</p>
        </div>
        <div className="flex flex-none items-center gap-2">
          {onCreateReference && (
            <button
              type="button"
              onClick={() => openAddSource(null)}
              className="flex-none rounded-lg border border-mk-border bg-mk-surface px-3 py-1.5 text-[12px] font-semibold text-mk-ink hover:bg-mk-bg"
            >
              ＋ 添加来源
            </button>
          )}
          {CARD_REGISTRY["rabbit-hole"] && (
            <button
              type="button"
              onClick={() => { setRabbitOpen(true); setRabbitLeadId(null); setRabbitNotice(null); }}
              className="flex-none rounded-lg border border-mk-border bg-mk-surface px-3 py-1.5 text-[12px] font-semibold text-mk-ink hover:bg-mk-bg"
            >
              ＋ 兔子洞
            </button>
          )}
        </div>
      </div>

      {addSourceOpen && onCreateReference && (
        <AddUntrackedSourceModal
          forLeadText={addSourceForLead ? (view.leads.find((l) => l.id === addSourceForLead)?.text ?? null) : null}
          busy={addSourceBusy}
          onSubmit={submitNewSource}
          onClose={() => { setAddSourceOpen(false); setAddSourceForLead(null); }}
        />
      )}

      {rabbitOpen && CARD_REGISTRY["rabbit-hole"] && (
        <div className="mb-4">
          {/* Item B · pick which open 线索 to dig from — the card's written
              thought becomes digDeeper's `thought`. Leaving it unselected keeps
              the old create-a-new-lead behavior. */}
          {openLeadsFlat.length > 0 && (
            <div className="mb-2 flex items-center gap-2 rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
              <label className="flex-none text-[12px] font-semibold text-mk-muted-2">从哪条线索挖？</label>
              <select
                value={rabbitLeadId ?? ""}
                onChange={(e) => setRabbitLeadId(e.target.value || null)}
                aria-label="从哪条线索挖"
                className="flex-1 rounded border border-mk-border bg-mk-bg px-2 py-1 text-[12.5px] text-mk-ink outline-none"
              >
                <option value="">不选——记一个全新的方向</option>
                {openLeadsFlat.map((l) => (
                  <option key={l.id} value={l.id}>{l.text}</option>
                ))}
              </select>
            </div>
          )}
          <StudioCardSheet
            spec={CARD_REGISTRY["rabbit-hole"]}
            onSubmit={async (env) => {
              setRabbitOpen(false);
              setLeadActionError(false);
              setRabbitNotice(null);
              const fv = env.field_values as Record<string, unknown>;
              const seed =
                [fv.explorable_direction, fv.anchor_note]
                  .map((v) => (typeof v === "string" ? v.trim() : ""))
                  .find((s) => s.length > 0) ?? "";
              const selectedLead = rabbitLeadId ? view.leads.find((l) => l.id === rabbitLeadId) ?? null : null;
              setRabbitLeadId(null);
              // FEEDBACK: route through the shared reflect turn (card_persist.go's
              // explorationDeckCards allowlist) — exactly ONE card_instance per
              // submit, plus a coach reply on what the student actually wrote,
              // consistent with every other deck card (forming/writing/reflection).
              // Persist first — independent of whether a dig/lead follows.
              const spec = CARD_REGISTRY["rabbit-hole"];
              const studentText = spec ? compileCardForCoach(spec, fv) : "";
              try {
                const res = await reflectProjectCard(
                  projectId,
                  "rabbit-hole",
                  env.field_values,
                  env.event_trace,
                  "find_sources",
                );
                onCardReflected?.(studentText, res.reply, res.card ?? undefined);
              } catch {
                setLeadActionError(true);
                onCardReflected?.(studentText, "刚才没接住这张卡，等下再试一次。");
              }
              try {
                if (selectedLead) {
                  // #13 · reuse the SAME metered dig-deeper mechanism, showing
                  // suggestions in the focus panel below — never invisible.
                  await digFromLead(selectedLead, seed);
                } else if (seed) {
                  await createLead(projectId, seed);
                } else {
                  // Nothing written and no lead chosen — say so rather than a
                  // silent no-op (the S3 invisibility bug).
                  setRabbitNotice("这次没记下新的方向或线索——想到了随时可以再记一笔。");
                }
              } catch {
                setLeadActionError(true);
              } finally {
                // Always re-fetch: even if a step above failed after the card
                // persisted, the graph should reflect reality (review Low).
                await refresh();
              }
            }}
            onSkip={() => { setRabbitOpen(false); setRabbitLeadId(null); }}
          />
        </div>
      )}

      {leadActionError && (
        <p className="mb-3 text-[12px] font-semibold text-mk-accent">刚才那步没接上，再试一次？</p>
      )}
      {rabbitNotice && (
        <p className="mb-3 text-[12px] font-semibold text-mk-muted-2">{rabbitNotice}</p>
      )}

      {empty ? (
        <div className="flex flex-1 flex-col items-center justify-center rounded-mk-lg border border-dashed border-mk-border bg-mk-surface px-6 py-10 text-center">
          <p className="text-[13.5px] font-bold text-mk-ink">这里还是空的</p>
          <p className="mt-1.5 max-w-sm text-[12.5px] leading-relaxed text-mk-muted">
            读完一篇来源、并在「归纳」时写下你发现的新线索，它们就会长到这里（比如从 NASA 报告牵到《自然·可持续发展》那篇论文）。也随时可以自己记一条待追的方向——下面就能加，或者直接「＋添加来源」把第一篇来源放进来。
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
                    childrenByParent={childrenByParent}
                    references={references}
                    busyLeadIds={busyLeadIds}
                    pickerFor={pickerFor}
                    onOpenPicker={setPickerFor}
                    onConnect={connectLead}
                    onPrune={pruneLead}
                    onDig={(l) => { setFocusLead(l); setThought(""); setDirections(null); }}
                    onAddBranch={addBranch}
                    sourcePickerFor={sourcePickerFor}
                    onOpenSourcePicker={setSourcePickerFor}
                    onAttachExisting={attachExistingSource}
                    onOpenAddSource={openAddSource}
                    onEnterReading={onEnterReading ? enterDangling : undefined}
                    enteringRefId={enteringRefId}
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
                          <SourceMetaButton ref_={ref} />
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
              <div className="mb-2 flex flex-col gap-2">
                {looseLeads.map((l) => (
                  <LeadWithBranches
                    key={l.id}
                    lead={l}
                    childrenByParent={childrenByParent}
                    references={references}
                    busyLeadIds={busyLeadIds}
                    pickerFor={pickerFor}
                    onOpenPicker={setPickerFor}
                    onConnect={connectLead}
                    onPrune={pruneLead}
                    onDig={(lead) => { setFocusLead(lead); setThought(""); setDirections(null); }}
                    onAddBranch={addBranch}
                    sourcePickerFor={sourcePickerFor}
                    onOpenSourcePicker={setSourcePickerFor}
                    onAttachExisting={attachExistingSource}
                    onOpenAddSource={openAddSource}
                    onEnterReading={onEnterReading ? enterDangling : undefined}
                    enteringRefId={enteringRefId}
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
  childrenByParent,
  references,
  busyLeadIds,
  pickerFor,
  onOpenPicker,
  onConnect,
  onPrune,
  onDig,
  onAddBranch,
  sourcePickerFor,
  onOpenSourcePicker,
  onAttachExisting,
  onOpenAddSource,
  onEnterReading,
  enteringRefId,
}: {
  ref_: Reference;
  leads: ExplorationLead[];
  childrenByParent: Map<string, ExplorationLead[]>;
  references: Reference[];
  busyLeadIds: Set<string>;
  pickerFor: string | null;
  onOpenPicker: (lid: string | null) => void;
  onConnect: (lid: string, connectedReferenceId: string) => void;
  onPrune: (lid: string) => void;
  onDig: (lead: ExplorationLead) => void;
  onAddBranch: (parentLeadId: string, text: string) => void;
  sourcePickerFor: string | null;
  onOpenSourcePicker: (lid: string | null) => void;
  onAttachExisting: (leadId: string, referenceId: string) => void;
  onOpenAddSource: (leadId: string) => void;
  onEnterReading?: (ref: Reference) => void;
  enteringRefId: string | null;
}) {
  const badge = readingBadge(ref_);
  return (
    <div>
      <div className="flex items-center gap-1.5 rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
        <span className="h-1.5 w-1.5 flex-none rounded-full bg-mk-green" />
        <span className="truncate text-[13px] font-semibold text-mk-ink">{ref_.title}</span>
        {badge && <span className={badge.cls}>{badge.label}</span>}
        {ref_.phaseTag && <span className={PHASE_CHIP}>{ref_.phaseTag}</span>}
        <SourceActions ref_={ref_} entering={enteringRefId === ref_.id} onEnter={onEnterReading ? () => onEnterReading(ref_) : undefined} />
      </div>
      <div className="ml-3 flex flex-col gap-1.5 border-l-2 border-mk-border pl-4 pt-2">
        {leads.map((l) => (
          <LeadWithBranches
            key={l.id}
            lead={l}
            childrenByParent={childrenByParent}
            references={references}
            busyLeadIds={busyLeadIds}
            pickerFor={pickerFor}
            onOpenPicker={onOpenPicker}
            onConnect={onConnect}
            onPrune={onPrune}
            onDig={onDig}
            onAddBranch={onAddBranch}
            sourcePickerFor={sourcePickerFor}
            onOpenSourcePicker={onOpenSourcePicker}
            onAttachExisting={onAttachExisting}
            onOpenAddSource={onOpenAddSource}
            onEnterReading={onEnterReading}
            enteringRefId={enteringRefId}
          />
        ))}
      </div>
    </div>
  );
}

// #12 · a lead plus its 分支, nested recursively. The chip carries the actions;
// beneath it, an indented rail holds child branches and an inline "add a 分支"
// input (toggled from the chip's ＋分支).
function LeadWithBranches({
  lead,
  childrenByParent,
  references,
  busyLeadIds,
  pickerFor,
  onOpenPicker,
  onConnect,
  onPrune,
  onDig,
  onAddBranch,
  sourcePickerFor,
  onOpenSourcePicker,
  onAttachExisting,
  onOpenAddSource,
  onEnterReading,
  enteringRefId,
}: {
  lead: ExplorationLead;
  childrenByParent: Map<string, ExplorationLead[]>;
  references: Reference[];
  busyLeadIds: Set<string>;
  pickerFor: string | null;
  onOpenPicker: (lid: string | null) => void;
  onConnect: (lid: string, refId: string) => void;
  onPrune: (lid: string) => void;
  onDig: (lead: ExplorationLead) => void;
  onAddBranch: (parentLeadId: string, text: string) => void;
  sourcePickerFor: string | null;
  onOpenSourcePicker: (lid: string | null) => void;
  onAttachExisting: (leadId: string, referenceId: string) => void;
  onOpenAddSource: (leadId: string) => void;
  onEnterReading?: (ref: Reference) => void;
  enteringRefId: string | null;
}) {
  const kids = childrenByParent.get(lead.id) ?? [];
  const [adding, setAdding] = useState(false);
  const [text, setText] = useState("");
  const submit = () => {
    const t = text.trim();
    if (!t) return;
    onAddBranch(lead.id, t);
    setText("");
    setAdding(false);
  };
  return (
    <div>
      <BranchLeadChip
        lead={lead}
        references={references}
        busy={busyLeadIds.has(lead.id)}
        pickerOpen={pickerFor === lead.id}
        onOpenPicker={() => onOpenPicker(pickerFor === lead.id ? null : lead.id)}
        onConnect={(refId) => onConnect(lead.id, refId)}
        onPrune={() => onPrune(lead.id)}
        onDig={() => onDig(lead)}
        onAddBranch={lead.status === "open" ? () => setAdding((v) => !v) : undefined}
        sourcePickerOpen={sourcePickerFor === lead.id}
        onOpenSourcePicker={() => onOpenSourcePicker(sourcePickerFor === lead.id ? null : lead.id)}
        onAttachExisting={(refId) => onAttachExisting(lead.id, refId)}
        onAddNewSource={() => onOpenAddSource(lead.id)}
        onEnterReading={onEnterReading}
        entering={enteringRefId != null && lead.connectedReferenceId === enteringRefId}
      />
      {(kids.length > 0 || adding) && (
        <div className="ml-3 mt-1.5 flex flex-col gap-1.5 border-l-2 border-mk-border/70 pl-3">
          {kids.map((k) => (
            <LeadWithBranches
              key={k.id}
              lead={k}
              childrenByParent={childrenByParent}
              references={references}
              busyLeadIds={busyLeadIds}
              pickerFor={pickerFor}
              onOpenPicker={onOpenPicker}
              onConnect={onConnect}
              onPrune={onPrune}
              onDig={onDig}
              onAddBranch={onAddBranch}
              sourcePickerFor={sourcePickerFor}
              onOpenSourcePicker={onOpenSourcePicker}
              onAttachExisting={onAttachExisting}
              onOpenAddSource={onOpenAddSource}
              onEnterReading={onEnterReading}
              enteringRefId={enteringRefId}
            />
          ))}
          {adding && (
            <div className="flex items-center gap-1.5 rounded-mk border border-mk-border bg-mk-surface px-2 py-1">
              <input
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") submit();
                  if (e.key === "Escape") { setAdding(false); setText(""); }
                }}
                autoFocus
                placeholder="这条线索下的一个分支……"
                className="flex-1 bg-transparent text-[12px] text-mk-ink outline-none placeholder:text-mk-muted-2"
              />
              <button type="button" onClick={submit} disabled={!text.trim()} className="flex-none rounded-full bg-mk-primary px-2 py-0.5 text-[11px] font-bold text-white disabled:opacity-50">加</button>
            </div>
          )}
        </div>
      )}
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
  onAddBranch,
  sourcePickerOpen,
  onOpenSourcePicker,
  onAttachExisting,
  onAddNewSource,
  onEnterReading,
  entering,
}: {
  lead: ExplorationLead;
  references: Reference[];
  busy: boolean;
  pickerOpen: boolean;
  onOpenPicker: () => void;
  onConnect: (refId: string) => void;
  onPrune: () => void;
  onDig?: () => void;
  onAddBranch?: () => void;
  sourcePickerOpen: boolean;
  onOpenSourcePicker: () => void;
  onAttachExisting: (refId: string) => void;
  onAddNewSource: () => void;
  onEnterReading?: (ref: Reference) => void;
  entering: boolean;
}) {
  if (lead.status !== "open") {
    return <ResolvedLeadChip lead={lead} references={references} onEnterReading={onEnterReading} entering={entering} />;
  }
  return (
    <div className="relative flex items-center gap-2 rounded-full bg-mk-primary-tint/60 px-3 py-1.5">
      <span className="text-[12.5px] font-semibold text-mk-ink">{lead.text}</span>
      <div className="ml-auto flex flex-none items-center gap-1.5">
        {onAddBranch && (
          <button
            type="button"
            onClick={onAddBranch}
            disabled={busy}
            title="在这条线索下加一个分支"
            className="rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[11px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
          >
            ＋分支
          </button>
        )}
        <button
          type="button"
          onClick={onOpenSourcePicker}
          disabled={busy}
          title="在这条线索下挂一个来源——线索自己保持开放，能继续加"
          className="rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[11px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
        >
          ＋来源
        </button>
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
      {sourcePickerOpen && (
        <AttachSourcePopover references={references} onPickExisting={onAttachExisting} onAddNew={onAddNewSource} onClose={onOpenSourcePicker} />
      )}
    </div>
  );
}

// Connected/pruned leads are resolved — no more actions on the LEAD itself,
// just a marker so the branch still reads honestly (nothing silently
// vanishes once acted on). When it's connected, the attached source is a
// first-class source node too — metadata peek + 进入阅读室 are offered on it
// (Item A #3/#4), same as any other source in the graph.
function ResolvedLeadChip({
  lead,
  references,
  onEnterReading,
  entering,
}: {
  lead: ExplorationLead;
  references: Reference[];
  onEnterReading?: (ref: Reference) => void;
  entering: boolean;
}) {
  if (lead.status === "connected") {
    const target = references.find((r) => r.id === lead.connectedReferenceId);
    return (
      <div className="flex items-center gap-1.5 rounded-full bg-mk-green-tint/60 px-3 py-1.5 text-[12.5px]">
        <span className="font-semibold text-mk-ink line-through decoration-mk-green/40">{lead.text}</span>
        <span className="flex-none rounded-full bg-mk-green px-2 py-0.5 text-[10.5px] font-bold text-white">
          → {target?.title ?? "已连接来源"}
        </span>
        {target && (
          <SourceActions ref_={target} entering={entering} onEnter={onEnterReading ? () => onEnterReading(target) : undefined} />
        )}
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

// A compact info-peek + 进入阅读室 pair — reused on every source node (branched
// source header, an attached-under-lead source, 悬空来源 rows).
function SourceActions({ ref_, entering, onEnter }: { ref_: Reference; entering?: boolean; onEnter?: () => void }) {
  return (
    <span className="ml-auto flex flex-none items-center gap-1.5">
      <SourceMetaButton ref_={ref_} />
      {onEnter && (
        <button
          type="button"
          onClick={onEnter}
          disabled={entering}
          className="rounded-full border border-mk-primary/40 bg-mk-surface px-2 py-0.5 text-[10.5px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
        >
          {entering ? "打开中…" : "进入阅读室"}
        </button>
      )}
    </span>
  );
}

// Item A #3 · a compact metadata peek (title/author-year-journal/abstract),
// reusing the Reference data already loaded via getLibrary — no re-fetch, no
// rebuild of the full 列表 Preview.
function SourceMetaButton({ ref_ }: { ref_: Reference }) {
  const [open, setOpen] = useState(false);
  const hasMeta = Boolean(ref_.author || ref_.year || ref_.journal || ref_.abstract?.trim());
  return (
    <span className="relative inline-flex flex-none">
      <button
        type="button"
        onClick={(e) => { e.stopPropagation(); setOpen((o) => !o); }}
        title="查看来源信息"
        className="flex h-5 w-5 items-center justify-center rounded-full border border-mk-border bg-mk-surface text-[10px] font-bold leading-none text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary"
      >
        ⓘ
      </button>
      {open && (
        <div className="absolute left-0 top-full z-20 mt-1 w-72 rounded-mk border border-mk-border bg-mk-surface p-3 text-left shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
          <div className="mb-1 flex items-start justify-between gap-2">
            <p className="text-[12.5px] font-bold leading-snug text-mk-ink">{ref_.title}</p>
            <button type="button" onClick={() => setOpen(false)} className="flex-none text-[13px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
          </div>
          {hasMeta ? (
            <>
              {(ref_.author || ref_.year || ref_.journal) && (
                <p className="text-[11.5px] text-mk-muted-2">{[ref_.author, ref_.year, ref_.journal].filter(Boolean).join(" · ")}</p>
              )}
              {ref_.abstract?.trim() && (
                <p className="mt-1.5 max-h-32 overflow-y-auto text-[12px] leading-relaxed text-mk-ink">{ref_.abstract}</p>
              )}
            </>
          ) : (
            <p className="text-[11.5px] text-mk-muted-2">暂无更多元信息。</p>
          )}
        </div>
      )}
    </span>
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

// Item A #1 · pick an EXISTING reference to attach as a source UNDER a lead
// (creates+connects a child branch), or jump straight to registering a new
// untracked one. Visually mirrors ConnectPicker but the action underneath is
// different (attach-under, not resolve-the-lead-itself).
function AttachSourcePopover({
  references,
  onPickExisting,
  onAddNew,
  onClose,
}: {
  references: Reference[];
  onPickExisting: (refId: string) => void;
  onAddNew: () => void;
  onClose: () => void;
}) {
  return (
    <div className="absolute left-0 top-full z-20 mt-1 max-h-64 w-64 overflow-y-auto rounded-mk border border-mk-border bg-mk-surface p-1.5 shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
      <div className="flex items-center justify-between px-1.5 pb-1">
        <span className="text-[11px] font-bold uppercase tracking-wider text-mk-muted-2">在这条线索下加一个来源</span>
        <button type="button" onClick={onClose} className="text-[13px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
      </div>
      <button
        type="button"
        onClick={onAddNew}
        className="mb-1 block w-full rounded px-2 py-1.5 text-left text-[12.5px] font-bold text-mk-primary hover:bg-mk-primary-tint"
      >
        ＋ 新来源……
      </button>
      {references.length === 0 ? (
        <p className="px-1.5 py-2 text-[12px] text-mk-muted-2">文献库还没有已有来源可选。</p>
      ) : (
        references.map((r) => (
          <button
            key={r.id}
            type="button"
            onClick={() => onPickExisting(r.id)}
            className="block w-full truncate rounded px-2 py-1.5 text-left text-[12.5px] text-mk-ink hover:bg-mk-primary-tint"
          >
            {r.title}
          </button>
        ))
      )}
    </div>
  );
}

// Item A #2 · register a brand-new untracked source without leaving the
// graph. A COMPACT twin of ReadingBlock's AddSourceModal (title+url only —
// this view doesn't own collections), optionally scoped to a lead.
function AddUntrackedSourceModal({
  forLeadText,
  busy,
  onSubmit,
  onClose,
}: {
  forLeadText: string | null;
  busy: boolean;
  onSubmit: (input: { title: string; url: string }) => void;
  onClose: () => void;
}) {
  const [title, setTitle] = useState("");
  const [url, setUrl] = useState("");
  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-mk-ink/30 px-6" onClick={onClose}>
      <div className="w-[400px] rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-sans text-[15px] font-bold text-mk-ink">添加来源</h3>
          <button type="button" onClick={onClose} className="text-[18px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
        </div>
        {forLeadText && (
          <p className="mb-3 rounded-mk bg-mk-primary-tint/50 px-2.5 py-1.5 text-[12px] font-semibold text-mk-primary">将挂在线索「{forLeadText}」下</p>
        )}
        <div className="space-y-2">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="来源标题"
            className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="链接（可选）"
            className="w-full rounded-mk border border-mk-border bg-mk-input-bg px-3 py-2 text-[13px] text-mk-ink outline-none placeholder:text-mk-muted-2 focus:border-mk-primary"
          />
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-mk border border-mk-border px-4 py-2 text-[13px] font-semibold text-mk-muted hover:text-mk-primary">取消</button>
          <button
            type="button"
            onClick={() => onSubmit({ title: title.trim(), url: url.trim() })}
            disabled={busy || (!title.trim() && !url.trim())}
            className="rounded-mk bg-mk-primary px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
          >
            {busy ? "添加中…" : "添加"}
          </button>
        </div>
      </div>
    </div>
  );
}
