import { describe, it, expect } from "vitest";
import { StudioProjection, Station, CoachMessage, MaterialSource } from "../src/studioState";

describe("StudioProjection (Slice 5b wire DTO)", () => {
  it("accepts a full projection", () => {
    const ok = StudioProjection.safeParse({
      project: { title: "T", qualLabel: "0457 个人报告" },
      stations: [
        { code: "S0", name: "任务解码", view: "评估", state: "done" },
        { code: "S4", name: "论证构建", view: "结构", state: "current", gate: { total: 7, passed: 2 } },
        { code: "S5", name: "成稿打磨", view: "写作", state: "locked" },
      ],
      activeStation: "S4",
      coach: {
        anchor: "论证图 · 治理决心主张",
        messages: [
          { kind: "flag", label: "孤儿证据", body: "…" },
          { kind: "ai", body: "…", tag: "D5", anchor: "论证图 · 治理决心主张" },
        ],
        equipment: [{ id: "e1", name: "钢人卡", spont: "提示后", meth: "concession" }],
      },
      onboarding: { restatePrompt: "…", rubricRows: [{ official: "o", plain: "p", weak: true }], planSteps: ["立题"] },
      materials: [],
    });
    expect(ok.success).toBe(true);
  });

  it("carries the ai anchor field (5a carry-forward)", () => {
    expect(CoachMessage.safeParse({ kind: "ai", body: "b", tag: "D5", anchor: "论证图 · 治理决心主张" }).success).toBe(true);
  });

  it("rejects an unknown station code", () => {
    expect(Station.safeParse({ code: "S9", name: "x", view: "结构", state: "done" }).success).toBe(false);
  });
});

describe("MaterialSource", () => {
  const valid = {
    id: "00000000-0000-0000-0000-000000000110",
    title: "《卫星图看中国变绿》",
    sourceUrl: "",
    kind: "article",
    origin: "fetched",
    blocks: [{ id: "b1", text: "过去二十年里……" }],
    locked: false,
    role: "",
    tier: "二手 · 需追源",
    takeaway: "把 NASA 的图转述成「中国让地球更可持续」。",
    anchors: [],
    timeSpentS: 240,
  };

  it("accepts a projected source", () => {
    expect(MaterialSource.parse(valid)).toEqual(valid);
  });

  it("requires every derived field — no optionals to hide a missing producer", () => {
    const { locked, ...withoutLocked } = valid;
    expect(() => MaterialSource.parse(withoutLocked)).toThrow();
  });

  it("requires timeSpentS (the source-log entry's accumulated reading time)", () => {
    const { timeSpentS, ...withoutTimeSpentS } = valid;
    expect(() => MaterialSource.parse(withoutTimeSpentS)).toThrow();
  });

  it("carries materials on the projection", () => {
    const proj = StudioProjection.parse({
      project: { title: "t", qualLabel: "q" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [valid],
    });
    expect(proj.materials[0]!.title).toBe("《卫星图看中国变绿》");
  });
});
