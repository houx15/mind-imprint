import { describe, it, expect } from "vitest";
import { Course, CourseSummary, CourseProgress, CourseSession } from "../src/course";

const summary = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 4 };
const step = { id: "s1", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "建立横向溯源意识", assets: [{ id: "a0", kind: "text", title: "开场", value: "最近一张卫星图刷屏。" }], challenge_type: null, authored_content: { subtitle: "…", body: ["…"], foreground_asset_id: null } };

describe("Course contracts", () => {
  it("parses a course summary", () => { expect(CourseSummary.parse(summary).step_count).toBe(4); });
  it("parses a full course with steps", () => { expect(Course.parse({ ...summary, steps: [step] }).steps).toHaveLength(1); });
  it("rejects an unknown step kind", () => { expect(Course.parse.bind(null, { ...summary, steps: [{ ...step, kind: "quiz" }] })).toThrow(); });
  it("parses progress with completed ordinals", () => { expect(CourseProgress.parse({ course_id: "co1", current_ordinal: 2, completed_ordinals: [0, 1], updated_at: "1" }).completed_ordinals).toEqual([0, 1]); });

  it("parses a rendered step", async () => {
    const { RenderedStep } = await import("../src/course");
    expect(RenderedStep.parse({ ordinal: 0, kind: "teaching", template: "teaching", content: { subtitle: "x" }, source: "generated" }).source).toBe("generated");
  });

  // Whole-branch Critical-2: openCards carries a card offer the session has
  // not yet dispositioned, so a reload can restore it into the ask panel.
  it("parses a session with an open card offer", () => {
    const parsed = CourseSession.parse({
      id: "s1", courseId: "co1", phase: "guided", phaseTitle: "引导", status: "active",
      messages: [], openCards: [{ cardInstanceId: "ci1", cardId: "craap", materialId: "m1" }],
    });
    expect(parsed.openCards).toHaveLength(1);
    expect(parsed.openCards[0]!.cardId).toBe("craap");
  });

  it("rejects a session missing openCards", () => {
    expect(CourseSession.parse.bind(null, {
      id: "s1", courseId: "co1", phase: "demonstrate", phaseTitle: "演示", status: "active", messages: [],
    })).toThrow();
  });
});
