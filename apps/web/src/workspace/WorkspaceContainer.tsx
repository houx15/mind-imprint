import { useCallback, useEffect, useState } from "react";
import type { MaterialSource, PhaseTag, WorkspaceProjection } from "@mind-imprint/contracts";
import { api } from "../api";
import { ReadingRoom } from "../studio/reading/ReadingRoom";
import { Icon, BLOCK_META } from "./Icon";
import { Directory } from "./Directory";
import { getWorkspace, postProjectSummary, patchReference, type ReferenceBib } from "./api/workspace";
import { PlanBlock } from "./blocks/PlanBlock";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
import { ReviewBlock } from "./blocks/ReviewBlock";
import type { BlockKey } from "./blocks/mockData";

// The top-level workspace: a project is a set of four rooms
// (项目管理 · 阅读 · 写作 · 回顾) the student moves between freely. No open
// project ⇒ the <Directory>; an open project ⇒ the left rail + the active
// room, landing in Project Management.
//
// Project Management (项目管理) is now fully API-backed (slice 2); the other
// three rooms still run on local mock state (slices 3–5). The shell (identity,
// qualification, the room swap) is live.
export function WorkspaceContainer({ onFinished }: { onFinished?: (projectId?: string) => void }) {
  const [projectId, setProjectId] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<WorkspaceProjection | null>(null);
  const [room, setRoom] = useState<BlockKey>("plan");
  const [error, setError] = useState<string | null>(null);
  // Full-screen mode: hide the left room-rail so the active room (esp. the
  // reading room's 兔子洞地图 and the wide library table) gets the whole width.
  // Applies to the ENTIRE project workspace, not one room; persisted so it
  // survives room swaps and reloads.
  const [fullscreen, setFullscreen] = useState<boolean>(() => {
    try {
      return localStorage.getItem("mk-workspace-fullscreen") === "1";
    } catch {
      return false;
    }
  });
  const toggleFullscreen = useCallback(() => {
    setFullscreen((f) => {
      const next = !f;
      try {
        localStorage.setItem("mk-workspace-fullscreen", next ? "1" : "0");
      } catch {
        /* best-effort; a blocked storage must never break the toggle */
      }
      return next;
    });
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

  // No project open — the all-projects directory (its own create form carries
  // the empty affordance).
  if (projectId == null) {
    return <Directory onOpen={openProject} onViewReport={onViewReport} />;
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

  return (
    <div className="flex h-full w-full bg-mk-bg font-sans text-mk-ink">
      {!fullscreen && (
        <Rail room={room} onRoom={setRoom} onBack={backToAll} workspace={workspace} onFullscreen={toggleFullscreen} />
      )}
      <main className="relative min-w-0 flex-1 overflow-hidden">
        {/* Full-screen mode · a small floating control to bring the rail back. */}
        {fullscreen && (
          <button
            type="button"
            onClick={toggleFullscreen}
            title="退出全屏"
            className="absolute left-3 top-3 z-40 flex items-center gap-1.5 rounded-full border border-mk-border bg-mk-surface/95 px-3 py-1.5 text-[12px] font-bold text-mk-muted-2 shadow-sm backdrop-blur hover:border-mk-primary hover:text-mk-primary"
          >
            <Icon name="back" size={15} /> 退出全屏
          </button>
        )}
        {workspace && ((summary && !summaryDismissed) || carryForward) && (
          <div className="pointer-events-none absolute inset-x-0 top-0 z-30 flex flex-col items-center gap-2 px-4 pt-4">
            {summary && !summaryDismissed && (
              <div className="pointer-events-auto flex w-full max-w-2xl items-start gap-3 rounded-mk-lg border border-mk-border bg-mk-surface px-4 py-3 shadow-[0_12px_40px_rgba(28,35,51,0.18)]">
                <span className="mt-0.5 text-mk-primary">
                  <Icon name="spark" size={16} />
                </span>
                <p className="flex-1 text-[13.5px] leading-relaxed text-mk-ink">{summary}</p>
                <button
                  type="button"
                  onClick={() => setSummaryDismissed(true)}
                  aria-label="收起"
                  className="-mt-0.5 px-1 text-[16px] leading-none text-mk-muted-2 hover:text-mk-ink"
                >
                  ×
                </button>
              </div>
            )}
            {carryForward && (
              <div className="pointer-events-auto flex w-full max-w-2xl items-start gap-3 rounded-mk-lg border border-mk-accent/40 bg-mk-accent-tint/50 px-4 py-3 shadow-[0_12px_40px_rgba(28,35,51,0.18)]">
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
                  className="-mt-0.5 px-1 text-[16px] leading-none text-mk-muted-2 hover:text-mk-ink"
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
          <div className="flex h-full items-center justify-center text-[14px] text-mk-muted-2">加载中…</div>
        ) : (
          <>
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
          </>
        )}
      </main>
    </div>
  );
}

// A small "expand to full screen" glyph (diagonal out-arrows). Local to the
// rail — the shared Icon set has no full-screen name and this is its only use.
function ExpandIcon() {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 4H4v5M15 4h5v5M9 20H4v-5M15 20h5v-5" />
    </svg>
  );
}

function Rail({
  room,
  onRoom,
  onBack,
  workspace,
  onFullscreen,
}: {
  room: BlockKey;
  onRoom: (b: BlockKey) => void;
  onBack: () => void;
  workspace: WorkspaceProjection | null;
  onFullscreen: () => void;
}) {
  return (
    <nav className="flex w-[236px] flex-none flex-col border-r border-mk-border bg-mk-surface">
      <div className="border-b border-mk-border px-5 py-5">
        <div className="mb-3 flex items-center justify-between">
          <button type="button" onClick={onBack} className="flex items-center gap-1 text-[13px] font-semibold text-mk-muted-2 hover:text-mk-primary">
            <Icon name="back" size={16} /> 全部项目
          </button>
          <button
            type="button"
            onClick={onFullscreen}
            title="全屏（收起侧栏）"
            aria-label="全屏"
            className="flex h-7 w-7 items-center justify-center rounded-mk text-mk-muted-2 hover:bg-mk-bg hover:text-mk-primary"
          >
            <ExpandIcon />
          </button>
        </div>
        <h1 className="font-sans text-[15.5px] font-bold leading-snug text-mk-ink">{workspace?.title || "未命名项目"}</h1>
        <span className="mt-2 inline-block rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-bold text-mk-primary">{workspace?.qualification || "项目"}</span>
      </div>

      <div className="flex flex-1 flex-col gap-1 px-3 py-4">
        {BLOCK_META.map((b, i) => {
          const active = room === b.key;
          return (
            <button
              key={b.key}
              type="button"
              onClick={() => onRoom(b.key)}
              className={`group relative flex items-center gap-3 rounded-mk px-3 py-2.5 text-left transition ${
                active ? "bg-mk-primary-tint" : "hover:bg-mk-bg"
              }`}
            >
              {active && <span className="absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r bg-mk-primary" />}
              <span className={active ? "text-mk-primary" : "text-mk-muted-2 group-hover:text-mk-muted"}>
                <Icon name={b.key} size={19} />
              </span>
              <span className="flex flex-col">
                <span className={`text-[14px] font-bold ${active ? "text-mk-primary" : "text-mk-ink"}`}>{b.label}</span>
                <span className="text-[11px] font-medium tracking-wide text-mk-muted-2">
                  {String(i + 1).padStart(2, "0")} · {b.sub}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
