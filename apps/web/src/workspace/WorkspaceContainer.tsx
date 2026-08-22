import { useCallback, useEffect, useRef, useState } from "react";
import type {
  CardProposalWire,
  LinkOffer,
  MaterialSource,
  NextStep,
  NoteProposal,
  PhaseTag,
  PlanItem,
  Proposal,
  QuestionProposal,
  Reference,
  ReviewVerdict,
  StudioState,
  WidthTier,
  WorkspaceProjection,
} from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { setReferenceEvidence, setReferenceTriage, archiveReference } from "@/api/evidenceMap";
import type { Dispatch, SetStateAction } from "react";
import { api } from "../api";
import { createLead, digExploration, adoptCandidate } from "@/api/exploration";
import { ReadingRoom } from "../studio/reading/ReadingRoom";
import { AiPanel, type AiPanelSide } from "../studio/ai/AiPanel";
import { StudioAiSlotContext } from "../studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg, type StudioChatValue, type ChatAction } from "../studio/ai/StudioChatContext";
import { StudioCoachChat } from "../studio/ai/StudioCoachChat";
import { StudioCardSheet } from "../studio/StudioCardSheet";
import { QuestionCardModal } from "../studio/QuestionCardModal";
import { compileCardForCoach } from "../studio/compileCard";
import { Icon as UiIcon, ArrowLeft } from "@/ui/Icon";
import { Badge } from "@/ui/feedback";
import { SplitPane } from "@/ui/SplitPane";
import { Icon } from "./Icon";
import { RoomSwitcher } from "./RoomSwitcher";
import { Directory } from "./Directory";
import {
  getWorkspace,
  getPlan,
  getCoachHistory,
  getStudioState,
  postProjectSummary,
  patchReference,
  coach,
  coachOpening,
  coachStart,
  coachAdvance,
  putProposal,
  reflectProjectCard,
  dismissProposal,
  createReference,
  type ReferenceBib,
} from "./api/workspace";
import { roomForResume } from "./studioResume";
import { PlanBlock } from "./blocks/PlanBlock";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
import { activeDocForStage } from "./activeDoc";
import type { WritingDocKind } from "../api/writing";
import { ReferencePanel } from "./blocks/ReferencePanel";
import { ReviewBlock } from "./blocks/ReviewBlock";
import type { BlockKey } from "./blocks/mockData";

// appendFrameworkVerdict renders the framework-readiness reviewer's read (slice
// 2) as a 印记 bubble — "（我读了一遍你的研究框架）<why>" + suggestions as markdown
// bullets. No-op without a verdict. Real coaching → a full markdown bubble, not
// a muted subagent hint.
export function appendFrameworkVerdict(
  verdict: ReviewVerdict | null | undefined,
  setMessages: Dispatch<SetStateAction<StudioChatMsg[]>>,
): void {
  if (!verdict) return;
  const lines = [`（我读了一遍你的研究框架）${verdict.why}`];
  if (verdict.suggestions.length > 0) {
    lines.push("", "可以再打磨：", ...verdict.suggestions.map((s) => `- ${s}`));
  }
  setMessages((c) => [...c, { role: "ai", text: lines.join("\n") }]);
}

/** Join truthy class fragments with a single space; drops falsy/empty ones
 * (copied from `ui/Card.tsx` — every `ui/`-adjacent file keeps its own local
 * copy rather than sharing an export, per the design-system convention). */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// The top-level workspace: a project is a set of four rooms
