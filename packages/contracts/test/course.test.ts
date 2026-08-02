import { describe, it, expect } from "vitest";
import { RenderCache, Interaction, CoursePlayerPayload, CourseSummary, CourseReport } from "../src/course";

describe("course v2 contracts", () => {
  it("parses an ordering interaction", () => {
    const i = Interaction.parse({
      id: "q1", type: "ordering", prompt: "排序",
      options: [{ id: "A", text: "一" }, { id: "B", text: "二" }],
      correct_answer: ["A", "B"], explanation: "因为", remediation_questions: [],
    });
    expect(i.type).toBe("ordering");
  });
  it("parses a render cache with teaching + structure segments", () => {
    const rc = RenderCache.parse({
      version: "v4", courseId: "a-mid", courseTitle: "T",
      steps: [{ stepId: "step_01", content: {
        title: "t", subtitle: "s",
        segments: [{ kind: "teaching", flow_block_id: "b1", text: "hi", asset_ids: ["m1"], items: [] }],
        interactions: [], board: [],
      } }],
    });
    expect(rc.steps[0]!.content.segments[0]!.kind).toBe("teaching");
  });
  it("rejects an unknown interaction type", () => {
    expect(() => Interaction.parse({ id: "x", type: "essay", prompt: "", options: [], correct_answer: [], explanation: "", remediation_questions: [] })).toThrow();
  });
  it("summary + payload + report shapes", () => {
    CourseSummary.parse({ slug: "a-mid", branch: "A", title: "t", blurb: "b", time_label: "20 分钟", card_ids: ["craap"], step_count: 4 });
    CourseReport.parse({ title: "t", goal: "g", teaching_thread: "th", completedStepTitles: ["s1"], cardIds: ["craap"], secondsSpent: 600, quiz: { total: 4, correct: 3 } });
  });
});
