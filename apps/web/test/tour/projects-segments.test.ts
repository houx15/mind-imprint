import { describe, it, expect, vi } from "vitest";
import { projectsSegments } from "@/tour/segments/projects";
import { coursesSegments } from "@/tour/segments/courses";
import { fullJourney } from "@/tour/journey";
import type { TourNavContext } from "@/tour/types";

describe("projects segments", () => {
  it("are well-formed: unique step ids, non-empty text, valid advance", () => {
    const ids = new Set<string>();
    for (const seg of projectsSegments) {
      expect(seg.steps.length).toBeGreaterThan(0);
      for (const s of seg.steps) {
        expect(s.text.length).toBeGreaterThan(0);
        expect(["next", "action"]).toContain(s.advance);
        if (s.advance === "action") expect(s.actionEvent).toBeTruthy();
        expect(ids.has(s.id)).toBe(false);
        ids.add(s.id);
      }
    }
  });

  it("the first segment's first step navigates (setTab and/or openDemoProject), standalone-safe", () => {
    const nav: TourNavContext = {
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn(),
      openDemoReport: vi.fn(),
    };
    projectsSegments[0]!.steps[0]!.onEnter?.(nav);
    expect(nav.setTab).toHaveBeenCalledWith("projects");
  });

  it("no step's onEnter calls both openDemoProject and setStudioRoom in the same tick", () => {
    for (const seg of projectsSegments) {
      for (const s of seg.steps) {
        if (!s.onEnter) continue;
        const calls: string[] = [];
        const nav: TourNavContext = {
          setTab: vi.fn(),
          openCourse: vi.fn(),
          setCoursesSub: vi.fn(),
          openDemoProject: vi.fn(() => calls.push("openDemoProject")),
          setStudioRoom: vi.fn(() => calls.push("setStudioRoom")),
          openDemoReport: vi.fn(),
        };
        s.onEnter(nav);
        expect(calls.includes("openDemoProject") && calls.includes("setStudioRoom")).toBe(false);
      }
    }
  });

  it("fullJourney includes both the courses group and the projects group", () => {
    const journeyIds = new Set(fullJourney.map((seg) => seg.id));
    for (const seg of coursesSegments) expect(journeyIds.has(seg.id)).toBe(true);
    for (const seg of projectsSegments) expect(journeyIds.has(seg.id)).toBe(true);
  });
});
