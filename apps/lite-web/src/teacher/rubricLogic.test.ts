import { describe, expect, it } from "vitest";
import {
  buildRubric,
  rubricAfterLangChange,
  rubricDraftFromPayload,
  rubricDraftOf,
  sameRubricDraft,
  UNSET_RUBRIC_DRAFT,
  validateRubricDraft,
  type RubricDraft,
} from "./rubricLogic";

describe("rubric draft", () => {
  // Ruling: this frontend keeps no copy of liteassign.DefaultRubric's zh/en
  // dimension names — a real writing payload always carries its own
  // effective rubric, so `rubricDraftFromPayload` only ever falls back to
  // the generic `UNSET_RUBRIC_DRAFT` placeholder, never a guessed default.
  it("reads a stored rubric off a real payload, or the generic placeholder if it has none", () => {
    expect(rubricDraftFromPayload({ rubric: { scale: "points", max: 20, dimensions: [{ name: "论证", note: "" }], focus: "" } })).toEqual({
      scale: "points",
      max: "20",
      dimensions: [{ name: "论证", note: "" }],
      focus: "",
    });
    expect(rubricDraftFromPayload({})).toEqual(UNSET_RUBRIC_DRAFT);
  });
  it("validates with the server's messages", () => {
    const d = UNSET_RUBRIC_DRAFT;
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
  // Ruling: a language switch never rewrites the rubric text — this frontend
  // has no honest way to produce the other language's default wording
  // without hardcoding it, so the draft is left exactly as it is, whether
  // that's the untouched placeholder, a loaded rubric, or an edited one.
  it("never rewrites the rubric text on a language switch", () => {
    expect(rubricAfterLangChange(UNSET_RUBRIC_DRAFT, "zh", "en")).toBe(UNSET_RUBRIC_DRAFT);
    const loaded: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "看立意" }], focus: "" };
    expect(rubricAfterLangChange(loaded, "zh", "en")).toBe(loaded);
    expect(rubricAfterLangChange(loaded, "en", "en")).toBe(loaded);
  });
});

describe("sameRubricDraft", () => {
  it("compares structurally, not by reference", () => {
    const a: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "" }], focus: "" };
    const b: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "内容", note: "" }], focus: "" };
    expect(sameRubricDraft(a, b)).toBe(true);
    expect(sameRubricDraft(a, { ...a, focus: "重点看论证" })).toBe(false);
  });
});
