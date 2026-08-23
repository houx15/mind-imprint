import { describe, it, expect, vi } from "vitest";
import { coursesSegments, coursesJourney } from "@/tour/segments/courses";
import type { TourNavContext } from "@/tour/types";

describe("courses segments", () => {
  it("are well-formed: unique step ids, non-empty text, valid advance", () => {
    const ids = new Set<string>();
    for (const seg of coursesSegments) {
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

  it("in-course mechanics are shown on real player elements, not centered text", () => {
    const player = coursesSegments.find((s) => s.id === "courses-player")!;
    expect(player).toBeTruthy();
    // Every player step must spotlight a REAL element (or open a demoModal) —
    // a `placement:"center"` step never spotlights (TourRunner short-circuits
    // anchor resolution on center), so an anchored step must be non-center.
    for (const s of player.steps) {
      const spotlights = Boolean(s.anchor) && s.placement !== "center";
      const modal = Boolean(s.demoModal);
      expect(spotlights || modal).toBe(true);
    }
    // courses-enter walks the real two-step entry: click a grid card → click
    // the detail page's start button (both are clickable "action" steps).
    const enter = coursesSegments.find((s) => s.id === "courses-enter")!;
    expect(enter.steps.every((s) => s.advance === "action")).toBe(true);
    expect(enter.steps.some((s) => s.anchor === '[data-tour="course-detail-start"]')).toBe(true);
  });

  it("the first segment's first step navigates to the courses tab (standalone-safe)", () => {
    const nav: TourNavContext = {
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn(),
      openDemoReport: vi.fn(),
      setReadingView: vi.fn(),
      openDemoReadingRoom: vi.fn(),
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
    };
    coursesJourney[0]!.steps[0]!.onEnter?.(nav);
    expect(nav.setTab).toHaveBeenCalledWith("courses");
  });
});
