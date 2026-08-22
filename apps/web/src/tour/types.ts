export type TourPlacement = "top" | "bottom" | "left" | "right" | "center";
export type NavTabKey = "home" | "projects" | "courses" | "me";
export type CoursesSub = "courses" | "history" | "gallery";
/** The five studio rooms (mirrors `BlockKey` in workspace/blocks/mockData —
 *  kept as a separate tour-facing type so `tour/` doesn't import `workspace/`). */
export type StudioRoom = "forming" | "plan" | "reading" | "writing" | "reflection";
/** The shared, world-readable demo project the projects tour opens into the
 *  studio (read-only — the backend 403s all writes for it). */
export const DEMO_PROJECT_ID = "00000000-0000-0000-0000-000000000200";

/** Setters the tour uses to drive the app. Assembled in StudentApp (§Task 9).
 *  P1 only needs the courses-side setters; P3 extends this. */
export interface TourNavContext {
  setTab: (t: NavTabKey) => void;
  openCourse: (slug: string) => void;
  setCoursesSub: (s: CoursesSub) => void;
  /** P3: open the shared demo project into the studio. */
  openDemoProject: () => void;
  /** P3: switch the open project's studio to this room. */
  setStudioRoom: (room: StudioRoom) => void;
}

export interface TourStep {
  id: string;
  /** Drive app state before the step renders (navigate, open a course, switch sub-tab). */
  onEnter?: (ctx: TourNavContext) => void | Promise<void>;
  /** CSS selector, convention [data-tour="<id>"]. Absent → centered bubble. */
  anchor?: string;
  /** Cut a hole over the anchor (default true when anchor set). */
  spotlight?: boolean;
  /** Bubble placement relative to the anchor; "center" = modal-like, ignores anchor. */
  placement?: TourPlacement;
  title?: string;
  /** 印记's line, rendered as Markdown. */
  text: string;
  /** "next" = advance on the 下一步 button; "action" = advance when the user does the thing. */
  advance: "next" | "action";
  /** For advance:"action": which element + event advances the step. */
  actionEvent?: { selector: string; type: "click" | "input" };
}

export interface TourSegment {
  id: string;
  name: string;
  steps: TourStep[];
}

export type TourJourney = TourSegment[];

export interface TourController {
  running: boolean;
  segment: TourSegment | null;
  step: TourStep | null;
  segmentIndex: number;
  stepIndex: number;
  /** 0-1 progress within the whole active journey. */
  progress: number;
  play: (target: TourSegment | TourJourney) => void;
  next: () => void;
  prev: () => void;
  skipSegment: () => void;
  stop: () => void;
}
