import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Proposal, ProjectStatus } from "@mind-imprint/contracts";
import { putBuffer, runDraftReview } from "../../api/writing";
import type { ReviewItem, ReviewVoice, DraftReviewResult, WritingDocKind } from "../../api/writing";
import { finishWriting, reopenWriting } from "../../api/projects";
import { ApiError } from "../../api/client";
import { exportDraftDocx } from "../export";
import { Icon } from "../Icon";
import { useStudioAiSlot } from "@/studio/ai/StudioAiSlot";
import { useStudioChat, type StudioChatMsg } from "@/studio/ai/StudioChatContext";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { withRecap } from "@/studio/ai/RecapHint";
import { Composer } from "@/studio/ai/Composer";
import { StudioTurnChips } from "@/studio/ai/StudioCoachChat";
import { Segmented, ReviewingHint } from "@/ui";
import { getOutline, putOutline, getSnippets, putSnippets, getDraft, reflectProjectCard } from "../api/workspace";
import { getProposalTrack } from "../../api/proposalTrack";
import { getEssayStatement } from "../../api/essayStatement";
import { getEssaySubmission } from "../../api/essaySubmission";
import type { SubQuestion } from "@mind-imprint/contracts";
import { guidedSectionLabel, isGuidedSection, sectionDoc } from "./sectionLabels";
import { FilledCardsFold, type FilledCard } from "./FilledCardsFold";
import { partSectionKey } from "./docSections";
import { scheduleCardRevision } from "../../api/revision";
import { recordCitation } from "../../api/citations";
import { parseSections, serializeSections, sectionsFromOutline, newSection, type DraftSection } from "./draftSections";
import { MarkdownPreview } from "./MarkdownPreview";
import { ProposalGuidePane } from "./ProposalGuide";
import { ProsePane } from "./ProsePane";
import { EssayStatementPane } from "./EssayStatementGuide";
import { EssaySubmissionPane } from "./EssaySubmissionGuide";
import { getProposalAnnotations, reviewProposalAnnotations } from "../../api/proposalAnnotations";
import { StudioCardSheet } from "../../studio/StudioCardSheet";
import { compileCardEnvelope, compileCardForCoach } from "../../studio/compileCard";
import { CARD_REGISTRY, type CardTurnRef } from "@mind-imprint/contracts";
import { CardTurnChip } from "./CardTurnChip";

// One outline bullet in local edit shape — flat-with-depth, the same model the
// prototype used (the persisted OutlineNode adds a server-owned `position`,
// which the array order carries here).
type Row = { id: string; text: string; depth: number };
// #9-second · quotedPart carries a referenced paragraph SEPARATELY from the
// question text, so the rail can render it as a styled callout above the
// bubble instead of baking a literal 【就这一段】 token into the message string.
// `card`, when set, marks a card-turn: the bubble renders as a content-first
// clickable chip opening a read-only view of the student's answers (not raw text).
type ChatMsg = { role: "ai" | "student"; text: string; quotedPart?: string; card?: CardTurnRef | null };

// Max outline nesting depth (0 = top level). Indent clamps here.
const MAX_DEPTH = 2;

// A monotonic client-side id for freshly-added rows before the server mints a
// real one. Any string is fine — the server re-assigns ids on every PUT.
let tempSeq = 0;
const tempId = () => `tmp-${tempSeq++}`;

// ── Outline keyboard reducer (pure, tested) ────────────────────────────────
// The keystroke semantics of a real outliner, factored out so the behavior can
// be unit-tested without a DOM:
//   enter     → insert a blank sibling right below at the same depth, focus it
//   indent    → depth + 1 (clamped ≤ MAX_DEPTH); caret stays put (no refocus)
//   outdent   → depth − 1 (clamped ≥ 0); caret stays put
//   backspace → only meaningful on an empty, non-first row: delete it and put
//               the caret at the end of the previous row
// `focus` is null when the caret should stay where the browser already has it
// (indent/outdent don't reorder the DOM, so focus is naturally retained).
export type OutlineKeyType = "enter" | "indent" | "outdent" | "backspace";
export type OutlineKeyResult = { rows: Row[]; focus: { id: string; atEnd: boolean } | null };

export function outlineKey(
  rows: Row[],
  type: OutlineKeyType,
  id: string,
  makeId: () => string = tempId,
): OutlineKeyResult {
  const i = rows.findIndex((r) => r.id === id);
  if (i < 0) return { rows, focus: null };
  const row = rows[i]!;
  switch (type) {
    case "enter": {
      const nid = makeId();
      const next = [...rows];
      next.splice(i + 1, 0, { id: nid, text: "", depth: row.depth });
      return { rows: next, focus: { id: nid, atEnd: false } };
    }
    case "indent": {
      const depth = Math.min(MAX_DEPTH, row.depth + 1);
      if (depth === row.depth) return { rows, focus: null };
      return { rows: rows.map((r, idx) => (idx === i ? { ...r, depth } : r)), focus: null };
    }
    case "outdent": {
      const depth = Math.max(0, row.depth - 1);
      if (depth === row.depth) return { rows, focus: null };
      return { rows: rows.map((r, idx) => (idx === i ? { ...r, depth } : r)), focus: null };
    }
    case "backspace": {
      // Only delete when the row is empty and there's a previous row to merge
      // the caret onto. Otherwise a no-op (the caller lets the default run).
      if (row.text !== "" || i === 0) return { rows, focus: null };
      const prev = rows[i - 1]!;
      return { rows: rows.filter((_, idx) => idx !== i), focus: { id: prev.id, atEnd: true } };
    }
  }
}

