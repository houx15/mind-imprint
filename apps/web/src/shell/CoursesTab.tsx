import { useEffect, useState } from "react";
import { Segmented } from "@/ui";
import { CoursesContainer } from "@/shell/courses/CoursesContainer";
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
}

export function CoursesTab({
  pendingCourseId,
  onPendingCourseConsumed,
  studentId,
  onGoPortal,
  onImmersiveChange,
}: CoursesTabProps) {
  const [sub, setSub] = useState<Sub>("courses");
  const [inCourse, setInCourse] = useState(false);
  // The course to open in CoursesContainer — seeded from the host deep-link,
  // or set when 学习记录 / 图鉴 requests a course. Consumed once.
  const [openId, setOpenId] = useState<string | null>(pendingCourseId ?? null);

  // Clear the host's one-shot deep-link on mount (openId already captured it).
  useEffect(() => {
    if (pendingCourseId) onPendingCourseConsumed();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const immersive = inCourse && sub === "courses";
  useEffect(() => {
    onImmersiveChange(immersive);
  }, [immersive, onImmersiveChange]);

  const requestOpen = (slug: string) => {
    setOpenId(slug);
    setSub("courses");
  };

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-mk-paper">
      {!immersive && (
        <div className="flex shrink-0 items-center justify-center border-b border-mk-border bg-mk-surface p-3">
          <Segmented
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
            initialCourseId={openId}
            onCourseConsumed={() => setOpenId(null)}
            studentId={studentId}
            onGoPortal={onGoPortal}
            onImmersiveChange={setInCourse}
          />
        ) : sub === "history" ? (
          <LearningHistory onOpenCourse={requestOpen} />
        ) : (
          <ToolkitCards onOpenCourse={requestOpen} />
        )}
      </div>
    </div>
  );
}
