import { describe, expect, it } from "vitest";
import {
  buildRubric,
  rubricDefaultNote,
  rubricDraftFromPayload,
  rubricDraftOf,
  sameRubricDraft,
  validateRubricDraft,
  type RubricDraft,
} from "./rubricLogic";

describe("rubric draft", () => {
  // Ruling: this frontend keeps no copy of liteassign.DefaultRubric's zh/en
  // dimension names — a real writing payload always carries its own
  // effective rubric, so `rubricDraftFromPayload` only ever falls back to
  // `null`, never a guessed default.
  it("reads a stored rubric off a real payload, or null if it has none", () => {
    expect(rubricDraftFromPayload({ rubric: { scale: "points", max: 20, dimensions: [{ name: "论证", note: "" }], focus: "" } })).toEqual({
      scale: "points",
      max: "20",
      dimensions: [{ name: "论证", note: "" }],
      focus: "",
    });
    expect(rubricDraftFromPayload({})).toBeNull();
  });
  it("validates with the server's messages", () => {
    const d: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "总评", note: "" }], focus: "" };
    expect(validateRubricDraft(d)).toBeNull();
    expect(validateRubricDraft({ ...d, scale: "points", max: "0" })).toBe("满分需在 1 到 100 之间");
    expect(validateRubricDraft({ ...d, dimensions: [] })).toBe("评分维度需有 1 到 6 项");
    expect(validateRubricDraft({ ...d, dimensions: [{ name: " ", note: "" }] })).toBe("维度名称不能为空，不超过 40 字");
    expect(validateRubricDraft({ ...d, dimensions: [{ name: "内容", note: "" }, { name: "内容 ", note: "" }] })).toBe("维度名称不能重复");
    expect(validateRubricDraft({ ...d, focus: "字".repeat(501) })).toBe("批改重点不超过 500 字");
  });
  it("builds a trimmed rubric; letter scale has no max", () => {
    expect(buildRubric({ scale: "letter", max: "20", dimensions: [{ name: " 内容 ", note: " 看立意 " }], focus: " 重点 " })).toEqual({
      scale: "letter",
      dimensions: [{ name: "内容", note: "看立意" }],
      focus: "重点",
    });
    expect(buildRubric({ scale: "points", max: "20", dimensions: [{ name: "论证", note: "" }], focus: "" }).max).toBe(20);
  });
  it("round-trips through rubricDraftOf/buildRubric", () => {
    const r = { scale: "letter" as const, dimensions: [{ name: "内容", note: "看立意" }], focus: "重点看论证" };
    expect(buildRubric(rubricDraftOf(r))).toEqual(r);
  });
});

describe("rubricDefaultNote", () => {
  // Ruling: no rubric editor on a new homework — this note names which
  // language's default the server will apply, not the default's own text
  // (which this frontend keeps no copy of).
  it("names the language the default will use", () => {
    expect(rubricDefaultNote("zh")).toContain("中文");
    expect(rubricDefaultNote("en")).toContain("英文");
    expect(rubricDefaultNote("zh")).not.toBe(rubricDefaultNote("en"));
  });
});

describe("sameRubricDraft", () => {
  it("compares structurally, not by reference", () => {
    const a: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "" }], focus: "" };
    const b: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "" }], focus: "" };
    expect(sameRubricDraft(a, b)).toBe(true);
    expect(sameRubricDraft(a, { ...a, focus: "重点看论证" })).toBe(false);
  });
  // Fix round 1: JSON.stringify comparison broke on this — a dimension
  // object built with its fields in a different order stringifies
  // differently even though it's the same data.
  it("treats the same dimension content as equal regardless of key order", () => {
    const a: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "看立意" }], focus: "" };
    const bDimension = { note: "看立意", name: "内容" };
    const b: RubricDraft = { scale: "letter", max: "", dimensions: [bDimension], focus: "" };
    expect(sameRubricDraft(a, b)).toBe(true);
  });
  it("ignores a stale max left over from a different scale", () => {
    const a: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "" }], focus: "" };
    const b: RubricDraft = { scale: "letter", max: "20", dimensions: [{ name: "内容", note: "" }], focus: "" };
    expect(sameRubricDraft(a, b)).toBe(true);
  });
});
