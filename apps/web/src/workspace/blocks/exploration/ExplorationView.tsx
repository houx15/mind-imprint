import { useEffect, useMemo, useState } from "react";
import type {
  CardTurnRef,
  DigCandidate,
  ExplorationLead,
  ExplorationView as ExplorationViewData,
  MaterialSource,
  PhaseTag,
  Reference,
} from "@mind-imprint/contracts";
import { enterReading, NoReadableContentError } from "../../api/workspace";
import { adoptCandidate, createLead, digExploration, getExploration, patchLead } from "../../../api/exploration";
import { DigTray, trayKey } from "./DigTray";

// A6 · "inside-a-hole" exploration view, decluttered to THREE actions:
//   1. 看地图 — one question-rooted tree (top-level question/keyword nodes as
//      roots, adopted papers + sub-questions nested beneath via border-l-2
//      rails). No graph library — CSS indent rails only, per the brief.
//   2. 深挖 — ONE primary control per node → digExploration({leadId}); plus a
//      single root free-text input that CREATES a new top-level question node
//      (it does NOT dig by bare keyword). Digging always targets a node's id;
//      adopting always nests under that node — so a paper is NEVER a root node.
//   3. 印记推荐 → 托盘 → 采纳 — dig results land in the DigTray; adopting is the
//      student's explicit action (铁律①), discarding drops from the tray only.
//
// Everything else the old surface carried (＋添加来源 / ＋兔子洞 / the rabbit-hole
// card sheet + its 从哪条线索挖 select / manual-lead input / ＋分支 / ＋来源 /
// 连接来源 / 深挖这条 / the three 有分支/悬空/待追 sections) is gone or demoted
// into a per-node compact menu. Rendered strings carry NO rabbit metaphor.
//
// Visual language (chips/badges) is still borrowed from ReadingBlock's list view
// so 列表 ⇄ 探索 reads as one room.

const READ_BADGE_DONE = "flex-none rounded px-1.5 py-0.5 text-[10px] font-bold bg-mk-green-tint text-mk-green";
const READ_BADGE_ACTIVE = "flex-none rounded px-1.5 py-0.5 text-[10px] font-bold bg-mk-primary-tint text-mk-primary";

