import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { EmptyState, Button } from "@/ui";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

// The course-completion report is retired for now (course model unsettled —
// docs/superpowers/specs/2026-08-14-retire-old-evaluation-pipeline-design.md
// B.4). This is an inert placeholder, not a real surface: no data fetch, no
// logic — just an honest "not yet" plus the two affordances a finish screen
// needs (back to courses / onward into the studio). Do not wire it back to
// getCourseReport or CourseReport; that pipeline is gone.
function CourseCompletionPlaceholder({ onBackToCourses, onGoPortal }: { onBackToCourses: () => void; onGoPortal: () => void }) {
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "var(--mk-paper)", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12 }}>
      <EmptyState
        illustration="completed"
        title="课程完成报告即将上线"
        body="你已经学完这门课——详细的学习报告正在打磨中，先去写作工作室用起来吧。"
        action={{ label: "去写作工作室，用起来", onClick: onGoPortal }}
      />
      <Button variant="ghost" onClick={onBackToCourses}>
        返回课程
      </Button>
    </div>
  );
}

export function CoursesContainer({ onGoPortal, initialCourseId }: { onGoPortal?: () => void; initialCourseId?: string | null }) {
  // Deep-link: opening a course from anywhere (home's course cards, the
  // gallery's "去学这张卡的课程" link) lands in the PLAYER so the student can
  // actually learn it (the player resumes at their saved step). Finishing the
  // course lands on the completion view (currently an "即将上线" placeholder).
  // Read once at mount — this component is remounted on every tab switch into 课程.
  const [view, setView] = useState<View>(initialCourseId ? { name: "player", courseId: initialCourseId } : { name: "grid" });

  if (view.name === "player") {
    return (
      <CoursePlayer
        courseId={view.courseId}
        onExit={() => setView({ name: "grid" })}
        onFinish={() => setView({ name: "report", courseId: view.courseId })}
      />
    );
  }
  if (view.name === "report") {
    return (
      <CourseCompletionPlaceholder
        onBackToCourses={() => setView({ name: "grid" })}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return <CoursesView onOpenCourse={(id) => setView({ name: "player", courseId: id })} />;
}
