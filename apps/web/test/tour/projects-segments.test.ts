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
      setReadingView: vi.fn(),
      openDemoReadingRoom: vi.fn(),
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
      setReadingView: vi.fn(),
      openDemoReadingRoom: vi.fn(),
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

  // P5 Task 5 · reading segments carry the real demo anchors (warren map +
  // 文献库 table), not centered-only text.
  it("reading-warren spotlights the real warren-map anchors in order", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    expect(seg.steps[1]!.anchor).toBe('[data-tour="reading-viewtoggle"]');
    expect(seg.steps[2]!.anchor).toBe('[data-tour="warren-question"]');
    expect(seg.steps[3]!.anchor).toBe('[data-tour="warren-unfiled"]');
  });

  it("reading-warren step 0 onEnter drives to the reading room's graph view", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const calls: Array<[string, unknown]> = [];
    const nav: TourNavContext = {
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn((room) => calls.push(["setStudioRoom", room])),
      openDemoReport: vi.fn(),
      setReadingView: vi.fn((view) => calls.push(["setReadingView", view])),
      openDemoReadingRoom: vi.fn(),
    };
    seg.steps[0]!.onEnter?.(nav);
    expect(calls).toContainEqual(["setStudioRoom", "reading"]);
    expect(calls).toContainEqual(["setReadingView", "graph"]);
  });

  it("reading-library spotlights the real library-table anchor and switches to list view", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-library")!;
    expect(seg.steps[1]!.anchor).toBe('[data-tour="library-table"]');

    const calls: Array<[string, unknown]> = [];
    const nav: TourNavContext = {
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn((room) => calls.push(["setStudioRoom", room])),
      openDemoReport: vi.fn(),
      setReadingView: vi.fn((view) => calls.push(["setReadingView", view])),
      openDemoReadingRoom: vi.fn(),
    };
    seg.steps[0]!.onEnter?.(nav);
    expect(calls).toContainEqual(["setStudioRoom", "reading"]);
    expect(calls).toContainEqual(["setReadingView", "list"]);
  });

  it("reading-room (精读 fallback) stays to at most 2 centered steps", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-room")!;
    expect(seg.steps.length).toBeLessThanOrEqual(2);
    for (const s of seg.steps) expect(s.placement).toBe("center");
  });

  // P5 Task 6 · the evaluation-report segment spotlights all 9 real report
  // sections in order, none of them centered (a `center` step never
  // resolves its anchor — see TourRunner.tsx).
  it("evaluation-report anchors #s1..#s9 in order, none centered", () => {
    const seg = projectsSegments.find((s) => s.id === "evaluation-report")!;
    const anchored = seg.steps.filter((s) => s.anchor);
    expect(anchored.map((s) => s.anchor)).toEqual([
      "#s1", "#s2", "#s3", "#s4", "#s5", "#s6", "#s7", "#s8", "#s9",
    ]);
    for (const s of anchored) expect(s.placement).not.toBe("center");
  });

  it("evaluation-report intro step has no anchor and stays centered", () => {
    const seg = projectsSegments.find((s) => s.id === "evaluation-report")!;
    const intro = seg.steps[0]!;
    expect(intro.anchor).toBeUndefined();
    expect(intro.placement).toBe("center");
  });

  // P5 Task 6 · 立题's 提问卡 step now shows the real static mock instead of
  // a plain centered bubble; the mock owns its own header, so no Modal title.
  it("forming-2 uses the question-card demoModal with no Modal title", () => {
    const seg = projectsSegments.find((s) => s.id === "forming")!;
    const step = seg.steps.find((s) => s.id === "forming-2")!;
    expect(step.demoModal?.kind).toBe("question-card");
    expect(step.demoModal?.title).toBeUndefined();
  });
});
