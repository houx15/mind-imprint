import type { ReactNode } from "react";
import "../learning/landing.css";
import "./teacher.css";

/**
 * Named measures for teacher pages. `default` and `wide` are the student
 * landings' measures (`learning-landing-measure`, `-wide`); `narrow` suits a
 * single form, `full` the report editor's two columns (draft and preview).
 */
export type TeacherPageWidth = "narrow" | "default" | "wide" | "full";

const WIDTH_CLASS: Record<TeacherPageWidth, string> = {
  narrow: " teacher-page--narrow",
  default: "",
  wide: " learning-landing-wide",
  full: " teacher-page--full",
};

/** The page wrapper every teacher page uses: the landing measure and padding, centred.
 *  `fill` (wide screens only): the page is exactly as tall as the scroll area
 *  and its content scrolls inside it — the AI workspaces (`WorkspacePanel`). */
export function TeacherPage({
  width = "default",
  fill = false,
  children,
}: {
  width?: TeacherPageWidth;
  fill?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={fill ? "min-h-full min-[900px]:h-full" : "min-h-full"}>
      <div className={`learning-landing-measure teacher-page mx-auto w-full${WIDTH_CLASS[width]}${fill ? " teacher-page--fill" : ""}`}>
        {children}
      </div>
    </div>
  );
}
