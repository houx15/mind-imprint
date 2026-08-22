import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// The report calls api.getCourseReport / getCardsCatalog / listCourses on mount; example mode
// must NOT. We assert the fixture renders and no fetch was attempted.
const getCourseReport = vi.fn();
vi.mock("@/api", () => ({
  api: {
    getCourseReport: (...a: unknown[]) => { getCourseReport(...a); return new Promise(() => {}); },
    getCardsCatalog: () => Promise.resolve([]),
    listCourses: () => Promise.resolve([]),
    getCourseAnswerReport: () => new Promise(() => {}),
  },
}));

import { CourseReport } from "@/shell/courses/CourseReport";
import { exampleCourseReport } from "@/tour/fixtures/exampleCourseReport";

describe("CourseReport example mode", () => {
  it("renders the fixture without fetching a real report", () => {
    render(
      <CourseReport
        courseId="__example__"
        exampleReport={exampleCourseReport}
        onBackToCourses={() => {}}
        onGoPortal={() => {}}
      />,
    );
    expect(screen.getByText(exampleCourseReport.title)).toBeInTheDocument();
    expect(getCourseReport).not.toHaveBeenCalled();
  });
});
