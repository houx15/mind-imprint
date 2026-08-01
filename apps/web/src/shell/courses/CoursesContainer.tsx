import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

export function CoursesContainer({ onGoPortal, initialCourseId }: { onGoPortal?: () => void; initialCourseId?: string | null }) {
  // Deep-link: when opened from a card's "去学这张卡的课程" link, land on that
  // course's overview (report view) rather than the grid. Read once at mount —
  // this component is remounted on every tab switch into 课程.
  const [view, setView] = useState<View>(initialCourseId ? { name: "report", courseId: initialCourseId } : { name: "grid" });

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
      <CourseReport
        courseId={view.courseId}
        onBackToCourses={() => setView({ name: "grid" })}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return <CoursesView onOpenCourse={(id) => setView({ name: "player", courseId: id })} />;
}
