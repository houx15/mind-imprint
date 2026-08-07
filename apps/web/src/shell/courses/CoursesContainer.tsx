import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

export function CoursesContainer({ onGoPortal, initialCourseId }: { onGoPortal?: () => void; initialCourseId?: string | null }) {
  // Deep-link: opening a course from anywhere (home's course cards, the
  // gallery's "去学这张卡的课程" link) lands in the PLAYER so the student can
  // actually learn it (the player resumes at their saved step). The report is
  // reached by finishing the course or from the growth history. Read once at
  // mount — this component is remounted on every tab switch into 课程.
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
      <CourseReport
        courseId={view.courseId}
        onBackToCourses={() => setView({ name: "grid" })}
        onGoPortal={() => (onGoPortal ? onGoPortal() : setView({ name: "grid" }))}
      />
    );
  }
  return <CoursesView onOpenCourse={(id) => setView({ name: "player", courseId: id })} />;
}
