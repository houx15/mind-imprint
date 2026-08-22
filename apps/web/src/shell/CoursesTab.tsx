import { useEffect, useState } from "react";
import { Segmented } from "@/ui";
import { CoursesContainer, type CourseOpenTarget } from "@/shell/courses/CoursesContainer";
import { LearningHistory } from "@/shell/courses/LearningHistory";
import { ToolkitCards } from "@/shell/growth/ToolkitCards";

/**
 * CoursesTab — the 课程 top-level surface after the nav restructure. A Segmented
 * switches between 课程 (the course list → player → report, `CoursesContainer`),
 * 学习记录 (`LearningHistory`), and 图鉴 (`ToolkitCards`, moved here from the
 * retired 评估 tab).
 *
 * While a course is being played (`CoursesContainer` reports immersive), the
 * Segmented hides and the host hides the nav rail — full-bleed, like the studio.
 * "Back from a course" returns to the 课程 grid WITH its chrome, which is the
 * "course page" the design asks for. Opening a course from 学习记录 / 图鉴
 * switches to the 课程 sub and hands the slug to `CoursesContainer`.
 */

type Sub = "courses" | "history" | "gallery";

export interface CoursesTabProps {
  /** One-shot: open this course on entry (home course cards / 图鉴 deep-link). */
  pendingCourseId: string | null;
  onPendingCourseConsumed: () => void;
  studentId?: string;
  /** Course report's "去写作工作室" — bubbles up so the host switches to 项目. */
  onGoPortal: () => void;
  /** True while a course is being played → host hides the nav rail. */
  onImmersiveChange: (immersive: boolean) => void;
  /** One-shot: open this sub-tab on entry (guided tour). */
  pendingSub?: Sub | null;
  onPendingSubConsumed?: () => void;
}

export function CoursesTab({
  pendingCourseId,
  onPendingCourseConsumed,
  studentId,
  onGoPortal,
  onImmersiveChange,
  pendingSub,
  onPendingSubConsumed,
}: CoursesTabProps) {
  const [sub, setSub] = useState<Sub>(pendingSub ?? "courses");
  const [inCourse, setInCourse] = useState(false);
  // The target to open in CoursesContainer — seeded from the host deep-link
  // (always a browse landing), or set when 学习记录 / 图鉴 requests a course.
  // Consumed once.
  const [openTarget, setOpenTarget] = useState<CourseOpenTarget | null>(
    pendingCourseId ? { slug: pendingCourseId, mode: "detail" } : null,
  );

  // Clear the host's one-shot deep-link on mount (openTarget already captured it).
  useEffect(() => {
    if (pendingCourseId) onPendingCourseConsumed();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // React to the tour's requested sub-tab. Seeded once in useState above (avoids
  // a first-paint flash), but CoursesTab stays mounted across tab switches, so
  // later setCoursesSub calls must re-derive `sub` here rather than relying on
  // the mount initializer.
  useEffect(() => {
    if (pendingSub) {
      setSub(pendingSub);
      onPendingSubConsumed?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingSub]);

  const immersive = inCourse && sub === "courses";
  useEffect(() => {
    onImmersiveChange(immersive);
  }, [immersive, onImmersiveChange]);

  // 图鉴 / home deep-links open the browse landing (detail).
  const requestOpen = (slug: string) => {
    setOpenTarget({ slug, mode: "detail" });
    setSub("courses");
  };
  // 学习记录 opens straight to the intent: 继续 (player) or 查看报告 (report).
  const requestOpenTarget = (t: CourseOpenTarget) => {
    setOpenTarget(t);
    setSub("courses");
  };

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-mk-paper">
      {!immersive && (
        <div className="flex shrink-0 items-center justify-center px-4 pb-1.5 pt-4" data-tour="courses-subswitcher">
          <Segmented
            variant="island"
            value={sub}
            onChange={(v) => setSub(v as Sub)}
            options={[
              { value: "courses", label: "课程" },
              { value: "history", label: "学习记录" },
              { value: "gallery", label: "图鉴" },
            ]}
          />
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-hidden">
        {sub === "courses" ? (
          <CoursesContainer
            initialOpen={openTarget}
            onCourseConsumed={() => setOpenTarget(null)}
            studentId={studentId}
            onGoPortal={onGoPortal}
            onImmersiveChange={setInCourse}
          />
        ) : sub === "history" ? (
          <LearningHistory onOpen={requestOpenTarget} />
        ) : (
          <ToolkitCards onOpenCourse={requestOpen} />
        )}
      </div>
    </div>
  );
}
