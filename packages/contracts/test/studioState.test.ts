import { describe, it, expect } from "vitest";
import { StudioProjection, Station, CoachMessage, MaterialSource, StructureCard } from "../src/studioState";

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
        equipment: [{ id: "e1", name: "钢人卡", spont: "提示后", meth: "concession", materialId: "" }],
      },
      onboarding: { restatePrompt: "…", rubricRows: [{ official: "o", plain: "p", weak: true }], planSteps: ["立题"] },
      materials: [],
      activeCard: null,
      structure: [],
    });
    expect(ok.success).toBe(true);
  });

  it("accepts an open activeCard", () => {
    const ok = StudioProjection.safeParse({
      project: { title: "T", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [],
      activeCard: { cardInstanceId: "ci1", cardId: "sift", status: "active", anchors: [], materialId: "m1" },
      structure: [],
    });
    expect(ok.success).toBe(true);
  });

  it("requires activeCard (no optional to hide a reload-bricks-the-workspace bug)", () => {
    const { activeCard, ...rest } = {
      project: { title: "T", qualLabel: "q" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [],
      activeCard: null,
      structure: [],
    };
    expect(StudioProjection.safeParse(rest).success).toBe(false);
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
    lateralRead: false,
    isLateralInstrument: false,
    siftSkipped: false,
    lateralRelation: "",
    lateralJudgment: "",
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

  it("requires lateralRead (Slice 6c: mirrors source_log_entry.lateral_read, no optional to hide a missing producer)", () => {
    const { lateralRead, ...withoutLateralRead } = valid;
    expect(() => MaterialSource.parse(withoutLateralRead)).toThrow();
  });

  it("requires isLateralInstrument (fix-wave finding [4]: the chip's summon-parity fact, no optional to hide a missing producer)", () => {
    const { isLateralInstrument, ...withoutIsLateralInstrument } = valid;
    expect(() => MaterialSource.parse(withoutIsLateralInstrument)).toThrow();
  });

  it("requires siftSkipped (FIX 3: the summon-parity fact behind the 需横向阅读 chip's honesty, no optional to hide a missing producer)", () => {
    const { siftSkipped, ...withoutSiftSkipped } = valid;
    expect(() => MaterialSource.parse(withoutSiftSkipped)).toThrow();
  });

  it("requires lateralRelation/lateralJudgment (the cross_check mint's own words, no optional to hide a missing producer)", () => {
    const { lateralRelation, ...withoutLateralRelation } = valid;
    expect(() => MaterialSource.parse(withoutLateralRelation)).toThrow();
    const { lateralJudgment, ...withoutLateralJudgment } = valid;
    expect(() => MaterialSource.parse(withoutLateralJudgment)).toThrow();
  });

  it("carries materials on the projection", () => {
    const proj = StudioProjection.parse({
      project: { title: "t", qualLabel: "q" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [valid],
      activeCard: null,
      structure: [],
    });
    expect(proj.materials[0]!.title).toBe("《卫星图看中国变绿》");
  });
});

describe("StructureCard", () => {
  it("parses a valid done card", () => {
    const card = { id: "claim", role: "核心主张", status: "done", preview: "主张句" };
    expect(StructureCard.parse(card)).toEqual(card);
  });
  it("rejects an unknown status", () => {
    expect(() => StructureCard.parse({ id: "claim", role: "核心主张", status: "active", preview: "" })).toThrow();
  });
});

describe("StudioProjection", () => {
  it("requires a structure array", () => {
    const base = {
      project: { title: "t", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S4",
      coach: { anchor: "a", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [],
      activeCard: null,
    };
    expect(() => StudioProjection.parse(base)).toThrow(); // missing structure
    expect(StudioProjection.parse({ ...base, structure: [] }).structure).toEqual([]);
  });
});
