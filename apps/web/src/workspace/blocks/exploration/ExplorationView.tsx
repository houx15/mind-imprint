import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { EmptyState } from "@/ui/Illustration";
import type {
  CardTurnRef,
  DigCandidate,
  ExplorationLead,
  ExplorationView as ExplorationViewData,
  MaterialSource,
  PhaseTag,
  Reference,
} from "@mind-imprint/contracts";
import { enterReading, pasteContent, NoReadableContentError, type SourceMeta } from "../../api/workspace";
import { setReferenceEvidence, setReferenceTriage, archiveReference } from "@/api/evidenceMap";
import type { QuestionEdgeLabel } from "@mind-imprint/contracts";
import {
  adoptCandidate,
  attachReference,
  createEdge,
  deleteEdge,
  deleteLead,
  digExploration,
  getExploration,
  patchEdge,
  proposeEdges,
} from "../../../api/exploration";
import { ExplorationSidebar, candidateKey, type DigMode, type PaperInList } from "./ExplorationSidebar";
import { PlacementPicker, type PlacementQuestion } from "./PlacementPicker";
import { QuestionMindmap } from "./QuestionMindmap";
import { WarrenMap } from "./WarrenMap";
import { countPapersByRoot } from "./warrenLayout";
import { RabbitHoleLoader } from "@/ui";
import { SubagentHint } from "@/studio/ai/SubagentHint";
import { SearchGuidanceBox } from "./SearchGuidanceBox";
import { ExplorationReviewBox } from "./ExplorationReviewBox";
import { NeedsResourcesBox } from "../NeedsResourcesBox";

// B4a · which zoom the student last left this project on. Persisted module-side
// (like ReadingBlock's viewModeMemo) so re-entering the room restores map ⇄ the
// question she was inside. "map" = the Level-1 overview graph of root questions
// (WarrenMap); "hole" = one question's subtree (GVb's React Flow mindmap +
// right sidebar), scoped to focusRootId.
type ZoomState = { mode: "map" | "hole" | "unfiled"; focusRootId: string | null };
const zoomMemo = new Map<string, ZoomState>();

// 未归类 = references with no NON-PRUNED connected lead (read or unread — the
// whole point is faithfulness), excluding archived ones. Computed client-side;
// the server's danglingSourceIds (read-but-unfollowed) is a different, sharper set.
export function unfiledReferences(references: Reference[], leads: ExplorationLead[]): Reference[] {
  const attached = new Set<string>();
  for (const l of leads) {
    if (l.status !== "pruned" && l.connectedReferenceId) attached.add(l.connectedReferenceId);
  }
  return references.filter((r) => !(r.archived ?? false) && !attached.has(r.id));
}

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
  // Task 8 (P2b) · bumped by WorkspaceContainer's `explorationRefreshNonce`
  // whenever 印记 proposes a question and the student confirms it into an
  // exploration lead (createLead now lives in the container's confirmQuestion,
  // not here — the manual question boxes this view used to own are gone). A
  // bump re-fetches the graph so the new root question appears without a
  // manual reload. Starts at 0; the mount effect above already fetches once,
  // so only a BUMP (not the initial render) should trigger another fetch.
  refreshNonce?: number;
  // Which edge the 印记 chat panel sits on (container-level). The reading room's
  // aux controls dock on the OPPOSITE edge so they never crowd/overlap the chat
  // — and so the graph stays the centered focal element (user request). Defaults
  // to "left" (the historical chat side) → controls on the right.
  aiSide?: "left" | "right";
};