// (项目管理 · 阅读 · 写作 · 回顾) the student moves between freely. No open
// project ⇒ the <Directory>; an open project ⇒ the left rail + the active
// room, landing in Project Management.
//
// Project Management (项目管理) is now fully API-backed (slice 2); the other
// three rooms still run on local mock state (slices 3–5). The shell (identity,
// qualification, the room swap) is live.
export function WorkspaceContainer({
  onFinished,
  initialProjectId,
  onInitialProjectIdConsumed,
  autoOpenCreate,
  onAutoOpenCreateConsumed,
  onInProjectChange,
  pendingRoom,
  onPendingRoomConsumed,
}: {
  onFinished?: (projectId?: string) => void;
  /** Fired when a project opens (true) or closes (false) — the shell uses
   * this to hide its platform nav for the immersive studio (spec §17). */
  onInProjectChange?: (inProject: boolean) => void;
  /** Open this project on mount (or whenever it changes to a new id) — the
   * "open from home" deep-link (Task 6). Undefined/null leaves the
   * directory showing, same as before this prop existed. */
  initialProjectId?: string | null;
  /** Fired once right after `initialProjectId` has been acted on, so the
   * caller can clear its pending-id state (else a stale-but-unchanged prop
   * would look "already handled" and never re-fire for a genuinely new
   * open-request with the same id after an intervening navigation). */
  onInitialProjectIdConsumed?: () => void;
  /** Open the directory's create drawer as soon as it mounts (home's
   * "新建" → 项目 tab deep-link, Task 6). Only read on Directory's mount. */
  autoOpenCreate?: boolean;
  onAutoOpenCreateConsumed?: () => void;
  /** Drive the manual room switcher to this room (the guided tour's studio
   * deep-link, P3 Task 1) — mirrors `initialProjectId`: a one-shot signal
   * acted on whenever it changes to a new, truthy value. */
  pendingRoom?: BlockKey | null;
  /** Fired once right after `pendingRoom` has been acted on, so the caller
   * can clear its pending-room state (else a stale-but-unchanged prop would
   * look "already handled"). */
  onPendingRoomConsumed?: () => void;
}) {
  const [projectId, setProjectId] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<WorkspaceProjection | null>(null);
  // The project plan's items — used to drive the in-chat plan walkthrough
  // (showPlanIntro). Empty until a plan is generated; refreshed alongside the
  // workspace and after any plan-mutating coach turn.
  const [planItems, setPlanItems] = useState<PlanItem[]>([]);
  // §3 gap G3 · the plan-intro walkthrough fires ONCE, only when the plan first
  // appears DURING this session (situation a). Set true on load if a plan already
  // exists (returning student = situation b, no intro).
  const planIntroShownRef = useRef(false);
  // §gap G2 · true while confirming the 4th dim triggers the funnel's plan
  // generation (a reasoning-model call) — drives an interesting rotating loader.
  const [generatingPlan, setGeneratingPlan] = useState(false);
  // #83 · the 写作 room defaults to the doc the stage wants (activeDocForStage),
  // but once the essay has begun the student may switch back to VIEW the finished
  // proposal. null = follow the stage; else this doc wins (if still an option).
  const [docOverride, setDocOverride] = useState<WritingDocKind | null>(null);
  // The writing room's active tab (大纲/片段/正文), reported up by WritingBlock so
  // the left reference panel can surface the student's 片段 only while on 正文.
  const [writingTab, setWritingTab] = useState<"outline" | "snippets" | "draft">("outline");
  // First-run guard for the room-change plan refetch (declared here so the load
  // effect can reset it on project change). See the room-change effect below.
  const didMountRoom = useRef(false);
  // Task 6 (start gate): guards the one-shot `coach/opening` fire in the load
  // effect below (a brand-new, not-yet-started project with an empty thread)
  // so a re-render or effect re-run for the SAME project never double-fires
  // it — keyed on the project id, mirroring `lastInitialProjectId`/
  // `didMountRoom`. Reset on every project switch (below).
  const openingFiredForProjectId = useRef<string | null>(null);
  // Busy flag around the student's 开始 tap (`startJourney`, below) — drives
  // the 开始 button's pending/disabled state in StudioCoachChat.
  const [starting, setStarting] = useState(false);
  const [room, setRoom] = useState<BlockKey>("plan");
  // §3 situation b · a returning student (past the framework) lands on the 管理
  // plan for a recap; this flag drives the recap banner + 继续工作 button there.
  // Cleared the moment 印记 morphs the room or the student navigates.
  const [recapLanding, setRecapLanding] = useState(false);
  // §gaps G3/G5/G6 · the current scripted in-chat action (mode-choice / outline
  // intro / plan walkthrough). Cleared on project switch.
  const [chatAction, setChatAction] = useState<ChatAction | null>(null);
  // §5 · true when the reading room was opened MANUALLY (via the switcher) and
  // the student hasn't yet confirmed starting an exploration; a guide-entry
  // (onOpenReading) opens directly with this false.
  const [readingConfirmNeeded, setReadingConfirmNeeded] = useState(false);
  // 印记's AI-managed status directive (stage/openTool/widthTier/reference),
  // loaded once per project (Task 8). Drives resume-at-stage: which room the
  // shell lands on, and whether the interactive area is chat-first (openTool
  // === "chat") instead of a board. `null` = not yet loaded → render the
  // chat-first landing, never a forced plan board. Task 9 re-applies this
  // after every turn (morphing status).
  const [studioState, setStudioState] = useState<StudioState | null>(null);
  // Task 6 fix round 1 (start gate): whether the `getStudioState` fetch for
  // the CURRENTLY OPEN project has settled (resolved OR rejected) — distinct
  // from `studioState` itself, which stays `null` for both "still loading"
  // and "fetch failed". `started` (below) needs to tell those two apart: a
  // still-loading fetch must default to chat-only/开始 (no tab flash for a
  // genuinely new project), while a FAILED fetch on a project that may
  // already be started must keep the switcher live as an escape hatch (a
  // pre-existing invariant). Reset to `false` on every project switch,
  // flipped `true` in both the `.then` and `.catch` of the load effect's
  // `getStudioState` call.
  const [studioStateResolved, setStudioStateResolved] = useState(false);
  // Manual-takeover flag (spec §6): while true, the switcher-chosen `room`
  // mounts even in chat-first / null / errored status — the student is never
  // trapped in the chat landing with a dead switcher. Taking over does NOT
  // touch `studioState` (manual browsing doesn't change 印记's status); the
  // flag only affects what <main> renders. 印记 reasserting the view
  // (applyStudioState) clears it.
  const [tookOver, setTookOver] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The constant AI panel's side + collapsed state — persisted so it survives
  // room swaps and reloads. Default side is "left" (agentic studio: 印记 is the
  // constant left companion; the student can flip it right).
  const [aiSide, setAiSide] = useState<AiPanelSide>(() => {
    try {
      const v = localStorage.getItem("mk-studio-ai-side");
      return v === "left" || v === "right" ? v : "left";
    } catch {
      return "left";
    }
  });
  const flipAiSide = useCallback(() => {
    setAiSide((s) => {
      const next: AiPanelSide = s === "left" ? "right" : "left";
      try {
        localStorage.setItem("mk-studio-ai-side", next);
      } catch {
        /* best-effort; a blocked storage must never break the toggle */
      }
      return next;
    });
  }, []);
  const [aiCollapsed, setAiCollapsed] = useState<boolean>(() => {
    try {
      return localStorage.getItem("mk-studio-ai-collapsed") === "1";
    } catch {
      return false;
    }
  });
  const toggleAiCollapsed = useCallback(() => {
    setAiCollapsed((c) => {
      const next = !c;
      try {
        localStorage.setItem("mk-studio-ai-collapsed", next ? "1" : "0");
      } catch {
        /* best-effort; a blocked storage must never break the toggle */
      }
      return next;
    });
  }, []);
  // The AiPanel's body DOM node, captured via a callback ref so rooms can
  // portal their coach content into it (StudioAiSlotContext, Task 4). A
  // callback ref (not a plain useRef) is required here: it must trigger a
  // re-render — and so a context update — whenever the node mounts/unmounts
  // (room switch, panel collapse, side flip), not just capture it once.
  const [aiSlotEl, setAiSlotEl] = useState<HTMLDivElement | null>(null);
  const aiSlotRef = useCallback((node: HTMLDivElement | null) => {
    setAiSlotEl(node);
  }, []);
  // The reading-room swap slot. When set, the focused ReadingRoom surface
  // replaces the rooms entirely (mirrors the shipped studio's own swap).
  const [readingSource, setReadingSourceState] = useState<MaterialSource | null>(null);
  // The reference row + suggested brief seed the reading room needs
  // alongside its MaterialSource (S2, Task 9) — see ReadingBlock's
  // setReadingSource for where these are captured. phaseTag/readingReason/
  // readingFocus (Task 9 fix) are the reference's PERSISTED brief — threaded
  // through so a reopened source seeds the banner from its true saved
  // values instead of a stale suggestion/always-blank, which is what used to
  // let one field's edit silently wipe the other on the next full-replace
  // save.
  const [readingRefId, setReadingRefId] = useState("");
  const [readingSuggestedReason, setReadingSuggestedReason] = useState("");
  const [readingPhaseTag, setReadingPhaseTag] = useState<PhaseTag | null>(null);
  const [readingReadingReason, setReadingReadingReason] = useState<string | null>(null);
  const [readingReadingFocus, setReadingReadingFocus] = useState<string | null>(null);
  const [readingReadingNote, setReadingReadingNote] = useState<string | null>(null);
  // #4 · the reference's persisted bib (abstract/journal/author/year/url) shown
  // in the Reading Room header.
  const [readingBib, setReadingBib] = useState<ReferenceBib | null>(null);
  // The full reference row the reading room needs for its 证据笔记 (moved here
  // from the warren-map sidebar). Optional — a paste-created source with no
  // saved evidence still passes its (fresh) reference so the note works.
  const [readingReference, setReadingReference] = useState<Reference | null>(null);

  function openReadingSource(
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
    readingNote?: string | null,
    bib?: ReferenceBib,
    reference?: Reference | null,
  ) {
    setReadingSourceState(m);
    setReadingRefId(referenceId);
    setReadingSuggestedReason(suggestedReason ?? "");
    setReadingPhaseTag(phaseTag ?? null);
    setReadingReadingReason(readingReason ?? null);
    setReadingReadingFocus(readingFocus ?? null);
    setReadingReadingNote(readingNote ?? null);
    setReadingBib(bib ?? null);
    setReadingReference(reference ?? null);
  }
  // EA · carry-forward acknowledgment: when the student 归纳'd a source before
  // leaving, show a brief "you just read X — it's carried forward" note so the
  // reading room doesn't feel like an island on exit (the takeaway now really
  // rides the coach's spine). Only on a finalized read; a mere browse says nothing.
  const [carryForward, setCarryForward] = useState<string | null>(null);
  function closeReadingSource(finalized?: boolean) {
    if (finalized && readingSource) setCarryForward(readingSource.title);
    setReadingSourceState(null);
    setReadingRefId("");
    setReadingSuggestedReason("");
    setReadingPhaseTag(null);
    setReadingReadingReason(null);
    setReadingReadingFocus(null);
    setReadingReadingNote(null);
    setReadingBib(null);
  }
  // S1 · summary-on-return: a compact re-entry paragraph, composed once per
  // project (first-open-wins), shown as a dismissible welcome-back toast. Only
  // for in-progress projects (a non-empty proposal) — a brand-new project has
  // nothing to summarise.
  const [summary, setSummary] = useState<string | null>(null);
  // The ONE continuous 印记 coach thread (立项 + 写作, surface="studio"), hoisted
  // here so it PERSISTS across room switches: each working room reads/appends to
  // this one store via `useStudioChat()` instead of holding its own local chat
  // state (which a room-swap unmount would throw away, forcing a re-fetch). The
  // in-flight `sending` flag rides up too so a reply landing after a room switch
  // still shows its busy state on whichever room is now mounted.
  const [studioMessages, setStudioMessages] = useState<StudioChatMsg[]>([]);
  const [studioSending, setStudioSending] = useState(false);
  // Task 5 (history pagination): the studio thread now loads its RECENT page
  // only (not the whole thread) — `historyCursor` is the cursor to pass as
  // `before` for the NEXT (older) page, `historyHasMore` gates the 载入更早的
  // 对话 control, `historyRecap` is the endpoint's digest prose for a long
  // thread (wins over the S1 `summary` re-entry paragraph when present).
  const [historyCursor, setHistoryCursor] = useState<string | null>(null);
  const [historyHasMore, setHistoryHasMore] = useState(false);
  const [historyRecap, setHistoryRecap] = useState<string | null>(null);
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  // Task 9b · 印记's per-turn OFFERS (铁律②: proposed, never auto-applied). A
  // note the student can confirm into the proposal board; a thinking-card she
  // can open. Both cleared at the start of the next turn and on project switch.
  const [pendingNote, setPendingNote] = useState<NoteProposal | null>(null);
  const [pendingCard, setPendingCard] = useState<CardProposalWire | null>(null);
  // After the student taps 记进「分区」, the actionable chip is replaced by a
  // quiet "记下了" acknowledgment (not a disappearance) so her tap has visible,
  // lasting confirmation. Cleared at the start of the next turn and on project
  // switch, exactly like `pendingNote`.
  const [confirmedNote, setConfirmedNote] = useState<NoteProposal | null>(null);
  // Task 7 (P2b) · 印记's per-turn `propose_question` OFFER — mirrors
  // `pendingNote` exactly, but confirming creates an exploration lead instead of
  // writing a proposal section.
  const [pendingQuestion, setPendingQuestion] = useState<QuestionProposal | null>(null);
  // Phase-agnostic link bridge · when the student drops a new URL in ANY phase,
  // 印记 offers to read it. Mirrors `pendingNote` (cleared at the next turn + on
  // project switch); the tap registers the reference and opens the reading room.
  const [pendingLinkOffer, setPendingLinkOffer] = useState<LinkOffer | null>(null);
  // The deterministic flow router's one-tap next-step offer (立项 done → 写提案,
  // etc.). Tapping it advances the status server-side (coachAdvance) — the ONLY
  // forward transition now the coach has no set_status/open_tool.
  const [pendingNextStep, setPendingNextStep] = useState<NextStep | null>(null);
  // Bumped after a confirmed question lands as an exploration lead, so the
  // exploration surface knows to re-fetch (threaded to ExplorationView in T8).
  const [explorationRefreshNonce, setExplorationRefreshNonce] = useState(0);
  // The AI-proposed card the student CHOSE to open — the only path to the shared
  // card sheet (triggering is automatic, opening is her tap · 铁律).
  const [openCardId, setOpenCardId] = useState<string | null>(null);
  // slice 3b · bumped after a 批注 review so the left ReferencePanel re-fetches
  // the proposal's layered colored 批注.
  const [annotationsVersion, setAnnotationsVersion] = useState(0);
  // Bug 7 · bumped when a coach turn added a keyword to the 还需要探索的 box
  // (reply.resourceNeedAdded) so NeedsResourcesBox re-fetches and shows it.
  const [needsVersion, setNeedsVersion] = useState(0);
  // Bug 2 follow-up · bumped when a coach turn mutated the plan (reply.planChanged)
  // so the 管理 board (PlanBlock) re-fetches and the change shows immediately.
  const [planVersion, setPlanVersion] = useState(0);
  // Live opened-project id for the rooms' cross-project append guard (see
  // StudioChatContext). Kept current every render so a late turn closure never
  // reads a stale value.
  const activeProjectIdRef = useRef<string | null>(null);
  activeProjectIdRef.current = projectId;
  // P3 · shared "insert a fragment into the draft at the caret" ref. The writing
  // room's DraftPane registers its inserter here on mount; the (sibling) left
  // ReferencePanel's 材料 fragments call it — the fold of the old floating
  // 材料 box, hoisted one level so the two SplitPane siblings share one path.
  const draftInsertRef = useRef<((t: string, referenceId?: string) => void) | null>(null);
  // Sibling bridge for S1 · click a 批注 → scroll+highlight the matching text in
  // the active writing surface (essay DraftPane or proposal ProsePane, whichever
  // is mounted registers it). Hoisted like draftInsertRef so ReferencePanel (left
  // SplitPane) reaches the draft (right SplitPane).
  const draftScrollRef = useRef<((a: { quote?: string; locator?: string }) => void) | null>(null);
  // Whether the writing draft (DraftPane, 正文 tab) is mounted + has registered
  // its inserter — gates the ReferencePanel 材料「插入」action so it is never a
  // dead no-op on the 大纲/片段 tabs (P3 review).
  const [insertReady, setInsertReady] = useState(false);

  // Apply a fresh directive from 印记: store it, and (unless it's chat-first)
  // swap the interactive area to the room it names. The load effect calls this
  // once on open (resume-at-stage); Task 9 calls it after every turn so the
  // status morphs live. chat is handled by the chat-first render, not a room,
  // so we don't touch `room` for it.
  const applyStudioState = useCallback((state: StudioState) => {
    setStudioState(state);
    // 印记 reasserting the view ends any manual takeover — the "继续印记
    // returns to status" seam (spec §6; Task 9/P4 build on this).
    setTookOver(false);
    // Any 印记-driven room morph leaves the recap landing (the load effect
    // re-sets it right after the initial apply, so the recap banner survives).
    setRecapLanding(false);
    if (state.openTool !== "chat") setRoom(roomForResume(state));
  }, []);

  // The switcher's manual override (the RoomSwitcher stage switcher):
  // swap the room AND flag the takeover so the chosen room mounts even while
  // 印记 is keeping chat primary (or its status hasn't loaded / failed). Does
  // not change `studioState` — manual browsing never changes 印记's status.
  const handleManualRoom = useCallback((r: BlockKey) => {
    setRoom(r);
    setTookOver(true);
    setRecapLanding(false);
    // §5 · a manual switch INTO the reading room asks to confirm first; any other
    // manual switch clears the gate. But the confirm-start nudge is only for a
    // LIVE project — once the essay has reached 回顾/retrospective (or the project
    // is finalized), reopening the reading room must land straight on the
    // already-built rabbit hole, never a "开始探索?" prompt (bug: it re-showed on
    // every re-entry of a done project and read as an empty/broken room).
    const alive = !studioState || studioState.stage !== "retrospective";
    setReadingConfirmNeeded(r === "reading" && alive);
  }, [studioState]);

  // 「继续印记」(P4, spec §6): while the student has manually taken over the
  // switcher, this re-asserts 印记's own view — re-fetch the current status
  // for freshness, then apply it (applyStudioState resets `tookOver` + opens
  // the status room). Falls back to the already-loaded studioState if the
  // refetch fails, so the control never dead-ends.
  const continueYinji = useCallback(async () => {
    const pid = activeProjectIdRef.current;
    if (!pid) return;
    try {
      const fresh = await getStudioState(pid);
      if (activeProjectIdRef.current === pid) applyStudioState(fresh);
    } catch {
      if (studioState) applyStudioState(studioState);
    }
  }, [applyStudioState, studioState]);

  // Re-pull the lean projection (title/qualification/proposal). Handed to rooms
  // so a persisted proposal edit can keep the rail in sync.
  const refreshWorkspace = useCallback(async () => {
    if (!projectId) return;
    try {
      const w = await getWorkspace(projectId);
      setWorkspace(w);
    } catch {
      /* keep the last-good projection; the room surfaces its own errors */
    }
    // Keep the plan spine in sync after a room mutates the plan (generate/edit).
    getPlan(projectId)
      .then(setPlanItems)
      .catch(() => {
        /* spine is a nicety; a failed refresh keeps the last-good stages */
      });
  }, [projectId]);

  // ── Task 9b · the ONE container-owned 印记 send loop ──────────────────────
  // Append the student turn, call the orchestrator (`coach(id, input)`), append
  // 印记's narration, APPLY the returned directive (auto-configure stage/room,
  // always overridable), and surface the reply's note/card OFFERS. Chat-first
  // AND both working rooms call this one loop, so the thread is continuous and
  // the status morphs live after every turn. Resolves true iff the turn landed
  // for the still-active project (so 写作 can clear its pinned part only then).
  const sendStudioTurn = useCallback(
    async (userInput: string, opts?: { quotedPart?: string }): Promise<boolean> => {
      const pid = activeProjectIdRef.current;
      if (!userInput.trim() || studioSending || !pid) return false;
      const isActive = () => activeProjectIdRef.current === pid;
      setStudioMessages((c) => [...c, { role: "student", text: userInput, quotedPart: opts?.quotedPart }]);
      setStudioSending(true);
      // A fresh turn clears any stale offer before the reply's own offers land.
      setPendingNote(null);
      setConfirmedNote(null);
      setPendingCard(null);
      setPendingQuestion(null);
      setPendingNextStep(null);
      setPendingLinkOffer(null);
      try {
        const reply = await coach(pid, userInput);
        if (!isActive()) return false;
        setStudioMessages((c) => [...c, { role: "ai", text: reply.narrate }]);
        // Task 7 · hidden-subagent post-hoc acknowledgments: `generate_plan`
        // and the backstop compaction ran silently during this awaited turn
        // (no per-phase SSE) — surface a one-line SubagentHint after 印记's
        // narrate so the student sees SOMETHING happened, without a second chat
        // exchange. Order: narrate first, then any hint lines.
        if (reply.planGenerated && !planIntroShownRef.current) {
          setStudioMessages((c) => [...c, { role: "ai", text: "", hint: "subagent 已整理研究计划" }]);
          // §3 gap G3 (situation a) · introduce the plan one-by-one in the chat.
          planIntroShownRef.current = true;
          showPlanIntro(0);
        }
        if (reply.compacted) {
          setStudioMessages((c) => [...c, { role: "ai", text: "", hint: "已整理较早的对话" }]);
        }
        // Bug 7 · 印记 added a keyword to the 还需要探索的 box this turn — refetch it.
        if (reply.resourceNeedAdded) setNeedsVersion((v) => v + 1);
        // Bug 2 follow-up · 印记 changed the plan this turn — refresh the board.
        if (reply.planChanged) setPlanVersion((v) => v + 1);
        // Slice 2 · the framework-readiness reviewer's read (surfaced once, the
        // turn after the plan auto-generated). Rendered as a 印记 bubble — real
        // coaching, not a subagent hint.
        appendFrameworkVerdict(reply.reviewVerdict, setStudioMessages);
        // 印记 auto-configures the view (spec: auto-configure, always overridable).
        applyStudioState(reply.directive);
        // Best-effort refresh: keep the container's planItems (the in-chat plan
        // walkthrough source) current after a plan-mutating tool call.
        getPlan(pid)
          .then((items) => {
            if (isActive()) setPlanItems(items);
          })
          .catch(() => {});
        setPendingNote(reply.note);
        setPendingCard(reply.card);
        setPendingQuestion(reply.question);
        setPendingNextStep(reply.nextStep ?? null);
        setPendingLinkOffer(reply.linkOffer ?? null);
        return true;
      } catch {
        if (isActive()) {
          setStudioMessages((c) => [...c, { role: "ai", text: "（网络好像有点卡，我没接住——再试一次？）" }]);
        }
        return false;
      } finally {
        if (isActive()) setStudioSending(false);
      }
    },
    [studioSending, applyStudioState],
  );

  // Task 6 (start gate) · the student's explicit 开始 tap: calls `coach/start`,
  // appends its narrate, and APPLIES the returned directive (started → true,
  // openTool → "forming") — applyStudioState's re-render is what reveals the
  // switcher/tabs (see `chatOnly`/`started` below), so no separate state flip
  // is needed here beyond the directive itself.
  const startJourney = useCallback(async (): Promise<void> => {
    const pid = activeProjectIdRef.current;
    if (!pid || starting) return;
    const isActive = () => activeProjectIdRef.current === pid;
    setStarting(true);
    try {
      const reply = await coachStart(pid);
      if (!isActive()) return;
      if (reply.narrate) setStudioMessages((c) => [...c, { role: "ai", text: reply.narrate }]);
      applyStudioState(reply.directive);
    } catch {
      if (isActive()) {
        setStudioMessages((c) => [...c, { role: "ai", text: "（网络好像有点卡，我没接住——再点一次「开始」？）" }]);
      }
    } finally {
      if (isActive()) setStarting(false);
    }
  }, [starting, applyStudioState]);

  // Advance the studio status forward (deterministic flow router, 铁律②: only on
  // a student action). Calls coachAdvance, appends 印记's greeting for the new
  // phase, applies the returned directive (stage + surface), refreshes the plan
  // AND the projection (so per-doc 完成写作 + status propagate), and carries any
  // fresh nextStep. Shared by the one-tap chip and the writing room's 完成 button
  // (proposal→essay, essay→review). Throws on failure so callers can surface it.
  const advanceStatusTo = useCallback(async (toStatus: string): Promise<void> => {
    const pid = activeProjectIdRef.current;
    if (!pid) return;
    const isActive = () => activeProjectIdRef.current === pid;
    setStudioSending(true);
    try {
      const reply = await coachAdvance(pid, toStatus);
      if (!isActive()) return;
      if (reply.narrate) setStudioMessages((c) => [...c, { role: "ai", text: reply.narrate }]);
      appendFrameworkVerdict(reply.reviewVerdict, setStudioMessages);
      applyStudioState(reply.directive);
      setPendingNextStep(reply.nextStep ?? null);
      getPlan(pid)
        .then((items) => {
          if (isActive()) setPlanItems(items);
        })
        .catch(() => {});
      await refreshWorkspace();
    } finally {
      if (isActive()) setStudioSending(false);
    }
  }, [applyStudioState, refreshWorkspace]);

  // §3 gap G3 · situation a · after the plan generates, 印记 introduces it
  // one-by-one IN THE CHAT (甘特图 → 看板 → 活动日志) with 下一步 buttons; the last
  // step advances to writing the proposal. A scripted client-side walkthrough.
  const showPlanIntro = useCallback((idx: number) => {
    const PLAN_INTRO = [
      "我根据你的框架整理了一份研究计划。先看这里默认的「甘特图」——它把整个研究按周排成一条时间线，你能一眼看到每件事大概在第几天做。",
      "再点上面的「看板」——任务分成 待办 / 进行中 / 完成 三列，你可以拖动卡片，随时更新自己的进度。",
      "最后是「活动日志」——它记录你一路上做了什么，大多会自动记下，你也可以自己补一笔。这份计划就是你接下来的地图。",
    ];
    const last = idx >= PLAN_INTRO.length - 1;
    setChatAction({
      id: `plan-intro-${idx}`,
      text: PLAN_INTRO[idx]!,
      actions: last
        ? [{ label: "开始写研究提案", primary: true, run: () => { setChatAction(null); void advanceStatusTo("proposal"); } }]
        : [{ label: "下一步 →", primary: true, run: () => showPlanIntro(idx + 1) }],
    });
  }, [advanceStatusTo]);

  // Finding C · the server funnel can auto-generate the plan on ANY proposal
  // write — a 记进 note-confirm OR a direct dim edit (e.g. filling 反例 in the
  // panel). Whichever path triggered it, introduce the plan walkthrough exactly
  // once. Shared by confirmNote and PlanBlock's edit-save so the edit path gets
  // the same loader + intro as note-confirm, not a silent plan.
  const surfaceGeneratedPlan = useCallback(async (pid: string) => {
    if (planIntroShownRef.current) return;
    const items = await getPlan(pid).catch(() => [] as PlanItem[]);
    if (items.length > 0 && activeProjectIdRef.current === pid) {
      planIntroShownRef.current = true;
      setPlanItems(items);
      showPlanIntro(0);
    }
  }, [showPlanIntro]);

  // Act on the one-tap nextStep (铁律②: her tap advances) — a thin wrapper over
  // advanceStatusTo that manages the chip (clear before, restore on failure).
  const advanceToNextStep = useCallback(async (): Promise<void> => {
    const step = pendingNextStep;
    if (!step || studioSending) return;
    setPendingNextStep(null);
    try {
      await advanceStatusTo(step.toStatus);
    } catch {
      setPendingNextStep(step); // restore the chip so she can retry
      setStudioMessages((c) => [...c, { role: "ai", text: "（网络好像有点卡，我没接住——再点一次？）" }]);
    }
  }, [pendingNextStep, studioSending, advanceStatusTo]);

  // Task 5 (history pagination) · page one OLDER page of the studio thread in,
  // prepending it above the currently-loaded messages. Guarded on a live
  // cursor + not-already-loading (StudioCoachChat also disables its button
  // while `loadingEarlier`, this is the belt-and-braces re-entrancy guard).
  const loadEarlier = useCallback(() => {
    const pid = activeProjectIdRef.current;
    if (!pid || !historyCursor || loadingEarlier) return;
    const isActive = () => activeProjectIdRef.current === pid;
    setLoadingEarlier(true);
    getCoachHistory(pid, "studio", { before: historyCursor })
      .then((page) => {
        if (!isActive()) return;
        setStudioMessages((prev) => [
          ...page.messages.map((m) => ({ role: m.role, text: m.text, card: m.card ?? null })),
          ...prev,
        ]);
        setHistoryCursor(page.nextCursor);
        setHistoryHasMore(page.hasMore);
      })
      .catch(() => {
        /* leave the cursor/hasMore as-is; the 载入更早 control stays so she can retry */
      })
      .finally(() => {
        if (isActive()) setLoadingEarlier(false);
      });
  }, [historyCursor, loadingEarlier]);

  // Confirm a proposed note into the proposal board (铁律②: her tap writes it).
  // Read-modify-write: re-read the current proposal, merge the note's value into
  // its section, persist, then refresh the board.
  //
  // `objective` is the single research question — the coach REFINES it turn by
  // turn (each 记进「目标」 supersedes the last, ending in the full RQ), and it also
  // drives the 研究问题 header + exploration-graph node. So the latest note REPLACES
  // it; newline-appending would pile the rough drafts on top of the final RQ. The
  // other sections (reason/activities/resources/counterpoints) are genuinely
  // multi-point, so a tap newline-joins there and never clobbers her own words.
  const confirmNote = useCallback(async () => {
    const pid = activeProjectIdRef.current;
    const note = pendingNote;
    if (!pid || !note) return;
    // Optimistic: swap the actionable chip for the "记下了" acknowledgment right
    // away so her tap reads as done, not vanished.
    setPendingNote(null);
    setConfirmedNote(note);
    try {
      const w = await getWorkspace(pid);
      const section = note.section as keyof Proposal;
      const existing = (w.proposal[section] ?? "").trim();
      const value =
        section === "objective" || !existing ? note.value : `${existing}\n${note.value}`;
      const merged: Proposal = { ...w.proposal, [section]: value };
      // §gap G2 · if this confirm fills the 4th required dim, the funnel will
      // generate the plan inside putProposal — show the plan-gen loader.
      const willGenPlan =
        !planIntroShownRef.current &&
        (["objective", "reason", "activities", "resources"] as (keyof Proposal)[]).every(
          (k) => (merged[k] ?? "").trim() !== "",
        );
      if (willGenPlan) setGeneratingPlan(true);
      await putProposal(pid, merged);
      if (activeProjectIdRef.current === pid) {
        await refreshWorkspace();
        // §3 gap G3 · confirming the 4th dim can auto-generate the plan server-side
        // (the funnel). If the plan just appeared this session, introduce it.
        await surfaceGeneratedPlan(pid);
      }
    } catch {
      // The write failed — roll back the acknowledgment and restore the
      // actionable chip so her tap isn't silently lost and she can retry, UNLESS
      // a newer offer already took the slot (don't clobber a fresher note the
      // next turn surfaced while this write was in flight).
      setConfirmedNote((cur) => (cur === note ? null : cur));
      setPendingNote((cur) => cur ?? note);
    } finally {
      setGeneratingPlan(false); // §gap G2
    }
  }, [pendingNote, refreshWorkspace]);

  const dismissNote = useCallback(() => setPendingNote(null), []);

  // Confirm a proposed question into an exploration lead (铁律②: her tap
  // creates it). Mirrors `confirmNote`'s resilience: clear the chip optimistically,
  // and on failure restore it UNLESS a newer offer already took the slot.
  const confirmQuestion = useCallback(async () => {
    const pid = activeProjectIdRef.current;
    const question = pendingQuestion;
    if (!pid || !question) return;
    setPendingQuestion(null);
    try {
      await createLead(pid, question.text);
      if (activeProjectIdRef.current === pid) setExplorationRefreshNonce((n) => n + 1);
    } catch {
      setPendingQuestion((cur) => cur ?? question);
    }
  }, [pendingQuestion]);

  const dismissQuestion = useCallback(() => setPendingQuestion(null), []);

  // Phase-agnostic link bridge · the student dropped a URL; her tap registers it
  // as a reference (best-effort — a failed create still clears the chip, she can
  // paste again). Read opens the reading room so she can 一起读; add just files it
  // in the library. 铁律②: 印记 offered, she chose.
  const registerLinkOffer = useCallback(
    async (pid: string, url: string) => {
      try {
        // Match the manual 链接/DOI add path: a cleaned, truncated title (not the
        // raw URL) + the 网页 classification, so the library/graph entry reads well.
        await createReference(pid, { url, title: url.replace(/^https?:\/\//, "").slice(0, 32), classification: "网页" });
        if (activeProjectIdRef.current === pid) {
          setExplorationRefreshNonce((n) => n + 1);
          void refreshWorkspace();
        }
      } catch {
        /* best-effort — the chip is already cleared */
      }
    },
    [refreshWorkspace],
  );

  const readLinkOffer = useCallback(() => {
    const pid = activeProjectIdRef.current;
    const offer = pendingLinkOffer;
    if (!pid || !offer) return;
    setPendingLinkOffer(null);
    void registerLinkOffer(pid, offer.url);
    setRoom("reading");
    setReadingConfirmNeeded(false);
  }, [pendingLinkOffer, registerLinkOffer]);

  const addLinkOffer = useCallback(() => {
    const pid = activeProjectIdRef.current;
    const offer = pendingLinkOffer;
    if (!pid || !offer) return;
    setPendingLinkOffer(null);
    void registerLinkOffer(pid, offer.url);
  }, [pendingLinkOffer, registerLinkOffer]);

  const dismissLinkOffer = useCallback(() => setPendingLinkOffer(null), []);

  // Opening a proposed card is the student's explicit choice (铁律). The sheet
  // (rendered at the container root) then records a coach turn on submit.
  const openCard = useCallback((cardId: string) => {
    setPendingCard(null);
    setOpenCardId(cardId);
  }, []);

  // Declining is an explicit "no": record it so 印记 stops offering this card.
  const dismissCard = useCallback((cardId: string) => {
    setPendingCard(null);
    const pid = activeProjectIdRef.current;
    if (pid) void dismissProposal(pid, cardId).catch(() => {});
  }, []);

  // Submit the opened card: persist the completed envelope AND get a coach turn
  // that RESPONDS to its content (reflectProjectCard), appending both the
  // content-first student chip and 印记's reply into the one continuous thread.
  // Surface tags the turn to the room the student is working in.
  const submitStudioCard = useCallback(
    async (fieldValues: Record<string, unknown>, eventTrace: unknown[]) => {
      const cardId = openCardId;
      setOpenCardId(null);
      const pid = activeProjectIdRef.current;
      if (!cardId || !pid) return;
      const isActive = () => activeProjectIdRef.current === pid;
      const spec = CARD_REGISTRY[cardId];
      const studentText = spec ? compileCardForCoach(spec, fieldValues) : "";
      const surface = room === "writing" ? "writing" : "forming";
      try {
        const { reply, card } = await reflectProjectCard(pid, cardId, fieldValues, eventTrace, surface);
        if (!isActive()) return;
        if (card) setStudioMessages((c) => [...c, { role: "student", text: studentText, card }]);
        else if (studentText) setStudioMessages((c) => [...c, { role: "student", text: studentText }]);
        if (reply) setStudioMessages((c) => [...c, { role: "ai", text: reply }]);
        else if (!card && !studentText) setStudioMessages((c) => [...c, { role: "ai", text: "这张卡还没填内容，先留着，想清楚了再来。" }]);
      } catch {
        if (!isActive()) return;
        if (studentText) setStudioMessages((c) => [...c, { role: "student", text: studentText }]);
        setStudioMessages((c) => [...c, { role: "ai", text: "刚才没接住这张卡，等下再试一次。" }]);
      }
    },
    [openCardId, room],
  );

  // Load the opened project's lean projection whenever the opened id changes.
  // Landing room is always 项目管理.
  useEffect(() => {
    if (!projectId) return;
    let cancelled = false;
    setWorkspace(null);
    setPlanItems([]);
    planIntroShownRef.current = false;
    setError(null);
    setSummary(null);
    // Reset the AI status back to "not yet loaded" so the shell shows the
    // chat-first landing (never the previous project's board) until this
    // project's studio_state resolves.
    setStudioState(null);
    // Task 6 fix round 1: reset the resolved flag too — else a project switch
    // could briefly inherit the PREVIOUS project's "resolved" (true) state
    // before this project's own getStudioState settles.
    setStudioStateResolved(false);
    // Reset the room too — else a project resumed into e.g. the reading room
    // leaves `room==="reading"` stuck while the NEXT project's real status is
    // still loading. applyStudioState below re-derives the real room once this
    // project's status resolves; "plan" is just the safe interim default
    // (mirrors the pre-load fallback).
    setRoom("plan");
    // Clear any manual takeover from the previous project — the new project
    // resumes at its own 印记 status.
    setTookOver(false);
    setStudioMessages([]);
    setChatAction(null);
    // Reset the pagination cursor/recap too — else a project switch could show
    // the PREVIOUS project's "载入更早" affordance or digest recap for a beat
    // before this project's first page resolves.
    setHistoryCursor(null);
    setHistoryHasMore(false);
    setHistoryRecap(null);
    setLoadingEarlier(false);
    // Reset the in-flight flag too — else a project opened while a PREVIOUS
    // project's turn is still in flight inherits sending=true and its composer
    // stays disabled until that unrelated reply resolves.
    setStudioSending(false);
    // Clear any stale per-turn offers / open card from the previous project.
    setPendingNote(null);
    setConfirmedNote(null);
    setPendingCard(null);
    setPendingQuestion(null);
    setPendingNextStep(null);
    setPendingLinkOffer(null);
    setOpenCardId(null);
    // Reset the room-effect's first-run guard for this new project, so its
    // getPlan fetch is skipped once here (this effect already fetches) rather
    // than firing a redundant duplicate on every project switch.
    didMountRoom.current = false;
    // Task 6 (start gate): reset the opening-fire guard + any stale busy flag
    // from the previous project.
    openingFiredForProjectId.current = null;
    setStarting(false);
    getPlan(projectId)
      .then((items) => {
        if (cancelled) return;
        setPlanItems(items);
        // A plan already exists on open → returning student (situation b): don't
        // replay the plan-introduction walkthrough.
        if (items.length > 0) planIntroShownRef.current = true;
      })
      .catch(() => {
        /* no plan yet (or fetch failed) → the spine simply doesn't render */
      });
    // Task 6 (start gate): coordinates the two loads below — once BOTH the
    // studio-state and the first history page have resolved for a project
    // that is genuinely not-yet-started with an empty thread, fire the ONE
    // real-AI opening turn. Local (not state) because it only needs to
    // survive within this effect's closure; `openingFiredForProjectId`
    // (component ref) is the actual re-render-proof double-fire guard.
    // A locally-narrowed alias: the effect's early `if (!projectId) return;`
    // above narrows `projectId` for direct use in this scope, but that
    // narrowing doesn't carry into the nested `maybeFireOpening` function
    // declaration below — `pid` is the `string` TS needs there.
    const pid = projectId;
    let stateLoaded = false;
    let loadedStarted = false;
    let historyLoaded = false;
    let historyEmpty = false;
    function maybeFireOpening() {
      if (cancelled || !stateLoaded || !historyLoaded) return;
      if (loadedStarted || !historyEmpty) return; // already started, or a resumed non-empty thread
      if (openingFiredForProjectId.current === pid) return;
      openingFiredForProjectId.current = pid;
      coachOpening(pid)
        .then((reply) => {
          if (cancelled) return;
          // The idempotent 200 (thread already has a turn — a race with
          // another tab/reload) narrates nothing; only seed when it did.
          if (reply.narrate) {
            setStudioMessages((prev) => (prev.length ? prev : [{ role: "ai", text: reply.narrate }]));
          }
          applyStudioState(reply.directive);
        })
        .catch(() => {
          // Best-effort: the pure chat-first landing (studioState.started
          // already false) still shows, just without the AI's opening line —
          // a reload retries.
        });
    }
    // Resume-at-stage (Task 8): land wherever 印记's AI-managed status says,
    // not on a forced plan board. `cancelled` guards a late response for a
    // project the student already switched away from. On error we simply stay
    // chat-first (studioState null) — chat is the safe primary surface, and
    // the opening never fires without a confirmed `started === false`.
    getStudioState(projectId)
      .then((state) => {
        if (cancelled) return;
        applyStudioState(state);
        // §3 situation b · a returning student past the framework (a plan
        // exists) first sees the 管理 plan for a recap, with a 继续工作 button
        // back to the current status. Gated on the working stages so a fresh
        // pre-plan project still lands chat-first / on 提案.
        const recapStages = ["proposal_writing", "proposal_review", "body_writing", "retrospective"];
        if (state.started && recapStages.includes(state.stage)) {
          setRoom("plan");
          setRecapLanding(true);
        }
        setStudioStateResolved(true);
        stateLoaded = true;
        loadedStarted = state.started;
        maybeFireOpening();
      })
      .catch(() => {
        /* no studio_state yet (or fetch failed) → stay on the chat-first landing.
           `studioStateResolved` still flips true here — a CONFIRMED failure
           (not "still loading") is what lets `started` fall back to `true`
           below, preserving the switcher-stays-live-on-error escape hatch. */
        if (!cancelled) setStudioStateResolved(true);
      });
    // Load the ONE continuous coach thread's RECENT PAGE ONCE per opened
    // project (立项 + 写作, surface="studio"), into the hoisted store both
    // rooms read. Empty → each room falls back to its own display-only
    // intro/greeting locally. `loadEarlier` (below) pages older turns in.
    getCoachHistory(projectId, "studio")
      .then((page) => {
        if (cancelled) return;
        // Don't clobber a turn the student optimistically sent in the small
        // window before this fetch resolved — only seed when still empty.
        setStudioMessages((prev) =>
          prev.length ? prev : page.messages.map((m) => ({ role: m.role, text: m.text, card: m.card ?? null })),
        );
        setHistoryCursor(page.nextCursor);
        setHistoryHasMore(page.hasMore);
        setHistoryRecap(page.recap);
        historyLoaded = true;
        historyEmpty = page.messages.length === 0;
        maybeFireOpening();
      })
      .catch(() => {
        /* keep the empty store; each room shows its intro and the next turn persists */
      });
    (async () => {
      try {
        const w = await getWorkspace(projectId);
        if (cancelled) return;
        setWorkspace(w);
        // Compose-on-first-open (server is first-open-wins → no repeat spend),
        // but only for an in-progress project.
        const p = w.proposal;
        const inProgress = [p.objective, p.reason, p.activities, p.resources].some(
          (s) => s.trim().length > 0,
        );
        if (inProgress) {
          postProjectSummary(projectId)
            .then((prose) => {
              if (!cancelled && prose.trim()) setSummary(prose);
            })
            .catch(() => {
              /* summary is a nicety; never block the room on it */
            });
        }
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId, applyStudioState]);

  function openProject(id: string) {
    setProjectId(id);
    // No forced landing room: the load effect resumes at 印记's status
    // (studio_state), so a proposal-stage project lands in chat, not a board.
    closeReadingSource();
  }

  function backToAll() {
    setProjectId(null);
    setWorkspace(null);
    closeReadingSource();
    setError(null);
  }

  // Refresh the plan spine when the student switches rooms — the plan is edited
  // in 立项, so navigating away is the natural moment to re-read its stages
  // (generation itself refreshes eagerly via refreshWorkspace). Skips the very
  // first render for each project (the load effect already fetched; it resets
  // this guard on project change).
  useEffect(() => {
    if (!projectId) return;
    if (!didMountRoom.current) {
      didMountRoom.current = true;
      return;
    }
    let cancelled = false;
    getPlan(projectId)
      .then((items) => {
        if (!cancelled) setPlanItems(items);
      })
      .catch(() => {
        /* keep last-good stages */
      });
    return () => {
      cancelled = true;
    };
  }, [room, projectId]);

  // Tell the shell whether a project is open, so it can hide the platform nav
  // for the immersive studio (spec §17). Fires on open/close and on unmount.
  useEffect(() => {
    onInProjectChange?.(projectId !== null);
    return () => onInProjectChange?.(false);
  }, [projectId, onInProjectChange]);

  // Open a finished project's process-evaluation report — routes up to the
  // 成长报告 tab, deep-linked to that project's entry (see StudentApp).
  const onViewReport = useCallback(
    (id: string) => {
      onFinished?.(id);
    },
    [onFinished],
  );

  // Open-from-home deep-link (Task 6): whenever `initialProjectId` changes to
  // a new, truthy id, open it — mirrors clicking that card in the directory.
  // A ref (not state) tracks the last id we acted on, so this only fires on
  // an actual change, never re-triggers after the student navigates away
  // (e.g. back to the directory) with the same prop value still passed down.
  const lastInitialProjectId = useRef<string | null>(null);
  useEffect(() => {
    if (initialProjectId && initialProjectId !== lastInitialProjectId.current) {
      lastInitialProjectId.current = initialProjectId;
      openProject(initialProjectId);
      onInitialProjectIdConsumed?.();
    }
  }, [initialProjectId, onInitialProjectIdConsumed]);

  // `pendingRoom` deep-link (P3 Task 1, guided tour): whenever it changes to a
  // new, truthy value, drive the manual switcher into that room — mirrors the
  // `initialProjectId` effect above, including the ref-guarded "only on
  // change" firing (a `lastPendingRoom` ref, not state, so re-consuming the
  // same room after the caller clears it never re-fires).
  const lastPendingRoom = useRef<BlockKey | null>(null);
  useEffect(() => {
    if (pendingRoom && pendingRoom !== lastPendingRoom.current) {
      lastPendingRoom.current = pendingRoom;
      handleManualRoom(pendingRoom);
      onPendingRoomConsumed?.();
    }
  }, [pendingRoom, onPendingRoomConsumed, handleManualRoom]);

  // No project open — the all-projects directory (its own create form carries
  // the empty affordance). `autoOpenCreate` (home's "新建" deep-link) is only
  // relevant here, one level in from the four-room shell.
  if (projectId == null) {
    return (
      <Directory
        onOpen={openProject}
        onViewReport={onViewReport}
        autoOpenCreate={autoOpenCreate}
        onAutoOpenCreateHandled={onAutoOpenCreateConsumed}
      />
    );
  }

  // A source open for reading replaces the whole workspace with the focused
  // ReadingRoom surface — one level in from the directory↔workspace swap.
  if (readingSource != null) {
    return (
      <ReadingRoom
        projectId={projectId}
        referenceId={readingRefId}
        source={readingSource}
        suggestedReason={readingSuggestedReason}
        phaseTag={readingPhaseTag}
        readingReason={readingReadingReason}
        readingFocus={readingReadingFocus}
        readingNote={readingReadingNote}
        bib={readingBib}
        onSaveNote={(note) => patchReference(projectId, readingRefId, { readingNote: note }).then(() => {})}
        reference={readingReference}
        onSetEvidence={(ev) => setReferenceEvidence(projectId, readingRefId, ev)}
        onSetTriage={(triage) => setReferenceTriage(projectId, readingRefId, triage)}
        onArchive={(archived) => archiveReference(projectId, readingRefId, archived)}
        onTraceCitation={(doi) => digExploration(projectId, { mode: "citation", doi }).then((r) => r.candidates)}
        onTraceSearch={(keyword) => digExploration(projectId, { mode: "similar", keyword }).then((r) => r.candidates)}
        onAdoptSource={(candidate) => adoptCandidate(projectId, candidate).then(() => {})}
        api={api}
        onBack={closeReadingSource}
      />
    );
  }

  // Morphing width (spec §3, P2a): 印记's `widthTier` drives the split between
  // the 印记 chat and the interactive area. `chat` → the chat IS the surface
  // (full width, no room); `half` → the chat as a prominent column beside a
  // ~half interactive area; `wide` → the interactive area fills, the chat
  // collapses toward a sidebar (and the student can collapse it further to the
  // slim rail). A manual takeover always shows a room, so it implies at least
  // `wide`. Null status
  // (still loading) = chat — we never flash a board before 印记's status lands.
  const widthTier: WidthTier = tookOver ? "wide" : (studioState?.widthTier ?? "chat");
  // Task 6 (start gate), fix round 1: THREE distinct states, not two.
  // - `studioState` present → the resolved truth: `studioState.started`.
  // - `studioState` null + NOT yet resolved (still in flight) → `false`
  //   (chat-only, 开始 button, no tabs) — this is what stops a brand-new
  //   project from flashing tabs/Composer for the whole network round-trip
  //   (the bug: defaulting to `true` here made the gate's "no tabs at all"
  //   promise hold only AFTER the fetch resolved, not during it). Matches
  //   `widthTier`'s own "safe/minimal default while loading" convention
  //   directly above.
  // - `studioState` null + resolved (a CONFIRMED fetch failure) → `true`,
  //   preserving the shell's pre-existing "switcher stays live as an escape
  //   hatch on a transient error" fallback — a resumed project hitting one
  //   flaky studio-state fetch must not lose its tabs and get stuck behind a
  //   开始 button it already passed.
  const started = studioState ? studioState.started : studioStateResolved;
  // The chat-only surface: no interactive area at all. `chatOnly` ⇒ the
  // full-width 印记 chat fills <main> INSTEAD of a room + side panel. Any
  // other tier ⇒ a room is mounted and the chat rides in the AiPanel. A
  // not-started project is unconditionally chat-only — stronger than the
  // width-tier gate alone, which a manual takeover could otherwise defeat.
  const chatOnly = !started || (widthTier === "chat" && !tookOver);
  // The expanded AiPanel's width follows the tier: a prominent 42% column in
  // `half`, the default sidebar in `wide` (collapsed always wins → slim rail).
  const aiPanelWidthClass = widthTier === "half" ? "w-[42%]" : "w-[320px]";
  const aiPanel = (
    <AiPanel
      side={aiSide}
      onFlip={flipAiSide}
      collapsed={aiCollapsed}
      onToggleCollapse={toggleAiCollapsed}
      widthClass={aiPanelWidthClass}
    >
      <div ref={aiSlotRef} data-tour="coach-rail" className="h-full" />
    </AiPanel>
  );
  // Task 3 (P2a): reading used to be the one exception — it owned its own
  // coach column (印记 · 找资料, a context-isolated "find_sources" thread) so
  // the constant panel would either sit empty beside it (list mode) or
  // duplicate it (graph mode). That room-owned coach is gone (ReadingBlock now
  // portals into the constant rail, same as every other room), so the
  // exception is gone too — the constant panel shows for every room. In
  // `chatOnly` there is no room and the chat fills <main> directly, so no side
  // panel renders at all.
  const showAiPanel = !chatOnly;

  // The ONE hoisted 印记 store — the continuous thread + the container-owned
  // send loop + note/card offers — shared by chat-first AND both working rooms
  // so there is a single source of truth for the conversation and 印记's status.
  const chatValue: StudioChatValue = {
    messages: studioMessages,
    setMessages: setStudioMessages,
    sending: studioSending,
    setSending: setStudioSending,
    activeProjectIdRef,
    sendStudioTurn,
    projectId,
    pendingNote,
    confirmedNote,
    pendingCard,
    confirmNote,
    dismissNote,
    openCard,
    dismissCard,
    // 提问卡 chatbox affordance: available only while the research question
    // isn't formed yet (proposal.objective empty) — retires the moment 目标 is
    // filled, mirroring the backend gate.
    questionCardAvailable: (workspace?.proposal?.objective?.trim() ?? "") === "",
    pendingQuestion,
    confirmQuestion,
    dismissQuestion,
    pendingLinkOffer,
    readLinkOffer,
    addLinkOffer,
    dismissLinkOffer,
    pendingNextStep,
    advanceToNextStep,
    recapContinue: recapLanding,
    onRecapContinue: () => void continueYinji(),
    chatAction,
    setChatAction,
    generatingPlan,
    advanceStatusTo,
    historyHasMore,
    loadEarlier,
    loadingEarlier,
    started,
    startJourney,
    starting,
    // Task 6 (P2, demo project): the shared, read-only demo project. The
    // backend 403s all writes regardless — this only disables the composer
    // so the demo reads honestly as read-only.
    isDemo: workspace?.isDemo ?? false,
  };

  return (
    <StudioChatContext.Provider value={chatValue}>
    <div className="flex h-full w-full flex-col bg-mk-paper font-sans text-mk-ink">
      <TopBar workspace={workspace} onBack={backToAll} />
      {/* Task 6 (P2, demo project): a small, honest read-only banner for the
          shared demo project. The backend is the real safety net (403s all
          writes for isDemo) — this is just legibility, not enforcement. */}
      {workspace?.isDemo && (
        <div className="shrink-0 border-b border-mk-border bg-mk-accent-50 px-6 py-2 text-mk-small font-medium text-mk-accent">
          演示项目 · 只读 — 这是一个示例项目，带你了解项目工作台
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        {aiSide === "left" && showAiPanel && aiPanel}
        <main className="relative flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* The persistent stage switcher lives at the top-left of the
            interactive area (spec §2) — beside the 印记 chat, not spanning it.
            AI-driven view changes flip `room`; this is the always-available
            manual override so the student is never lost. Task 6 (start gate):
            a not-started project renders NO tabs at all — stronger than the
            old chatOnly (which still rendered this row, just with no segment
            highlighted). The whole row (switcher + plan spine + 继续印记)
            only exists once the journey has actually started. */}
        {started && (
        <div data-tour="room-bar" className="flex shrink-0 items-center gap-3 overflow-x-auto border-b border-mk-border bg-mk-paper px-4 py-2">
          <RoomSwitcher
            // While chat-only (印记 keeps the chat primary), `room` is the stale
            // interim default — highlighting it would falsely mark a segment the
            // student isn't on. Pass a non-matching value so NO segment lights up
            // until a real room opens (live-journey finding).
            value={chatOnly ? "" : room}
            onChange={(v) => handleManualRoom(v as BlockKey)}
          />
          {/* 「继续印记」(P4, spec §6): shown only while the student has
              manually taken over the switcher — returns the view to 印记's
              own status step. Trailing end of the row so it reads as "back
              to the guided flow", not another switcher option. */}
          {tookOver && workspace && (
            <button
              type="button"
              onClick={continueYinji}
              className="ml-auto flex shrink-0 items-center gap-1 rounded-mk-lg border border-mk-accent bg-mk-accent-50 px-3 py-1.5 text-mk-body font-medium text-mk-accent"
            >
              继续印记 <span aria-hidden="true">→</span>
            </button>
          )}
        </div>
        )}
        <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        {/* The re-entry recap now lives INSIDE the continuous chat (passed as
            `recap` to the working rooms), not as a banner here — the chat is the
            primary surface. Only the reading carry-forward stays a toast. */}
        {workspace && carryForward && (
          <div className="pointer-events-none absolute inset-x-0 top-0 z-30 flex flex-col items-center gap-2 px-4 pt-4">
            {carryForward && (
              <div className="pointer-events-auto flex w-full max-w-2xl items-start gap-3 rounded-mk-lg border border-mk-accent/40 bg-mk-accent-50 px-4 py-3 shadow-mk-lg">
                <span className="mt-0.5 text-mk-accent">
                  <Icon name="spark" size={16} />
                </span>
                <p className="flex-1 text-[14px] leading-relaxed text-mk-ink">
                  刚读完《{carryForward}》——你确认的发现和判断已经带进来了，写作时印记都记得。
                </p>
                <button
                  type="button"
                  onClick={() => setCarryForward(null)}
                  aria-label="收起"
                  className="-mt-0.5 px-1 text-[16px] leading-none text-mk-faint hover:text-mk-ink"
                >
                  ×
                </button>
              </div>
            )}
          </div>
        )}
        {error ? (
          <div className="flex h-full items-center justify-center text-[14px] font-semibold text-mk-accent">{error}</div>
        ) : chatOnly ? (
          // Morphing width (spec §3): in the `chat` tier the 印记 chat IS the
          // surface — it fills the full content width, comfortably max-width-
          // centered, with NO interactive area and NO side rail. This renders
          // the ONE hoisted thread directly (no portal); the working rooms
          // instead portal their own coach into the AiPanel. `data-testid`
          // keeps the chat-first surface addressable in tests.
          <div
            data-testid="chat-first"
            className="mx-auto flex h-full w-full max-w-3xl flex-col overflow-hidden"
          >
            <StudioCoachChat recap={historyRecap ?? summary} />
          </div>
        ) : !workspace ? (
          <div className="flex h-full items-center justify-center text-[14px] text-mk-faint">加载中…</div>
        ) : (
          // StudioAiSlotContext: the room→panel portal contract. A room reads
          // `useStudioAiSlot()` and portals its coach content into the AiPanel's
          // body via `createPortal` — the room renders its WORK directly here in
          // <main>. plan / writing / reading / reflection ALL portal their
          // coach into the constant panel now (Task 3, P2a) — 印记 is the one
          // constant rail, no room owns its own coach column.
          <StudioAiSlotContext.Provider value={aiSlotEl}>
            {(room === "forming" || room === "plan") && (
              <PlanBlock
                key={projectId}
                projectId={projectId}
                title={workspace.title}
                qualification={workspace.qualification}
                proposal={workspace.proposal}
                createdAt={workspace.createdAt}
                phase={room === "forming" ? "forming" : "working"}
                refreshWorkspace={refreshWorkspace}
                recap={historyRecap ?? summary}
                onStudioStateChanged={continueYinji}
                onGeneratingPlan={setGeneratingPlan}
                onPlanMaybeGenerated={surfaceGeneratedPlan}
                planRefreshSignal={planVersion}
              />
            )}
            {room === "reading" && (
              <ReadingBlock
                key={projectId}
                projectId={projectId}
                title={workspace.title}
                setReadingSource={openReadingSource}
                refreshNonce={explorationRefreshNonce}
                essayStage={studioState?.essayTrack?.stage}
                onStudioStateChanged={continueYinji}
                confirmStart={readingConfirmNeeded}
                onConfirmStart={() => setReadingConfirmNeeded(false)}
                aiSide={aiSide}
              />
            )}
            {room === "writing" && (
              // Writing stage (spec §2/§6): the interactive area splits into a
              // read-only reference sub-pane (提案要点/阅读笔记/批注) and the
              // writing area, with a draggable divider. The coach still portals
              // to the constant AiPanel, independent of this split.
              <SplitPane
                storageKey="mk-studio-write-split"
                defaultRatio={0.34}
                left={
                  <ReferencePanel
                    key={projectId}
                    projectId={projectId}
                    reference={studioState?.reference ?? []}
                    stage={studioState?.stage ?? "body_writing"}
                    proposal={workspace.proposal}
                    onInsert={(t, referenceId) => draftInsertRef.current?.(t, referenceId)}
                    canInsert={insertReady}
                    annotationsVersion={annotationsVersion}
                    needsVersion={needsVersion}
                    showSnippets={writingTab === "draft"}
                    onOpenReading={() => { setRoom("reading"); setReadingConfirmNeeded(false); }}
                    onJumpToAnchor={(a) => draftScrollRef.current?.(a)}
                  />
                }
                right={(() => {
                  // Phase B · the active writing document follows the status. The
                  // proposal (写研究提案) and essay (写正文) are distinct docs; each
                  // locks on its OWN 完成写作 milestone (writingFinish[doc]).
                  const stageForDoc = studioState?.stage ?? "body_writing";
                  const stageDoc = activeDocForStage(stageForDoc);
                  // Both docs are viewable once the essay has begun (body_writing /
                  // review): the proposal is finished and worth looking back at.
                  // Before that, only the stage's own doc.
                  const docOptions: WritingDocKind[] =
                    stageForDoc === "body_writing" || stageForDoc === "retrospective"
                      ? ["proposal", "essay"]
                      : [stageDoc];
                  const writeDoc: WritingDocKind =
                    docOverride && docOptions.includes(docOverride) ? docOverride : stageDoc;
                  const docFinished =
                    writeDoc === "proposal"
                      ? (workspace.writingFinish?.proposal ?? false)
                      : (workspace.writingFinish?.essay ?? workspace.writingFinished ?? false);
                  // "Finalized" = the whole project has entered 回顾/review or beyond
                  // (the point past which finished work is read-only). Distinct from a
                  // doc's own 完成 (finishing the proposal to move to the essay is
                  // reversible and must NOT freeze its cards — the student can still
                  // edit them until the project itself is in review).
                  const finalized =
                    studioState?.stage === "retrospective" ||
                    workspace.status === "evaluating" ||
                    workspace.status === "done";
                  return (
                    <WritingBlock
                      key={`${projectId}:${writeDoc}`}
                      projectId={projectId}
                      title={workspace.title}
                      proposal={workspace.proposal}
                      status={workspace.status}
                      doc={writeDoc}
                      docOptions={docOptions}
                      onSwitchDoc={setDocOverride}
                      writingFinished={docFinished}
                      finalized={finalized}
                      draftInsertRef={draftInsertRef}
                      draftScrollRef={draftScrollRef}
                      onInsertReady={setInsertReady}
                      refreshWorkspace={refreshWorkspace}
                      recap={historyRecap ?? summary}
                      onOpenReading={() => { setRoom("reading"); setReadingConfirmNeeded(false); }}
                      onAnnotationsChanged={() => setAnnotationsVersion((v) => v + 1)}
                      essayStage={studioState?.essayTrack?.stage}
                      onStudioStateChanged={continueYinji}
                      onActiveTabChange={setWritingTab}
                    />
                  );
                })()}
              />
            )}
            {room === "reflection" && (
              <ReviewBlock
                key={projectId}
                projectId={projectId}
                proposal={workspace.proposal}
                status={workspace.status}
                // slice 4c · gate on the ESSAY's own finish (the paper is the
                // thing 回顾 reflects on), falling back to the legacy scalar.
                writingFinished={workspace.writingFinish?.essay ?? workspace.writingFinished ?? false}
                onFinished={backToAll}
              />
            )}
          </StudioAiSlotContext.Provider>
        )}
        </div>
        </main>
        {aiSide === "right" && showAiPanel && aiPanel}
      </div>
    </div>
    {/* slice 3a · the 提问卡 is a sub-agent card — route it to its chat modal
        (not the form sheet). Committing fills 目标, refreshes, and drops a
        card-used chip into the coach thread. */}
    {openCardId === "question-card" && (
      <QuestionCardModal
        projectId={projectId}
        onClose={() => setOpenCardId(null)}
        onCommitted={(objective) => {
          void refreshWorkspace();
          // §2 gap G1 · the 提问卡 is done → 印记 CONTINUES the thread with a real
          // coach turn (durable in history + it guides the next step), instead of
          // a client-only static line that vanished on reload.
          void sendStudioTurn(`我用提问卡把研究问题想清楚了：${objective}`);
        }}
      />
    )}

    {/* The shared card sheet for an AI-proposed card (openCard). Triggering is
        automatic; opening is the student's tap, and the sheet then fills the
        modal. Submit records a coach turn into the one continuous thread. */}
    {openCardId && openCardId !== "question-card" && CARD_REGISTRY[openCardId] && (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={() => setOpenCardId(null)}>
        <div className="max-h-[88vh] w-full max-w-lg overflow-y-auto rounded-mk-lg bg-mk-surface shadow-mk-lg" onClick={(e) => e.stopPropagation()}>
          <StudioCardSheet
            spec={CARD_REGISTRY[openCardId]}
            onSubmit={(env) => void submitStudioCard(env.field_values, env.event_trace)}
            onSkip={() => setOpenCardId(null)}
          />
        </div>
      </div>
    )}
    </StudioChatContext.Provider>
  );
}

// The top bar (spec §17): a small 「← 返回」capsule back to the project list
// (the Directory) and the project's title + qualification. The stage switcher no
// longer lives here — it moved into the interactive area's top-left (spec §2),
// beside the chat. Back returns to the student's OWN project list (not the
// platform home): "返回" is one level up, and the nav rail reappears there.
function TopBar({
  workspace,
  onBack,
}: {
  workspace: WorkspaceProjection | null;
  onBack: () => void;
}) {
  // Bug 4: the header prompt is content the student needs to read, not a hint.
  // Once the research question (目标) is confirmed it REPLACES the raw essay
  // prompt here — the question is what the project is now about. A tap/click
  // wraps the full text IN PLACE (bounded by the header's own width, so it can
  // never exceed the viewport the way an unbounded hover bubble did); the
  // native `title` attr is a no-overflow hover fallback. No custom Tooltip:
  // its `whitespace-nowrap`, unbounded bubble ran off-screen for long prompts.
  const [titleExpanded, setTitleExpanded] = useState(false);
  const essayPrompt = workspace?.title || "未命名项目";
  const researchQuestion = workspace?.proposal?.objective?.trim() || "";
  const showingQuestion = researchQuestion !== "";
  const headline = showingQuestion ? researchQuestion : essayPrompt;
  return (
    <header className={cx("flex shrink-0 items-center gap-4 border-b border-mk-border bg-mk-paper px-6 py-3")}>
      <button
        type="button"
        onClick={onBack}
        className={cx(
          "inline-flex shrink-0 items-center gap-1 rounded-mk-full border border-mk-border bg-mk-surface",
          "px-3 py-1.5 text-mk-small font-medium text-mk-muted transition-colors duration-[120ms] ease-mk",
          "hover:bg-mk-paper hover:text-mk-ink",
        )}
      >
        <UiIcon icon={ArrowLeft} size={14} />
        返回
      </button>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        {showingQuestion && (
          <span className="shrink-0 rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-mk-small font-semibold text-mk-accent">
            研究问题
          </span>
        )}
        <h1
          role="button"
          tabIndex={0}
          aria-label={titleExpanded ? "收起完整标题" : "展开完整标题"}
          aria-expanded={titleExpanded}
          title={showingQuestion ? `研究问题：${headline}\n\n题目：${essayPrompt}` : headline}
          onClick={() => setTitleExpanded((v) => !v)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              setTitleExpanded((v) => !v);
            }
          }}
          className={cx(
            "min-w-0 cursor-pointer text-mk-h2 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent",
            titleExpanded ? "whitespace-normal break-words" : "truncate",
          )}
        >
          {headline}
        </h1>
        <Badge tone="progress" className="shrink-0">
          {workspace?.qualification || "项目"}
        </Badge>
      </div>
    </header>
  );
}
