import type { TourJourney } from "./types";
import { coursesSegments } from "./segments/courses";
import { projectsSegments } from "./segments/projects";
import { settingsSegment } from "./segments/settings";

// P4: the welcome modal's 课程/项目 choice picks which group goes first; both
// orders end with the settings/accent finale segment.
export function journeyStarting(start: "courses" | "projects"): TourJourney {
  const groups = start === "projects"
    ? [...projectsSegments, ...coursesSegments]
    : [...coursesSegments, ...projectsSegments];
  return [...groups, settingsSegment];
}

export const coursesJourney: TourJourney = coursesSegments;
export const fullJourney: TourJourney = journeyStarting("courses"); // nav-footer restart default
