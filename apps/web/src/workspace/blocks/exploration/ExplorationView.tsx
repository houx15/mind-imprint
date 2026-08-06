import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
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
import type { QuestionEdgeLabel } from "@mind-imprint/contracts";
import {
  adoptCandidate,
  createEdge,
  createLead,
  deleteEdge,
  deleteLead,
  digExploration,
  getExploration,
  patchEdge,
  proposeEdges,
} from "../../../api/exploration";
import { ExplorationSidebar, candidateKey, type DigMode, type PaperInList } from "./ExplorationSidebar";
import { QuestionMindmap } from "./QuestionMindmap";
import { WarrenMap } from "./WarrenMap";
import { countPapersByRoot } from "./warrenLayout";

// B4a · which zoom the student last left this project on. Persisted module-side
// (like ReadingBlock's viewModeMemo) so re-entering the room restores map ⇄ the
// question she was inside. "map" = the Level-1 overview graph of root questions
// (WarrenMap); "hole" = one question's subtree (GVb's React Flow mindmap +
// right sidebar), scoped to focusRootId.
type ZoomState = { mode: "map" | "hole"; focusRootId: string | null };
const zoomMemo = new Map<string, ZoomState>();

// GVb · "钻进一个洞" (Level-2): the focused question as a React Flow MINDMAP
// (QuestionMindmap) + a right sidebar (NodeSidebar). Clicking a node selects it;
// the sidebar shows its metadata + two search actions ("找相似文献" →
// digExploration({leadId}); a keyword box → digExploration({keyword})). Dig
// results are suggestions in the sidebar — a candidate only joins the mindmap
// when the student taps 采纳 (铁律①), which always nests it UNDER the node the
// dig launched from (parentLeadId = that node) so a paper is NEVER a root.
// × on a node → a confirm modal → deleteLead. No bottom tray, no 剪枝.

export type ExplorationViewProps = {
  projectId: string;
  references: Reference[];
  // GVf · the project's title, threaded down from ReadingBlock/WorkspaceContainer
  // (WorkspaceProjection.title). There is no dedicated "research question" field
  // reachable from the client (the old S1 立题 studio framing is dead in the new
  // four-room workspace), so the project title IS the driving-question fallback
  // the brief calls for. Optional so existing callers/tests without it still
  // render the plain empty state (current behavior).
  projectTitle?: string;
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
  // Reload the library (ReadingBlock's getLibrary). 采纳 creates a NEW reference
  // server-side; without reloading, that reference isn't in `references`, so a
  // freshly-adopted paper node has no bib to show (every field renders 「—」).
  onLibraryChanged?: () => void;
  onCardReflected?: (studentText: string, reply: string, card?: CardTurnRef) => void;
  // GVd · the 印记·找资料 coach, rendered as the unified right sidebar's DEFAULT
  // ('ai') state. Owned by ReadingBlock (it holds the coach's chat/link/card
  // state); this view just docks it into the sidebar so there's ONE right panel,
  // not a separate coach column beside a node sidebar.
  coach?: ReactNode;
};

