import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";
import { CourseReport } from "./CourseReport";

type View = { name: "grid" } | { name: "player"; courseId: string } | { name: "report"; courseId: string };

export function CoursesContainer({ onGoPortal }: { onGoPortal?: () => void }) {
  const [view, setView] = useState<View>({ name: "grid" });

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
