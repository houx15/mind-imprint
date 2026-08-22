import type { TourJourney } from "./types";
import { coursesSegments } from "./segments/courses";

export const coursesJourney: TourJourney = coursesSegments;
// P1: the full journey is the courses group only. P3 concatenates the projects group,
// and the welcome modal's courses/projects choice reorders the two groups.
export const fullJourney: TourJourney = coursesSegments;
