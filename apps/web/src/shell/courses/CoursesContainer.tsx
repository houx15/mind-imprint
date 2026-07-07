import { useState } from "react";
import { CoursesView } from "./CoursesView";
import { CoursePlayer } from "./CoursePlayer";

export function CoursesContainer() {
  const [activeCourseId, setActiveCourseId] = useState<string | null>(null);
  if (activeCourseId) {
    return <CoursePlayer courseId={activeCourseId} onExit={() => setActiveCourseId(null)} />;
  }
  return <CoursesView onOpenCourse={(id) => setActiveCourseId(id)} />;
}
