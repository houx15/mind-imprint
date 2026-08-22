import type { TourJourney } from "./types";
import { coursesSegments } from "./segments/courses";
import { projectsSegments } from "./segments/projects";

export const coursesJourney: TourJourney = coursesSegments;
// P3: the full journey is courses followed by projects. The welcome modal's
// courses/projects choice reorders the two groups.
export const fullJourney: TourJourney = [...coursesSegments, ...projectsSegments];
