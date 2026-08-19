import { describe, it, expect } from "vitest";
import { RenderCache, Interaction, CoursePlayerPayload, CourseSummary, CourseCategory, COURSE_CATEGORIES, CourseIntroduction } from "../src/course";

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
  it("summary shape", () => {
    CourseSummary.parse({ slug: "a-mid", branch: "A", title: "t", blurb: "b", time_label: "20 分钟", card_ids: ["craap"], step_count: 4 });
  });

  const basePayload = {
    slug: "a-mid", title: "t", branch: "A", cardIds: ["craap"],
    structure: { id: "a-mid", title: "t", steps: [] },
    renderCache: { courseId: "a-mid", steps: [] },
  };

  it("accepts audioKeys as a pieceId→objectKey record", () => {
    const p = CoursePlayerPayload.parse({
      ...basePayload,
      audioKeys: { "s0#0": "courses/audio/a-mid/s0_0_ab12cd34.mp3" },
    });
    expect(p.audioKeys).toEqual({ "s0#0": "courses/audio/a-mid/s0_0_ab12cd34.mp3" });
  });

  it("defaults audioKeys to {} when omitted", () => {
    const p = CoursePlayerPayload.parse(basePayload);
    expect(p.audioKeys).toEqual({});
  });
});

describe("course catalog metadata", () => {
  it("has exactly the 7 no-emoji categories in declared order", () => {
    expect(COURSE_CATEGORIES.map((c) => c.slug)).toEqual([
      "stance-value", "source-check", "media-literacy",
      "self-knowledge", "data-literacy", "research-process", "argument-writing",
    ]);
    for (const c of COURSE_CATEGORIES) {
      expect(c.label).not.toMatch(/\p{Emoji_Presentation}/u);
    }
  });

  it("rejects a category outside the 7", () => {
    expect(CourseCategory.safeParse("misc").success).toBe(false);
    expect(CourseCategory.safeParse("source-check").success).toBe(true);
  });

  it("defaults the three new summary fields to null when absent", () => {
    const s = CourseSummary.parse({
      slug: "x", branch: "A", title: "T", blurb: "b", time_label: "10m",
      card_ids: [], step_count: 3,
    });
    expect(s.category).toBeNull();
    expect(s.introduction).toBeNull();
    expect(s.featuredRank).toBeNull();
  });

  it("parses a full introduction with alignment + takeaways", () => {
    const intro = CourseIntroduction.parse({
      hook: "h", whatYouDo: "w", takeaways: ["t1"],
      alignment: { ib: ["TOK"], otherIntl: ["AP"], domestic: ["语文"] },
      keywords: ["k"],
    });
    expect(intro.takeaways).toEqual(["t1"]);
    expect(intro.alignment.ib).toEqual(["TOK"]);
  });
});
