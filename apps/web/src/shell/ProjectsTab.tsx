import { useEffect, useRef, useState } from "react";
import { Segmented } from "@/ui";
import { WorkspaceContainer } from "@/workspace/WorkspaceContainer";
import { ReportsView } from "@/shell/report/ReportsView";
import type { StudioRoom, TourWritingView } from "@/tour/types";
import type { MaterialSource } from "@mind-imprint/contracts";

/**
 * ProjectsTab — the 项目 top-level surface after the nav restructure. A
 * Segmented switches between 我的项目 (the project directory → four-room studio,
 * `WorkspaceContainer`, unchanged internally) and 评估报告 (`ReportsView`, the
 * finished-project 过程评估报告 timeline that used to live under the retired 评估
 * tab).
 *
 * While a project is OPEN in the studio (`inProject`), the Segmented hides so
 * the studio is full-bleed — matching the platform nav rail, which the host
 * (`StudentApp`) hides via `onImmersiveChange`. Finishing a project drops out of
 * the studio and lands on that project's report (评估报告 sub, deep-linked).
 */

type Sub = "projects" | "reports";

export interface ProjectsTabProps {
  /** One-shot: open this project on entry (home's recent-project tiles). */
  pendingProjectId: string | null;
  onPendingProjectConsumed: () => void;
  /** One-shot: open the create drawer on entry (home's 新建 tiles). */
  autoOpenCreate: boolean;
  onAutoOpenCreateConsumed: () => void;
  /** One-shot: open this finished project's 评估报告 on entry (home project
   * card's ⋯ menu → 查看评估报告). */
  pendingReportId?: string | null;
  onPendingReportConsumed?: () => void;
  /** One-shot: drive the open project's studio to this room (the guided
   * tour's studio deep-link, P3 Task 1). */
  pendingRoom?: StudioRoom | null;
  onPendingRoomConsumed?: () => void;
  /** One-shot: drive the open reading room's inner 列表/探索图谱 view (the
   * guided tour's P5 deep-link). Threaded straight to WorkspaceContainer. */
  pendingReadingView?: "list" | "graph" | null;
  onPendingReadingViewConsumed?: () => void;
  /** One-shot: drive the open writing room to a document + tab (提案 片段 → the
   * 片段引导/写作卡, the guided tour's P6 Task 9 deep-link). Threaded straight to
   * WorkspaceContainer. */
  pendingWritingView?: TourWritingView | null;
  onPendingWritingViewConsumed?: () => void;
  /** One-shot: open this already-fetched demo `MaterialSource` into the real
   * immersive Reading Room as a read-only replay (the guided tour's P6
   * `openDemoReadingRoom` deep-link). Threaded straight to WorkspaceContainer. */
  pendingDemoReading?: { source: MaterialSource; referenceId: string } | null;
  onPendingDemoReadingConsumed?: () => void;
  /** True while a project is open (studio full-bleed) → host hides the nav rail. */
  onImmersiveChange: (immersive: boolean) => void;
  /** Task 9: the demo project's guard modal's 好，带我逛一遍 — threaded straight
   * to WorkspaceContainer → Directory. StudentApp implements it by playing
   * `journeyStarting("projects")`. */
  onRequestDemoTour?: () => void;
}

export function ProjectsTab({
  pendingProjectId,
  onPendingProjectConsumed,
  autoOpenCreate,
  onAutoOpenCreateConsumed,
  pendingReportId,
  onPendingReportConsumed,
  pendingRoom,
  onPendingRoomConsumed,
  pendingReadingView,
  onPendingReadingViewConsumed,
  pendingWritingView,
  onPendingWritingViewConsumed,
  pendingDemoReading,
  onPendingDemoReadingConsumed,
  onImmersiveChange,
  onRequestDemoTour,
}: ProjectsTabProps) {
  // A home 查看评估报告 deep-link lands on the 评估报告 sub, focused on that
  // project. Captured at mount (ProjectsTab remounts on each tab entry) so the
  // one-shot is consumed once — re-entering 项目 later shows 我的项目.
  const landOnReport = useRef(pendingReportId ?? null);
  const [sub, setSub] = useState<Sub>(landOnReport.current ? "reports" : "projects");
  const [inProject, setInProject] = useState(false);
  // Deep-link a specific report after finishing (consumed once so backing out
  // of the report lands on the list, not the same report again).
  const [reportFocus, setReportFocus] = useState<string | null>(landOnReport.current);

  useEffect(() => {
    if (landOnReport.current) onPendingReportConsumed?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A `pendingReportId` that arrives AFTER mount (the guided tour's
  // evaluation-report step deep-links the demo report while ProjectsTab is
  // already open in the studio — the mount-time `landOnReport` ref can't catch
  // it). Ref-guarded so it fires once per new id (mirrors WorkspaceContainer's
  // `initialProjectId`/`pendingRoom` one-shot pattern), and initialised to the
  // mount value so the same id isn't re-handled here after the mount path.
  const lastReport = useRef<string | null>(landOnReport.current);
  useEffect(() => {
    if (pendingReportId && pendingReportId !== lastReport.current) {
      lastReport.current = pendingReportId;
      setInProject(false);
      setSub("reports");
      setReportFocus(pendingReportId);
      onPendingReportConsumed?.();
    }
  }, [pendingReportId, onPendingReportConsumed]);

  useEffect(() => {
    onImmersiveChange(inProject);
  }, [inProject, onImmersiveChange]);

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-mk-paper">
      {!inProject && (
        <div className="flex shrink-0 items-center justify-center px-4 pb-1.5 pt-4">
          <Segmented
            variant="island"
            value={sub}
            onChange={(v) => setSub(v === "reports" ? "reports" : "projects")}
            options={[
              { value: "projects", label: "我的项目" },
              { value: "reports", label: "评估报告" },
            ]}
          />
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-hidden">
        {sub === "projects" ? (
          <WorkspaceContainer
            onFinished={(projectId?: string) => {
              setInProject(false);
              setReportFocus(projectId ?? null);
              setSub("reports");
            }}
            initialProjectId={pendingProjectId}
            onInitialProjectIdConsumed={onPendingProjectConsumed}
            pendingRoom={pendingRoom}
            onPendingRoomConsumed={onPendingRoomConsumed}
            pendingReadingView={pendingReadingView}
            onPendingReadingViewConsumed={onPendingReadingViewConsumed}
            pendingWritingView={pendingWritingView}
            onPendingWritingViewConsumed={onPendingWritingViewConsumed}
            pendingDemoReading={pendingDemoReading}
            onPendingDemoReadingConsumed={onPendingDemoReadingConsumed}
            autoOpenCreate={autoOpenCreate}
            onAutoOpenCreateConsumed={onAutoOpenCreateConsumed}
            onInProjectChange={setInProject}
            onRequestDemoTour={onRequestDemoTour}
          />
        ) : (
          <ReportsView initialProjectId={reportFocus} onFocusConsumed={() => setReportFocus(null)} />
        )}
      </div>
    </div>
  );
}