// The Write block: two gears — 提纲 (outline) and 写作 (a single draft panel) —
// with a slim goal strip up top so you write against your thesis, and an AI
// rail that talks about your outline/draft (never writes it). The proposal
// stays in Project Management; here we only link back to it. Outline + draft are
// API-backed (slice 4): outline edits debounce to PUT /outline, the draft
// debounces to PUT /buffer, and the rail calls POST /coach (writing scope).
export function WritingBlock({
  projectId,
  title,
  proposal,
  status,
  doc = "essay",
  docOptions,
  onSwitchDoc,
  writingFinished,
  finalized = false,
  draftInsertRef,
  draftScrollRef,
  onInsertReady,
  refreshWorkspace,
  recap,
  onOpenReading,
  onAnnotationsChanged,
  essayStage,
  onStudioStateChanged,
  onActiveTabChange,
  forceTab,
  onForceTabConsumed,
}: {
  projectId: string;
  title: string;
  proposal: Proposal;
  status: ProjectStatus;
  // Phase B · which document this writing room is editing, derived from the
  // studio status (proposal_writing → "proposal", body_writing → "essay"). The
  // proposal renders a plain prose surface (ProsePane); the essay keeps
  // 大纲/片段/正文. Buffer/snapshots/finish are keyed on it end-to-end.
  doc?: WritingDocKind;
  // #83 · the documents the student may switch between here (proposal ⇆ 正文).
  // The container auto-selects `doc` for the current stage; when both are
  // offered a small toggle lets her look back at the finished proposal while
  // writing the essay. undefined / single-entry → no toggle.
  docOptions?: WritingDocKind[];
  onSwitchDoc?: (doc: WritingDocKind) => void;
  // #20 · the 完成写作 milestone for THIS document (Phase B: the active doc's
  // finish state) — the surface is read-only once true. Separate from status
  // (evaluating/done terminally lock too).
  writingFinished: boolean;
  // Project-level finalize: true once the project has entered 回顾/review or beyond
  // (evaluating/done). The finished guided cards are re-editable until THIS is
  // true — a doc's own reversible 完成 (writingFinished) never freezes them, so a
  // finished proposal's parts stay editable while the essay is being written.
  finalized?: boolean;
  /** Shared insert-at-caret ref (P3): DraftPane registers its inserter here on
   * mount; the sibling left ReferencePanel's 材料 fragments call it (the fold
   * of the old floating 材料 box). Hoisted to WorkspaceContainer. Optional — an
   * isolated unit render falls back to a local ref. */
  draftInsertRef?: { current: ((t: string, referenceId?: string) => void) | null };
  /** S1 · shared scroll-to-anchor ref: the active writing surface (essay
   * DraftPane / proposal ProsePane) registers a fn that finds a 批注's quote or
   * 第N段 locator in the draft and scrolls+selects it. Clicked from the sibling
   * ReferencePanel's 批注. Hoisted to WorkspaceContainer. */
  draftScrollRef?: { current: ((a: { quote?: string; locator?: string }) => void) | null };
  /** Notified when DraftPane registers (正文 mounted) / unregisters its inserter,
   * so the container can gate the ReferencePanel 「插入」 action (P3 review). */
  onInsertReady?: (ready: boolean) => void;
  // Re-pull the projection so a 完成写作 / 重新打开写作 toggle propagates to both
  // rooms (WritingBlock's lock + ReviewBlock's gate) without a full remount.
  refreshWorkspace: () => Promise<void> | void;
  /** Re-entry recap shown as 印记's opening note inside the continuous chat. */
  recap?: string | null;
  /** slice 3a · open the reading room (proposal guide's needs-resources /
   * explore-first jumps). Provided by the container's room switcher. */
  onOpenReading?: (note?: string) => void;
  /** slice 3b · fired after a 批注 review so the left panel re-fetches. */
  onAnnotationsChanged?: () => void;
  /** slice 4b · the essay stage; the statement guide shows when "statement". */
  essayStage?: string;
  /** slice 4b-2 · re-assert 印记's studio state after the statement→submission
   * advance so `essayStage` flips and the statement pane unmounts. */
  onStudioStateChanged?: () => void;
  /** Reports the active writing tab (大纲/片段/正文) up so the container can, e.g.,
   * surface the student's 片段 in the left panel only while they're on 正文. */
  onActiveTabChange?: (tab: "outline" | "snippets" | "draft") => void;
  /** Guided-tour deep-link (P6, Task 9): force the active tab (大纲/片段/正文) once.
   * A ref-guarded one-shot (mirrors WorkspaceContainer's `pendingRoom` /
   * ReadingBlock's `forceView`) so it never fights the student's own later
   * tab clicks — `onForceTabConsumed` retracts it right after it's applied. */
  forceTab?: "outline" | "snippets" | "draft" | null;
  onForceTabConsumed?: () => void;
}) {
  // The proposal opens on 片段 — where its cards live and where the current part
  // is written; the essay opens on 大纲. Doc-scoped because WritingBlock remounts
  // per doc (key includes the doc), so this re-picks when the student toggles.
  const [tab, setTab] = useState<"outline" | "snippets" | "draft">(doc === "proposal" ? "snippets" : "outline");
  // Report the active tab up (the left panel surfaces 片段 only on 正文). Reset to
  // a neutral tab on unmount so a stale "draft" never lingers after leaving.
  useEffect(() => {
    onActiveTabChange?.(tab);
    return () => onActiveTabChange?.("outline");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab]);
  // Guided-tour deep-link (P6, Task 9): apply `forceTab` into the live tab once.
  // Ref-guarded (mirrors ReadingBlock's `forceView`) so each new value applies
  // exactly once and the student's own later tab clicks are never re-fought; the
  // `onForceTabConsumed` retraction stops a stale-but-truthy value re-applying.
  const lastForcedTab = useRef<string | null>(null);
  useEffect(() => {
    if (!forceTab || lastForcedTab.current === forceTab) return;
    lastForcedTab.current = forceTab;
    setTab(forceTab);
    onForceTabConsumed?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [forceTab]);
  // WC · part-by-part: the draft part the student has pinned to think through
  // with 印记 (lifted so DraftPane can set it and the rail can consume it).
  const [focusPart, setFocusPart] = useState<string | null>(null);
  // #8-second · a voice-scoped 体检 requested from the persistent rail shelf
  // (CoachRail, a tab-sibling of DraftPane) — lifted here as a queued request
  // rather than a direct call, since the shelf is reachable from any tab and
  // DraftPane may not be mounted yet. requestReview switches to 正文 (so the
  // review panel is visible) and queues the request; DraftPane's own
  // mount-effect flushes it once it's actually mounted (and its draft loaded).
  const [pendingReview, setPendingReview] = useState<{ scope?: string; voice: ReviewVoice } | null>(null);
  function requestReview(scope: string | undefined, v: ReviewVoice) {
    setTab("draft");
    setPendingReview({ scope, voice: v });
  }
  // #5/#20 · 完成写作 is a guarded moment. Clicking it opens a confirm modal;
  // confirming locks the draft read-only (writing_finished_at) AND routes to 回顾
  // (which only unlocks once writing is finished). Reversible via 重新打开写作
  // until the project is archived. Once archived (评估中 / 已完成) the draft is
  // terminally read-only — the true point of no return.
  const [showFinishModal, setShowFinishModal] = useState(false);
  const [finishingWriting, setFinishingWriting] = useState(false);
  const [finishWritingError, setFinishWritingError] = useState<string | null>(null);
  // §99 · after finishing the proposal, a congrats modal offers export + 继续 to
  // the next step (essay). proposalExportText holds the proposal buffer to export.
  const [showProposalCongrats, setShowProposalCongrats] = useState(false);
  const [proposalExportText, setProposalExportText] = useState("");
  // slice 3b · the finish-proposal comment-first flow (§4). null = not yet
  // checked; the count of the proposal's current 批注 (0 → offer a review first).
  const [proposalAnnoCount, setProposalAnnoCount] = useState<number | null>(null);
  const [runningCheck, setRunningCheck] = useState(false);
  // Phase B · finishing a document advances the studio status deterministically
  // (proposal → essay; essay → review). One student tap (the 完成 button IS the
  // confirmation — 铁律②). advanceStatusTo lives on the hoisted coach store.
  // Task 6 (P2, demo project): the shared, read-only demo. The backend 403s
  // the write regardless — this just keeps the primary 完成 button from
  // inviting a tap that would only bounce off a 403.
  const { advanceStatusTo, sendStudioTurn, isDemo } = useStudioChat();
  const isProposal = doc === "proposal";
  // archived = the terminal finalize path has begun (can't reopen writing then).
  const archived = status === "evaluating" || status === "done";
  const locked = archived || writingFinished;

  // #20 / Phase B · confirm → lock THIS document, then advance the studio status
  // (proposal→写正文; essay→复盘). finishWriting is idempotent; 422 draft_empty
  // when there's nothing written yet.
  async function doFinishWriting() {
    if (finishingWriting) return;
    setFinishingWriting(true);
    setFinishWritingError(null);
    try {
      await finishWriting(projectId, doc);
      await refreshWorkspace();
      setShowFinishModal(false);
      if (isProposal) {
        // §99 · celebrate + offer export before moving on; 继续 advances to essay.
        setProposalExportText(await getDraft(projectId, "proposal").catch(() => ""));
        setShowProposalCongrats(true);
      } else {
        await advanceStatusTo("review");
      }
    } catch (e) {
      setFinishWritingError(
        e instanceof ApiError ? e.message || "还不能完成写作，请稍后再试。" : "刚才没接上，稍等再试一次。",
      );
    } finally {
      setFinishingWriting(false);
    }
  }

  // slice 3b · opening the finish modal for a proposal first checks whether 印记
  // has ever commented; if not, the modal offers a review before locking (§4).
  async function openFinish() {
    setProposalAnnoCount(null);
    setShowFinishModal(true);
    if (isProposal) {
      try {
        setProposalAnnoCount((await getProposalAnnotations(projectId)).length);
      } catch {
        setProposalAnnoCount(0);
      }
    }
  }

  // "先让印记看一遍" — run the whole-draft 批注 review, then close so the student
  // sees the 批注 in the left panel (they can re-open 完成提案 when ready).
  async function runCommentFirst() {
    if (runningCheck) return;
    setRunningCheck(true);
    try {
      await reviewProposalAnnotations(projectId);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort */
    } finally {
      setRunningCheck(false);
      setShowFinishModal(false);
    }
  }

  // Bug 5a · the whole-paper 批注 review, reachable directly from the proposal
  // 正文 tab (not only buried in the 完成提案 modal). 印记 reads the assembled
  // proposal and leaves colored 批注 in the left 材料·AI批注 panel — never
  // rewrites it (铁律①). Same call as runCommentFirst, minus the modal close.
  async function runProposalReview() {
    if (runningCheck) return;
    setRunningCheck(true);
    try {
      await reviewProposalAnnotations(projectId);
      onAnnotationsChanged?.();
    } catch {
      /* best-effort */
    } finally {
      setRunningCheck(false);
    }
  }

  // #20 (铁律②) · reopen — reversible until the project is archived.
  async function doReopenWriting() {
    try {
      await reopenWriting(projectId, doc);
      await refreshWorkspace();
    } catch {
      /* best-effort; the affordance stays and can be retried */
    }
  }
  // #9 · DraftPane registers its insert-at-caret fn into the shared
  // `draftInsertRef` (a prop, hoisted to WorkspaceContainer) on mount; the left
  // ReferencePanel's 材料 fragments call it when 正文 is active. Falls back to a
  // local ref when the prop is absent (isolated unit tests).
  const localDraftInsertRef = useRef<((t: string, referenceId?: string) => void) | null>(null);
  const insertTarget = draftInsertRef ?? localDraftInsertRef;
  // S1 · same pattern for the scroll-to-anchor bridge (批注 click → jump).
  const localDraftScrollRef = useRef<((a: { quote?: string; locator?: string }) => void) | null>(null);
  const scrollTarget = draftScrollRef ?? localDraftScrollRef;
  const snip = useSnippets(projectId);
  // #6 · the outline headings the student has explicitly imported as snippet
  // board sections (see importedSectionsMemo above) — lifted here so both the
  // 片段 board (renders them as foldable groups) and the materials sidebar
  // (the import button + its "already imported" state) share one source of
  // truth, and it survives switching tabs (SnippetsPane unmounts on tab-away).
  const [importedSections] = useState<string[]>(
    () => importedSectionsMemo.get(projectId) ?? [],
  );
  // Section labels a card's compiled 片段 can be filed under (Item C's 收进片段
  // offer): the imported outline groups plus any label already in live use on a
  // snippet (e.g. an探索线索 the student filed one under).
  const knownSectionLabels = useMemo(
    () => dedupe([...importedSections, ...snip.snippets.map((s) => s.section).filter((x): x is string => !!x)]),
    [importedSections, snip.snippets],
  );
  return (
    <div className="flex h-full flex-col">
      {/* goal strip */}
      <div className="flex items-center gap-3 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        {/* #83 · proposal ⇆ 正文 switch — only when both are available (the essay
            has begun). Auto-selected to the current doc; lets the student look
            back at the finished proposal without leaving the room. */}
        {docOptions && docOptions.length > 1 ? (
          <div data-tour="writing-docswitch" className="flex flex-none items-center gap-0.5 rounded-mk-full border border-mk-border bg-mk-paper p-0.5">
            {docOptions.map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => onSwitchDoc?.(d)}
                className={
                  "rounded-mk-full px-2.5 py-0.5 text-[12px] font-bold transition-colors " +
                  (doc === d ? "bg-mk-accent text-white" : "text-mk-muted hover:text-mk-accent")
                }
              >
                {d === "proposal" ? "提案" : "正文"}
              </button>
            ))}
          </div>
        ) : (
          <span className="flex-none rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">{isProposal ? "提案" : "论点"}</span>
        )}
        <p className="min-w-0 flex-1 truncate text-[14px] text-mk-ink">{proposal.objective || "还没有写下你的论点——先去开题里想清楚。"}</p>
        {/* Task 9 (P6 demo): a finished demo is archived, which normally hides the
            finish/reopen area entirely — but the tour must spotlight 完成写作
            (data-tour="writing-finish"). So for the demo we ALWAYS render the
            完成写作 button branch (disabled, read-only), even when archived. The
            backend 403s the write regardless; disabled here means the click can
            never even reach openFinish (no confirm modal, no bounce). */}
        {(!archived || isDemo) &&
          (isDemo ? (
            <button
              type="button"
              data-tour="writing-finish"
              disabled
              className="flex-none rounded-mk-md bg-mk-accent px-3 py-1 text-[12px] font-bold text-white disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-mk-accent"
              title="演示项目为只读，无法完成"
            >
              {isProposal ? "完成提案" : "完成写作"}
            </button>
          ) : writingFinished ? (
            // #20 · reversible — 重新打开 unlocks this document again (铁律②).
            <button type="button" onClick={() => void doReopenWriting()} className="flex-none rounded-mk-md border border-mk-border px-3 py-1 text-[12px] font-bold text-mk-muted hover:text-mk-accent" title={isProposal ? "重新编辑提案" : "重新打开写作，继续修改初稿"}>{isProposal ? "重新编辑提案" : "重新打开写作"}</button>
          ) : (
            <button
              type="button"
              data-tour="writing-finish"
              onClick={() => void openFinish()}
              className="flex-none rounded-mk-md bg-mk-accent px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600"
              title={
                isProposal
                  ? "提案写好了？点这里定稿，进入写正文"
                  : "写完了？点这里锁定初稿、进入回顾（之后仍可重新打开）"
              }
            >
              {isProposal ? "完成提案" : "完成写作"}
            </button>
          ))}
      </div>

      {/* tabs — 大纲/片段/正文, for BOTH the proposal and the essay. The proposal's
          正文 is its assembled prose (ProsePane); the essay's is the full draft. */}
      <div data-tour="writing-tabs" className="flex items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2.5">
        <Tab active={tab === "outline"} onClick={() => setTab("outline")} icon="plan">大纲</Tab>
        <Tab active={tab === "snippets"} onClick={() => setTab("snippets")} icon="spark">片段</Tab>
        <Tab active={tab === "draft"} onClick={() => setTab("draft")} icon="writing">正文</Tab>
      </div>

      {/* The ESSAY's guided statement/submission walk sits above the tabs on ALL
          tabs (gated !locked): height-capped on the 正文 tab, natural on 大纲/片段. */}
      {!isProposal && !locked && (essayStage === "statement" || essayStage === "submission") && (
        <div className={tab === "draft" ? "max-h-[32vh] shrink-0 overflow-y-auto" : "shrink-0"}>
          {essayStage === "statement" ? (
            <EssayStatementPane projectId={projectId} onAnnotationsChanged={onAnnotationsChanged} onStageAdvanced={onStudioStateChanged} />
          ) : (
            <EssaySubmissionPane
              projectId={projectId}
              onGoToDraft={() => setTab("draft")}
              onRequestFinish={() => void openFinish()}
            />
          )}
        </div>
      )}

      <div className="relative flex min-h-0 flex-1 flex-col">
        {tab === "outline" ? (
          <OutlinePane projectId={projectId} title={title} doc={isProposal ? "proposal" : "essay"} />
        ) : tab === "snippets" ? (
          isProposal ? (
            // 片段 · the proposal's writing surface — where ALL its cards live
            // (writing a part = writing a snippet). ONE header, then the current
            // part's writing card (gated !locked), then the finished parts +
            // collected snippets — all in a single scroll, no divider between them.
            // 正文 stays the assembled paper only.
            <div className="min-h-0 flex-1 overflow-y-auto">
              <div className="px-8 pt-6 pb-2">
                <div className="mx-auto max-w-2xl">
                  <h2 className="font-sans text-[18px] font-bold text-mk-ink">片段</h2>
                  <p className="mt-1 text-[14px] text-mk-muted">攒下引文、笔记、灵光一现的句子——把它们归到大纲的章节或探索的线索下（拖 ⠿ 或用「归到」），写作时一目了然。从右侧「材料」也能一键收进来。</p>
                </div>
              </div>
              {/* Task 9 (P6 demo): a finished proposal is locked, which hides the
                  片段引导/写作卡. The tour must spotlight it (data-tour=
                  "writing-aicard"), so render it for the demo too — but pass
                  `locked || isDemo` so ProposalGuidePane is fully READ-ONLY (no
                  active inputs, no AI-write — 铁律①). */}
              {(!locked || isDemo) && (
                <div data-tour="writing-aicard">
                  <ProposalGuidePane projectId={projectId} locked={locked || !!isDemo} onOpenReading={onOpenReading ?? (() => {})} onAnnotationsChanged={onAnnotationsChanged} />
                </div>
              )}
              <SnippetsPane snip={snip} projectId={projectId} doc="proposal" locked={finalized} embedded importedSections={importedSections} />
            </div>
          ) : (
            <SnippetsPane snip={snip} projectId={projectId} doc="essay" locked={finalized} importedSections={importedSections} />
          )
        ) : isProposal ? (
          // 正文 · the proposal's assembled paper (prose) only, plus a whole-paper
          // 批注 review affordance (bug 5a): 印记 reads the full proposal and
          // leaves colored 批注 in the left 材料·AI批注 panel — never rewrites it.
          <div className="flex min-h-0 flex-1 flex-col">
            {/* Task 5 (P7 demo slice): a finished/demo proposal hides this trigger
                (locked), so the guided tour can't show HOW 批注 gets triggered.
                For isDemo we ALSO render a DISABLED copy (mirrors the 完成写作
                disabled pattern above) with its own tour anchor — no onClick, so
                the click can never even reach runProposalReview (no POST, no
                403). Non-demo behavior is unchanged: the real button stays
                gated `!locked` exactly as before. */}
            {(!locked || isDemo) &&
              (isDemo ? (
                <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2">
                  <button
                    type="button"
                    data-tour="writing-review-trigger"
                    disabled
                    className="rounded-mk-md bg-mk-accent px-3 py-1 text-[12px] font-bold text-white disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-mk-accent"
                    title="演示项目为只读，无法运行批注"
                  >
                    让印记通读并批注
                  </button>
                  <span className="text-[12px] text-mk-muted">批注会出现在左侧「材料 · AI批注」里。</span>
                </div>
              ) : (
                <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-8 py-2">
                  <button
                    type="button"
                    onClick={() => void runProposalReview()}
                    disabled={runningCheck}
                    className="rounded-mk-md bg-mk-accent px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-60"
                    title="让印记通读整篇提案，在左侧「材料 · AI批注」里逐段给批注"
                  >
                    {runningCheck ? "印记正在通读…" : "让印记通读并批注"}
                  </button>
                  <span className="text-[12px] text-mk-muted">批注会出现在左侧「材料 · AI批注」里。</span>
                </div>
              ))}
            <div className="min-h-0 flex-1">
              {/* Bug 5b · select-to-quote stages the selection as CoachRail's
                  focusPart pill (editable/cancelable), same as the essay draft —
                  never sent immediately. */}
              <ProsePane projectId={projectId} doc="proposal" locked={locked} onSendToCoach={(t) => setFocusPart(t)} />
            </div>
          </div>
        ) : (
          <DraftPane
            projectId={projectId}
            title={title}
            locked={locked}
            onFocusPart={setFocusPart}
            registerInsert={(fn) => { insertTarget.current = fn; onInsertReady?.(!!fn); }}
            registerScroll={(fn) => { scrollTarget.current = fn; }}
            pendingReview={pendingReview}
            onPendingReviewHandled={() => setPendingReview(null)}
          />
        )}
      </div>

      {/* COACH — portals into the constant AiPanel (Task 4/5's pattern); renders
          nothing at this position itself except its own fixed-overlay card modal.
          In the proposal doc the 正文·检查 examiner shelf is hidden (activePanel
          ≠ "draft") — proposal review is coach-narrated, not a snapshot panel. */}
      <CoachRail
        projectId={projectId}
        focusPart={focusPart}
        onClearFocus={() => setFocusPart(null)}
        locked={locked}
        onCardArtifact={(text, section) => snip.add(text, section)}
        sectionOptions={knownSectionLabels}
        onRunReview={requestReview}
        activePanel={isProposal ? "outline" : tab}
        recap={recap}
      />

      {/* #5/#20 · 完成写作 confirm — the first guarded moment. Confirming LOCKS the
          draft read-only and unlocks 回顾. Reversible via 重新打开写作 until you 归档
          there, at which point 正文与回顾都会锁定、不能再改，并生成过程评估。 */}
      {showFinishModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-mk-lg">
            {/* slice 3b · comment-first: a proposal with NO 批注 yet is offered a
                review before locking (§4). Skippable (铁律②). */}
            {isProposal && proposalAnnoCount === 0 ? (
              <>
                <h2 className="font-sans text-[18px] font-bold text-mk-ink">先让印记看一遍？</h2>
                <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
                  印记还没批注过你的提案。要不要先让它像老师一样看一遍、给点批注，再决定完成？
                </p>
                <div className="mt-6 flex justify-end gap-3">
                  <button
                    type="button"
                    disabled={runningCheck || finishingWriting}
                    onClick={() => { void doFinishWriting(); }}
                    className="rounded-mk-md border border-mk-border px-4 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-ink disabled:opacity-50"
                  >
                    {finishingWriting ? "定稿中……" : "跳过，直接完成"}
                  </button>
                  <button
                    type="button"
                    disabled={runningCheck}
                    onClick={() => { void runCommentFirst(); }}
                    className="rounded-mk-md bg-mk-accent px-5 py-2 text-[14px] font-bold text-white transition hover:bg-mk-accent-600 disabled:opacity-50"
                  >
                    {runningCheck ? "印记在看……" : "先让印记看一遍"}
                  </button>
                </div>
              </>
            ) : (
              <>
                <h2 className="font-sans text-[18px] font-bold text-mk-ink">{isProposal ? "完成提案？" : "完成写作？"}</h2>
                <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
                  {isProposal ? (
                    <>确认后会<span className="font-bold text-mk-ink">定下提案</span>、进入<span className="font-bold text-mk-ink">写正文</span>。之后<span className="font-bold text-mk-accent">仍可重新编辑提案</span>。</>
                  ) : (
                    <>确认后会<span className="font-bold text-mk-ink">锁定初稿</span>、解锁<span className="font-bold text-mk-ink">回顾</span>。之后<span className="font-bold text-mk-accent">仍可重新打开写作</span>继续改；只有在回顾里<span className="font-bold text-mk-accent">定稿评估</span>后才真正锁定。</>
                  )}
                </p>
                {finishWritingError && (
                  <p className="mt-3 text-[14px] font-semibold text-mk-danger">{finishWritingError}</p>
                )}
                <div className="mt-6 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setShowFinishModal(false)}
                    className="rounded-mk-md border border-mk-border px-4 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-ink"
                  >
                    再改改
                  </button>
                  <button
                    type="button"
                    disabled={finishingWriting}
                    onClick={() => { void doFinishWriting(); }}
                    className="rounded-mk-md bg-mk-accent px-5 py-2 text-[14px] font-bold text-white transition hover:bg-mk-accent-600 disabled:opacity-50"
                  >
                    {finishingWriting ? (isProposal ? "定稿中……" : "锁定中……") : (isProposal ? "进入写正文" : "锁定初稿")}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* §99 · proposal-done congrats: celebrate, offer .docx export, continue. */}
      {showProposalCongrats && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-6">
          <div className="w-full max-w-md rounded-mk-lg border border-mk-border bg-mk-surface p-7 shadow-mk-lg">
            <h2 className="font-sans text-[18px] font-bold text-mk-ink">🎉 提案完成！</h2>
            <p className="mt-3 text-[14px] leading-relaxed text-mk-muted">
              你已经写好了一份扎实的研究提案。可以先把它导出带走，然后进入下一步——去做文献研究、写正文。
            </p>
            <div className="mt-6 flex items-center justify-between gap-3">
              <button
                type="button"
                onClick={() => { void exportDraftDocx(proposalExportText, { title }).catch(() => {}); }}
                className="rounded-mk-md border border-mk-border px-4 py-2 text-[14px] font-semibold text-mk-muted hover:text-mk-accent"
              >
                导出提案 .docx
              </button>
              <button
                type="button"
                onClick={() => { setShowProposalCongrats(false); void advanceStatusTo("essay"); }}
                className="rounded-mk-md bg-mk-accent px-5 py-2 text-[14px] font-bold text-white transition hover:bg-mk-accent-600"
              >
                继续下一步 →
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ---------- #23 · snippets (片段) ---------- */

// #6 · outline→snippet-groups is opt-in ONLY (imported once via an explicit
// 「把大纲导入为片段分组」action in the materials sidebar) — the live outline is
// never auto-rendered as snippet board sections (the student must choose to
// bring it in). The imported heading set is a client-side per-project memory,
// mirroring ReadingBlock's viewModeMemo: in-memory only (lost on a hard
// refresh), no DB migration for this UI-only grouping.
const importedSectionsMemo = new Map<string, string[]>();

export type Snip = { id: string; text: string; section: string | null };

// useSnippets owns the 片段 board's load + debounced whole-set save (mirrors
// OutlinePane's persistence), lifted so both SnippetsPane and the materials
// sidebar mutate one source of truth. section (#5) files a snippet under an
// outline heading / 线索 label (null = 未归类).
export type SnippetsHandle = {
  snippets: Snip[];
  // Returns the new snippet's (temp, stable-until-save) id, so a caller that
  // just created a blank row (「+ 在此加片段」) can immediately focus it into
  // edit mode without guessing which row is new.
  add: (text: string, section?: string | null) => string;
  update: (id: string, text: string) => void;
  remove: (id: string) => void;
  setSection: (id: string, section: string | null) => void;
  // Write `text` to the (single) snippet under `section`, resolving against the
  // LIVE snapshot (ref.current) — creates the row if absent, updates it if
  // present. Unlike add/update-by-id this never depends on a caller-held id ref
  // that can go stale across step transitions, so a guided part's text always
  // lands in ITS OWN section slot (fixes the proposal-guide wrong-slot bug where
  // one part's draft overwrote a neighbour's under a fast edit→advance race).
  upsertSection: (section: string, text: string) => void;
  // A synchronous read of the live rows (ref.current), for assembling the full
  // doc immediately after an upsert (the `snippets` state lags a render).
  all: () => Snip[];
};
export function useSnippets(projectId: string): SnippetsHandle {
  const [snippets, setSnippets] = useState<Snip[]>([]);
  const ref = useRef<Snip[]>([]);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  async function save(rows: Snip[]) {
    try {
      // Whole-set replace ignores incoming ids (the server mints fresh ones), so
      // the client id is purely a local React key — keep it STABLE across saves.
      // Adopting the server id here would change the key and remount the
      // textarea mid-edit, dropping focus/caret (review MEDIUM). So don't swap.
      await putSnippets(projectId, rows.map((s) => ({ text: s.text, section: s.section })));
    } catch {
      /* keep local; the next debounced save retries */
    }
  }
  function commit(next: Snip[]) {
    ref.current = next;
    setSnippets(next);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => save(next), 700);
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await getSnippets(projectId);
        if (cancelled) return;
        const rows = loaded.map((s) => ({ id: s.id, text: s.text, section: s.section }));
        ref.current = rows;
        setSnippets(rows);
      } catch {
        /* leave empty */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void save(ref.current);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  return {
    snippets,
    add: (text, section = null) => {
      const id = tempId();
      commit([...ref.current, { id, text, section }]);
      return id;
    },
    update: (id, text) => commit(ref.current.map((s) => (s.id === id ? { ...s, text } : s))),
    remove: (id) => commit(ref.current.filter((s) => s.id !== id)),
    setSection: (id, section) => commit(ref.current.map((s) => (s.id === id ? { ...s, section } : s))),
    upsertSection: (section, text) => {
      const rows = ref.current;
      const i = rows.findIndex((s) => s.section === section);
      if (i >= 0) commit(rows.map((s, j) => (j === i ? { ...s, text } : s)));
      else commit([...rows, { id: tempId(), text, section }]);
    },
    all: () => ref.current,
  };
}

// UNFILED is the sentinel value for the 未归类 option in the 归到 <select>
// (an empty option value maps to section=null).
const UNFILED = "__unfiled__";
const dedupe = (xs: string[]) => Array.from(new Set(xs));

// #5/#6/Item C (batch5 follow-up) · the 片段 board, organized into foldable
// sections. A section is ONLY an outline heading the student explicitly
// IMPORTED (see importedSectionsMemo — the live outline is never auto-rendered
// here) — the board no longer auto-creates a section for every open 探索 线索
// (that read as noise the student didn't ask for; 铁律② no manipulation via
// surprise structure). A snippet already filed under a stale/vanished label
// (e.g. one left over from before this change, or a renamed/deleted heading)
// degrades gracefully into an "orphan" section showing that stored label,
// rather than disappearing. Filing is via drag (a ⠿ handle onto a section
// header) or the 归到 <select>. The draft itself stays a plain textarea — this
// is organizing thinking material, not a structured document editor (铁律②).
function SnippetsPane({ snip, projectId, doc, locked = false, embedded = false, importedSections }: { snip: SnippetsHandle; projectId: string; doc: "proposal" | "essay"; locked?: boolean; embedded?: boolean; importedSections: string[] }) {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [dragId, setDragId] = useState<string | null>(null);
  const [dropLabel, setDropLabel] = useState<string | null>(null);
  // The guided tracks give two things per step: the sub-question text (so a
  // claim/subq section reads 「论点 2：…」 not `claim:<uuid>`) AND the step's saved
  // guide card (the guidance + reference the student wrote against). We load all
  // three tracks once and fold them into a section→guide map + a reading order.
  const [subQuestions, setSubQuestions] = useState<SubQuestion[]>([]);
  const [guideBySection, setGuideBySection] = useState<Map<string, { prompt: string; example?: string | null }>>(new Map());
  const [orderBySection, setOrderBySection] = useState<Map<string, number>>(new Map());
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const guide = new Map<string, { prompt: string; example?: string | null }>();
      const order = new Map<string, number>();
      let i = 0;
      const add = (section: string, card?: { prompt: string; example?: string } | null) => {
        if (!order.has(section)) order.set(section, i++);
        if (card && card.prompt) guide.set(section, { prompt: card.prompt, example: card.example ?? null });
      };
      // proposal parts key as prop:<stepkey>; essay statement/submission use the
      // bare step key. Each track's step list carries the SAVED guide in `card`.
      try {
        const prop = await getProposalTrack(projectId);
        if (!cancelled) setSubQuestions(prop.subQuestions ?? []);
        for (const st of prop.steps ?? []) add(partSectionKey(st.key), st.card);
      } catch { /* proposal track absent → labels/guides degrade gracefully */ }
      try {
        const es = await getEssayStatement(projectId);
        for (const st of es.steps ?? []) add(st.key, st.card);
      } catch { /* not yet at essay → skip */ }
      try {
        const sub = await getEssaySubmission(projectId);
        for (const st of sub.steps ?? []) add(st.key, st.card);
      } catch { /* not yet at submission → skip */ }
      if (!cancelled) { setGuideBySection(guide); setOrderBySection(order); }
    })();
    return () => { cancelled = true; };
  }, [projectId]);

  // A section's human name: guided-writing parts (prop:*/claim:*/sub:*/…) get
  // their friendly part name; a real student label is its own text.
  const labelFor = (section: string) => guidedSectionLabel(section, subQuestions) ?? section;

  const knownLabels = useMemo(() => dedupe(importedSections), [importedSections]);

  // The FINISHED guided parts, as re-readable cards: part name → saved guidance
  // + reference + the student's own writing. Ordered by the document's step
  // order. Read-only once the project is finished (no onEdit below). This is the
  // "finished-cards list" — one snippet per guided section.
  const guidedCards: FilledCard[] = useMemo(() => {
    const seen = new Set<string>();
    const cards: FilledCard[] = [];
    for (const s of snip.snippets) {
      if (s.section == null || !isGuidedSection(s.section) || seen.has(s.section)) continue;
      // Only THIS document's guided parts — 提案 parts (prop:*) and 正文 parts
      // (claim:/sub:/…) no longer merge onto one board.
      if (sectionDoc(s.section) !== doc) continue;
      seen.add(s.section);
      const g = guideBySection.get(s.section);
      cards.push({ key: s.section, title: labelFor(s.section), text: s.text, guidance: g?.prompt, example: g?.example ?? null });
    }
    cards.sort((a, b) => (orderBySection.get(a.key) ?? 1e9) - (orderBySection.get(b.key) ?? 1e9));
    return cards;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [snip.snippets, guideBySection, orderBySection, subQuestions, doc]);

  // Free (student-made) groups only — guided parts are rendered above as the
  // finished-cards fold, not as orphan sections. Imported outline headings, then
  // any orphaned student label still present on a snippet (renamed/deleted
  // heading, or a stale 线索 label — never vanish), then 未归类 last.
  const { groups, unfiled } = useMemo(() => {
    const bySection = new Map<string, Snip[]>();
    const un: Snip[] = [];
    for (const s of snip.snippets) {
      if (s.section == null) un.push(s);
      else if (isGuidedSection(s.section)) continue; // rendered as a finished card
      else { const arr = bySection.get(s.section) ?? []; arr.push(s); bySection.set(s.section, arr); }
    }
    const ordered: { label: string; displayLabel: string; kind: "outline" | "orphan"; snips: Snip[] }[] = [];
    for (const l of importedSections) ordered.push({ label: l, displayLabel: l, kind: "outline", snips: bySection.get(l) ?? [] });
    for (const [label, snips] of bySection) {
      if (knownLabels.includes(label)) continue;
      ordered.push({ label, displayLabel: label, kind: "orphan", snips });
    }
    return { groups: ordered, unfiled: un };
  }, [snip.snippets, importedSections, knownLabels]);

  function dropOnto(label: string | null) {
    if (dragId) snip.setSection(dragId, label);
    setDragId(null);
    setDropLabel(null);
  }

  // Edit a finished part in place (finished ≠ frozen): write by SECTION against
  // the live snapshot. Suppressed ONLY once the project is finalized (in 回顾 /
  // evaluating / done — the `locked` flag here carries that project-level state,
  // NOT a doc's reversible 完成), so a finished proposal's cards stay editable
  // while the essay is still being written.
  const editFilledPart = locked
    ? undefined
    : (key: string, text: string) => { snip.upsertSection(key, text); scheduleCardRevision(projectId); };

  return (
    <div className={embedded ? "px-8 pb-6" : "min-h-0 overflow-y-auto px-8 py-6"}>
      <div className="mx-auto max-w-2xl">
        {/* Header lives on the tab itself when embedded (the proposal's 片段 tab
            shows ONE header above the writing card + these cards). */}
        {!embedded && (
          <div className="mb-4">
            <h2 className="font-sans text-[18px] font-bold text-mk-ink">片段</h2>
            <p className="mt-1 text-[14px] text-mk-muted">攒下引文、笔记、灵光一现的句子——把它们归到大纲的章节或探索的线索下（拖 ⠿ 或用「归到」），写作时一目了然。从右侧「材料」也能一键收进来。</p>
          </div>
        )}
        {/* 写作部分 — the finished guided parts, each re-readable as its own card
            (part name → guidance + reference + your writing). Read-only once the
            project is finished. */}
        {guidedCards.length > 0 && (
          <div className="mb-4">
            <FilledCardsFold cards={guidedCards} heading="写作部分" onEdit={editFilledPart} />
          </div>
        )}
        <div className="flex flex-col gap-4">
          {groups.map((g) => (
            <SnippetSection
              key={`${g.kind}:${g.label}`}
              label={g.label}
              displayLabel={g.displayLabel}
              kind={g.kind}
              snips={g.snips}
              collapsed={collapsed.has(g.label)}
              isDropTarget={dropLabel === g.label}
              sectionOptions={knownLabels}
              labelFor={labelFor}
              onToggle={() => setCollapsed((c) => { const n = new Set(c); n.has(g.label) ? n.delete(g.label) : n.add(g.label); return n; })}
              onAdd={() => snip.add("", g.label)}
              onDragOverHead={() => setDropLabel(g.label)}
              onDropHead={() => dropOnto(g.label)}
              snip={snip}
              onDragStart={setDragId}
            />
          ))}
          {/* 未归类 — also the drop target for un-filing */}
          <SnippetSection
            label="未归类"
            displayLabel="未归类"
            kind="unfiled"
            snips={unfiled}
            collapsed={collapsed.has(UNFILED)}
            isDropTarget={dropLabel === UNFILED}
            sectionOptions={knownLabels}
            labelFor={labelFor}
            onToggle={() => setCollapsed((c) => { const n = new Set(c); n.has(UNFILED) ? n.delete(UNFILED) : n.add(UNFILED); return n; })}
            onAdd={() => snip.add("", null)}
            onDragOverHead={() => setDropLabel(UNFILED)}
            onDropHead={() => dropOnto(null)}
            snip={snip}
            onDragStart={setDragId}
          />
        </div>
      </div>
    </div>
  );
}

