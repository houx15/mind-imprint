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
    markDemoNodeAdopted: vi.fn(),
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
      markDemoNodeAdopted: vi.fn(),
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
      markDemoNodeAdopted: vi.fn(),
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
      markDemoNodeAdopted: vi.fn(),
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
      markDemoNodeAdopted: vi.fn(),
    };
    seg.steps[0]!.onEnter?.(nav);
    expect(calls).toContainEqual(["setStudioRoom", "reading"]);
    expect(calls).toContainEqual(["setReadingView", "list"]);
  });

  // P8 Task 12 · reading-room is the REAL immersive 精读 walk: its first step
  // opens the demo reading room, a highlighted `<mark>` click reveals the real
  // rr-inline-card lens, and every step spotlights a real anchor with a
  // non-center placement.
  it("reading-room opens the demo reading room and spotlights the real rr-* anchors (+ inline-card click)", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-room")!;

    // step 0's onEnter opens the immersive 精读 room.
    const calls: string[] = [];
    seg.steps[0]!.onEnter?.(makeNav((n) => (n.openDemoReadingRoom = vi.fn(() => calls.push("openDemoReadingRoom")))));
    expect(calls).toContain("openDemoReadingRoom");

    // every step is a real, non-center spotlight of an rr-*/mark anchor, in
    // the click-highlight → inline-card → chat → deck → notes → finish order.
    expect(seg.steps.map((s) => s.anchor)).toEqual([
      '[data-tour="rr-article"]',
      '[data-tour="rr-article"] mark',
      '[data-tour="rr-inline-card"]',
      '[data-tour="rr-chat"]',
      '[data-tour="rr-deck"]',
      '[data-tour="rr-notes"]',
      '[data-tour="rr-finish"]',
    ]);
    for (const s of seg.steps) expect(s.placement).not.toBe("center");

    // the highlight click is the segment's one action step, gated on the same
    // selector as its anchor.
    const actionSteps = seg.steps.filter((s) => s.advance === "action");
    expect(actionSteps.map((s) => s.id)).toEqual(["reading-room-1"]);
    expect(actionSteps[0]!.actionEvent).toEqual({ selector: '[data-tour="rr-article"] mark', type: "click" });
  });

  // P8 Task 12, ruling 5 · 我的笔记 is reworded student-relatable copy with no
  // forward reference to evaluation.
  it("reading-room-5 (我的笔记) never mentions evaluation", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-room")!;
    const step = seg.steps.find((s) => s.id === "reading-room-5")!;
    expect(step.anchor).toBe('[data-tour="rr-notes"]');
    expect(step.text).not.toMatch(/评估/);
    expect(step.text).toContain("印记不替你写");
  });

  // P8 Task 12 · the exploration walk now runs the REAL search→adopt scene:
  // 检索卡 close → drill-into-question → 建议检索方向 → 搜索 → open a paper →
  // 采纳, in that order, each gated on its own real `[data-tour=…]` selector.
  it("reading-warren's action steps run the real 检索卡-close→drill→search→adopt scene in order", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const actions = seg.steps.filter((s) => s.advance === "action");
    expect(actions.map((s) => s.id)).toEqual([
      "reading-warren-4",
      "reading-warren-6",
      "reading-warren-7",
      "reading-warren-8",
      "reading-warren-9",
      "reading-warren-11",
    ]);
    expect(actions.map((s) => s.actionEvent?.selector)).toEqual([
      '[data-tour="search-card"] button',
      '[data-id="00000000-0000-0000-0000-000000000292"] [data-tour="warren-question"]',
      '[data-tour="explore-directions"]',
      '[data-tour="explore-search"]',
      '[data-tour="explore-result"]',
      '[data-tour="explore-adopt"]',
    ]);
    for (const s of actions) {
      expect(s.actionEvent?.type).toBe("click");
      expect(s.placement).not.toBe("center");
    }
  });

  // P8 Task 12 · the drill step is scoped to the ONE root the demo seeded a
  // real 2nd layer under (…0292) — not the generic `warren-question` selector
  // every root card carries — so the search/adopt scene downstream lands
  // somewhere real.
  it("reading-warren-6 (drill) is scoped to root …0292 via a data-id ancestor selector", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const step = seg.steps.find((s) => s.id === "reading-warren-6")!;
    expect(step.anchor).toBe('[data-id="00000000-0000-0000-0000-000000000292"] [data-tour="warren-question"]');
    expect(step.anchor).toContain('[data-tour="warren-question"]');
  });

  // P8 Task 12 · reading-warren-4 opens the real 检索卡 modal directly
  // (openSearchCard) and requires closing it (its one <button>) before the
  // tour continues — the modal stays mounted for many more real steps now, so
  // nothing else would tear it down. reading-warren-5 spotlights 还需要探索.
  it("reading-warren-4 opens 检索卡 via openSearchCard and gates on closing it; reading-warren-5 spotlights needs-resources", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;

    const cardStep = byId("reading-warren-4");
    expect(cardStep.anchor).toBe('[data-tour="search-card"]');
    expect(cardStep.placement).not.toBe("center");
    expect(cardStep.actionEvent).toEqual({ selector: '[data-tour="search-card"] button', type: "click" });
    const calls: string[] = [];
    cardStep.onEnter?.(makeNav((n) => (n.openSearchCard = vi.fn(() => calls.push("openSearchCard")))));
    expect(calls).toContain("openSearchCard");

    expect(byId("reading-warren-5").anchor).toBe('[data-tour="needs-resources"]');
  });

  // P8 Task 12, ruling 1 · adopt never calls markDemoNodeAdopted itself — the
  // real explore-adopt click (intercepted by ExplorationView's onDemoAdopt for
  // the write-blocked demo project) does that; the tour step only clicks.
  it("reading-warren-11 (采纳) has no onEnter and never calls markDemoNodeAdopted", () => {
    const seg = projectsSegments.find((s) => s.id === "reading-warren")!;
    const step = seg.steps.find((s) => s.id === "reading-warren-11")!;
    expect(step.onEnter).toBeUndefined();
    expect(step.anchor).toBe('[data-tour="explore-adopt"]');
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

  // P7 Task 10 · the rebuilt writing walk spotlights the real docswitch / 大纲 /
  // refpanel / 片段引导 / 批注 / review-trigger / needs-resources / 完成写作
  // anchors (铁律①② copy: AI never writes the body text).
  it("writing spotlights the deeper docswitch/outline/aicard/annotations/finish anchors", () => {
    const seg = projectsSegments.find((s) => s.id === "writing")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("writing-1").anchor).toBe('[data-tour="writing-docswitch"]');
    expect(byId("writing-2").anchor).toBe('[data-tour="writing-outline"]');
    expect(byId("writing-3").anchor).toBe('[data-tour="writing-refpanel"]');
    expect(byId("writing-4").anchor).toBe('[data-tour="writing-aicard"]');
    expect(byId("writing-5").anchor).toBe('[data-tour="writing-annotations"]');
    expect(byId("writing-6").anchor).toBe('[data-tour="writing-review-trigger"]');
    expect(byId("writing-7").anchor).toBe('[data-tour="writing-refpanel"]');
    expect(byId("writing-8").anchor).toBe('[data-tour="needs-resources"]');
    expect(byId("writing-9").anchor).toBe('[data-tour="writing-finish"]');
    // 铁律①: the 片段引导 step never claims the AI writes the essay.
    expect(byId("writing-4").text).toContain("正文");
    expect(byId("writing-4").text).toMatch(/自己写|不替|不.*代写|绝不替/);
    // real-scene: every anchored writing step is a non-center spotlight.
    for (const id of ["writing-1", "writing-2", "writing-3", "writing-4", "writing-5", "writing-6", "writing-7", "writing-8", "writing-9"]) {
      expect(byId(id).placement).not.toBe("center");
    }
  });

  // P7 Task 10 · the writing walk drives the writing room to the exact doc + tab
  // where each anchor renders (setWritingView deep-link) — the tour couldn't
  // reach `writing-aicard` (proposal 片段) or `writing-review-trigger`
  // (proposal 正文) without it.
  it("writing steps deep-link the right doc+tab via setWritingView", () => {
    const seg = projectsSegments.find((s) => s.id === "writing")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;

    const capture = (id: string) => {
      const calls: Array<[string, unknown]> = [];
      byId(id).onEnter?.(
        makeNav((n) => {
          n.setWritingView = vi.fn((v) => calls.push(["setWritingView", v]));
          n.selectRefPanelTab = vi.fn((t) => calls.push(["selectRefPanelTab", t]));
        }),
      );
      return calls;
    };

    expect(capture("writing-2")).toContainEqual(["setWritingView", { doc: "essay", tab: "outline" }]);
    expect(capture("writing-4")).toContainEqual(["setWritingView", { doc: "proposal", tab: "snippets" }]);
    expect(capture("writing-6")).toContainEqual(["setWritingView", { doc: "proposal", tab: "draft" }]);
    expect(capture("writing-7")).toContainEqual(["setWritingView", { doc: "essay", tab: "draft" }]);
    expect(capture("writing-9")).toContainEqual(["setWritingView", { doc: "essay", tab: "draft" }]);
  });

  // P7 Task 10 · 🚨 the 批注-tab constraint (T6): `selectRefPanelTab("anno")`
  // must fire while the writing room is NOT on the 正文/draft tab, otherwise the
  // showSnippets→snippets override clobbers it. writing-5 selects 批注 and must
  // NOT itself drive the room to the draft tab (it stays on the proposal 片段
  // tab left by writing-4).
  it("writing-5 selects the 批注 tab and does NOT switch to the 正文/draft tab", () => {
    const seg = projectsSegments.find((s) => s.id === "writing")!;
    const step = seg.steps.find((s) => s.id === "writing-5")!;
    const calls: Array<[string, unknown]> = [];
    step.onEnter?.(
      makeNav((n) => {
        n.selectRefPanelTab = vi.fn((t) => calls.push(["selectRefPanelTab", t]));
        n.setWritingView = vi.fn((v) => calls.push(["setWritingView", v]));
      }),
    );
    expect(calls).toContainEqual(["selectRefPanelTab", "anno"]);
    // never moves to the draft tab in the same step.
    expect(calls.some(([k, v]) => k === "setWritingView" && (v as { tab: string }).tab === "draft")).toBe(false);
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
    const sections = seg.steps.filter((s) => s.anchor?.startsWith("#s"));
    expect(sections.map((s) => s.anchor)).toEqual([
      "#s1", "#s2", "#s3", "#s4", "#s5", "#s6", "#s7", "#s8", "#s9",
    ]);
    for (const s of sections) expect(s.placement).not.toBe("center");
  });

  // P7 Task 10 · ㉕ after the whole-section #s5 (D) / #s6 (A) steps, the walk
  // spotlights 1–2 representative dimension cards per axis via the real
  // `axis-dim-{code}` anchors (T8) — the D dims sit right after #s5, the A dims
  // right after #s6, none centered.
  it("evaluation-report spotlights representative axis dimensions after #s5/#s6", () => {
    const seg = projectsSegments.find((s) => s.id === "evaluation-report")!;
    const byId = (id: string) => seg.steps.find((s) => s.id === id)!;
    expect(byId("eval-dim-D2").anchor).toBe('[data-tour="axis-dim-D2"]');
    expect(byId("eval-dim-D6").anchor).toBe('[data-tour="axis-dim-D6"]');
    expect(byId("eval-dim-A4").anchor).toBe('[data-tour="axis-dim-A4"]');
    expect(byId("eval-dim-A5").anchor).toBe('[data-tour="axis-dim-A5"]');
    for (const id of ["eval-dim-D2", "eval-dim-D6", "eval-dim-A4", "eval-dim-A5"]) {
      expect(byId(id).placement).not.toBe("center");
    }
    // D dims come after #s5 and before #s6; A dims after #s6.
    const idx = (id: string) => seg.steps.findIndex((s) => s.id === id);
    expect(idx("evaluation-report-5")).toBeLessThan(idx("eval-dim-D2"));
    expect(idx("eval-dim-D6")).toBeLessThan(idx("evaluation-report-6"));
    expect(idx("evaluation-report-6")).toBeLessThan(idx("eval-dim-A4"));
    expect(idx("eval-dim-A4")).toBeLessThan(idx("eval-dim-A5"));
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
