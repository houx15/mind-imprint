import { useCallback, useEffect, useRef, useState } from "react";
import type { MaterialSource, PhaseTag, WorkspaceProjection } from "@mind-imprint/contracts";
import { api } from "../api";
import { ReadingRoom } from "../studio/reading/ReadingRoom";
import { AiPanel, type AiPanelSide } from "../studio/ai/AiPanel";
import { StudioAiSlotContext } from "../studio/ai/StudioAiSlot";
import { Icon as UiIcon, ArrowLeft } from "@/ui/Icon";
import { Badge, Segmented } from "@/ui/feedback";
import { Icon, BLOCK_META } from "./Icon";
import { Directory } from "./Directory";
import { getWorkspace, postProjectSummary, patchReference, type ReferenceBib } from "./api/workspace";
import { PlanBlock } from "./blocks/PlanBlock";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
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
}: {
  onFinished?: (projectId?: string) => void;
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
  const [room, setRoom] = useState<BlockKey>("plan");
  const [error, setError] = useState<string | null>(null);
  // The constant AI panel's side + collapsed state (spec §17) — persisted so
  // it survives room swaps and reloads, same spirit as the old fullscreen
  // toggle it replaces. Default side is "right" per the brief.
  const [aiSide, setAiSide] = useState<AiPanelSide>(() => {
    try {
      const v = localStorage.getItem("mk-studio-ai-side");
      return v === "left" || v === "right" ? v : "right";
    } catch {
      return "right";
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
  const [summaryDismissed, setSummaryDismissed] = useState(false);

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
  }, [projectId]);

  // Load the opened project's lean projection whenever the opened id changes.
  // Landing room is always 项目管理.
  useEffect(() => {
    if (!projectId) return;
    let cancelled = false;
    setWorkspace(null);
    setError(null);
    setSummary(null);
    setSummaryDismissed(false);
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
  }, [projectId]);

  function openProject(id: string) {
    setProjectId(id);
    setRoom("plan");
    closeReadingSource();
  }

  function backToAll() {
    setProjectId(null);
    setWorkspace(null);
    closeReadingSource();
    setError(null);
  }

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

  const aiPanel = (
    <AiPanel side={aiSide} onFlip={flipAiSide} collapsed={aiCollapsed} onToggleCollapse={toggleAiCollapsed}>
      <div ref={aiSlotRef} className="h-full" />
    </AiPanel>
  );
  // The reading room is a distinct full-screen surface that owns its own coach
  // column (印记 · 找资料) — the shell's constant AiPanel would otherwise sit
  // empty beside it (list mode) or duplicate it as a second 印记 column (graph
  // mode). So the constant panel shows for every room EXCEPT reading.
  const showAiPanel = room !== "reading";

  return (
    <div className="flex h-full w-full flex-col bg-mk-paper font-sans text-mk-ink">
      <TopBar workspace={workspace} room={room} onRoom={setRoom} onBack={backToAll} />
      <div className="flex min-h-0 flex-1">
        {aiSide === "left" && showAiPanel && aiPanel}
        <main className="relative min-w-0 flex-1 overflow-hidden">
        {workspace && ((summary && !summaryDismissed) || carryForward) && (
          <div className="pointer-events-none absolute inset-x-0 top-0 z-30 flex flex-col items-center gap-2 px-4 pt-4">
            {summary && !summaryDismissed && (
              <div className="pointer-events-auto flex w-full max-w-2xl items-start gap-3 rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 shadow-mk-lg">
                <span className="mt-0.5 text-mk-accent">
                  <Icon name="spark" size={16} />
                </span>
                <p className="flex-1 text-[13.5px] leading-relaxed text-mk-ink">{summary}</p>
                <button
                  type="button"
                  onClick={() => setSummaryDismissed(true)}
                  aria-label="收起"
                  className="-mt-0.5 px-1 text-[16px] leading-none text-mk-faint hover:text-mk-ink"
                >
                  ×
                </button>
              </div>
            )}
            {carryForward && (
              <div className="pointer-events-auto flex w-full max-w-2xl items-start gap-3 rounded-mk-lg border border-mk-accent/40 bg-mk-accent-50/50 px-4 py-3 shadow-mk-lg">
                <span className="mt-0.5 text-mk-accent">
                  <Icon name="spark" size={16} />
                </span>
                <p className="flex-1 text-[13.5px] leading-relaxed text-mk-ink">
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
        ) : !workspace ? (
          <div className="flex h-full items-center justify-center text-[14px] text-mk-faint">加载中…</div>
        ) : (
          // StudioAiSlotContext: the room→panel portal contract. A room reads
          // `useStudioAiSlot()` and portals its coach content into the AiPanel's
          // body via `createPortal` — the room renders its WORK directly here in
          // <main>. plan / writing / reflection all portal their coach into the
          // constant panel. reading is the exception: it's a distinct
          // full-screen surface with its own coach column (see showAiPanel).
          <StudioAiSlotContext.Provider value={aiSlotEl}>
            {room === "plan" && (
              <PlanBlock
                key={projectId}
                projectId={projectId}
                title={workspace.title}
                qualification={workspace.qualification}
                proposal={workspace.proposal}
                createdAt={workspace.createdAt}
                onOpenRoom={setRoom}
                refreshWorkspace={refreshWorkspace}
              />
            )}
            {room === "reading" && (
              <ReadingBlock key={projectId} projectId={projectId} title={workspace.title} setReadingSource={openReadingSource} />
            )}
            {room === "writing" && (
              <WritingBlock
                key={projectId}
                projectId={projectId}
                title={workspace.title}
                proposal={workspace.proposal}
                status={workspace.status}
                writingFinished={workspace.writingFinished ?? false}
                onOpenRoom={setRoom}
                refreshWorkspace={refreshWorkspace}
              />
            )}
            {room === "reflection" && (
              <ReviewBlock
                key={projectId}
                projectId={projectId}
                proposal={workspace.proposal}
                status={workspace.status}
                writingFinished={workspace.writingFinished ?? false}
                onOpenRoom={setRoom}
                onFinished={backToAll}
              />
            )}
          </StudioAiSlotContext.Provider>
        )}
        </main>
        {aiSide === "right" && showAiPanel && aiPanel}
      </div>
    </div>
  );
}

// The top bar (spec §17): a small 「← 主页」capsule back to the Directory,
// the project's title + qualification, and the room switcher. Replaces the
// old left `Rail` — there is no more in-project sidebar.
function TopBar({
  workspace,
  room,
  onRoom,
  onBack,
}: {
  workspace: WorkspaceProjection | null;
  room: BlockKey;
  onRoom: (b: BlockKey) => void;
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
      <Segmented
        className="shrink-0"
        options={BLOCK_META.map((b) => ({ value: b.key, label: b.label }))}
        value={room}
        onChange={(v) => onRoom(v as BlockKey)}
      />
    </header>
  );
}