function SnippetSection({
  label, displayLabel, kind, snips, collapsed, isDropTarget, sectionOptions, labelFor, onToggle, onAdd, onDragOverHead, onDropHead, snip, onDragStart,
}: {
  label: string;
  // The header text: for a guided-writing part this is the friendly part name
  // (「论点 2：…」/「引言」), never the raw section key. Filing still uses `label`.
  displayLabel: string;
  kind: "outline" | "guided" | "orphan" | "unfiled";
  snips: Snip[];
  collapsed: boolean;
  isDropTarget: boolean;
  sectionOptions: string[];
  // Friendly name for any section value (used in the 归到 dropdown so a guided
  // part's kept option reads as its part name, not `claim:<uuid>`).
  labelFor: (section: string) => string;
  onToggle: () => void;
  // Returns the newly-created snippet's id so the caller can focus it into
  // edit mode immediately (a student-initiated "+加片段" isn't the
  // programmatic-artifact case #9 wants defaulted to read mode).
  onAdd: () => string;
  onDragOverHead: () => void;
  onDropHead: () => void;
  snip: SnippetsHandle;
  onDragStart: (id: string | null) => void;
}) {
  const tag = kind === "outline" ? "章节" : kind === "guided" ? "写作部分" : kind === "orphan" ? "旧标签" : "";
  const tone = kind === "orphan" ? "text-mk-faint" : "text-mk-accent";
  // #9 · a snippet defaults to a READ view; double-click enters edit mode (a
  // taller textarea + a ✓ to leave it). Local to this section instance — a
  // freshly-programmatic snippet (AI rail / card compile / materials) is never
  // in this set, so it lands in READ mode as required.
  const [editingIds, setEditingIds] = useState<Set<string>>(new Set());
  function startEditing(id: string) {
    setEditingIds((ids) => { const n = new Set(ids); n.add(id); return n; });
  }
  function stopEditing(id: string) {
    setEditingIds((ids) => { if (!ids.has(id)) return ids; const n = new Set(ids); n.delete(id); return n; });
  }
  return (
    <section className={`rounded-mk-lg border ${isDropTarget ? "border-mk-accent bg-mk-accent-50" : "border-mk-border bg-mk-paper"}`}>
      <div
        onDragOver={(e) => { e.preventDefault(); onDragOverHead(); }}
        onDrop={(e) => { e.preventDefault(); onDropHead(); }}
        className="flex items-center gap-2 px-3 py-2"
      >
        <button type="button" onClick={onToggle} className="flex min-w-0 flex-1 items-center gap-1.5 text-left">
          <span className={`text-mk-faint transition ${collapsed ? "" : "rotate-90"}`}>▸</span>
          {tag && <span className={`flex-none rounded-full bg-mk-surface px-1.5 py-0.5 text-[12px] font-bold ${tone}`}>{tag}</span>}
          <span className="min-w-0 flex-1 truncate text-[14px] font-bold text-mk-ink">{displayLabel}</span>
          <span className="flex-none text-[12px] font-semibold text-mk-faint">{snips.length}</span>
        </button>
      </div>
      {!collapsed && (
        <div className="flex flex-col gap-2 px-3 pb-3">
          {snips.map((s) => {
            const editing = editingIds.has(s.id);
            return (
            <div
              key={s.id}
              className="group rounded-mk-md border border-mk-border bg-mk-surface p-2.5 shadow-mk-xs"
            >
              <div className="flex items-start gap-1.5">
                <span
                  draggable
                  onDragStart={() => onDragStart(s.id)}
                  onDragEnd={() => onDragStart(null)}
                  title="拖到某个章节/线索下"
                  className="mt-1 flex-none cursor-grab text-[14px] leading-none text-mk-faint active:cursor-grabbing"
                >
                  ⠿
                </span>
                {editing ? (
                  <textarea
                    autoFocus
                    value={s.text}
                    onChange={(e) => snip.update(s.id, e.target.value)}
                    onBlur={() => stopEditing(s.id)}
                    rows={6}
                    placeholder="写下或粘贴一个片段……"
                    className="min-h-[7rem] w-full resize-y bg-transparent text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint"
                  />
                ) : (
                  <button
                    type="button"
                    onDoubleClick={() => startEditing(s.id)}
                    title="双击编辑"
                    className="w-full flex-1 cursor-text whitespace-pre-wrap break-words text-left text-[14px] leading-relaxed text-mk-ink"
                  >
                    {s.text.trim() ? s.text : <span className="text-mk-faint">写下或粘贴一个片段……（双击编辑）</span>}
                  </button>
                )}
                {editing && (
                  <button
                    type="button"
                    title="完成编辑"
                    onClick={() => stopEditing(s.id)}
                    className="mt-0.5 flex-none rounded px-1 text-[14px] font-bold leading-none text-mk-accent hover:text-mk-accent-600"
                  >
                    ✓
                  </button>
                )}
              </div>
              <div className="mt-1 flex items-center justify-end gap-2">
                <label className="flex items-center gap-1 text-[12px] text-mk-faint">
                  归到
                  <select
                    value={s.section ?? UNFILED}
                    onChange={(e) => snip.setSection(s.id, e.target.value === UNFILED ? null : e.target.value)}
                    aria-label="把片段归到"
                    className="max-w-[10rem] rounded border border-mk-border bg-mk-surface px-1.5 py-0.5 text-[12px] text-mk-ink outline-none focus:border-mk-accent"
                  >
                    <option value={UNFILED}>未归类</option>
                    {/* keep a stale/orphan/guided section selectable so its value
                        shows — labelled by its friendly part name, never a raw key */}
                    {s.section && !sectionOptions.includes(s.section) && <option value={s.section}>{labelFor(s.section)}</option>}
                    {sectionOptions.map((o) => <option key={o} value={o}>{o}</option>)}
                  </select>
                </label>
                <button type="button" onClick={() => snip.remove(s.id)} className="text-[12px] font-semibold text-mk-faint opacity-0 transition hover:text-mk-danger group-hover:opacity-100">删除</button>
              </div>
            </div>
            );
          })}
          <button
            type="button"
            onClick={() => startEditing(onAdd())}
            className="rounded-mk-md border border-dashed border-mk-border py-2 text-[14px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent"
          >
            + 在此加片段
          </button>
        </div>
      )}
    </section>
  );
}