export type ExplorationViewProps = {
  projectId: string;
  references: Reference[];
  // Matches ReadingBlock's setReadingSource signature — passed straight through
  // so a paper node's 进入阅读室 reuses the same swap slot.
  onEnterReading?: (
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
  ) => void;
  // Kept for backward-compatibility with ReadingBlock's call site (A6 removed
  // the surfaces that used these — the ＋添加来源 button, the rabbit-hole card
  // and its coach bridge — but the parent still passes them; a later task
  // reworks ReadingBlock). Unused here on purpose.
  onCreateReference?: (input: { title: string; url?: string }) => Promise<Reference>;
  onCardReflected?: (studentText: string, reply: string, card?: CardTurnRef) => void;
  openRabbitHoleRequested?: boolean;
  onRabbitHoleOpenConsumed?: () => void;
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
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [enteringRefId, setEnteringRefId] = useState<string | null>(null);
  const [actionError, setActionError] = useState(false);

  // Action 2 · the single root input that creates a NEW top-level question node.
  const [newQuestion, setNewQuestion] = useState("");
  const [creatingQuestion, setCreatingQuestion] = useState(false);

  // Action 3 · the dig tray. digTarget names the node we dug from (its id is the
  // parentLeadId every adopt nests under — the papers-never-roots invariant).
  const [tray, setTray] = useState<DigCandidate[]>([]);
  const [digTarget, setDigTarget] = useState<{ leadId: string; text: string } | null>(null);
  const [digging, setDigging] = useState(false);
  const [digError, setDigError] = useState(false);
  const [adopting, setAdopting] = useState<Set<string>>(new Set());

  const refresh = useMemo(
    () => async () => {
      try {
        setView(await getExploration(projectId));
      } catch {
        /* keep the last-good view; a failed refresh must never blank the tree */
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

  function pruneLead(lid: string) {
    setMenuFor(null);
    setActionError(false);
    void withBusy(lid, () => patchLead(projectId, lid, { status: "pruned" })).catch(() => setActionError(true));
  }

  // Action 2 · root input → a brand-new top-level question node (parentLeadId
  // null). NOT a dig: digging always targets an existing node's id.
  async function createQuestion() {
    const text = newQuestion.trim();
    if (!text || creatingQuestion) return;
    setCreatingQuestion(true);
    setActionError(false);
    try {
      await createLead(projectId, text);
      setNewQuestion("");
      await refresh();
    } catch {
      setActionError(true); // keep her draft so she can just retry
    } finally {
      setCreatingQuestion(false);
    }
  }

  // Action 2/3 · dig off ONE node → related papers into the tray. Works the same
  // for a question node and a paper node (the backend picks keyword-search vs
  // related-works from the lead). Never persists — that's 采纳's job.
  async function runDig(lead: ExplorationLead) {
    setMenuFor(null);
    setDigTarget({ leadId: lead.id, text: lead.text });
    setTray([]);
    setDigging(true);
    setDigError(false);
    setAdopting(new Set());
    try {
      const res = await digExploration(projectId, { leadId: lead.id });
      setTray(res.candidates);
    } catch {
      setDigError(true);
    } finally {
      setDigging(false);
    }
  }

  // Action 3 · adopt a candidate UNDER the dug node (parentLeadId = digTarget) so
  // the paper is never a root; then refetch + drop it from the tray. Explicit
  // student action only — 印记 never auto-adopts (铁律①).
  async function adopt(c: DigCandidate) {
    if (!digTarget) return;
    const key = trayKey(c);
    if (adopting.has(key)) return;
    setAdopting((s) => new Set(s).add(key));
    setActionError(false);
    try {
      await adoptCandidate(projectId, c, { parentLeadId: digTarget.leadId });
      setTray((t) => t.filter((x) => trayKey(x) !== key));
      await refresh();
    } catch {
      setActionError(true);
    } finally {
      setAdopting((s) => {
        const n = new Set(s);
        n.delete(key);
        return n;
      });
    }
  }

  function discard(c: DigCandidate) {
    const key = trayKey(c);
    setTray((t) => t.filter((x) => trayKey(x) !== key));
  }

  function closeTray() {
    setDigTarget(null);
    setTray([]);
    setDigError(false);
  }

  async function enterSource(ref: Reference) {
    if (!onEnterReading || enteringRefId) return;
    setMenuFor(null);
    setEnteringRefId(ref.id);
    try {
      const { source, suggestedReason } = await enterReading(projectId, ref.id);
      onEnterReading(source, ref.id, suggestedReason, ref.phaseTag, ref.readingReason, ref.readingFocus);
    } catch (e) {
      // NoReadableContentError has a paste fallback in the Library preview — this
      // view just nudges toward the Library rather than duplicating it.
      void (e instanceof NoReadableContentError);
    } finally {
      setEnteringRefId(null);
    }
  }

  // Papers + sub-questions nest under their parent via parentLeadId. Index
  // children by parent for the recursive render; roots are parentLeadId == null.
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
  const roots = useMemo(() => view.leads.filter((l) => l.parentLeadId == null), [view.leads]);

  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载探索图谱中…</div>;
  }

  return (
    <div className="relative flex min-h-0 flex-1 flex-col bg-mk-bg/40">
      <div className="flex-1 overflow-y-auto px-6 py-5">
        <div className="mb-4">
          <h2 className="font-sans text-[16px] font-bold text-mk-ink">探索图谱</h2>
          <p className="mt-0.5 text-[12px] text-mk-muted-2">
            从一个你想弄清楚的问题出发，让印记帮你往下挖相关论文——一眼看全这条线怎么长出来的
          </p>
        </div>

        {/* Action 2 · the single root input — creates a top-level question node. */}
        <div className="mb-5 flex items-center gap-2 rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
          <input
            value={newQuestion}
            onChange={(e) => setNewQuestion(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") createQuestion();
            }}
            placeholder="记一个你想弄清楚的问题……（比如「中国碳排放全球第一，这跟可持续矛盾吗？」）"
            className="flex-1 bg-transparent text-[12.5px] text-mk-ink outline-none placeholder:text-mk-muted-2"
          />
          <button
            type="button"
            onClick={createQuestion}
            disabled={creatingQuestion || !newQuestion.trim()}
            className="flex-none rounded-mk bg-mk-primary px-3 py-1.5 text-[12px] font-bold text-white hover:bg-mk-primary-hover disabled:opacity-60"
          >
            {creatingQuestion ? "记录中…" : "记下问题"}
          </button>
        </div>

        {actionError && <p className="mb-3 text-[12px] font-semibold text-mk-accent">刚才那步没接上，再试一次？</p>}

        {roots.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center rounded-mk-lg border border-dashed border-mk-border bg-mk-surface px-6 py-10 text-center">
            <p className="text-[13.5px] font-bold text-mk-ink">这里还是空的</p>
            <p className="mt-1.5 max-w-sm text-[12.5px] leading-relaxed text-mk-muted">
              在上面记下一个你想弄清楚的问题，再点它的「深挖」——印记就会顺着它给你几篇相关论文，采纳的会挂到这条线下面，慢慢长成一张图。
            </p>
          </div>
        ) : (
          <div className="flex flex-col gap-2.5">
            {roots.map((l) => (
              <LeadNode
                key={l.id}
                lead={l}
                childrenByParent={childrenByParent}
                references={references}
                busyLeadIds={busyLeadIds}
                menuFor={menuFor}
                onOpenMenu={(id) => setMenuFor(id)}
                onDig={runDig}
                onPrune={pruneLead}
                onEnterReading={onEnterReading ? enterSource : undefined}
                enteringRefId={enteringRefId}
              />
            ))}
          </div>
        )}
      </div>

      {/* Action 3 · the tray, docked at the bottom while the tree scrolls above. */}
      {digTarget && (
        <DigTray
          targetText={digTarget.text}
          candidates={tray}
          digging={digging}
          error={digError}
          adopting={adopting}
          onAdopt={adopt}
          onDiscard={discard}
          onClose={closeTray}
        />
      )}
    </div>
  );
}

/* ---------- one node + its nested children (papers / sub-questions) ---------- */

// A node renders recursively: its chip carries the ONE primary 深挖 + a compact
// menu; beneath it a border-l-2 rail holds children. (Parent prune/delete
// cascades server-side, so no orphaned children.)
function LeadNode({
  lead,
  childrenByParent,
  references,
  busyLeadIds,
  menuFor,
  onOpenMenu,
  onDig,
  onPrune,
  onEnterReading,
  enteringRefId,
}: {
  lead: ExplorationLead;
  childrenByParent: Map<string, ExplorationLead[]>;
  references: Reference[];
  busyLeadIds: Set<string>;
  menuFor: string | null;
  onOpenMenu: (id: string | null) => void;
  onDig: (lead: ExplorationLead) => void;
  onPrune: (lid: string) => void;
  onEnterReading?: (ref: Reference) => void;
  enteringRefId: string | null;
}) {
  const kids = childrenByParent.get(lead.id) ?? [];
  return (
    <div>
      <NodeChip
        lead={lead}
        references={references}
        busy={busyLeadIds.has(lead.id)}
        menuOpen={menuFor === lead.id}
        onOpenMenu={() => onOpenMenu(menuFor === lead.id ? null : lead.id)}
        onDig={() => onDig(lead)}
        onPrune={() => onPrune(lead.id)}
        onEnterReading={onEnterReading}
        entering={enteringRefId != null && lead.connectedReferenceId === enteringRefId}
      />
      {kids.length > 0 && (
        <div className="ml-3 mt-1.5 flex flex-col gap-1.5 border-l-2 border-mk-border/70 pl-3">
          {kids.map((k) => (
            <LeadNode
              key={k.id}
              lead={k}
              childrenByParent={childrenByParent}
              references={references}
              busyLeadIds={busyLeadIds}
              menuFor={menuFor}
              onOpenMenu={onOpenMenu}
              onDig={onDig}
              onPrune={onPrune}
              onEnterReading={onEnterReading}
              enteringRefId={enteringRefId}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// The chip: a question node (open), a paper node (connected → its Reference), or
// a pruned marker. ONE primary 深挖 (except when pruned) + a compact ⋯ menu that
// holds 进入阅读室 / 来源信息 / 剪枝. No jargon in any label.
function NodeChip({
  lead,
  references,
  busy,
  menuOpen,
  onOpenMenu,
  onDig,
  onPrune,
  onEnterReading,
  entering,
}: {
  lead: ExplorationLead;
  references: Reference[];
  busy: boolean;
  menuOpen: boolean;
  onOpenMenu: () => void;
  onDig: () => void;
  onPrune: () => void;
  onEnterReading?: (ref: Reference) => void;
  entering: boolean;
}) {
  const [metaOpen, setMetaOpen] = useState(false);

  if (lead.status === "pruned") {
    return (
      <div className="flex items-center gap-1.5 rounded-full bg-mk-bg px-3 py-1.5 text-[12.5px]">
        <span className="font-semibold text-mk-muted-2 line-through">{lead.text}</span>
        <span className="flex-none rounded-full bg-mk-muted px-2 py-0.5 text-[10.5px] font-bold text-white">已剪枝</span>
      </div>
    );
  }

  const source = lead.connectedReferenceId ? references.find((r) => r.id === lead.connectedReferenceId) ?? null : null;
  const connected = lead.status === "connected" && source != null;
  const hasMeta = source ? Boolean(source.author || source.year || source.journal || source.abstract?.trim()) : false;
  const chipTone = connected ? "bg-mk-green-tint/60" : "bg-mk-primary-tint/60";

  return (
    <div className={`relative flex items-center gap-2 rounded-full ${chipTone} px-3 py-1.5`}>
      <span className={`text-[12.5px] font-semibold ${connected ? "text-mk-ink" : "text-mk-ink"}`}>{lead.text}</span>
      {connected && source && (
        <span className="flex-none truncate rounded-full bg-mk-green px-2 py-0.5 text-[10.5px] font-bold text-white">
          论文 · {source.title}
        </span>
      )}
      <div className="ml-auto flex flex-none items-center gap-1.5">
        <button
          type="button"
          onClick={onDig}
          disabled={busy}
          title="让印记顺着这条给你几篇相关论文"
          className="rounded-full border border-mk-primary/40 bg-mk-surface px-2.5 py-0.5 text-[11px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-50"
        >
          深挖
        </button>
        <button
          type="button"
          onClick={onOpenMenu}
          disabled={busy}
          aria-label="更多"
          className="flex h-6 w-6 items-center justify-center rounded-full border border-mk-border bg-mk-surface text-[13px] font-bold leading-none text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary disabled:opacity-50"
        >
          ⋯
        </button>
      </div>

      {menuOpen && (
        <div className="absolute right-0 top-full z-20 mt-1 w-44 rounded-mk border border-mk-border bg-mk-surface p-1 text-left shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
          {connected && source && onEnterReading && (
            <button
              type="button"
              onClick={() => onEnterReading(source)}
              disabled={entering}
              className="block w-full rounded px-2.5 py-1.5 text-left text-[12.5px] font-semibold text-mk-primary hover:bg-mk-primary-tint disabled:opacity-50"
            >
              {entering ? "打开中…" : "进入阅读室"}
            </button>
          )}
          {connected && hasMeta && source && (
            <button
              type="button"
              onClick={() => setMetaOpen((o) => !o)}
              className="block w-full rounded px-2.5 py-1.5 text-left text-[12.5px] text-mk-ink hover:bg-mk-bg"
            >
              来源信息
            </button>
          )}
          {metaOpen && source && (
            <div className="mx-1 my-1 rounded-mk border border-mk-border bg-mk-bg/60 p-2">
              {(source.author || source.year || source.journal) && (
                <p className="text-[11.5px] text-mk-muted-2">
                  {[source.author, source.year, source.journal].filter(Boolean).join(" · ")}
                </p>
              )}
              {source.abstract?.trim() && (
                <p className="mt-1 max-h-32 overflow-y-auto text-[12px] leading-relaxed text-mk-ink">{source.abstract}</p>
              )}
              {!source.author && !source.year && !source.journal && !source.abstract?.trim() && (
                <p className="text-[11.5px] text-mk-muted-2">暂无更多元信息。</p>
              )}
            </div>
          )}
          <button
            type="button"
            onClick={onPrune}
            disabled={busy}
            className="block w-full rounded px-2.5 py-1.5 text-left text-[12.5px] font-semibold text-mk-muted hover:bg-mk-bg hover:text-mk-accent disabled:opacity-50"
          >
            剪枝
          </button>
        </div>
      )}
    </div>
  );
}