export function ExplorationView({
  projectId,
  references,
  onEnterReading,
  onCreateReference,
  onLibraryChanged,
  coach,
  refreshNonce,
  aiSide = "left",
}: ExplorationViewProps) {
  const [view, setView] = useState<ExplorationViewData>({ leads: [], danglingSourceIds: [], edges: [] });
  const [loading, setLoading] = useState(true);
  const [busyLeadIds, setBusyLeadIds] = useState<Set<string>>(new Set());
  const [enteringRefId, setEnteringRefId] = useState<string | null>(null);
  const [actionError, setActionError] = useState(false);
  // When enter-reading 422s (paper's full text can't be fetched — most paywalled
  // papers), we don't dead-click: carry the friendly message + any recovered DOI
  // metadata so the sidebar shows an inline paste box, mirroring the Library.
  const [pasteFor, setPasteFor] = useState<{ refId: string; msg: string; meta?: SourceMeta } | null>(null);
  const [pasteBusy, setPasteBusy] = useState(false);

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
    setPasteFor(null);
    setSelectedId(id);
  };
  // GVd · "← 印记" from 'node'/'results' → back to the coach: deselect + clear dig.
  const deselect = () => {
    resetDig();
    setPasteFor(null);
    setSelectedId(null);
  };

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

  // Task 8 (P2b) · re-fetch when the container bumps `explorationRefreshNonce`
  // (a 印记-proposed question just got confirmed into a lead elsewhere — the
  // chat chip, not this view). Skips the initial render: the mount effect
  // above already fetches once for nonce=0, so only a genuine BUMP should
  // trigger another round-trip (mirrors WorkspaceContainer's `didMountRoom`).
  const didMountRefreshNonce = useRef(false);
  useEffect(() => {
    if (!didMountRefreshNonce.current) {
      didMountRefreshNonce.current = true;
      return;
    }
    void refresh();
  }, [refreshNonce, refresh]);

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

  // A keyword search launched from the CONTROLS column (印记's 检索方向 box, the
  // 还需要探索的 notes) — where NO node is selected, so runDig would no-op. The
  // results land in a controls-level tray; 采纳 there creates a reference into
  // 未归类 (no parent question yet), which the student then places (fixes the
  // "clicked 搜索, nothing happens" dead-click).
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchTray, setSearchTray] = useState<DigCandidate[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState(false);
  const [savingRef, setSavingRef] = useState<Set<string>>(new Set());

  async function runControlsSearch(keyword: string) {
    const kw = keyword.trim();
    if (!kw || searching) return;
    setSearchKeyword(kw);
    setSearchTray([]);
    setSearching(true);
    setSearchError(false);
    try {
      const res = await digExploration(projectId, { keyword: kw, mode: "similar" });
      setSearchTray(res.candidates);
    } catch {
      setSearchError(true);
    } finally {
      setSearching(false);
    }
  }
  // 采纳 a controls-search result into 未归类: create the reference (bib via the
  // candidate's DOI/url; server DOI-resolves), then it shows in the 未归类 node
  // ready to be placed under a question. No lead is created here (铁律①: the
  // student decides where it belongs via the placement picker).
  async function saveSearchResult(c: DigCandidate) {
    if (!onCreateReference) return;
    const key = candidateKey(c);
    if (savingRef.has(key)) return;
    setSavingRef((s) => new Set(s).add(key));
    setActionError(false);
    try {
      const url = c.url || (c.doi ? "https://doi.org/" + c.doi : undefined);
      await onCreateReference({ title: c.title, url });
      setSearchTray((t) => t.filter((x) => candidateKey(x) !== key));
      await refresh();
      onLibraryChanged?.();
    } catch {
      setActionError(true);
    } finally {
      setSavingRef((s) => {
        const n = new Set(s);
        n.delete(key);
        return n;
      });
    }
  }
  function discardSearchResult(c: DigCandidate) {
    const key = candidateKey(c);
    setSearchTray((t) => t.filter((x) => candidateKey(x) !== key));
  }

  // A keyword search: with a node selected it digs into that node's sidebar tray
  // (adopt → under the node); with none selected (the controls column) it runs
  // the controls search above instead of silently no-op'ing.
  const keywordSearch = (keyword: string) => (selectedId ? runDig({ keyword, mode: "similar" }) : void runControlsSearch(keyword));

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

  // 未归类 panel · attach an EXISTING reference to a question lead (the
  // self-added-source twin of adopt — no new reference is created, just a new
  // connected lead under parentLeadId). Explicit student action only (铁律①).
  const [attaching, setAttaching] = useState<string | null>(null); // referenceId in flight
  async function attach(referenceId: string, parentLeadId: string) {
    if (attaching) return;
    setAttaching(referenceId);
    setActionError(false);
    try {
      await attachReference(projectId, referenceId, parentLeadId);
      await refresh();
      onLibraryChanged?.();
    } catch {
      setActionError(true);
    } finally {
      setAttaching(null);
    }
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
    setPasteFor(null);
    try {
      const { source, suggestedReason } = await enterReading(projectId, ref.id);
      onEnterReading(source, ref.id, suggestedReason, ref.phaseTag, ref.readingReason, ref.readingFocus);
    } catch (e) {
      // 422 = the paper's full text can't be fetched (most paywalled papers).
      // Don't dead-click: open an inline paste box right here so she can drop the
      // body in and enter the reading room, carrying any DOI metadata we recovered.
      if (e instanceof NoReadableContentError) {
        setPasteFor({ refId: ref.id, msg: e.message, meta: e.meta });
      } else {
        setActionError(true);
      }
    } finally {
      setEnteringRefId(null);
    }
  }

  // Paste-body fallback for the selected paper: create its material from the
  // pasted text, then enter the reading room exactly as the fetch path would.
  async function submitPaste(ref: Reference, text: string) {
    if (!onEnterReading || pasteBusy) return;
    setPasteBusy(true);
    setActionError(false);
    try {
      const source = await pasteContent(projectId, ref.id, text);
      setPasteFor(null);
      onEnterReading(source, ref.id, "", ref.phaseTag, ref.readingReason, ref.readingFocus);
    } catch {
      setActionError(true);
    } finally {
      setPasteBusy(false);
    }
  }

  const roots = useMemo(() => view.leads.filter((l) => l.parentLeadId == null), [view.leads]);

  // 未归类 · references with no non-pruned connected lead, and the open
  // questions they can be attached under (roots + their sub-questions).
  const unfiled = useMemo(() => unfiledReferences(references, view.leads), [references, view.leads]);
  // Never strand the student on an empty 未归类 panel — whether they just filed
  // the last source, or re-entered the room with a persisted "unfiled" zoom.
  // Fall back to the map (final-review #5).
  useEffect(() => {
    if (zoom.mode === "unfiled" && unfiled.length === 0) backToMap();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [zoom.mode, unfiled.length]);
  const placementQuestions = useMemo<PlacementQuestion[]>(
    () =>
      view.leads
        .filter((l) => l.status !== "pruned" && l.connectedReferenceId == null)
        .map((l) => ({ id: l.id, text: l.text, parentId: l.parentLeadId })),
    [view.leads],
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
      pastePrompt={pasteFor && selectedRef && pasteFor.refId === selectedRef.id ? { msg: pasteFor.msg, meta: pasteFor.meta } : undefined}
      pasteBusy={pasteBusy}
      onPaste={onEnterReading && selectedRef ? (text: string) => submitPaste(selectedRef, text) : undefined}
      onSetEvidence={selectedRef ? (ev) => void setReferenceEvidence(projectId, selectedRef.id, ev).then(() => onLibraryChanged?.()) : undefined}
      onSetTriage={selectedRef ? (tri) => void setReferenceTriage(projectId, selectedRef.id, tri).then(() => onLibraryChanged?.()) : undefined}
      onArchive={selectedRef ? () => void archiveReference(projectId, selectedRef.id, !(selectedRef.archived ?? false)).then(() => onLibraryChanged?.()) : undefined}
    />
  );

  // The exploration CONTROLS (印记's search-direction guidance, 理一理材料, 还需要
  // 探索的 notes, 找关系) — a fixed-width sidebar docked OPPOSITE the 印记 chat so
  // the graph stays the centered focal element and the controls never overlap the
  // chat. Shown at BOTH levels (map + a question's hole) whenever no node is
  // selected; when a node IS selected the SAME column swaps to its detail panel
  // (`sidebar`), which has its own ← back to these controls. So the sidebar never
  // disappears — it's a two-page column: controls ⇄ node details.
  const auxOnLeft = aiSide === "right";
  const controlsColumn = (
    <aside
      className={
        "mk-scroll flex w-[320px] flex-none flex-col gap-2.5 overflow-y-auto bg-mk-surface p-4 border-mk-border " +
        (auxOnLeft ? "border-r" : "border-l")
      }
    >
      {/* slice 5 (§113/§115/§116) · 印记's search-direction guidance + the
          student's 还需要探索的 notes (「去探索」 runs a note as a search). */}
      <SearchGuidanceBox projectId={projectId} onSearch={keywordSearch} />
      {/* Results of a controls-level keyword search (印记 检索方向 / 还需要探索的).
          采纳 lands a paper in 未归类 for the student to place. */}
      {(searching || searchError || searchTray.length > 0) && (
        <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
          <p className="mb-2 text-[12px] font-bold text-mk-faint">「{searchKeyword}」的检索结果</p>
          {searching && <p className="text-[12px] text-mk-muted">印记正在检索…</p>}
          {searchError && !searching && <p className="text-[12px] font-semibold text-mk-accent">这次没搜到，换个关键词再试。</p>}
          {!searching && !searchError && searchTray.length === 0 && (
            <p className="text-[12px] text-mk-faint">没有结果，换个关键词试试。</p>
          )}
          <ul className="flex flex-col gap-2">
            {searchTray.map((c) => {
              const key = candidateKey(c);
              return (
                <li key={key} className="rounded-mk border border-mk-border bg-mk-paper px-2.5 py-2">
                  <p className="text-[13px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                  {(c.journal || c.year) && (
                    <p className="mt-0.5 text-[11px] text-mk-faint">{[c.journal, c.year].filter(Boolean).join(" · ")}</p>
                  )}
                  <div className="mt-1.5 flex gap-2">
                    <button
                      type="button"
                      onClick={() => void saveSearchResult(c)}
                      disabled={savingRef.has(key)}
                      className="rounded-mk bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
                    >
                      {savingRef.has(key) ? "收下中…" : "收进未归类"}
                    </button>
                    <button
                      type="button"
                      onClick={() => discardSearchResult(c)}
                      className="rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-faint hover:text-mk-accent"
                    >
                      忽略
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {/* §5 follow-up · once ≥2 sources are collected, ask 印记 to review them
          (below that it has too little to compare — final-review/user note). */}
      {references.length >= 2 && <ExplorationReviewBox projectId={projectId} />}
      <NeedsResourcesBox projectId={projectId} onExplore={(note) => { if (note) keywordSearch(note); }} />
      {actionError && <p className="text-[12px] font-semibold text-mk-accent">刚才那步没接上，再试一次？</p>}
      {/* B4b · 印记 proposes relationships between the questions. Only meaningful
          with ≥2 root questions (an edge needs two ends). */}
      {roots.length >= 2 && (
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={proposeRelations}
            disabled={proposing}
            className="rounded-full border border-mk-accent/40 bg-mk-surface px-3 py-1.5 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-60"
          >
            让印记找找问题之间的关系
          </button>
          {/* Task 7 · the question-relation proposer is a HIDDEN subagent: a
              status line while it runs, never a chat. */}
          {proposing && <SubagentHint text="subagent 正在梳理问题关系…" />}
          {proposeNote && <span className="text-[12px] text-mk-faint">{proposeNote}</span>}
        </div>
      )}
    </aside>
  );
  // No node selected → the controls; a selected node → its detail panel.
  const auxColumn = sidebarState === "ai" ? controlsColumn : sidebar;

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <RabbitHoleLoader caption="加载探索图谱中…" />
      </div>
    );
  }

  if (zoom.mode === "unfiled") {
    /* ---------- 未归类 panel · sources with no attached question yet, each with
       进入阅读室 + a PlacementPicker to挂 it under a question ---------- */
    const unfiledMain = (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-4 py-2.5">
          <button
            type="button"
            onClick={backToMap}
            className="flex-none rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
          >
            ← 返回兔子洞地图
          </button>
          <h2 className="min-w-0 truncate font-sans text-[14px] font-bold text-mk-ink">未归类的来源 · {unfiled.length} 篇</h2>
        </div>
        <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-6 py-4">
          {unfiled.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState illustration="warren" title="都归好位了" body="每一篇来源都挂到了某个问题下——干净。" />
            </div>
          ) : (
            <ul className="flex flex-col gap-3">
              {unfiled.map((ref) => (
                <li key={ref.id} className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
                  <p className="text-[14px] font-bold text-mk-ink">{ref.title || "未命名来源"}</p>
                  <div className="mt-2 flex flex-col gap-2">
                    {onEnterReading && (
                      <button
                        type="button"
                        onClick={() => void enterSource(ref)}
                        disabled={enteringRefId === ref.id}
                        className="self-start rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
                      >
                        {enteringRefId === ref.id ? "打开中…" : "进入阅读室"}
                      </button>
                    )}
                    <div className="rounded-mk border border-mk-border bg-mk-paper p-2.5">
                      <p className="mb-2 text-[12px] font-bold text-mk-faint">挂到问题下</p>
                      <PlacementPicker
                        questions={placementQuestions}
                        suggestedLeadId={null}
                        reason=""
                        busy={attaching === ref.id}
                        showUnfiledOption={false}
                        onPick={(leadId) => leadId && void attach(ref.id, leadId)}
                      />
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    );
    return (
      <div className="relative flex h-full min-h-0 bg-mk-paper">
        {auxOnLeft && auxColumn}
        {unfiledMain}
        {!auxOnLeft && auxColumn}
      </div>
    );
  }

  if (inHole) {
    /* ---------- HOLE (Level-2) · one question's mindmap + the SAME two-page aux
       sidebar as the map (controls ⇄ node details), docked opposite the chat ---------- */
    const holeMain = (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-4 py-2.5">
          <button
            type="button"
            onClick={backToMap}
            className="flex-none rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
          >
            ← 返回兔子洞地图
          </button>
          <h2 className="min-w-0 truncate font-sans text-[14px] font-bold text-mk-ink">{focusRoot!.text}</h2>
        </div>
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
    );
    return (
      <div className="relative flex h-full min-h-0 bg-mk-paper">
        {auxOnLeft && auxColumn}
        {holeMain}
        {!auxOnLeft && auxColumn}
      </div>
    );
  }

  /* ---------- MAP (Level-1) · the overview graph of root questions + the SAME
     two-page aux sidebar (controls ⇄ node details), docked opposite the chat ---------- */
  return (
    <div className="relative flex h-full min-h-0 bg-mk-paper">
      {auxOnLeft && auxColumn}
      <div className="relative flex min-h-0 flex-1 flex-col">
        {/* Map body — the whole column is the graph now (no header above it), so
            it's the centered focal element and grows with the viewport. */}
        <div className="min-h-0 flex-1 px-6 py-4">
          {roots.length === 0 ? (
            // Task 8 (P2b) · neutral — 印记 proposes questions in the chat now
            // (no imperative to type one herself; 铁律①: she still confirms).
            <div className="flex h-full items-center justify-center">
              <EmptyState
                illustration="warren"
                title="这里还是空的"
                body="聊聊你想弄清楚的问题，印记会在合适的时候提出来——你确认后它就会出现在这里，点开再「深挖」，采纳的文献会挂到这条线下面，慢慢长成一张图。"
              />
            </div>
          ) : (
            <WarrenMap
              projectId={projectId}
              roots={roots}
              countByRoot={countByRoot}
              edges={view.edges}
              onZoom={zoomInto}
              unfiledCount={unfiled.length}
              onOpenUnfiled={() => goZoom({ mode: "unfiled", focusRootId: null })}
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
      {!auxOnLeft && auxColumn}
    </div>
  );
}
