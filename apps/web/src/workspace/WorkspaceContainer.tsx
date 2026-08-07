import { useCallback, useEffect, useRef, useState } from "react";
import type {
  CardProposalWire,
  MaterialSource,
  NoteProposal,
  PhaseTag,
  PlanItem,
  Proposal,
  QuestionProposal,
  StudioState,
  WidthTier,
  WorkspaceProjection,
} from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { api } from "../api";
import { createLead } from "@/api/exploration";
import { ReadingRoom } from "../studio/reading/ReadingRoom";
import { AiPanel, type AiPanelSide } from "../studio/ai/AiPanel";
import { StudioAiSlotContext } from "../studio/ai/StudioAiSlot";
import { StudioChatContext, type StudioChatMsg, type StudioChatValue } from "../studio/ai/StudioChatContext";
import { StudioCoachChat } from "../studio/ai/StudioCoachChat";
import { StudioCardSheet } from "../studio/StudioCardSheet";
import { compileCardForCoach } from "../studio/compileCard";
import { Icon as UiIcon, ArrowLeft } from "@/ui/Icon";
import { Badge, Segmented } from "@/ui/feedback";
import { SplitPane } from "@/ui/SplitPane";
import { Icon, BLOCK_META } from "./Icon";
import { Directory } from "./Directory";
import {
  getWorkspace,
  getPlan,
  getCoachHistory,
  getStudioState,
  postProjectSummary,
  patchReference,
  coach,
  putProposal,
  reflectProjectCard,
  dismissProposal,
  type ReferenceBib,
} from "./api/workspace";
import { roomForResume } from "./studioResume";
import { PlanBlock } from "./blocks/PlanBlock";
import { PlanSpine } from "./blocks/PlanSpine";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
import { ReferencePanel } from "./blocks/ReferencePanel";
import { ReviewBlock } from "./blocks/ReviewBlock";
import type { BlockKey } from "./blocks/mockData";

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
  onExitToHome,
}: {
  onFinished?: (projectId?: string) => void;
  /** Fired when a project opens (true) or closes (false) — the shell uses
   * this to hide its platform nav for the immersive studio (spec §17). */
  onInProjectChange?: (inProject: boolean) => void;
  /** The studio top bar's 「← 主页」capsule — exits the immersive studio all
   * the way back to the home page (spec §17). Falls back to the internal
   * directory return when not supplied. */
  onExitToHome?: () => void;
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
}) {
  const [projectId, setProjectId] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<WorkspaceProjection | null>(null);
  // The project plan's items → the PlanSpine "你在这一步" indicator (spec §3).
  // Empty until a plan is generated; refreshed alongside the workspace.
  const [planItems, setPlanItems] = useState<PlanItem[]>([]);
  // First-run guard for the room-change plan refetch (declared here so the load
  // effect can reset it on project change). See the room-change effect below.
  const didMountRoom = useRef(false);
  const [room, setRoom] = useState<BlockKey>("plan");
  // 印记's AI-managed status directive (stage/openTool/widthTier/reference),
  // loaded once per project (Task 8). Drives resume-at-stage: which room the
  // shell lands on, and whether the interactive area is chat-first (openTool
  // === "chat") instead of a board. `null` = not yet loaded → render the
  // chat-first landing, never a forced plan board. Task 9 re-applies this
  // after every turn (morphing status).
  const [studioState, setStudioState] = useState<StudioState | null>(null);
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

  function openReadingSource(
    m: MaterialSource,
    referenceId: string,
    suggestedReason?: string,
    phaseTag?: PhaseTag | null,
    readingReason?: string | null,
    readingFocus?: string | null,
    readingNote?: string | null,
    bib?: ReferenceBib,
  ) {
    setReadingSourceState(m);
    setReadingRefId(referenceId);
    setReadingSuggestedReason(suggestedReason ?? "");
    setReadingPhaseTag(phaseTag ?? null);
    setReadingReadingReason(readingReason ?? null);
    setReadingReadingFocus(readingFocus ?? null);
    setReadingReadingNote(readingNote ?? null);
    setReadingBib(bib ?? null);
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
  // Task 9b · 印记's per-turn OFFERS (铁律②: proposed, never auto-applied). A
  // note the student can confirm into the proposal board; a thinking-card she
  // can open. Both cleared at the start of the next turn and on project switch.
  const [pendingNote, setPendingNote] = useState<NoteProposal | null>(null);
  const [pendingCard, setPendingCard] = useState<CardProposalWire | null>(null);
  // Task 7 (P2b) · 印记's per-turn `propose_question` OFFER — mirrors
  // `pendingNote` exactly, but confirming creates an exploration lead instead of
  // writing a proposal section.
  const [pendingQuestion, setPendingQuestion] = useState<QuestionProposal | null>(null);
  // Bumped after a confirmed question lands as an exploration lead, so the
  // exploration surface knows to re-fetch (threaded to ExplorationView in T8).
  const [explorationRefreshNonce, setExplorationRefreshNonce] = useState(0);
  // The AI-proposed card the student CHOSE to open — the only path to the shared
  // card sheet (triggering is automatic, opening is her tap · 铁律).
  const [openCardId, setOpenCardId] = useState<string | null>(null);
  // Live opened-project id for the rooms' cross-project append guard (see
  // StudioChatContext). Kept current every render so a late turn closure never
  // reads a stale value.
  const activeProjectIdRef = useRef<string | null>(null);
  activeProjectIdRef.current = projectId;
  // P3 · shared "insert a fragment into the draft at the caret" ref. The writing
  // room's DraftPane registers its inserter here on mount; the (sibling) left
  // ReferencePanel's 材料 fragments call it — the fold of the old floating
  // 材料 box, hoisted one level so the two SplitPane siblings share one path.
  const draftInsertRef = useRef<((t: string) => void) | null>(null);
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
    if (state.openTool !== "chat") setRoom(roomForResume(state));
  }, []);

  // The switcher's manual override (the Segmented stage switcher):
  // swap the room AND flag the takeover so the chosen room mounts even while
  // 印记 is keeping chat primary (or its status hasn't loaded / failed). Does
  // not change `studioState` — manual browsing never changes 印记's status.
  const handleManualRoom = useCallback((r: BlockKey) => {
    setRoom(r);
    setTookOver(true);
  }, []);

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
      setPendingCard(null);
      setPendingQuestion(null);
      try {
        const reply = await coach(pid, userInput);
        if (!isActive()) return false;
        setStudioMessages((c) => [...c, { role: "ai", text: reply.narrate }]);
        // 印记 auto-configures the view (spec: auto-configure, always overridable).
        applyStudioState(reply.directive);
        // Best-effort refresh: a `generate_plan` (or any plan-mutating) tool call
        // this turn needs to show on the PlanSpine without waiting for a remount.
        getPlan(pid)
          .then((items) => {
            if (isActive()) setPlanItems(items);
          })
          .catch(() => {});
        setPendingNote(reply.note);
        setPendingCard(reply.card);
        setPendingQuestion(reply.question);
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

  // Confirm a proposed note into the proposal board (铁律②: her tap writes it).
  // Read-modify-write: re-read the current proposal, append the note's value to
  // its section (newline-join when the section already has content, so a tap
  // never clobbers her own words), persist, then refresh the board.
  const confirmNote = useCallback(async () => {
    const pid = activeProjectIdRef.current;
    const note = pendingNote;
    if (!pid || !note) return;
    setPendingNote(null);
    try {
      const w = await getWorkspace(pid);
      const section = note.section as keyof Proposal;
      const existing = (w.proposal[section] ?? "").trim();
      const value = existing ? `${existing}\n${note.value}` : note.value;
      const merged: Proposal = { ...w.proposal, [section]: value };
      await putProposal(pid, merged);
      if (activeProjectIdRef.current === pid) await refreshWorkspace();
    } catch {
      // The write failed — restore the chip so her tap isn't silently lost and
      // she can retry, UNLESS a newer offer already took the slot (don't clobber
      // a fresher note the next turn surfaced while this write was in flight).
      setPendingNote((cur) => cur ?? note);
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
    setError(null);
    setSummary(null);
    // Reset the AI status back to "not yet loaded" so the shell shows the
    // chat-first landing (never the previous project's board) until this
    // project's studio_state resolves.
    setStudioState(null);
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
    // Reset the in-flight flag too — else a project opened while a PREVIOUS
    // project's turn is still in flight inherits sending=true and its composer
    // stays disabled until that unrelated reply resolves.
    setStudioSending(false);
    // Clear any stale per-turn offers / open card from the previous project.
    setPendingNote(null);
    setPendingCard(null);
    setPendingQuestion(null);
    setOpenCardId(null);
    // Reset the room-effect's first-run guard for this new project, so its
    // getPlan fetch is skipped once here (this effect already fetches) rather
    // than firing a redundant duplicate on every project switch.
    didMountRoom.current = false;
    getPlan(projectId)
      .then((items) => {
        if (!cancelled) setPlanItems(items);
      })
      .catch(() => {
        /* no plan yet (or fetch failed) → the spine simply doesn't render */
      });
    // Resume-at-stage (Task 8): land wherever 印记's AI-managed status says,
    // not on a forced plan board. `cancelled` guards a late response for a
    // project the student already switched away from. On error we simply stay
    // chat-first (studioState null) — chat is the safe primary surface.
    getStudioState(projectId)
      .then((state) => {
        if (!cancelled) applyStudioState(state);
      })
      .catch(() => {
        /* no studio_state yet (or fetch failed) → stay on the chat-first landing */
      });
    // Load the ONE continuous coach thread ONCE per opened project (立项 + 写作,
    // surface="studio"), into the hoisted store both rooms read. Empty → each
    // room falls back to its own display-only intro/greeting locally.
    getCoachHistory(projectId, "studio")
      .then((msgs) => {
        if (cancelled) return;
        // Don't clobber a turn the student optimistically sent in the small
        // window before this fetch resolved — only seed when still empty.
        setStudioMessages((prev) =>
          prev.length ? prev : msgs.map((m) => ({ role: m.role, text: m.text, card: m.card ?? null })),
        );
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
  // The chat-only surface: no interactive area at all. `chatOnly` ⇒ the
  // full-width 印记 chat fills <main> INSTEAD of a room + side panel. Any
  // other tier ⇒ a room is mounted and the chat rides in the AiPanel.
  const chatOnly = widthTier === "chat" && !tookOver;
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
      <div ref={aiSlotRef} className="h-full" />
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
    pendingCard,
    confirmNote,
    dismissNote,
    openCard,
    dismissCard,
    pendingQuestion,
    confirmQuestion,
    dismissQuestion,
  };

  return (
    <StudioChatContext.Provider value={chatValue}>
    <div className="flex h-full w-full flex-col bg-mk-paper font-sans text-mk-ink">
      <TopBar workspace={workspace} onBack={onExitToHome ?? backToAll} />
      <div className="flex min-h-0 flex-1">
        {aiSide === "left" && showAiPanel && aiPanel}
        <main className="relative flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* The persistent stage switcher lives at the top-left of the
            interactive area (spec §2) — beside the 印记 chat, not spanning it.
            AI-driven view changes flip `room`; this is the always-available
            manual override so the student is never lost. */}
        <div className="flex shrink-0 items-center gap-3 overflow-x-auto border-b border-mk-border bg-mk-paper px-4 py-2">
          <Segmented
            options={BLOCK_META.map((b) => ({ value: b.key, label: b.label }))}
            value={room}
            onChange={(v) => handleManualRoom(v as BlockKey)}
          />
          {/* The plan spine (spec §3): a read-only "你在这一步" indicator, shown
              once a plan exists. 印记 drives navigation; the switcher is the
              manual override — no next-step nudge chrome here (印记 cues it). */}
          <PlanSpine items={planItems} />
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
            <StudioCoachChat recap={summary} />
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
                recap={summary}
              />
            )}
            {room === "reading" && (
              <ReadingBlock
                key={projectId}
                projectId={projectId}
                title={workspace.title}
                setReadingSource={openReadingSource}
                refreshNonce={explorationRefreshNonce}
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
                    onInsert={(t) => draftInsertRef.current?.(t)}
                    canInsert={insertReady}
                  />
                }
                right={
                  <WritingBlock
                    key={projectId}
                    projectId={projectId}
                    title={workspace.title}
                    proposal={workspace.proposal}
                    status={workspace.status}
                    writingFinished={workspace.writingFinished ?? false}
                    draftInsertRef={draftInsertRef}
                    onInsertReady={setInsertReady}
                    refreshWorkspace={refreshWorkspace}
                    recap={summary}
                  />
                }
              />
            )}
            {room === "reflection" && (
              <ReviewBlock
                key={projectId}
                projectId={projectId}
                proposal={workspace.proposal}
                status={workspace.status}
                writingFinished={workspace.writingFinished ?? false}
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
    {/* The shared card sheet for an AI-proposed card (openCard). Triggering is
        automatic; opening is the student's tap, and the sheet then fills the
        modal. Submit records a coach turn into the one continuous thread. */}
    {openCardId && CARD_REGISTRY[openCardId] && (
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

// The top bar (spec §17): a small 「← 主页」capsule back to the Directory and
// the project's title + qualification. The stage switcher no longer lives here
// — it moved into the interactive area's top-left (spec §2), beside the chat.
function TopBar({
  workspace,
  onBack,
}: {
  workspace: WorkspaceProjection | null;
  onBack: () => void;
}) {
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
        主页
      </button>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <h1 className="truncate text-mk-h2">{workspace?.title || "未命名项目"}</h1>
        <Badge tone="progress" className="shrink-0">
          {workspace?.qualification || "项目"}
        </Badge>
      </div>
    </header>
  );
}