function Tab({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded-mk-md px-3.5 py-1.5 text-[14px] font-bold transition ${active ? "bg-mk-accent-50 text-mk-accent" : "text-mk-faint hover:text-mk-ink"}`}
    >
      <Icon name={icon} size={15} /> {children}
    </button>
  );
}

/* ---------- 提纲 · outline ---------- */

function OutlinePane({ projectId, title, doc }: { projectId: string; title: string; doc: "proposal" | "essay" }) {
  const [nodes, setNodes] = useState<Row[]>([]);
  const [view, setView] = useState<"list" | "map">("list");
  // nodesRef mirrors the latest committed rows so the debounced save (and the
  // id-reconcile after it resolves) reads current state without stale closures.
  const nodesRef = useRef<Row[]>([]);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Focus plumbing so typing flows like a real outliner: each row registers its
  // input by id, and a key action parks a pending focus target that the effect
  // applies once the new/changed rows have rendered.
  const inputRefs = useRef(new Map<string, HTMLInputElement>());
  const pendingFocus = useRef<{ id: string; atEnd: boolean } | null>(null);
  const registerInput = (id: string, el: HTMLInputElement | null) => {
    if (el) inputRefs.current.set(id, el);
    else inputRefs.current.delete(id);
  };

  // Persist the whole set. The server re-mints ids on every PUT, but the client
  // id is only a local React key — keep it STABLE across saves. Adopting the
  // server id here (the old behavior) changed key={n.id} on the mounted rows,
  // remounting the focused <input> ~700ms after every keystroke and dropping
  // focus/caret — the "Enter/Tab/mouse-move drops me out of editing" bug. So we
  // deliberately DON'T swap ids (mirrors useSnippets above). On failure keep
  // local; the next debounce retries.
  async function save(rows: Row[]) {
    try {
      await putOutline(projectId, rows.map((r) => ({ id: r.id, text: r.text, depth: r.depth })), doc);
    } catch {
      /* keep local; the next debounced save retries */
    }
  }

  function scheduleSave(rows: Row[]) {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => save(rows), 700);
  }

  // Single mutation entry point: update state + ref, then debounce a save. Every
  // edit (list or mind-map) flows through here, so the two views stay in sync.
  function commit(next: Row[]) {
    nodesRef.current = next;
    setNodes(next);
    scheduleSave(next);
  }

  // Load on mount. Empty outline → a single blank editable row (not yet saved;
  // it persists once the student types).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await getOutline(projectId, doc);
        if (cancelled) return;
        const rows: Row[] = loaded.length
          ? loaded.map((n) => ({ id: n.id, text: n.text, depth: n.depth }))
          : [{ id: tempId(), text: "", depth: 0 }];
        nodesRef.current = rows;
        setNodes(rows);
      } catch {
        if (cancelled) return;
        const rows: Row[] = [{ id: tempId(), text: "", depth: 0 }];
        nodesRef.current = rows;
        setNodes(rows);
      }
    })();
    return () => {
      cancelled = true;
    };
    // Reload when the document changes — 提案 and 正文 have separate outlines.
  }, [projectId, doc]);

  // Flush a pending save on unmount so a last edit inside the debounce window
  // isn't lost when the student leaves the room.
  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void save(nodesRef.current);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const edit = (id: string, text: string) => commit(nodesRef.current.map((n) => (n.id === id ? { ...n, text } : n)));
  const bump = (id: string, dir: 1 | -1) => commit(nodesRef.current.map((n) => (n.id === id ? { ...n, depth: Math.max(0, Math.min(2, n.depth + dir)) } : n)));
  const remove = (id: string) => {
    const next = nodesRef.current.filter((n) => n.id !== id);
    // Never leave the outline with zero rows — keep one blank editable row.
    commit(next.length ? next : [{ id: tempId(), text: "", depth: 0 }]);
  };
  const addAfter = (id: string) => {
    const xs = nodesRef.current;
    const i = xs.findIndex((n) => n.id === id);
    const depth = xs[i]?.depth ?? 0;
    const nid = tempId();
    const next = [...xs];
    next.splice(i < 0 ? next.length : i + 1, 0, { id: nid, text: "", depth });
    pendingFocus.current = { id: nid, atEnd: false };
    commit(next);
  };

  // Add a child under a map node (or, for the synthetic root, a new top-level
  // row). We insert right after the parent with depth+1 so the flat→tree build
  // adopts it as that parent's child. Writes to the same `nodes` state, so the
  // list view sees it too.
  const addChild = (mapNodeId: string) => {
    const xs = nodesRef.current;
    const nid = tempId();
    if (mapNodeId === "root") {
      pendingFocus.current = { id: nid, atEnd: false };
      commit([{ id: nid, text: "", depth: 0 }, ...xs]);
      return;
    }
    const i = xs.findIndex((n) => n.id === mapNodeId);
    if (i < 0) return;
    const depth = Math.min(MAX_DEPTH, xs[i]!.depth + 1);
    const next = [...xs];
    next.splice(i + 1, 0, { id: nid, text: "", depth });
    pendingFocus.current = { id: nid, atEnd: false };
    commit(next);
  };

  // Translate a keydown on a row into an outline mutation. Returns true when it
  // handled the key (so the caller can preventDefault); false lets the browser
  // do its normal thing (typing, caret backspace inside text).
  const onRowKey = (id: string, e: React.KeyboardEvent<HTMLInputElement>): void => {
    let type: OutlineKeyType | null = null;
    if (e.key === "Enter") type = "enter";
    else if (e.key === "Tab") type = e.shiftKey ? "outdent" : "indent";
    else if (e.key === "Backspace") {
      const row = nodesRef.current.find((n) => n.id === id);
      // Only intercept backspace on an already-empty row; inside text the
      // default deletes a character.
      if (!row || row.text !== "") return;
      type = "backspace";
    }
    if (!type) return;
    e.preventDefault();
    const res = outlineKey(nodesRef.current, type, id);
    if (res.focus) pendingFocus.current = res.focus;
    commit(res.rows);
  };

  // Mind-map keyboard parity (#11): the map's inputs carried no keydown, so a
  // student couldn't grow the map without the mouse. XMind convention — Enter
  // adds a sibling after this node, Tab adds a child (depth+1). Both reuse the
  // add* helpers, which park pendingFocus so the new node's input takes focus
  // once rendered (the map inputs register into inputRefs too).
  const onNodeKey = (id: string, e: React.KeyboardEvent<HTMLInputElement>): void => {
    if (e.key === "Enter") { e.preventDefault(); addAfter(id); }
    else if (e.key === "Tab" && !e.shiftKey) { e.preventDefault(); addChild(id); }
  };

  // Apply a parked focus target after the rows it references have rendered.
  useEffect(() => {
    const pf = pendingFocus.current;
    if (!pf) return;
    const el = inputRefs.current.get(pf.id);
    if (!el) return;
    el.focus();
    if (pf.atEnd) {
      const len = el.value.length;
      el.setSelectionRange(len, len);
    }
    pendingFocus.current = null;
  }, [nodes]);

  return (
    <div data-tour="writing-outline" className="flex min-h-0 flex-col">
      <div className="flex items-center justify-between px-8 pt-6 pb-3">
        <div>
          <h2 className="font-sans text-[19px] font-bold text-mk-ink">提纲</h2>
          <p className="mt-0.5 text-[14px] text-mk-muted">先把骨架搭出来。</p>
        </div>
        <Segmented
          options={[
            { value: "list", label: "大纲" },
            { value: "map", label: "思维导图" },
          ]}
          value={view}
          onChange={(v) => setView(v as "list" | "map")}
        />
      </div>

      {/* #84 · a paper needs a sub-structure. When the outline has no branch yet
          (only the main question, or blank), guide the student to decide the
          angles / sub-questions she'll answer the main question from — those
          become the body paragraphs. Sub-questions from the proposal seed this
          automatically; this banner is the fallback when there were none. */}
      {!nodes.some((n) => n.depth >= 1 && n.text.trim() !== "") && (
        <div className="mx-8 mb-3 rounded-mk-md border border-mk-accent bg-mk-accent-50 px-4 py-3">
          <p className="text-[13.5px] leading-relaxed text-mk-ink">
            <strong>先想清楚这篇论文的结构。</strong>你打算从哪几个角度、用哪几个子问题来回答你的主问题？把每个角度写成主问题下的一条分支（回车加一条、Tab 缩进为子级）——这些角度就会长成你论文的主体段落。想不清楚，就问问右边的印记。
          </p>
        </div>
      )}

      {view === "list" ? (
        <div className="min-h-0 flex-1 overflow-y-auto px-8 pb-7">
          <div className="mx-auto max-w-2xl">
            <div className="flex flex-col">
              {nodes.map((n) => (
                <OutlineRow
                  key={n.id}
                  node={n}
                  registerInput={(el) => registerInput(n.id, el)}
                  onKey={(e) => onRowKey(n.id, e)}
                  onEdit={(t) => edit(n.id, t)}
                  onIndent={() => bump(n.id, 1)}
                  onOutdent={() => bump(n.id, -1)}
                  onAdd={() => addAfter(n.id)}
                  onRemove={() => remove(n.id)}
                />
              ))}
            </div>
            <button type="button" onClick={() => addAfter(nodes[nodes.length - 1]?.id ?? "")} className="mt-2 rounded-mk-md border border-dashed border-mk-border px-3 py-2 text-[14px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent">
              + 新增一条
            </button>
          </div>
        </div>
      ) : (
        <MindMap nodes={nodes} title={title} onEdit={edit} onAddChild={addChild} registerInput={registerInput} onNodeKey={onNodeKey} />
      )}
    </div>
  );
}

/* ----- mind map view (same outline data, laid out as a tree) ----- */

type MapNode = { id: string; text: string; depth: number; children: MapNode[]; row: number; cx: number; cy: number };

const COL = 250;
const ROW = 56;
const NODE_W = 200;

function MindMap({ nodes, title, onEdit, onAddChild, registerInput, onNodeKey }: { nodes: Row[]; title: string; onEdit: (id: string, t: string) => void; onAddChild: (id: string) => void; registerInput: (id: string, el: HTMLInputElement | null) => void; onNodeKey: (id: string, e: React.KeyboardEvent<HTMLInputElement>) => void }) {
  // Build a tree from the flat depth list, with the project title as the root.
  const root: MapNode = { id: "root", text: title || "未命名项目", depth: -1, children: [], row: 0, cx: 0, cy: 0 };
  const lastAtDepth: Record<number, MapNode> = { [-1]: root };
  for (const n of nodes) {
    const node: MapNode = { id: n.id, text: n.text, depth: n.depth, children: [], row: 0, cx: 0, cy: 0 };
    const parent = lastAtDepth[n.depth - 1] ?? root;
    parent.children.push(node);
    lastAtDepth[n.depth] = node;
    Object.keys(lastAtDepth).forEach((k) => { if (Number(k) > n.depth) delete lastAtDepth[Number(k)]; });
  }

  // Assign a row to every leaf; internal nodes centre on their children.
  let slot = 0;
  let maxDepth = 0;
  const layout = (node: MapNode) => {
    maxDepth = Math.max(maxDepth, node.depth);
    if (node.children.length === 0) node.row = slot++;
    else { node.children.forEach(layout); node.row = (node.children[0]!.row + node.children[node.children.length - 1]!.row) / 2; }
    node.cx = (node.depth + 1) * COL;
    node.cy = node.row * ROW + ROW / 2 + 20;
  };
  layout(root);

  const flat: MapNode[] = [];
  const collect = (n: MapNode) => { flat.push(n); n.children.forEach(collect); };
  collect(root);

  const width = (maxDepth + 2) * COL + 40;
  const height = slot * ROW + 60;

  // #11 · nesting depth needs genuinely distinguishable hues (root/0/1/2+ all
  // render simultaneously on screen) — a blind mk-primary→mk-accent /
  // mk-accent→mk-accent rename would have collapsed root+depth0 (was
  // mk-primary family) with depth1 (was mk-accent) into the SAME color, since
  // the redesign's tailwind.config aliases both legacy families onto the one
  // --mk-accent var. depth1 is reassigned onto the taro macaron instead —
  // keeping four visually distinct levels, mirroring PlanTag's 3-way split.
  const tone = (d: number) =>
    d < 0 ? "bg-mk-accent text-white border-mk-accent"
      : d === 0 ? "bg-mk-accent-50 text-mk-accent border-mk-accent/30"
        : d === 1 ? "bg-mk-taro-bg text-mk-taro-fg border-mk-taro/30"
          : "bg-mk-success-bg text-mk-success border-mk-success/30";

  return (
    <div className="min-h-0 flex-1 overflow-auto px-8 pb-8">
      <div className="relative" style={{ width, height }}>
        <svg className="absolute inset-0" width={width} height={height} style={{ pointerEvents: "none" }}>
          {flat.flatMap((p) =>
            p.children.map((c) => {
              const x1 = p.cx + NODE_W, y1 = p.cy, x2 = c.cx, y2 = c.cy;
              return <path key={`${p.id}-${c.id}`} d={`M ${x1} ${y1} C ${x1 + 46} ${y1}, ${x2 - 46} ${y2}, ${x2} ${y2}`} className="stroke-mk-border" fill="none" strokeWidth={1.6} />;
            }),
          )}
        </svg>
        {flat.map((n) => (
          <div
            key={n.id}
            className={`group absolute flex items-center rounded-mk-md border px-3 shadow-mk-xs ${tone(n.depth)}`}
            style={{ left: n.cx, top: n.cy - 18, width: NODE_W, height: 36 }}
          >
            {n.depth < 0 ? (
              <span className="truncate text-[14px] font-bold">{n.text}</span>
            ) : (
              <input
                ref={(el) => registerInput(n.id, el)}
                value={n.text}
                onChange={(e) => onEdit(n.id, e.target.value)}
                onKeyDown={(e) => onNodeKey(n.id, e)}
                placeholder="写一条……（回车加同级、Tab 加子节点）"
                className={`w-full truncate bg-transparent text-[14px] outline-none placeholder:opacity-60 ${n.depth === 0 ? "font-bold" : "font-semibold"}`}
              />
            )}
            {/* Add a child under this node (depth clamps ≤ MAX_DEPTH). Hidden
                until hover so the map stays calm; sits just off the right edge. */}
            {n.depth < MAX_DEPTH && (
              <button
                type="button"
                title="加一个子节点"
                onClick={() => onAddChild(n.id)}
                className="absolute -right-3 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full border border-mk-border bg-mk-surface text-[15px] font-bold leading-none text-mk-faint opacity-0 shadow-mk-xs transition hover:border-mk-accent hover:text-mk-accent group-hover:opacity-100"
              >
                +
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function OutlineRow({ node, registerInput, onKey, onEdit, onIndent, onOutdent, onAdd, onRemove }: { node: Row; registerInput: (el: HTMLInputElement | null) => void; onKey: (e: React.KeyboardEvent<HTMLInputElement>) => void; onEdit: (t: string) => void; onIndent: () => void; onOutdent: () => void; onAdd: () => void; onRemove: () => void }) {
  // #11 · mirrors MindMap's tone() — depth1 sits on the taro macaron (not
  // mk-accent) so it stays visually distinct from depth0, which shares the
  // mk-accent family root/0 uses above.
  const dot = node.depth === 0 ? "bg-mk-accent" : node.depth === 1 ? "bg-mk-taro" : "bg-mk-success";
  return (
    <div className="group flex items-center gap-2 rounded-mk-md py-1 hover:bg-mk-paper" style={{ paddingLeft: node.depth * 26 }}>
      <span className={`h-1.5 w-1.5 flex-none rounded-full ${dot}`} />
      <input
        ref={registerInput}
        value={node.text}
        onChange={(e) => onEdit(e.target.value)}
        onKeyDown={onKey}
        placeholder="写一条……（回车换行、Tab 缩进）"
        className={`min-w-0 flex-1 rounded bg-transparent px-1.5 py-1 text-mk-ink outline-none transition placeholder:text-mk-faint focus:bg-mk-surface ${node.depth === 0 ? "text-[14.5px] font-bold" : "text-[14px]"}`}
      />
      <div className="flex flex-none items-center gap-0.5 opacity-0 transition group-hover:opacity-100">
        <IconBtn onClick={onOutdent} title="升级">←</IconBtn>
        <IconBtn onClick={onIndent} title="缩进">→</IconBtn>
        <IconBtn onClick={onAdd} title="下面加一条">+</IconBtn>
        <IconBtn onClick={onRemove} title="删除">×</IconBtn>
      </div>
    </div>
  );
}

function IconBtn({ onClick, title, children }: { onClick: () => void; title: string; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} title={title} className="flex h-6 w-6 items-center justify-center rounded text-[14px] font-bold text-mk-faint hover:bg-mk-surface hover:text-mk-accent">
      {children}
    </button>
  );
}

/* ---------- 写作 · single draft panel ---------- */

// The extensions we can read client-side as plain text. Binary formats
// (.docx/.pdf) are accepted but parked with a note — real parsing is later.
const TEXT_EXT = [".md", ".txt", ".markdown"];

// #6 · the 整稿体检 lenses, each with a one-line plain-language description so a
// student knows what a voice does before picking it (shown inline in the
// dropdown, and echoed under the selector). The voice only changes the coach's
// tone/lens on the server — never the rubric or the "never rewrite" rule.
const VOICE_META: Record<ReviewVoice, { label: string; desc: string }> = {
  board: { label: "评审团", desc: "像考官那样全面权衡" },
  sceptic: { label: "质疑者", desc: "专挑论证漏洞与反例" },
  layperson: { label: "门外汉", desc: "用常识追问你没交代的" },
  executioner: { label: "审判者", desc: "只看能不能站得住" },
};
const VOICE_ORDER: ReviewVoice[] = ["board", "sceptic", "layperson", "executioner"];

// #10 · classify a failed 体检 so the student sees an honest, distinguishing
// message instead of one blanket "体检没跑完" string. `review_rejected` is the
// server telling us it ran the review and threw the output away (enforcement
// / unparseable model reply) — a DIFFERENT situation from a transient network
// hiccup, so it gets its own copy. Everything else (network drop, timeout,
// unexpected HTTP failure) reads as transient.
function classifyReviewError(err: unknown): string {
  if (err instanceof ApiError && err.code === "review_rejected") {
    return "这次体检没通过内部校验，请再试一次。";
  }
  return "体检没跑完（可能是网络不稳定），稍后再试一次。";
}

// paragraphAtCaret returns the blank-line-separated paragraph the caret sits in
// (trimmed). Bounds are computed from the ACTUAL separators (a blank-line gap is
// 2..n chars), so it never drifts; a caret in a gap attaches to the following
// block; empty text → "". Exported for direct unit testing (WC · M1).
export function paragraphAtCaret(src: string, caret: number): string {
  const bounds: Array<[number, number]> = [];
  const re = /\n{2,}/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(src)) !== null) {
    bounds.push([last, m.index]);
    last = m.index + m[0].length;
  }
  bounds.push([last, src.length]);
  for (const [s, e] of bounds) {
    if (caret <= e) return src.slice(s, e).trim();
  }
  const [s] = bounds[bounds.length - 1]!;
  return src.slice(s).trim();
}

function DraftPane({
  projectId,
  title,
  locked,
  onFocusPart,
  registerInsert,
  registerScroll,
  pendingReview,
  onPendingReviewHandled,
}: {
  projectId: string;
  title: string;
  locked: boolean;
  onFocusPart: (part: string) => void;
  registerInsert: (fn: ((t: string, referenceId?: string) => void) | null) => void;
  // S1 · register a scroll-to-anchor fn (批注 click → find quote/第N段 in the
  // draft, scroll+select it). Null on unmount.
  registerScroll?: (fn: ((a: { quote?: string; locator?: string }) => void) | null) => void;
  // #8-second · a voice-scoped 体检 requested from the persistent rail shelf
  // (a tab-sibling of this pane) — queued here rather than called directly so
  // it's never lost to the mount race when the request also switches the tab
  // to 正文 (this pane's own mount-effect below flushes it once mounted).
  pendingReview: { scope?: string; voice: ReviewVoice } | null;
  onPendingReviewHandled: () => void;
}) {
  const [mode, setMode] = useState<"write" | "upload">("write");
  const [pane, setPane] = useState<"edit" | "preview">("edit");
  // #5-follow-on · 自由 (one free textarea) vs 分节 (write under outline-driven
  // headings). A toggle — free-form is always available (never a cage).
  const [layout, setLayout] = useState<"free" | "sections">("free");
  // When 分节 is active, the sections editor registers its own insert here so
  // the materials sidebar drops a fragment into the focused section.
  const sectionInsertRef = useRef<((t: string, referenceId?: string) => void) | null>(null);
  const [text, setText] = useState("");
  const [uploadNote, setUploadNote] = useState<string | null>(null);
  // #7 · a floating "问印记" chip that appears next to a text selection. selPop
  // carries the selected text + where to float the chip (coords relative to the
  // editor pane). Clicking it pins that part into the coach composer.
  const [selPop, setSelPop] = useState<{ x: number; y: number; text: string } | null>(null);
  const fileInput = useRef<HTMLInputElement | null>(null);
  const draftRef = useRef<HTMLTextAreaElement | null>(null);
  const paneRef = useRef<HTMLDivElement | null>(null);
  // #9 · textRef mirrors the latest draft so the sidebar's insert (registered
  // once, below) reads current content without a stale closure. hasFocusedRef
  // tracks whether the caret is meaningful yet (review L3).
  const textRef = useRef("");
  const hasFocusedRef = useRef(false);

  // #7 · on mouse-up in the draft, if there's a highlighted range, float the
  // "问印记" chip just above the pointer. No selection (or a locked draft) hides
  // it. Coordinates are relative to the editor pane so the chip tracks the text.
  function onDraftMouseUp(e: React.MouseEvent<HTMLTextAreaElement>) {
    if (locked) return;
    const ta = e.currentTarget;
    const part = ta.value.slice(ta.selectionStart, ta.selectionEnd).trim();
    const host = paneRef.current;
    if (part && host) {
      const rect = host.getBoundingClientRect();
      setSelPop({ x: e.clientX - rect.left, y: e.clientY - rect.top, text: part });
    } else {
      setSelPop(null);
    }
  }
  // Q6 · autosave resilience — debounce ~1.2s after the student stops typing,
  // plus a periodic safety-net flush (~18s) so a long uninterrupted typing
  // stretch never goes unsaved just because the debounce keeps getting reset.
  // dirtyRef/savingRef are refs (not state) so the interval/backoff timers
  // always read the LATEST value without a stale closure. A failed save is
  // never allowed to touch `text` — the textarea only ever gets loaded once
  // (on mount, above), so local edits can't be clobbered by a stale refetch;
  // this just has to keep retrying until the same content lands.
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const retryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const dirtyRef = useRef(false);
  const savingRef = useRef(false);
  const retryAttempt = useRef(0);
  // Q6 · "saved" is the steady resting state (also covers a freshly-loaded,
  // untouched draft — its persisted content already counts as saved); "dirty"
  // covers the gap between a keystroke and the debounce firing. No "idle"/null
  // state — the indicator always occupies its slot so it never flickers or
  // shifts the 字数 counter next to it.
  const [saveStatus, setSaveStatus] = useState<"saved" | "dirty" | "saving" | "retrying">("saved");
  const words = text.replace(/\s+/g, "").length;

  // WA · 整稿体检: save a version + run the whole-draft review, render its advice
  // read-only. Never edits the draft — 印记 checks argument/structure, you revise.
  const [voice, setVoice] = useState<ReviewVoice>("board");
  const [reviewing, setReviewing] = useState(false);
  const [review, setReview] = useState<DraftReviewResult | null>(null);
  const [reviewError, setReviewError] = useState<string | null>(null);
  // #8 · the 体检 voices work on the whole draft OR just a selected paragraph —
  // scope drives the panel's wording so it's clear what got checked.
  const [reviewScope, setReviewScope] = useState<"draft" | "part">("draft");

  // Run the chosen voice's review. No arg → whole draft; a scopeText → just that
  // paragraph (from the selection chip, or the shelf's 这段 button). Never
  // rewrites — 印记 checks argument and structure, the student revises in her
  // own words. `voiceOverride` lets the persistent rail shelf pick a voice
  // directly (its per-voice buttons) without going through the toolbar <select>
  // first — it also updates that <select> so the two stay in sync.
  async function runReview(scopeText?: string, voiceOverride?: ReviewVoice) {
    const useVoice = voiceOverride ?? voice;
    if (voiceOverride && voiceOverride !== voice) setVoice(voiceOverride);
    const target = scopeText ?? text;
    if (reviewing || locked || target.trim() === "") return;
    setReviewing(true);
    setReviewError(null);
    setReviewScope(scopeText ? "part" : "draft");
    // flush any pending autosave so the persisted buffer matches what's on screen
    if (saveTimer.current) clearTimeout(saveTimer.current);
    if (retryTimer.current) { clearTimeout(retryTimer.current); retryTimer.current = null; }
    try {
      await putBuffer(projectId, text);
      // this save landed synchronously (not via the debounce/retry path) —
      // reflect it in the same status indicator so the two paths never disagree.
      dirtyRef.current = false;
      retryAttempt.current = 0;
      flashSaved();
      try {
        setReview(await runDraftReview(projectId, target, useVoice));
      } catch (err) {
        // #10 · one retry, but only for a TRANSIENT failure. `review_rejected`
        // means the server already ran the review and threw the output away
        // (its own retry included, see agent.ProposeReview) — retrying the
        // identical content client-side would just resend the same rejected
        // request, so that one surfaces immediately instead.
        if (err instanceof ApiError && err.code === "review_rejected") throw err;
        setReview(await runDraftReview(projectId, target, useVoice));
      }
    } catch (err) {
      setReviewError(classifyReviewError(err));
    } finally {
      setReviewing(false);
    }
  }

  // #8-second · flush a review requested from the persistent rail shelf. A
  // whole-draft request needs `text` loaded first — the request may have also
  // just switched the tab here, remounting this pane fresh, so the effect
  // below hasn't populated `text` yet. Wait for draftReady in that case; a
  // paragraph-scoped request carries its own text and can run immediately.
  const [draftReady, setDraftReady] = useState(false);
  useEffect(() => {
    if (!pendingReview) return;
    if (!pendingReview.scope && !draftReady) return;
    void runReview(pendingReview.scope, pendingReview.voice);
    onPendingReviewHandled();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingReview, draftReady]);

  // Load the persisted draft on mount ("" when there's no buffer yet).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const content = await getDraft(projectId);
        if (!cancelled) { setText(content); textRef.current = content; }
      } catch {
        /* leave empty; the placeholder shows */
      } finally {
        if (!cancelled) setDraftReady(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Q6 · mark the draft as saved — a steady, persistent "已保存 ✓", not a
  // toast that fades back out. It stays exactly as-is until the next edit
  // flips it to "dirty" (see onChange), so it never appears/disappears.
  function flashSaved() {
    setSaveStatus("saved");
  }

  // Q6 · the one place that actually calls putBuffer for the debounce/periodic
  // path. Always sends the LATEST content (textRef, read at call time) rather
  // than whatever triggered this attempt, so a retry after a failure never
  // resends stale text once the student has kept typing. On failure the local
  // (dirty) text is never touched — only the status flips to "retrying" and a
  // backoff timer schedules another attempt.
  async function flushSave() {
    if (savingRef.current || !dirtyRef.current) return;
    savingRef.current = true;
    setSaveStatus("saving");
    const attemptedContent = textRef.current;
    try {
      await putBuffer(projectId, attemptedContent);
      retryAttempt.current = 0;
      if (textRef.current === attemptedContent) {
        // nothing changed while the request was in flight — clean.
        dirtyRef.current = false;
        flashSaved();
      } else {
        // more edits landed mid-save — still dirty, catch up shortly.
        setSaveStatus("dirty");
        scheduleSave(600);
      }
    } catch {
      setSaveStatus("retrying");
      const attempt = ++retryAttempt.current;
      const delay = Math.min(3000 * 2 ** (attempt - 1), 20000); // 3s, 6s, 12s, capped at 20s
      if (retryTimer.current) clearTimeout(retryTimer.current);
      retryTimer.current = setTimeout(() => { void flushSave(); }, delay);
    } finally {
      savingRef.current = false;
    }
  }

  function scheduleSave(delayMs: number) {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => { void flushSave(); }, delayMs);
  }

  // Q6 · a periodic safety-net flush independent of the debounce — a student
  // typing continuously keeps resetting the debounce timer, so without this a
  // long unbroken stretch of edits would never actually save until she pauses.
  useEffect(() => {
    const id = setInterval(() => {
      if (dirtyRef.current && !savingRef.current) void flushSave();
    }, 18000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  // Flush a pending autosave on unmount/tab-switch so a last edit is never
  // lost — best-effort (fire-and-forget; there's no component left to show a
  // retry state to), same content-freshness guarantee as flushSave above.
  useEffect(
    () => () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
      if (retryTimer.current) clearTimeout(retryTimer.current);
      if (dirtyRef.current) {
        void putBuffer(projectId, textRef.current).catch(() => {/* best-effort; nothing left to retry against */});
      }
    },
    [projectId],
  );

  function onChange(next: string) {
    setText(next);
    textRef.current = next;
    dirtyRef.current = true;
    setSelPop(null); // any edit invalidates the floating selection chip
    // a fresh edit deserves a prompt retry cadence again, not whatever backoff
    // a previous failure had climbed to.
    retryAttempt.current = 0;
    if (retryTimer.current) { clearTimeout(retryTimer.current); retryTimer.current = null; }
    // reflect the edit immediately (unless a save is already in flight, whose
    // "saving" label takes precedence) so "已保存" doesn't linger stale.
    if (!savingRef.current) setSaveStatus("dirty");
    scheduleSave(1200);
  }

  async function handleFile(file: File) {
    const name = file.name.toLowerCase();
    if (TEXT_EXT.some((ext) => name.endsWith(ext))) {
      const content = await file.text();
      setText(content);
      textRef.current = content;
      dirtyRef.current = true;
      setUploadNote(null);
      setMode("write");
      scheduleSave(300); // route through the same retry/status pipeline as onChange
    } else {
      // .docx / .pdf and friends — accepted but not parsed yet. Don't crash;
      // just tell the student we've noted it.
      setUploadNote(`已上传「${file.name}」，正文解析稍后支持。`);
    }
  }

  // #9 · insert a fragment (from the materials sidebar) into the draft at the
  // caret — 印记 never authors, the STUDENT places her own material. Reads the
  // live text/caret from refs; separates with blank lines; no-op when locked.
  function insertAtCaret(t: string, referenceId?: string) {
    if (locked) return;
    const frag = t.trim();
    if (!frag) return;
    setMode("write");
    setPane("edit");
    // #5 · in 分节 mode the sections editor owns placement (into the focused
    // section); the free textarea path below only runs in 自由 mode. G3 · the
    // referenceId (when the fragment came from a library source) rides along so
    // the sections editor can record the source→section citation link.
    if (layout === "sections" && sectionInsertRef.current) {
      sectionInsertRef.current(frag, referenceId);
      return;
    }
    const cur = textRef.current;
    const ta = draftRef.current;
    // honor the caret only once the textarea has been focused — a never-focused
    // textarea reports selectionStart 0, which would insert above the intro; in
    // that case append at the end instead (review L3).
    const start = ta && hasFocusedRef.current ? ta.selectionStart : cur.length;
    const end = ta && hasFocusedRef.current ? ta.selectionEnd : cur.length;
    const before = cur.slice(0, start);
    const after = cur.slice(end);
    const lead = before && !before.endsWith("\n") ? "\n\n" : "";
    const tail = after && !after.startsWith("\n") ? "\n\n" : "";
    const next = before + lead + frag + tail + after;
    onChange(next);
    const caret = (before + lead + frag).length;
    requestAnimationFrame(() => {
      const el = draftRef.current;
      if (el) { el.focus(); el.setSelectionRange(caret, caret); }
    });
  }
  // Register the inserter once; a ref holds the latest closure so the stable
  // registered fn always sees current state. Unregister on unmount so the
  // sidebar's insert no-ops when the draft tab isn't mounted.
  const insertRef = useRef<(t: string, referenceId?: string) => void>(() => {});
  insertRef.current = insertAtCaret;
  useEffect(() => {
    registerInsert((t, referenceId) => insertRef.current(t, referenceId));
    return () => registerInsert(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // S1 · scroll+select the text a 批注 points at. Anchor = the sentence `quote`
  // (found by indexOf) or a "第N段" `locator` (the Nth blank-line block). Reads
  // the live textarea (draftRef) so it's robust to edits; approximate if the
  // draft was edited after the 批注 was written. No-op in 分节 mode / when the
  // anchor isn't found (nothing to jump to).
  function scrollToAnchor(a: { quote?: string; locator?: string }) {
    setMode("write");
    setPane("edit");
    requestAnimationFrame(() => {
      const ta = draftRef.current;
      if (!ta) return;
      const src = ta.value;
      let start = -1;
      let end = -1;
      const q = a.quote?.trim();
      if (q) {
        const i = src.indexOf(q);
        if (i >= 0) { start = i; end = i + q.length; }
      }
      if (start < 0 && a.locator) {
        const n = parseInt(a.locator.replace(/[^0-9]/g, ""), 10);
        if (n >= 1) {
          const blocks: Array<[number, number]> = [];
          const re = /\n{2,}/g;
          let last = 0;
          let m: RegExpExecArray | null;
          while ((m = re.exec(src)) !== null) { blocks.push([last, m.index]); last = m.index + m[0].length; }
          blocks.push([last, src.length]);
          const b = blocks[n - 1];
          if (b) { start = b[0]; end = b[1]; }
        }
      }
      if (start < 0) return;
      ta.focus();
      ta.setSelectionRange(start, end);
      // Textareas have no "scroll selection into view", so approximate from the
      // fraction of text before the anchor and center it a third down the box.
      const frac = start / Math.max(1, src.length);
      ta.scrollTop = Math.max(0, frac * ta.scrollHeight - ta.clientHeight / 3);
    });
  }
  const scrollRefFn = useRef(scrollToAnchor);
  scrollRefFn.current = scrollToAnchor;
  useEffect(() => {
    registerScroll?.((a) => scrollRefFn.current(a));
    return () => registerScroll?.(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Q2 · once a 整稿体检 result (or an in-flight/errored attempt) exists, lay
  // the draft out left/review right instead of stacking the panel below the
  // textarea — the student can read the advice next to her own words. Widen
  // the pane's max-width only in that state so the review column has room;
  // stacks back to one column below the lg breakpoint (narrow widths).
  const showReview = mode === "write" && !!(reviewError || reviewing || review);

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-y-auto px-8 py-3">
      <div
        ref={paneRef}
        className={`relative mx-auto flex min-h-0 w-full flex-1 flex-col ${showReview ? "max-w-6xl" : "max-w-2xl"}`}
      >
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            {!locked && (
              <Segmented
                options={[
                  { value: "write", label: "写在这里" },
                  { value: "upload", label: "我在别处写了" },
                ]}
                value={mode}
                onChange={(v) => setMode(v as "write" | "upload")}
              />
            )}
            {mode === "write" && (
              <Segmented
                options={[
                  { value: "edit", label: "写" },
                  { value: "preview", label: "预览" },
                ]}
                value={pane}
                onChange={(v) => { if (v === "preview") setSelPop(null); setPane(v as "edit" | "preview"); }}
              />
            )}
            {mode === "write" && pane === "edit" && (
              <Segmented
                options={[
                  { value: "free", label: "自由" },
                  { value: "sections", label: "分节" },
                ]}
                value={layout}
                onChange={(v) => setLayout(v as "free" | "sections")}
              />
            )}
          </div>
          {mode === "write" && (
            <div className="flex items-center gap-2">
              <span className="text-[12px] font-semibold text-mk-faint">{words} 字</span>
              {!locked && <SaveStatusIndicator status={saveStatus} />}
              {!locked && (
                <>
                  {/* #6 · 体检视角: each option carries a plain-language description so
                      the lens is legible before picking; the closed control shows the
                      chosen lens + what it does. */}
                  <span className="text-[12px] font-semibold text-mk-faint">视角</span>
                  <select
                    value={voice}
                    onChange={(e) => setVoice(e.target.value as ReviewVoice)}
                    aria-label="体检视角"
                    title="换个视角，印记体检整稿的侧重就不同"
                    className="max-w-[13rem] rounded-mk-md border border-mk-border bg-mk-surface px-2 py-1 text-[12px] text-mk-ink outline-none focus:border-mk-accent"
                  >
                    {VOICE_ORDER.map((v) => (
                      <option key={v} value={v}>{VOICE_META[v].label} · {VOICE_META[v].desc}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => void runReview()}
                    disabled={reviewing || text.trim() === ""}
                    className="rounded-mk-md bg-mk-accent px-3 py-1.5 text-[14px] font-bold text-white transition hover:bg-mk-accent-600 disabled:opacity-50"
                  >
                    {reviewing ? "体检中…" : "让印记体检整稿"}
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={() => { void exportDraftDocx(text, { title }).catch(() => {/* never crash the room */}); }}
                disabled={text.trim() === ""}
                className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[14px] font-semibold text-mk-muted hover:text-mk-accent disabled:opacity-50"
              >
                导出成品 .docx
              </button>
            </div>
          )}
        </div>
        {locked && (
          <p className="mb-3 rounded-mk-md border border-mk-border bg-mk-paper px-3 py-2 text-[14px] font-semibold text-mk-faint">这篇已归档，正文只读——你仍可预览与导出。</p>
        )}

        {/* Q2 · left/right split once a 体检 result (or an in-flight/errored
            attempt) is present: draft on the left, review on the right —
            responsive, stacks to one column below lg. With no review to show,
            this collapses back to the single centered column. */}
        <div className={`grid min-h-0 flex-1 gap-5 ${showReview ? "grid-cols-1 lg:grid-cols-2" : "grid-cols-1"}`}>
          <div className="flex min-h-0 flex-col">
            {mode === "write" ? (
              pane === "edit" ? (
                layout === "sections" ? (
                  <SectionedDraft
                    projectId={projectId}
                    text={text}
                    onChange={onChange}
                    locked={locked}
                    registerInsert={(fn) => { sectionInsertRef.current = fn; }}
                  />
                ) : (
                  <textarea
                    ref={draftRef}
                    value={text}
                    onChange={(e) => onChange(e.target.value)}
                    onFocus={() => { hasFocusedRef.current = true; }}
                    onMouseUp={onDraftMouseUp}
                    onScroll={() => setSelPop(null)}
                    readOnly={locked}
                    placeholder="在这里写你的草稿……（支持 Markdown）"
                    className={`min-h-0 flex-1 resize-none rounded-mk-lg border border-mk-input-border p-5 font-sans text-mk-body leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint ${locked ? "bg-mk-paper cursor-default" : "bg-mk-surface focus:border-mk-accent"}`}
                  />
                )
              ) : (
                <MarkdownPreview text={text} />
              )
            ) : (
              <div
                className="flex min-h-0 flex-1 flex-col items-center justify-center rounded-mk-lg border-2 border-dashed border-mk-input-border bg-mk-surface px-6 text-center"
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault();
                  const f = e.dataTransfer.files[0];
                  if (f) void handleFile(f);
                }}
              >
                <span className="text-mk-accent"><Icon name="writing" size={28} /></span>
                <p className="mt-3 text-[15px] font-bold text-mk-ink">把你写好的文档拖进来</p>
                <p className="mt-1 text-[14px] text-mk-faint">Word / PDF / Markdown——印记读进来后，也能和你聊这一稿</p>
                <input
                  ref={fileInput}
                  type="file"
                  accept=".md,.markdown,.txt,.docx,.pdf"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void handleFile(f);
                    e.target.value = "";
                  }}
                />
                <button type="button" onClick={() => fileInput.current?.click()} className="mt-4 rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600">选择文件</button>
                {uploadNote && <p className="mt-3 text-[14px] font-semibold text-mk-warning">{uploadNote}</p>}
              </div>
            )}
            {/* #7 · floating "问印记" chip — appears next to a text selection; clicking
                it pins that part into the coach composer (replaces the old top button).
                onMouseDown preventDefault keeps the textarea selection alive through the
                click, so we still have the pinned text. Positioned against paneRef
                (the outer relative container), so it stays correct regardless of this
                inner column wrapper — which isn't itself a positioning context. */}
            {selPop && mode === "write" && pane === "edit" && !locked && (
              <div
                // sit above the pointer, but flip below when the selection is near the
                // pane top so the chip never clips over the toolbar (review L3).
                style={{ left: selPop.x, top: selPop.y, transform: selPop.y < 44 ? "translate(-50%, 45%)" : "translate(-50%, -130%)" }}
                className="absolute z-20 flex items-center gap-1 whitespace-nowrap rounded-full bg-mk-accent p-1 shadow-mk-md"
                onMouseDown={(e) => e.preventDefault()}
              >
                {/* #7 · chat about this part */}
                <button
                  type="button"
                  onClick={() => { onFocusPart(selPop.text); setSelPop(null); }}
                  className="flex items-center gap-1 rounded-full px-2.5 py-1 text-[12px] font-bold text-white hover:bg-white/15"
                >
                  <Icon name="spark" size={13} /> 问印记
                </button>
                <span className="h-3.5 w-px bg-white/30" />
                {/* #8 · run the chosen voice's 体检 on just this paragraph */}
                <button
                  type="button"
                  onClick={() => { const t = selPop.text; setSelPop(null); void runReview(t); }}
                  className="rounded-full px-2.5 py-1 text-[12px] font-bold text-white hover:bg-white/15"
                >
                  体检这段
                </button>
              </div>
            )}
          </div>
          {showReview && (
            <div className="min-h-0">
              <DraftReviewPanel scope={reviewScope} reviewing={reviewing} error={reviewError} review={review} onClose={() => { setReview(null); setReviewError(null); }} />
            </div>
          )}
        </div>
        <p className="mt-2 text-center text-[12px] text-mk-faint">你写，印记只在一旁陪你想——它不替你写正文。</p>
      </div>
    </div>
  );
}

// #5 · the 分节 draft editor: write under outline-driven headings. A VIEW over
// the same Markdown draft string (parses in, serializes out via onChange), so
// 体检 / 导出 / 预览 all consume the draft unchanged. Free-form text is one
// toggle away — this organizes the student's structure, it never authors or
// lays out her deliverable (铁律②).
function SectionedDraft({
  projectId,
  text,
  onChange,
  locked,
  registerInsert,
}: {
  projectId: string;
  text: string;
  onChange: (next: string) => void;
  locked: boolean;
  registerInsert: (fn: ((t: string, referenceId?: string) => void) | null) => void;
}) {
  const [sections, setSections] = useState<DraftSection[]>(() => parseSections(text));
  const [outlineHeads, setOutlineHeads] = useState<string[]>([]);
  const [focusId, setFocusId] = useState<string | null>(null);
  const sectionsRef = useRef(sections);
  sectionsRef.current = sections;

  useEffect(() => {
    let alive = true;
    getOutline(projectId)
      .then((nodes) => { if (alive) setOutlineHeads(nodes.filter((n) => n.depth === 0 && n.text.trim()).map((n) => n.text.trim())); })
      .catch(() => {/* leave empty — the generate button just won't show */});
    return () => { alive = false; };
  }, [projectId]);

  function commit(next: DraftSection[]) {
    setSections(next);
    onChange(serializeSections(next));
  }
  const setHeading = (id: string, heading: string) => commit(sectionsRef.current.map((s) => (s.id === id ? { ...s, heading } : s)));
  const setBody = (id: string, body: string) => commit(sectionsRef.current.map((s) => (s.id === id ? { ...s, body } : s)));
  const removeSection = (id: string) => commit(sectionsRef.current.filter((s) => s.id !== id));
  const addSection = () => { const s = newSection(); commit([...sectionsRef.current, s]); setFocusId(s.id); };
  const generate = () => { const gen = sectionsFromOutline(outlineHeads, sectionsRef.current); if (gen.length) commit([...sectionsRef.current, ...gen]); };

  // Insert a fragment (materials sidebar) into the focused section's body — else
  // the last section, else a new intro when there are none yet. G3 · when the
  // fragment came from a library source (referenceId set), record the
  // source→section citation link, keyed by the target section's heading (the
  // faithful "where in the essay" available here; the freeform sections carry
  // headings, not machine claim keys). Best-effort — never blocks the insert.
  function insert(t: string, referenceId?: string) {
    if (locked) return;
    const frag = t.trim();
    if (!frag) return;
    const cur = sectionsRef.current;
    const target = (focusId && cur.some((s) => s.id === focusId) ? focusId : cur[cur.length - 1]?.id) ?? null;
    if (!target) {
      commit([{ ...newSection(0), body: frag }]);
      if (referenceId) void recordCitation(projectId, referenceId, "正文").catch(() => {});
      return;
    }
    if (referenceId) {
      const heading = cur.find((s) => s.id === target)?.heading.trim();
      void recordCitation(projectId, referenceId, heading || "正文").catch(() => {});
    }
    commit(cur.map((s) => (s.id === target ? { ...s, body: s.body ? `${s.body}\n\n${frag}` : frag } : s)));
  }
  const insertRef = useRef(insert);
  insertRef.current = insert;
  useEffect(() => {
    registerInsert((t, referenceId) => insertRef.current(t, referenceId));
    return () => registerInsert(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-4">
      {!locked && (
        <div className="mb-3 flex flex-wrap items-center gap-2">
          {outlineHeads.length > 0 && (
            <button type="button" onClick={generate} className="rounded-mk-md border border-mk-accent/40 px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50">＋ 从大纲生成章节</button>
          )}
          <span className="text-[12px] text-mk-faint">在小标题下写；从右侧「材料」插入会落到你正在写的这一节。</span>
        </div>
      )}
      {sections.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-10 text-center">
          <p className="text-[14px] text-mk-faint">还没有章节。{outlineHeads.length > 0 ? "用大纲生成，或" : ""}加一节，在标题下写。</p>
          {!locked && <button type="button" onClick={addSection} className="rounded-mk-md bg-mk-accent px-3 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600">＋ 加一节</button>}
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {sections.map((s) => (
            <section key={s.id} className="group rounded-mk-lg border border-mk-border bg-mk-paper p-3">
              {s.level > 0 ? (
                <input
                  value={s.heading}
                  onChange={(e) => setHeading(s.id, e.target.value)}
                  onFocus={() => setFocusId(s.id)}
                  readOnly={locked}
                  placeholder="小标题……"
                  className={`w-full bg-transparent font-sans font-bold text-mk-ink outline-none placeholder:text-mk-faint ${s.level === 1 ? "text-[16px]" : "text-[14px]"}`}
                />
              ) : (
                <div className="mb-1 text-[12px] font-bold uppercase tracking-wide text-mk-faint">开头（无标题）</div>
              )}
              <textarea
                value={s.body}
                onChange={(e) => setBody(s.id, e.target.value)}
                onFocus={() => setFocusId(s.id)}
                readOnly={locked}
                rows={4}
                placeholder="在这一节写……"
                className="mt-1.5 w-full resize-y bg-transparent font-sans text-[14px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint"
              />
              {!locked && (
                <div className="mt-1 flex justify-end">
                  <button type="button" onClick={() => removeSection(s.id)} className="text-[12px] font-semibold text-mk-faint opacity-0 transition hover:text-mk-danger group-hover:opacity-100">删除本节</button>
                </div>
              )}
            </section>
          ))}
          {!locked && (
            <button type="button" onClick={addSection} className="rounded-mk-md border border-dashed border-mk-border py-2 text-[14px] font-semibold text-mk-faint hover:border-mk-accent hover:text-mk-accent">＋ 加一节</button>
          )}
        </div>
      )}
    </div>
  );
}

// DraftReviewPanel — WA: renders the 整稿体检 advice read-only. Each row names a
// criterion, which descriptor band the draft evidences, what's still missing,
// and a suggested DIRECTION (not a rewrite). The student revises in her own words.
function DraftReviewPanel({
  scope,
  reviewing,
  error,
  review,
  onClose,
}: {
  scope: "draft" | "part";
  reviewing: boolean;
  error: string | null;
  review: DraftReviewResult | null;
  onClose: () => void;
}) {
  // Q2 · this now sits BESIDE the draft (not below it), so it fills its
  // column's height and scrolls its own content instead of capping at a fixed
  // max-height — the header (title + 收起) stays put while the list scrolls.
  //
  // #11 · reassigned off the (collapsed) mk-accent onto the peach macaron —
  // this "AI is highlighting/reviewing your text" family (this panel, the
  // quoted-part callout, 就这一段 pin, 体检这段 chip) needs to stay visually
  // distinct from mk-accent's everyday-chrome role (buttons, tabs) once both
  // legacy families alias onto the same var.
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-mk-lg border border-mk-peach/40 bg-mk-peach-bg p-4">
      <div className="mb-2 flex flex-none items-center justify-between">
        <p className="text-[14px] font-bold text-mk-ink">{scope === "part" ? "印记体检了你选中的这一段" : "印记的整稿体检"} · 供你参考，不替你改字</p>
        <button type="button" onClick={onClose} className="text-[12px] font-semibold text-mk-faint hover:text-mk-muted">收起</button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {reviewing ? (
          <ReviewingHint />
        ) : error ? (
          <p className="text-[14px] font-semibold text-mk-danger">{error}</p>
        ) : review ? (
          review.items.length === 0 ? (
            <p className="text-[14px] text-mk-faint">这一稿没跑出具体条目——可能正文还太短，先多写一点再体检。</p>
          ) : (
            <>
              <p className="mb-2 text-[12px] text-mk-faint">已存一版（{review.wordCount} 字{review.inBand ? " · 在字数区间内" : " · 字数偏离区间"}）。</p>
              <ul className="flex flex-col gap-2">
                {review.items.map((it: ReviewItem, i: number) => (
                  <li key={i} className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
                    <div className="flex items-baseline gap-2">
                      <span className="text-[14px] font-bold text-mk-accent">{it.criterion_name || it.criterion_code}</span>
                      {it.band && <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">{it.band}</span>}
                    </div>
                    {it.evidence && <p className="mt-1 text-[14px] text-mk-ink"><span className="font-semibold">现在做到：</span>{it.evidence}</p>}
                    {it.missing && <p className="mt-1 text-[14px] text-mk-muted"><span className="font-semibold">还差：</span>{it.missing}</p>}
                    {it.fix && <p className="mt-1 text-[14px] text-mk-peach-fg"><span className="font-semibold">可以往哪想：</span>{it.fix}</p>}
                  </li>
                ))}
              </ul>
            </>
          )
        ) : null}
      </div>
    </div>
  );
}

// Q6 · a small, quiet autosave indicator beside the 字数 counter — never a
// nagging modal. Always renders SOMETHING in a fixed-width slot (never null)
// so it can't appear/disappear or shift the 字数 counter next to it; "已保存"
// is the calm resting state (also covers a freshly-loaded, untouched draft)
// and stays put — it is never faded back out to nothing.
function SaveStatusIndicator({ status }: { status: "saved" | "dirty" | "saving" | "retrying" }) {
  const label =
    status === "saving" ? "保存中…"
    : status === "retrying" ? "未保存 · 正在重试…"
    : status === "dirty" ? "未保存…"
    : "已保存 ✓";
  return (
    <span
      // #11 · retrying is a genuine "still trying, having trouble" state — a
      // real semantic upgrade onto mk-warning rather than the old reused accent hue.
      className={`inline-block min-w-[8.5em] text-[12px] font-semibold ${status === "retrying" ? "text-mk-warning" : "text-mk-faint"}`}
    >
      {label}
    </span>
  );
}

/* ---------- right · AI rail ---------- */

const RAIL_GREETING: ChatMsg = {
  role: "ai",
  text: "把你正在纠结的那一段贴过来，或者告诉我它想让读者信什么——我们从这个目的倒推它够不够。",
};

// Batch5 follow-up · the persistent shelf's card GROUP depends on which of the
// three main panels (大纲/片段/正文) is active — different thinking work needs
// different tools, though a card may reasonably serve more than one panel
// (e.g. argument-map helps both skeleton-planning and final-draft checking).
// All still submit through the same reflectProjectCard(..., "writing") path —
// only the OFFERED set changes, not the plumbing (all persist through
// /cards/persist's writing-deck allowlist).
// all-statuses.md §4/§6 · card subsets per doc/panel. Proposal = no cards (its
// forced panel is "outline", kept empty). Essay = 写作卡 only while writing
// claims: PEE写作卡 (pee) + 论证解剖/论证地图 (argument-map). The outline panel is
// structure work, not argument writing → no cards.
const PANEL_DECK: Record<"outline" | "snippets" | "draft", string[]> = {
  outline: [],
  snippets: ["pee", "argument-map"],
  draft: ["pee", "argument-map"],
};
const PANEL_LABEL: Record<"outline" | "snippets" | "draft", string> = {
  outline: "大纲",
  snippets: "片段",
  draft: "正文",
};

// CoachRail's own ChatMsg[] history → the shared ChatLog's ChatMessage[]. A
// card-turn (msg.card set) maps to a "system"-role message carrying only
// `node` — ChatLog's system row has no bubble chrome, letting `CardTurnChip`
// be the whole message instead of nesting inside a second bubble (mirrors
// PlanBlock/ReviewBlock's mapper). A quoted part (#9-second, 就这一段) rides
// as its own callout ABOVE the turn's text, inside the same `node` — ChatLog
// has no separate above-bubble slot, so the callout renders inside the
// bubble; peach (not mk-accent) keeps it visually distinct from the
// surrounding accent-tinted student bubble. An AI turn's text renders through
// the shared `ChatMarkdown` (bold/lists/links/code); a student turn stays
// plain text — she types prose, not markup.
function toChatMessages(chat: StudioChatMsg[]): ChatMessage[] {
  return chat.map((m, i) =>
    m.card
      ? { id: String(i), role: "system", node: <CardTurnChip card={m.card} /> }
      : {
          id: String(i),
          role: m.role === "ai" ? "assistant" : "student",
          node: (
            <>
              {m.quotedPart && (
                <blockquote className="mb-1 rounded-mk-sm border-l-[3px] border-mk-peach bg-mk-peach-bg px-2.5 py-1.5 text-[12px] italic leading-snug text-mk-muted">
                  {m.quotedPart}
                </blockquote>
              )}
              {m.role === "ai" ? (
                <ChatMarkdown text={m.text} />
              ) : (
                <span className="whitespace-pre-wrap">{m.text}</span>
              )}
            </>
          ),
        },
  );
}

function CoachRail({
  projectId,
  focusPart,
  onClearFocus,
  locked,
  onCardArtifact,
  sectionOptions,
  onRunReview,
  activePanel,
  recap,
}: {
  projectId: string;
  focusPart: string | null;
  onClearFocus: () => void;
  locked: boolean;
  // #8 · adds the compiled card paragraph as a snippet filed under `section`
  // (null = 未归类) — only called when the student explicitly taps 收进片段.
  onCardArtifact: (text: string, section: string | null) => void;
  // Known 片段 board section labels the student can file the artifact under.
  sectionOptions: string[];
  // #8-second · request a voice-scoped 体检 from the persistent shelf below.
  // No scope arg → whole draft; a scope string → just that paragraph.
  onRunReview: (scope: string | undefined, voice: ReviewVoice) => void;
  // Batch5 follow-up · which of the three main panels is active — selects the
  // shelf's card group (PANEL_DECK) and gates 正文·检查 (examiner voices only
  // make sense once there's prose to check).
  activePanel: "outline" | "snippets" | "draft";
  recap?: string | null;
}) {
  // The coach thread is HOISTED to WorkspaceContainer (persists across 写作↔立项
  // room swaps, loaded once per project). This rail reads/appends the shared
  // store instead of holding its own chat state; the greeting is now a
  // display-only fallback (see `displayChat`), never stored.
  const { messages, setMessages, sending, activeProjectIdRef, sendStudioTurn } = useStudioChat();
  // Guard shared-store writes after an await: if the student switched PROJECTS
  // while a turn was in flight, don't append its reply into (or clear the busy
  // flag of) the now-different project. A plain room switch (same project) passes.
  const isActiveProject = () => activeProjectIdRef.current === projectId;
  const [draft, setDraft] = useState("");
  // `openCardId` is the card the student CHOSE to open from the self-summon
  // shelf — the only path to a card sheet, so triggering stays automatic while
  // opening is the student's tap (铁律). 印记's OWN card proposal is now the
  // container's pendingCard → the shared card sheet (StudioTurnChips), not here.
  const [openCardId, setOpenCardId] = useState<string | null>(null);
  // #8 · a finished card's compiled paragraph, offered (never auto-added) as a
  // 片段 once the coach has responded to it. cardName is kept for the offer's
  // copy; cleared once collected or dismissed.
  const [pendingArtifact, setPendingArtifact] = useState<{ cardName: string; text: string } | null>(null);
  const [artifactSection, setArtifactSection] = useState<string>(UNFILED);

  async function send() {
    const text = draft.trim();
    if (!text || sending || locked) return;
    // WC · if a draft part is pinned, scope this turn to it so 印记 checks THAT
    // part's argument/function — never rewriting it. The pinned part also rides
    // as `quotedPart`, rendered as a styled callout above the bubble.
    const turnText = focusPart ? `就这一段想（帮我看它的论证与功能，别替我改写）：\n「${focusPart}」\n\n${text}` : text;
    setDraft("");
    // The ONE container-owned send loop appends the turn, calls the orchestrator,
    // applies the directive, and surfaces its note/card offers (StudioTurnChips).
    const ok = await sendStudioTurn(turnText, { quotedPart: focusPart ?? undefined });
    // Clear the pinned part only on success — a failed turn keeps it so she
    // needn't re-pin.
    if (ok) onClearFocus();
  }

  // Opening a shelf card is the student's explicit choice (铁律). On submit the
  // completed envelope persists (过程即数据) and the rail acknowledges it.
  function openProposedCard(cardId: string) {
    setOpenCardId(cardId);
  }
  // #8 · finishing a card now runs a coach turn that responds to what the
  // student wrote FIRST (mirrors the forming flow's reflect path) — it no
  // longer silently dumps a labeled-value block into 片段. Adding it as a
  // snippet is then a separate, explicit 收进片段 offer below.
  async function submitProposedCard(fieldValues: Record<string, unknown>, eventTrace: unknown[]) {
    const cardId = openCardId;
    setOpenCardId(null);
    if (!cardId) return;
    const spec = CARD_REGISTRY[cardId];
    const studentText = spec ? compileCardForCoach(spec, fieldValues) : "";
    try {
      const { reply, card } = await reflectProjectCard(projectId, cardId, fieldValues, eventTrace, "writing");
      if (!isActiveProject()) return; // student switched projects mid-reflect
      // A card turn renders as a content-first chip (card set); fall back to raw
      // compiled text only if the server didn't echo a card.
      if (card) setMessages((c) => [...c, { role: "student", text: studentText, card }]);
      else if (studentText) setMessages((c) => [...c, { role: "student", text: studentText }]);
      if (reply) {
        setMessages((c) => [...c, { role: "ai", text: reply }]);
      } else if (!card && !studentText) {
        setMessages((c) => [...c, { role: "ai", text: "这张卡还没填内容，先留着，想清楚了再来。" }]);
      }
      // #7/#8 · offer (never silently add) the compiled paragraph as a 片段.
      const artifact = spec ? compileCardEnvelope(spec, fieldValues) : "";
      if (artifact) {
        setArtifactSection(UNFILED);
        setPendingArtifact({ cardName: spec!.name, text: artifact });
      }
    } catch {
      if (!isActiveProject()) return; // student switched projects mid-reflect
      if (studentText) setMessages((c) => [...c, { role: "student", text: studentText }]);
      setMessages((c) => [...c, { role: "ai", text: "刚才没接住这张卡，等下再试一次。" }]);
    }
  }

  function collectArtifact() {
    if (!pendingArtifact) return;
    onCardArtifact(pendingArtifact.text, artifactSection === UNFILED ? null : artifactSection);
    setMessages((c) => [...c, { role: "ai", text: "收进了「片段」——去那儿看看、改改，随时能插进正文。" }]);
    setPendingArtifact(null);
    setArtifactSection(UNFILED);
  }

  // The room→panel contract (Task 4/5/6, spec §17): this room's WORK — the
  // 大纲/片段/正文 tabs + editor — renders in <main> (see WritingBlock above);
  // this COACH portals into the constant AiPanel via `useStudioAiSlot`. `slot`
  // is null when the panel is collapsed or this component renders outside a
  // studio shell (e.g. some tests) — in either case the coach content simply
  // doesn't render, never crashes.
  const slot = useStudioAiSlot();
  // The greeting is DISPLAY-ONLY now (never stored): an empty hoisted store (a
  // project with no coach turns yet) falls back to it for rendering. Once any
  // turn lands the store is non-empty and IT is what shows.
  const displayChat: StudioChatMsg[] = messages.length ? messages : [RAIL_GREETING];

  return (
    <>
      {slot &&
        createPortal(
          <div className="flex h-full flex-col gap-3 p-4">
            <header className="flex-none">
              <div className="flex items-center gap-2 text-mk-accent">
                <Icon name="spark" size={16} />
                <h2 className="font-sans text-[15px] font-bold">印记 · 陪你写</h2>
              </div>
              <p className="mt-1 text-[12px] text-mk-faint">聊提纲、挑逻辑、撞反例——但不替你写正文。</p>
            </header>

            <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
              <ChatLog messages={withRecap(recap, toChatMessages(displayChat))} thinking={sending} />
              {/* 印记's per-turn note/card OFFERS come from the ONE container store
                  (StudioTurnChips) — identical chips in chat-first, 立项 and 写作.
                  The self-summon writing-card shelf lives further below. */}
              {!openCardId && <StudioTurnChips />}
              {/* #8 · 收进片段 is an explicit offer, never a silent add — the coach has
                  already responded to the card's content above; this just asks
                  whether the compiled paragraph should also become a 片段. */}
              {pendingArtifact && !openCardId && (
                <div className="rounded-mk-md border border-mk-border bg-mk-paper p-3">
                  <p className="text-[12px] font-semibold text-mk-faint">要不要把《{pendingArtifact.cardName}》里写的收进「片段」？</p>
                  <p className="mt-1.5 max-h-28 overflow-y-auto whitespace-pre-wrap text-[14px] leading-relaxed text-mk-ink">{pendingArtifact.text}</p>
                  <div className="mt-2 flex flex-wrap items-center justify-end gap-2">
                    <label className="flex items-center gap-1 text-[12px] text-mk-faint">
                      归到
                      <select
                        value={artifactSection}
                        onChange={(e) => setArtifactSection(e.target.value)}
                        aria-label="把片段归到"
                        className="max-w-[9rem] rounded border border-mk-border bg-mk-surface px-1.5 py-0.5 text-[12px] text-mk-ink outline-none focus:border-mk-accent"
                      >
                        <option value={UNFILED}>未归类</option>
                        {sectionOptions.map((o) => <option key={o} value={o}>{o}</option>)}
                      </select>
                    </label>
                    <button type="button" onClick={() => setPendingArtifact(null)} className="text-[12px] font-semibold text-mk-faint hover:text-mk-ink">先不收</button>
                    <button type="button" onClick={collectArtifact} className="rounded-full bg-mk-accent px-3 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600">收进片段</button>
                  </div>
                </div>
              )}
            </div>

            {/* #8-second · the persistent tool shelf — always visible (no ＋ to
                hide it), sitting between the scrollable chat and the composer so
                it never needs scrolling to find. Grouped by what it targets:
                the active panel's writing thinking-cards (summoned onto a
                paragraph) and 正文·检查 (the four examiner voices, each
                choosable against the WHOLE draft or — once a paragraph is
                pinned via 问印记 — just 这段). Triggering a card/voice is
                automatic UI; OPENING the card sheet or SEEING the check result
                still needs her tap/click (铁律 · 不操纵). */}
            {/* all-statuses.md §4/§6 · the writing card shelf appears ONLY where the
                doc calls for writing cards: the proposal has NO summonable cards
                (activePanel is forced to "outline", whose deck is empty), and the
                essay offers cards only while writing claims (片段/正文). An empty
                deck + non-draft panel renders nothing (no empty shelf). */}
            {!locked && (PANEL_DECK[activePanel].length > 0 || activePanel === "draft") && (
              <div className="flex-none rounded-mk-md border border-mk-border bg-mk-paper p-2.5">
                {PANEL_DECK[activePanel].length > 0 && (
                  <>
                    <p className="mb-1.5 text-[12px] font-bold text-mk-faint">{PANEL_LABEL[activePanel]} · 挑一张写作卡，想清楚这一段的论证——你填，印记不替你写</p>
                    <div className="flex flex-wrap gap-1.5">
                      {PANEL_DECK[activePanel].map((id) => CARD_REGISTRY[id] && (
                        <button
                          key={id}
                          type="button"
                          onClick={() => openProposedCard(id)}
                          title={CARD_REGISTRY[id]!.purpose}
                          className="rounded-mk-sm border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-ink hover:border-mk-accent hover:text-mk-accent"
                        >
                          {CARD_REGISTRY[id]!.name}
                        </button>
                      ))}
                    </div>
                  </>
                )}
                {/* 正文·检查 (the four examiner voices) only makes sense once there's
                    prose to check — scoped to the 正文 panel, unlike the card shelf
                    above which spans all three. Peach (not mk-accent) so this
                    "AI is checking your writing" affordance stays visually
                    distinct from the shelf's mk-accent hover above (#11). */}
                {activePanel === "draft" && (
                  <>
                    <p className="mb-1.5 mt-2.5 text-[12px] font-bold text-mk-faint">正文·检查 · 换个视角体检{focusPart ? "（整稿，或只查你选中的这段）" : "（整稿）"}</p>
                    <div className="flex flex-col gap-1">
                      {VOICE_ORDER.map((v) => (
                        <div key={v} className="flex items-center gap-1.5">
                          <button
                            type="button"
                            onClick={() => onRunReview(undefined, v)}
                            title={`${VOICE_META[v].desc} · 体检整稿`}
                            className="rounded-mk-sm border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-ink hover:border-mk-peach hover:text-mk-peach-fg"
                          >
                            {VOICE_META[v].label}
                          </button>
                          {focusPart && (
                            <button
                              type="button"
                              onClick={() => onRunReview(focusPart, v)}
                              title="只体检你目前选中的这一段"
                              className="rounded-full border border-mk-peach/50 px-2 py-0.5 text-[12px] font-semibold text-mk-peach-fg hover:bg-mk-peach-bg"
                            >
                              这段
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}
            {focusPart && (
              <div className="flex flex-none items-center gap-2 rounded-mk-md border border-mk-peach/40 bg-mk-peach-bg px-3 py-2">
                <span className="flex-none text-[12px] font-bold text-mk-peach-fg">就这一段</span>
                <span className="min-w-0 flex-1 truncate text-[12px] text-mk-muted">{focusPart}</span>
                <button type="button" onClick={onClearFocus} className="flex-none text-[12px] font-semibold text-mk-faint hover:text-mk-muted">✕</button>
              </div>
            )}

            {locked ? (
              // #5 (review L2) · an archived project's process is sealed — the
              // writing coach takes no new turns/cards so 过程即数据 stays true
              // to the record.
              <div className="flex-none rounded-mk-md border border-mk-border bg-mk-paper p-3 text-center text-[12px] font-semibold text-mk-faint">这篇已归档——过程已封存，印记不再新增这里的思考。</div>
            ) : (
              <Composer
                value={draft}
                onChange={setDraft}
                onSend={() => void send()}
                state={sending ? "replying" : undefined}
                placeholder={focusPart ? "就这一段，你想问什么？" : "问问这段逻辑、这个结构……"}
                className="flex-none"
              />
            )}
          </div>,
          slot,
        )}

      {/* #3 · the opened tool card is a centered modal over the whole room, not a
          card buried in the coach panel. Triggering stays automatic (the proposal
          chip / deck); OPENING is the student's tap, and the sheet then fills the
          modal. Backdrop / 收起 closes it. */}
      {openCardId && CARD_REGISTRY[openCardId] && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={() => setOpenCardId(null)}>
          <div className="max-h-[88vh] w-full max-w-lg overflow-y-auto rounded-mk-lg bg-mk-surface shadow-mk-lg" onClick={(e) => e.stopPropagation()}>
            <StudioCardSheet
              spec={CARD_REGISTRY[openCardId]}
              onSubmit={(env) => submitProposedCard(env.field_values, env.event_trace)}
              onSkip={() => setOpenCardId(null)}
            />
          </div>
        </div>
      )}
    </>
  );
}
