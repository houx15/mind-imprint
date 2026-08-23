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
  SearchSuggestion,
} from "@mind-imprint/contracts";
import { enterReading, pasteContent, NoReadableContentError, type SourceMeta, type ReferenceBib } from "../../api/workspace";
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
import { PaperDetail, candidateToPaperView } from "./PaperDetail";
import { PlacementPicker, type PlacementQuestion } from "./PlacementPicker";
import { QuestionMindmap } from "./QuestionMindmap";
import { WarrenMap } from "./WarrenMap";
import { anyReferenceDoneByRoot, countPapersByRoot } from "./warrenLayout";
import { RabbitHoleLoader } from "@/ui";
import { SubagentHint } from "@/studio/ai/SubagentHint";
import { ExplorationReviewBox } from "./ExplorationReviewBox";
import { NeedsResourcesBox } from "../NeedsResourcesBox";
import { SearchCardModal } from "./SearchCardModal";
import { proposeSearchGuidance } from "../../../api/searchGuidance";

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
// paperToken normalizes a url or DOI into a comparable token (scheme + doi.org
// prefix stripped), so a candidate's DOI matches a reference stored as a
// doi.org URL — used to tell "已在图谱" from "待加入".
export function paperToken(urlOrDoi: string | undefined): string {
  let s = (urlOrDoi ?? "").trim().toLowerCase();
  if (!s) return "";
  s = s.replace(/^https?:\/\//, "").replace(/^www\./, "").replace(/^(dx\.)?doi\.org\//, "");
  return s.replace(/\/+$/, "");
}

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
    readingNote?: string | null,
    bib?: ReferenceBib,
    reference?: Reference | null,
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
  /** P7 · the guided tour's `openSearchCard` deep-link — a bumped nonce
   * (forwarded from WorkspaceContainer/ReadingBlock) that opens the 检索卡
   * teaching modal once per new value. Ref-guarded (mirrors ReadingBlock's own
   * `forceView`): applied at most once per distinct value, never re-fights the
   * student's own later open/close of the modal. */
  forceOpenSearchCard?: number | null;
  /** Fired once right after `forceOpenSearchCard` has been applied, so the
   * caller can retract it (mirrors every other force* one-shot). */
  onForceOpenSearchCardConsumed?: () => void;
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
  forceOpenSearchCard,
  onForceOpenSearchCardConsumed,
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

  // Wave 2 · the controls-column search is a 4-page drill-down: 检索方向 (印记's
  // suggested directions) → 检索结果 list → 论文 detail → add. Only the map/hole
  // controls page (nothing selected) uses this; the node-dig sidebar is separate.
  // 铁律①: the student taps to search, taps to add — nothing auto-fetches.
  const [searchStage, setSearchStage] = useState<"idle" | "directions" | "list" | "detail">("idle");
  const [directions, setDirections] = useState<SearchSuggestion[] | null>(null);
  const [proposingDir, setProposingDir] = useState(false);
  const [searchDetail, setSearchDetail] = useState<DigCandidate | null>(null);
  const [showSearchCard, setShowSearchCard] = useState(false);

  // P7 · guided-tour deep-link: apply `forceOpenSearchCard` once (ref-guarded,
  // mirrors ReadingBlock's `forceView`) so a tour step can open 检索卡
  // deterministically without ever re-fighting the student's own later
  // open/close. This component remounts fresh whenever the reading room
  // toggles 列表⇄探索图谱 (ReadingBlock only renders it in "graph"), so the ref
  // guard is per-mount — the retraction below (via the consumed callback,
  // which nulls the nonce upstream) is what stops a later remount from
  // re-opening the modal off a stale truthy prop.
  const lastOpenSearchCard = useRef<number | null>(null);
  useEffect(() => {
    if (!forceOpenSearchCard || lastOpenSearchCard.current === forceOpenSearchCard) return;
    lastOpenSearchCard.current = forceOpenSearchCard;
    setShowSearchCard(true);
    onForceOpenSearchCardConsumed?.();
  }, [forceOpenSearchCard, onForceOpenSearchCardConsumed]);

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

  // Wave 2 handlers · 检索方向 → 检索结果 → 论文 → add. proposeDirections biases the
  // AI toward the question layer the student is currently inside (item 3.1).
  async function proposeDirections() {
    if (proposingDir) return;
    setProposingDir(true);
    setActionError(false);
    try {
      setDirections(await proposeSearchGuidance(projectId, inHole && focusRoot ? focusRoot.text : undefined));
    } catch {
      setDirections([]);
    } finally {
      setSearchStage("directions");
      setProposingDir(false);
    }
  }
  function openDirection(keyword: string) {
    setSearchStage("list");
    void runControlsSearch(keyword);
  }
  function openDetail(c: DigCandidate) {
    setSearchDetail(c);
    setSearchStage("detail");
  }
  // Search MORE papers from the paper being viewed (相似/引用/被引) — feeds back
  // into the results list → single-paper loop. "similar" searches off the title;
  // citation/cited off the candidate's own DOI (backend #1 accepts a doi param),
  // so this works for a candidate that isn't on the map yet.
  async function digFromCandidate(c: DigCandidate, mode: DigMode) {
    if (searching) return;
    setSearchDetail(null);
    setSearchKeyword(c.title);
    setSearchTray([]);
    setSearchStage("list");
    setSearching(true);
    setSearchError(false);
    try {
      const res = await digExploration(projectId, mode === "similar" ? { keyword: c.title, mode } : { doi: c.doi, mode });
      setSearchTray(res.candidates);
    } catch {
      setSearchError(true);
    } finally {
      setSearching(false);
    }
  }
  // Add the paper being viewed: inside a question → adopt UNDER that root
  // (papers-never-roots); on the top-level map (no "second layer") → into 未归类
  // via saveSearchResult. After adding, drop it from the list and go back.
  async function addFromDetail(c: DigCandidate) {
    const key = candidateKey(c);
    if (savingRef.has(key)) return;
    if (inHole && focusRoot) {
      setSavingRef((s) => new Set(s).add(key));
      setActionError(false);
      try {
        await adoptCandidate(projectId, c, { parentLeadId: focusRoot.id });
        setSearchTray((t) => t.filter((x) => candidateKey(x) !== key));
        await refresh();
        onLibraryChanged?.();
        setSearchDetail(null);
        setSearchStage("list");
      } catch {
        setActionError(true);
      } finally {
        setSavingRef((s) => {
          const n = new Set(s);
          n.delete(key);
          return n;
        });
      }
    } else {
      // saveSearchResult owns its own busy guard + tray removal + refresh; it
      // only drops the row on success, so a failed add leaves it for a retry.
      await saveSearchResult(c);
      setSearchDetail(null);
      setSearchStage("list");
    }
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
      onEnterReading(source, ref.id, suggestedReason, ref.phaseTag, ref.readingReason, ref.readingFocus, ref.readingNote, undefined, ref);
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
      onEnterReading(source, ref.id, "", ref.phaseTag, ref.readingReason, ref.readingFocus, ref.readingNote, undefined, ref);
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
  // Tokens of every reference already in the library — a search candidate whose
  // DOI/url matches one is "已在图谱" (待加入 otherwise).
  const addedTokens = useMemo(() => {
    const s = new Set<string>();
    for (const r of references) {
      const t = paperToken(r.url);
      if (t) s.add(t);
    }
    return s;
  }, [references]);
  const candidateAdded = (c: DigCandidate) => {
    const byDoi = paperToken(c.doi);
    const byUrl = paperToken(c.url);
    return (byDoi !== "" && addedTokens.has(byDoi)) || (byUrl !== "" && addedTokens.has(byUrl));
  };
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
  // "已读" badge — roots with ≥1 contained reference (own or descendant's)
  // whose readingStatus is "done". referenceStatus is a plain id→status
  // lookup off the same `references` prop the sidebar already joins against.
  const referenceStatusById = useMemo(() => {
    const m = new Map<string, string>();
    for (const r of references) m.set(r.id, r.readingStatus);
    return m;
  }, [references]);
  const readByRoot = useMemo(
    () => anyReferenceDoneByRoot(view.leads, referenceStatusById),
    [view.leads, referenceStatusById],
  );

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
      {/* Wave 2 · the search area is a 4-page drill-down. The OTHER controls
          (整理评审 / 还需要探索的 / 找关系) live only on the idle page. */}
      {searchStage === "idle" && (
        <>
          {/* 检索方向 propose box — styled like the old SearchGuidanceBox shell,
              plus the 检索卡 teaching entry. */}
          <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
            <div className="flex flex-col gap-2">
              <div className="flex items-start gap-2">
                <span className="flex-none rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">检索方向</span>
                <p className="min-w-0 flex-1 text-[13px] leading-relaxed text-mk-muted">不知道搜什么？让印记根据你的问题给几个方向。</p>
              </div>
              <button
                type="button"
                onClick={() => void proposeDirections()}
                disabled={proposingDir}
                className="w-full rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
              >
                {proposingDir ? "印记在想…" : "让印记建议检索方向"}
              </button>
              <button
                type="button"
                data-tour="search-card-trigger"
                onClick={() => setShowSearchCard(true)}
                className="w-full rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50"
              >
                如何检索资料？检索卡
              </button>
            </div>
          </div>
          {/* §5 follow-up · once ≥2 sources are collected, ask 印记 to review them
              (below that it has too little to compare — final-review/user note). */}
          {references.length >= 2 && <ExplorationReviewBox projectId={projectId} />}
          <NeedsResourcesBox
            projectId={projectId}
            onExplore={(note) => {
              if (note) {
                setSearchStage("list");
                void runControlsSearch(note);
              }
            }}
          />
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
        </>
      )}

      {searchStage === "directions" && (
        <>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setSearchStage("idle")}
              className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
            >
              ← 返回
            </button>
            <button
              type="button"
              onClick={() => setShowSearchCard(true)}
              className="ml-auto rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
            >
              检索卡
            </button>
          </div>
          <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
            <div className="flex items-center gap-2">
              <span className="flex-none rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">检索方向</span>
              <button
                type="button"
                onClick={() => void proposeDirections()}
                disabled={proposingDir}
                className="ml-auto rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
              >
                {proposingDir ? "印记在想…" : "换一批"}
              </button>
            </div>
            {directions && directions.length > 0 && (
              <ul className="mt-2.5 flex flex-col gap-1.5">
                {directions.map((s, i) => (
                  <li key={i} className="flex items-start gap-2 rounded-mk border border-mk-border bg-mk-paper px-2.5 py-1.5">
                    <div className="min-w-0 flex-1">
                      <p className="text-[13.5px] font-semibold text-mk-ink">{s.keyword}</p>
                      {s.why && <p className="mt-0.5 text-[12px] leading-relaxed text-mk-muted">{s.why}</p>}
                    </div>
                    <button
                      type="button"
                      onClick={() => openDirection(s.keyword)}
                      className="flex-none rounded-mk bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600"
                    >
                      搜索
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {directions && directions.length === 0 && !proposingDir && (
              <p className="mt-2 text-[12px] text-mk-faint">这次没给出方向，先确认你已经写下研究问题，再试一次。</p>
            )}
          </div>
        </>
      )}

      {searchStage === "list" && (
        <>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setSearchStage("directions")}
              className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
            >
              ← 返回
            </button>
          </div>
          {searching ? (
            <div className="flex flex-1 items-center justify-center py-6">
              <RabbitHoleLoader caption="subagent 正在检索来源……" />
            </div>
          ) : searchError ? (
            <p className="text-[12px] font-semibold text-mk-accent">这次没搜到，换个关键词再试。</p>
          ) : (
            <>
              <p className="text-[12px] font-bold text-mk-faint">「{searchKeyword}」的检索结果</p>
              {searchTray.length === 0 ? (
                <p className="text-[12px] text-mk-faint">没有结果，换个关键词试试。</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {searchTray.map((c) => {
                    const key = candidateKey(c);
                    const meta = [c.authors, c.year, c.journal].map((s) => s?.trim()).filter(Boolean).join(" · ");
                    return (
                      <li key={key}>
                        <button
                          type="button"
                          onClick={() => openDetail(c)}
                          className="w-full cursor-pointer rounded-mk border border-mk-border bg-mk-paper px-2.5 py-2 text-left hover:border-mk-accent hover:bg-mk-accent-50"
                        >
                          <p className="text-[13px] font-semibold leading-snug text-mk-ink">{c.title}</p>
                          {meta && <p className="mt-0.5 text-[11px] text-mk-faint">{meta}</p>}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </>
          )}
        </>
      )}

      {searchStage === "detail" && searchDetail && (
        <>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => {
                setSearchDetail(null);
                setSearchStage("list");
              }}
              className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
            >
              ← 返回
            </button>
          </div>
          {(() => {
            const c = searchDetail;
            const key = candidateKey(c);
            const busy = savingRef.has(key);
            const added = candidateAdded(c);
            return (
              <PaperDetail
                paper={candidateToPaperView(c, added)}
                primaryAction={
                  added
                    ? undefined
                    : { label: inHole ? "采纳到当前问题" : "收进未归类", onClick: () => void addFromDetail(c), busy }
                }
                secondaryAction={
                  added
                    ? undefined
                    : {
                        label: "丢弃",
                        onClick: () => {
                          discardSearchResult(c);
                          setSearchDetail(null);
                          setSearchStage("list");
                        },
                      }
                }
                onFind={(mode) => void digFromCandidate(c, mode)}
                finding={searching}
              />
            );
          })()}
        </>
      )}

      {showSearchCard && <SearchCardModal onClose={() => setShowSearchCard(false)} />}
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
            // When there are NO question lines yet but the student already has
            // sources (added by hand / from search), don't strand them: the map
            // is empty of questions, but a 未归类 entry must still be reachable
            // here (WarrenMap — which owns that entry — isn't rendered at 0 roots).
            <div className="flex h-full flex-col items-center justify-center gap-4">
              <EmptyState
                illustration="warren"
                title={unfiled.length > 0 ? "还没有问题线，先把来源理一理" : "这里还是空的"}
                body={
                  unfiled.length > 0
                    ? "印记会在合适的时候把问题提出来，你确认后就会长成一张图。你已经收了一些来源——先看看它们，挂到对应的问题下。"
                    : "聊聊你想弄清楚的问题，印记会在合适的时候提出来——你确认后它就会出现在这里，点开再「深挖」，采纳的文献会挂到这条线下面，慢慢长成一张图。"
                }
              />
              {unfiled.length > 0 && (
                <button
                  type="button"
                  onClick={() => goZoom({ mode: "unfiled", focusRootId: null })}
                  className="rounded-mk-md border border-mk-accent bg-mk-accent-50 px-4 py-2 text-[14px] font-bold text-mk-accent hover:bg-mk-accent-100"
                >
                  查看 {unfiled.length} 篇未归类的来源 →
                </button>
              )}
            </div>
          ) : (
            <WarrenMap
              projectId={projectId}
              roots={roots}
              countByRoot={countByRoot}
              readByRoot={readByRoot}
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
