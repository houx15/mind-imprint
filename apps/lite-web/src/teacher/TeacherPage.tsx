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

/** The page wrapper every teacher page uses: the landing measure and padding, centred. */
export function TeacherPage({ width = "default", children }: { width?: TeacherPageWidth; children: ReactNode }) {
  return (
    <div className="min-h-full">
      <div className={`learning-landing-measure teacher-page mx-auto w-full${WIDTH_CLASS[width]}`}>{children}</div>
    </div>
  );
}