export function ExplorationView({ projectId, references, projectTitle, onEnterReading, onLibraryChanged, coach }: ExplorationViewProps) {
  const [view, setView] = useState<ExplorationViewData>({ leads: [], danglingSourceIds: [], edges: [] });
  const [loading, setLoading] = useState(true);
  const [busyLeadIds, setBusyLeadIds] = useState<Set<string>>(new Set());
  const [enteringRefId, setEnteringRefId] = useState<string | null>(null);
  const [actionError, setActionError] = useState(false);

  // GVb · which node inside the focused question's mindmap is selected → the
  // sidebar's metadata + search target. Zooming in selects the root question.
  const [selectedId, setSelectedId] = useState<string | null>(null);

  // GVb · sidebar dig state. `digFromId` is the node the dig launched from — its
  // id is the parentLeadId every 采纳 nests under (the papers-never-roots
  // invariant). Clearing it on selection change keeps results tied to their node.
  const [tray, setTray] = useState<DigCandidate[]>([]);
  const [digFromId, setDigFromId] = useState<string | null>(null);
  const [digging, setDigging] = useState(false);
  const [digError, setDigError] = useState(false);
  const [adopting, setAdopting] = useState<Set<string>>(new Set());

  function resetDig() {
    setTray([]);
    setDigFromId(null);
    setDigging(false);
    setDigError(false);
    setAdopting(new Set());
  }

  // B4a · two-level zoom. Restore where she was; default to the map overview.
  const [zoom, setZoom] = useState<ZoomState>(() => zoomMemo.get(projectId) ?? { mode: "map", focusRootId: null });
  function goZoom(next: ZoomState) {
    zoomMemo.set(projectId, next);
    setZoom(next);
  }
  // GVd · Zoom into a question → the sidebar stays on the coach ('ai') until she
  // clicks a node inside (no auto-select). Level-1 → Level-2 both default to 'ai'.
  const zoomInto = (rootId: string) => {
    resetDig();
    setSelectedId(null);
    goZoom({ mode: "hole", focusRootId: rootId });
  };
  const backToMap = () => {
    resetDig();
    setSelectedId(null);
    goZoom({ mode: "map", focusRootId: null });
  };
  // Pick a node in the mindmap → its sidebar ('node'); a fresh selection clears
  // the previous node's dig results.
  const selectNode = (id: string) => {
    resetDig();
    setSelectedId(id);
  };
  // GVd · "← 印记" from 'node'/'results' → back to the coach: deselect + clear dig.
  const deselect = () => {
    resetDig();
    setSelectedId(null);
  };

  // Action 2 · the single root input that creates a NEW top-level question node.
  const [newQuestion, setNewQuestion] = useState("");
  const [creatingQuestion, setCreatingQuestion] = useState(false);

  // GVf · the note-promotion picker: a small dropdown near the create input
  // listing reading notes the student can turn into a question.
  const [notePickerOpen, setNotePickerOpen] = useState(false);

  // B4b · 印记-proposed labeled edges. `proposing` gates the propose button while
  // the LLM runs; `proposeNote` gently surfaces a zero-result run. `busyEdgeIds`
  // disables an edge's controls mid-mutation. Nothing here auto-confirms (铁律②).
  const [proposing, setProposing] = useState(false);
  const [proposeNote, setProposeNote] = useState<string | null>(null);
  const [busyEdgeIds, setBusyEdgeIds] = useState<Set<string>>(new Set());

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

  // GVf · promote a reading note into a root question. sourceReferenceId lets
  // the server attach the note's reference + set origin "note" (C1) so the
  // question carries its provenance. Student-confirmed: she picks the note from
  // the list herself (铁律①) — nothing here runs without that click.
  async function createFromNote(ref: Reference) {
    const text = (ref.readingNote ?? "").trim();
    if (!text) return;
    setNotePickerOpen(false);
    setActionError(false);
    try {
      await createLead(projectId, text, { sourceReferenceId: ref.id });
      await refresh();
    } catch {
      setActionError(true);
    }
  }

  // GVb · dig off the SELECTED node into the sidebar. "找相似文献" digs by the
  // node's id ({leadId}); the keyword box digs by a fresh term ({keyword}).
  // Either way the results hang under the SELECTED node when adopted, so a paper
  // is never a root. Never persists — that's 采纳's job.
  async function runDig(opts: { leadId?: string; keyword?: string; mode?: DigMode }) {
    if (!selectedId) return;
    setDigFromId(selectedId);
    setTray([]);
    setDigging(true);
    setDigError(false);
    setAdopting(new Set());
    try {
      const res = await digExploration(projectId, opts);
      setTray(res.candidates);
    } catch {
      setDigError(true);
    } finally {
      setDigging(false);
    }
  }
  // GVd · a node's find-actions: dig by the selected node's id in one of the
  // three OpenAlex modes (相似 / 它引用的 / 引用它的). Adopting lands the paper
  // under this node (papers-never-roots).
  const digFromNode = (mode: DigMode) => selectedId && runDig({ leadId: selectedId, mode });
  const keywordSearch = (keyword: string) => runDig({ keyword, mode: "similar" });

  // GVb · adopt a candidate UNDER the dug node (parentLeadId = digFromId) so the
  // paper is never a root; then refetch + drop it from the sidebar list. Explicit
  // student action only — 印记 never auto-adopts (铁律①).
  async function adopt(c: DigCandidate) {
    if (!digFromId) return;
    const key = candidateKey(c);
    if (adopting.has(key)) return;
    setAdopting((s) => new Set(s).add(key));
    setActionError(false);
    try {
      await adoptCandidate(projectId, c, { parentLeadId: digFromId });
      setTray((t) => t.filter((x) => candidateKey(x) !== key));
      await refresh();
      // Pull the freshly-created reference into `references` so its bib
      // (author/year/journal/abstract/url) is there when the student opens the
      // new paper node — otherwise every metadata field would read 「—」.
      onLibraryChanged?.();
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
    const key = candidateKey(c);
    setTray((t) => t.filter((x) => candidateKey(x) !== key));
  }

  // B4b · ask 印记 to propose labeled edges between root questions, then refetch
  // so the new (dashed, status:"proposed") edges appear. A zero-result run gets a
  // gentle inline note rather than an error. 印记 proposes; the student confirms.
  async function proposeRelations() {
    if (proposing) return;
    setProposing(true);
    setProposeNote(null);
    setActionError(false);
    try {
      const proposed = await proposeEdges(projectId);
      await refresh();
      if (proposed.length === 0) setProposeNote("印记暂时没找到新的关系，先接着挖挖看。");
    } catch {
      setActionError(true);
    } finally {
      setProposing(false);
    }
  }

  // B4b · a single edge mutation + refetch, guarded by busyEdgeIds so its chip's
  // controls disable while the round-trip runs. Confirm/relabel go through
  // patchEdge; dismiss/delete through deleteEdge. Always refetch → server truth.
  async function withEdgeBusy(eid: string, fn: () => Promise<unknown>) {
    setProposeNote(null);
    setActionError(false);
    setBusyEdgeIds((s) => new Set(s).add(eid));
    try {
      await fn();
      await refresh();
    } catch {
      setActionError(true);
    } finally {
      setBusyEdgeIds((s) => {
        const n = new Set(s);
        n.delete(eid);
        return n;
      });
    }
  }
  const confirmEdge = (eid: string) => void withEdgeBusy(eid, () => patchEdge(projectId, eid, { status: "confirmed" }));
  const dismissEdge = (eid: string) => void withEdgeBusy(eid, () => deleteEdge(projectId, eid));
  const relabelEdge = (eid: string, label: QuestionEdgeLabel) =>
    void withEdgeBusy(eid, () => patchEdge(projectId, eid, { label }));

  // GVa · a student-drawn relation (dragged node-to-node on the map). createEdge
  // lands confirmed server-side (it's the student's own action), then refetch so
  // the new solid edge appears. A brand-new edge has no id yet → no busy guard.
  async function createRelation(fromLeadId: string, toLeadId: string, label: QuestionEdgeLabel) {
    setActionError(false);
    try {
      await createEdge(projectId, { fromLeadId, toLeadId, label });
      await refresh();
    } catch {
      setActionError(true);
    }
  }
  // GVa · × on a map node → deleteLead (children cascade server-side), then refetch.
  const removeRoot = (leadId: string) =>
    void withBusy(leadId, () => deleteLead(projectId, leadId)).catch(() => setActionError(true));

  // GVb · × on a mindmap node (Level-2) → deleteLead + refetch. If the deleted
  // node was selected, fall the sidebar back to the focused root (or, if the root
  // itself was deleted, the map fallback below takes over).
  const removeNode = (leadId: string) =>
    void withBusy(leadId, () => deleteLead(projectId, leadId))
      .then(() => {
        // Deleting the selected node falls the sidebar back to the coach ('ai').
        if (leadId === selectedId) deselect();
      })
      .catch(() => setActionError(true));

  async function enterSource(ref: Reference) {
    if (!onEnterReading || enteringRefId) return;
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

  const roots = useMemo(() => view.leads.filter((l) => l.parentLeadId == null), [view.leads]);

  // GVf · the driving-question seed (empty state) — see projectTitle's doc
  // comment above for why the project title is the fallback source.
  const drivingQuestion = useMemo(() => (projectTitle ?? "").trim(), [projectTitle]);

  // GVf · reading notes eligible for promotion into a question — references
  // whose readingNote is a non-empty string.
  const notesWithText = useMemo(
    () => references.filter((r) => (r.readingNote ?? "").trim().length > 0),
    [references],
  );

  // GVa map data. countByRoot = "文献 x 篇" — descendant PAPERS (adopted leads
  // carrying a connectedReferenceId) under each root, not raw descendant count.
  const countByRoot = useMemo(() => countPapersByRoot(view.leads), [view.leads]);

  // The root the student is currently zoomed into (hole mode). If it vanished
  // (deleted elsewhere), fall back to the map rather than a blank subtree.
  const focusRoot = zoom.mode === "hole" ? roots.find((r) => r.id === zoom.focusRootId) ?? null : null;
  const inHole = zoom.mode === "hole" && focusRoot != null;

  // GVb · the selected mindmap node (hole mode) + its joined reference (when a
  // paper) → the sidebar's metadata + search target.
  const selectedLead = inHole ? view.leads.find((l) => l.id === selectedId) ?? null : null;
  const selectedRef =
    selectedLead?.connectedReferenceId != null
      ? references.find((r) => r.id === selectedLead.connectedReferenceId) ?? null
      : null;

  // GVd · a selected QUESTION node's 论文列表 — its descendant leads carrying a
  // connectedReferenceId, projected to {id,title} (title from the joined
  // reference, falling back to the lead's own text). Clicking one selects that
  // paper node. Papers can nest (a paper's finds adopt under it), so walk the
  // whole subtree, not just direct children.
  const selectedQuestionPapers = useMemo<PaperInList[]>(() => {
    if (!selectedLead || selectedLead.connectedReferenceId != null) return [];
    const childrenOf = new Map<string, ExplorationLead[]>();
    for (const l of view.leads) {
      if (l.parentLeadId) childrenOf.set(l.parentLeadId, [...(childrenOf.get(l.parentLeadId) ?? []), l]);
    }
    const out: PaperInList[] = [];
    const stack = [...(childrenOf.get(selectedLead.id) ?? [])];
    while (stack.length) {
      const l = stack.pop()!;
      if (l.connectedReferenceId != null) {
        const ref = references.find((r) => r.id === l.connectedReferenceId);
        out.push({ id: l.id, title: ref?.title ?? l.text });
      }
      stack.push(...(childrenOf.get(l.id) ?? []));
    }
    return out;
  }, [selectedLead, view.leads, references]);

  // GVd · the unified sidebar's state. 'ai' whenever nothing is selected (Level-1
  // always, Level-2 until a node is clicked); 'results' once a dig launched from
  // the selected node (digFromId set); else 'node'.
  const sidebarState: "ai" | "node" | "results" = selectedId == null ? "ai" : digFromId != null ? "results" : "node";

  const sidebar = (
    <ExplorationSidebar
      state={sidebarState}
      coach={coach}
      node={selectedLead}
      reference={selectedRef}
      papers={selectedQuestionPapers}
      onSelectPaper={selectNode}
      digging={digging}
      digError={digError}
      candidates={tray}
      adopting={adopting}
      onDig={digFromNode}
      onKeywordSearch={keywordSearch}
      onAdopt={adopt}
      onDiscard={discard}
      onBackToAi={deselect}
      onBackToNode={resetDig}
      onEnterReading={onEnterReading && selectedRef ? () => enterSource(selectedRef) : undefined}
      entering={enteringRefId != null && enteringRefId === selectedRef?.id}
    />
  );

  if (loading) {
    return <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载探索图谱中…</div>;
  }

  if (inHole) {
    /* ---------- HOLE (Level-2) · one question's mindmap + right sidebar ---------- */
    return (
      <div className="relative flex h-full min-h-0 bg-mk-bg/40">
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-4 py-2.5">
            <button
              type="button"
              onClick={backToMap}
              className="flex-none rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-primary hover:border-mk-primary hover:bg-mk-primary/10"
            >
              ← 返回兔子洞地图
            </button>
            <h2 className="min-w-0 truncate font-sans text-[14px] font-bold text-mk-ink">{focusRoot!.text}</h2>
          </div>
          {actionError && (
            <p className="flex-none border-b border-mk-border bg-mk-surface px-4 py-2 text-[12px] font-semibold text-mk-accent">
              刚才那步没接上，再试一次？
            </p>
          )}
          <div className="min-h-0 flex-1">
            <QuestionMindmap
              projectId={projectId}
              root={focusRoot!}
              leads={view.leads}
              selectedId={selectedId}
              onSelect={selectNode}
              onDeleteLead={removeNode}
            />
          </div>
        </div>
        {sidebar}
      </div>
    );
  }

  /* ---------- MAP (Level-1) · the overview graph of root questions + sidebar (always 'ai') ---------- */
  return (
    <div className="relative flex h-full min-h-0 bg-mk-bg/40">
      <div className="relative flex min-h-0 flex-1 flex-col">
      {/* Controls stay a fixed header; the map below fills the remaining height
          (so it grows with the viewport / full-screen mode instead of sitting in
          a short fixed box). */}
      <div className="flex-none px-6 pt-5 pb-3">
        {/* Action 2 · the single root input — creates a top-level question node.
            The map's own title (兔子洞地图 + its ? explainer) lives inside WarrenMap,
            so no separate section heading here. */}
        <div className="flex items-center gap-2 rounded-mk border border-mk-border bg-mk-surface px-3 py-2">
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

        {/* GVf · 从笔记新建问题 — a plain-copy affordance near the create input that
            opens a small picker of the project's reading notes (references whose
            readingNote is set). Picking one promotes it into a root question,
            carrying its source via sourceReferenceId (origin becomes "note"). */}
        <div className="relative mt-2.5">
          <button
            type="button"
            onClick={() => setNotePickerOpen((o) => !o)}
            className="rounded-full border border-mk-border bg-mk-surface px-3 py-1.5 text-[12px] font-bold text-mk-primary hover:border-mk-primary hover:bg-mk-primary/10"
          >
            从笔记新建问题
          </button>
          {notePickerOpen && (
            <div className="absolute left-0 top-full z-30 mt-1 w-80 rounded-mk border border-mk-border bg-mk-surface p-2 shadow-[0_12px_32px_rgba(28,35,51,0.18)]">
              {notesWithText.length === 0 ? (
                <p className="px-2 py-2 text-[12px] text-mk-muted-2">还没有阅读笔记</p>
              ) : (
                notesWithText.map((ref) => (
                  <button
                    key={ref.id}
                    type="button"
                    onClick={() => void createFromNote(ref)}
                    className="block w-full rounded px-2 py-1.5 text-left hover:bg-mk-primary-tint"
                  >
                    <p className="line-clamp-2 text-[12px] font-semibold leading-snug text-mk-ink">
                      {(ref.readingNote ?? "").trim()}
                    </p>
                    <p className="mt-0.5 truncate text-[11px] text-mk-muted-2">{ref.title}</p>
                  </button>
                ))
              )}
            </div>
          )}
        </div>

        {actionError && <p className="mt-2.5 text-[12px] font-semibold text-mk-accent">刚才那步没接上，再试一次？</p>}

        {/* B4b · let 印记 propose relationships between the questions. Lives in the
            fixed header so the map body below is pure canvas. Only meaningful with
            ≥2 root questions (an edge needs two ends). */}
        {roots.length >= 2 && (
          <div className="mt-2.5 flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={proposeRelations}
              disabled={proposing}
              className="rounded-full border border-mk-primary/40 bg-mk-surface px-3 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary/10 disabled:opacity-60"
            >
              {proposing ? "印记在找关系…" : "让印记找找问题之间的关系"}
            </button>
            {proposeNote && <span className="text-[12px] text-mk-muted-2">{proposeNote}</span>}
          </div>
        )}
      </div>

      {/* Map body — fills the remaining height so the graph grows with the room
          (and with full-screen mode), instead of sitting in a short fixed box. */}
      <div className="min-h-0 flex-1 px-6 pb-5">
        {roots.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center rounded-mk-lg border border-dashed border-mk-border bg-mk-surface px-6 py-10 text-center">
            <p className="text-[13.5px] font-bold text-mk-ink">这里还是空的</p>
            <p className="mt-1.5 max-w-sm text-[12.5px] leading-relaxed text-mk-muted">
              在上面记下一个你想弄清楚的问题，点开它再「深挖」——印记就会顺着它给你几篇相关论文，采纳的会挂到这条线下面，慢慢长成一张图。
            </p>
            {/* GVf · the driving-question seed: pre-fill (not auto-create) the
                create input with the project's own research question so the
                student's first question isn't a blank page. She still has to
                confirm/edit and click 记下问题 herself (铁律①). */}
            {drivingQuestion && (
              <button
                type="button"
                onClick={() => setNewQuestion(drivingQuestion)}
                className="mt-3 rounded-full border border-mk-primary/40 bg-mk-surface px-3 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary/10"
              >
                用我的研究问题开始
              </button>
            )}
          </div>
        ) : (
          <WarrenMap
            projectId={projectId}
            roots={roots}
            countByRoot={countByRoot}
            edges={view.edges}
            onZoom={zoomInto}
            busyEdgeIds={busyEdgeIds}
            onConfirmEdge={confirmEdge}
            onDismissEdge={dismissEdge}
            onRelabelEdge={relabelEdge}
            onCreateEdge={createRelation}
            onDeleteLead={removeRoot}
          />
        )}
      </div>
      </div>
      {sidebar}
    </div>
  );
}
