import { describe, it, expect, vi } from "vitest";
import { projectsSegments } from "@/tour/segments/projects";
import { coursesSegments } from "@/tour/segments/courses";
import { fullJourney } from "@/tour/journey";
import type { TourNavContext } from "@/tour/types";

/** A fully-stubbed TourNavContext; pass a mutator to override specific setters
 *  with instrumented spies (P7 Task 9). */
function makeNav(mutate?: (n: TourNavContext) => void): TourNavContext {
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
    setPlanView: vi.fn(),
    openSearchCard: vi.fn(),
    markDemoNodeRead: vi.fn(),
  };
  mutate?.(nav);
  return nav;
}

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
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
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
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
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
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
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
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
    };
    seg.steps[0]!.onEnter?.(nav);
    expect(calls).toContainEqual(["setStudioRoom", "reading"]);
    expect(calls).toContainEqual(["setReadingView", "list"]);
  });

  // P6 Task 7 · reading-room is now the REAL immersive 精读 walk: its first
  // step opens the demo reading room, and every step spotlights a real rr-*
  // anchor with a non-center placement.
  it("reading-room opens the demo reading room and spotlights the real rr-* anchors", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-room")!;

    // step 0's onEnter opens the immersive 精读 room.
    const calls: string[] = [];
    const nav: TourNavContext = {
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn(),
      openDemoReport: vi.fn(),
      setReadingView: vi.fn(),
      openDemoReadingRoom: vi.fn(() => calls.push("openDemoReadingRoom")),
      setWritingView: vi.fn(),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
    };
    seg.steps[0]!.onEnter?.(nav);
    expect(calls).toContain("openDemoReadingRoom");

    // every step is a real, non-center spotlight of an rr-* anchor.
    expect(seg.steps.map((s) => s.anchor)).toEqual([
      '[data-tour="rr-article"]',
      '[data-tour="rr-chat"]',
      '[data-tour="rr-deck"]',
      '[data-tour="rr-notes"]',
      '[data-tour="rr-finish"]',
    ]);
    for (const s of seg.steps) expect(s.placement).not.toBe("center");
  });

  // P7 Task 9 · the exploration walk's single reachable ACTION step is the
  // drill-into-a-question gesture (reading-warren-6): clicking a real
  // warren-question root card zooms into its Level-2 layer. Every other
  // exploration beat is a "next" step (spotlight or narrated) — an action gated
  // on the selection-only find controls would dead-end the tour.
  it("reading-warren has exactly one action step, the real warren-question drill-in", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const actions = seg.steps.filter((s) => s.advance === "action");
    expect(actions.length).toBe(1);
    const step = actions[0]!;
    expect(step.id).toBe("reading-warren-6");
    expect(step.actionEvent).toEqual({ selector: '[data-tour="warren-question"]', type: "click" });
    expect(step.anchor).toBe('[data-tour="warren-question"]');
    expect(step.placement).not.toBe("center");
  });

  // P7 Task 9 · the exploration walk spotlights the real search/needs controls
  // and opens the static 检索卡 modal (openSearchCard) — all reachable read-only.
  it("reading-warren spotlights search-card-trigger + needs-resources and opens 检索卡", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("reading-warren-4").anchor).toBe('[data-tour="search-card-trigger"]');
    expect(byId("reading-warren-5").anchor).toBe('[data-tour="needs-resources"]');

    const cardStep = byId("reading-warren-9");
    expect(cardStep.anchor).toBe('[data-tour="search-card"]');
    expect(cardStep.placement).not.toBe("center");
    const calls: string[] = [];
    cardStep.onEnter?.(makeNav((n) => (n.openSearchCard = vi.fn(() => calls.push("openSearchCard")))));
    expect(calls).toContain("openSearchCard");
  });

  // P7 Task 9 · 管理 walk drives the plan room to 甘特图 then 活动日志 via setPlanView.
  it("plan-manage lands on the gantt then the activity-log view via setPlanView", () => {
    const seg = projectsSegments.find((s) => s.id === "plan-manage")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("plan-manage-3").anchor).toBe('[data-tour="manage-activity-log"]');

    const gantt: unknown[] = [];
    byId("plan-manage-2").onEnter?.(makeNav((n) => (n.setPlanView = vi.fn((v) => gantt.push(v)))));
    expect(gantt).toContain("gantt");

    const log: unknown[] = [];
    byId("plan-manage-3").onEnter?.(makeNav((n) => (n.setPlanView = vi.fn((v) => log.push(v)))));
    expect(log).toContain("log");
  });

  // P7 Task 9 · after 精读, the walk returns to the graph and client-badges the
  // just-read node (markDemoNodeRead) so its 已读 badge is a real spotlight,
  // then points at the top view-toggle.
  it("reading-nodedone badges the read node then spotlights the view toggle", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-nodedone")!;
    const steps = seg.steps;
    expect(steps[0]!.anchor).toBe('[data-tour="warren-node-read"]');
    expect(steps[1]!.anchor).toBe('[data-tour="reading-viewtoggle"]');
    for (const s of steps) expect(s.placement).not.toBe("center");

    const calls: Array<[string, unknown]> = [];
    steps[0]!.onEnter?.(
      makeNav((n) => {
        n.setReadingView = vi.fn((v) => calls.push(["setReadingView", v]));
        n.markDemoNodeRead = vi.fn((id) => calls.push(["markDemoNodeRead", id]));
      }),
    );
    expect(calls).toContainEqual(["setReadingView", "graph"]);
    expect(calls).toContainEqual(["markDemoNodeRead", "00000000-0000-0000-0000-000000000290"]);
  });

  // P7 Task 9 · the 文献库 walk spotlights the real list controls (all render
  // read-only: add button, default-selected preview panel, enter-reading).
  it("reading-library spotlights add / preview / enter-reading, none centered", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-library")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("reading-library-2").anchor).toBe('[data-tour="library-add"]');
    expect(byId("reading-library-3").anchor).toBe('[data-tour="library-preview"]');
    expect(byId("reading-library-4").anchor).toBe('[data-tour="library-enter-reading"]');
    for (const id of ["reading-library-2", "reading-library-3", "reading-library-4"]) {
      expect(byId(id).placement).not.toBe("center");
    }
  });

  // P6 Task 7 · the deeper writing walk spotlights the real 片段引导 / 批注 /
  // 完成写作 anchors (铁律① copy: AI never writes the body text).
  it("writing spotlights the deeper aicard/annotations/finish anchors", () => {
    const seg = projectsSegments.find((s) => s.id === "writing")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("writing-2").anchor).toBe('[data-tour="writing-aicard"]');
    expect(byId("writing-4").anchor).toBe('[data-tour="writing-annotations"]');
    expect(byId("writing-5").anchor).toBe('[data-tour="writing-finish"]');
    // 铁律①: never claim the AI writes the essay.
    expect(byId("writing-2").text).toContain("正文");
    expect(byId("writing-2").text).toMatch(/自己写|不替|不.*代写|绝不替/);
    // real-scene: the anchored writing steps are non-center spotlights.
    for (const id of ["writing-2", "writing-4", "writing-5"]) expect(byId(id).placement).not.toBe("center");
  });

  // P6 Task 9 · the deeper writing walk drives the writing room to the exact
  // doc + tab where each anchor renders (setWritingView deep-link) — the tour
  // couldn't reach `writing-aicard` (proposal 片段) without it.
  it("writing-2 lands on the PROPOSAL 片段 tab, writing-5 on the 正文 doc, via setWritingView", () => {
    const seg = projectsSegments.find((s) => s.id === "writing")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;

    const calls: Array<[string, unknown]> = [];
    const makeNav = (): TourNavContext => ({
      setTab: vi.fn(),
      openCourse: vi.fn(),
      setCoursesSub: vi.fn(),
      openDemoProject: vi.fn(),
      setStudioRoom: vi.fn(),
      openDemoReport: vi.fn(),
      setReadingView: vi.fn(),
      openDemoReadingRoom: vi.fn(),
      setWritingView: vi.fn((v) => calls.push(["setWritingView", v])),
      selectRefPanelTab: vi.fn(),
      setPlanView: vi.fn(),
      openSearchCard: vi.fn(),
      markDemoNodeRead: vi.fn(),
    });

    byId("writing-2").onEnter?.(makeNav());
    byId("writing-5").onEnter?.(makeNav());
    expect(calls).toContainEqual(["setWritingView", { doc: "proposal", tab: "snippets" }]);
    expect(calls).toContainEqual(["setWritingView", { doc: "essay", tab: "draft" }]);
  });

  // P6 Task 7 · the reflection walk warns about the point-of-no-return lock.
  it("reflection spotlights review-finalize with the lock warning", () => {
    const seg = projectsSegments.find((s) => s.id === "reflection")!;
    const step = seg.steps.find((s) => s.id === "reflection-2")!;
    expect(step.anchor).toBe('[data-tour="review-finalize"]');
    expect(step.placement).not.toBe("center");
    expect(step.text).toMatch(/锁定|无法再修改/);
  });

  // P6 Task 7 · engine invariants across the whole projects journey:
  //   - a step never both defines an anchor AND uses placement:"center"
  //     (a centered step ignores its anchor — see TourRunner.tsx).
  //   - every anchored spotlight uses a non-center placement.
  //   - every action step carries an actionEvent with a real [data-tour=…]
  //     selector (so the document-capture click delegate can resolve it).
  it("no step both anchors and centers; action steps carry a real selector", () => {
    for (const seg of projectsSegments) {
      for (const s of seg.steps) {
        if (s.anchor) expect(s.placement).not.toBe("center");
        if (s.advance === "action") {
          expect(s.actionEvent).toBeTruthy();
          expect(s.actionEvent!.selector).toMatch(/\[data-tour="[^"]+"\]/);
        }
      }
    }
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
