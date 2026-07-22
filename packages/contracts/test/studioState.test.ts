import { describe, it, expect, test } from "vitest";
import { StudioProjection, Station, CoachMessage, MaterialSource, StructureCard, WritingProjection, WritingReviewItem, Gauge } from "../src/studioState";
import { FramingFx, PerspectivesFx, FramingSubmitBody, PerspectivesSubmitBody } from "../src/studioState";
import { SpotCheckItem, SpotCheckFx } from "../src/studioState";

const emptyWriting = {
  buffer: "",
  latestSnapshot: null,
  wordBudget: { min: 1500, max: 2000 },
  citationsMatched: false,
  review: { items: [] },
};

const emptySpotChecks = {
  evaluateSources: { items: [], orderable: false },
  buildArgument: { items: [], orderable: false },
};

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
      onboarding: { restatePrompt: "…", rubricRows: [{ official: "o", plain: "p", weak: true }], planSteps: ["立题"], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [],
      activeCard: null,
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
    });
    expect(ok.success).toBe(true);
  });

  it("accepts an open activeCard", () => {
    const ok = StudioProjection.safeParse({
      project: { title: "T", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [],
      activeCard: { cardInstanceId: "ci1", cardId: "sift", status: "active", anchors: [], materialId: "m1" },
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
    });
    expect(ok.success).toBe(true);
  });

  it("requires activeCard (no optional to hide a reload-bricks-the-workspace bug)", () => {
    const { activeCard, ...rest } = {
      project: { title: "T", qualLabel: "q" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [],
      activeCard: null,
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
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
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [valid],
      activeCard: null,
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
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
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [],
      activeCard: null,
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
    };
    expect(() => StudioProjection.parse(base)).toThrow(); // missing structure
    expect(StudioProjection.parse({ ...base, structure: [] }).structure).toEqual([]);
  });
});

describe("WritingProjection", () => {
  test("WritingProjection parses a full writing view", () => {
    const ok = {
      buffer: "我在写",
      latestSnapshot: { id: "s1", seq: 3, committedAt: "2026-07-15T00:00:00Z", wordCount: 1723, inBand: true, budget: { state: "in", delta: 0 } },
      wordBudget: { min: 1500, max: 2000 },
      citationsMatched: false,
      review: { items: [
        { interventionId: "i1", criterion: "表E 分析", band: "5–6 段", evidence: "e", missing: "m", fix: "f",
          voice: "board", disposition: { action: "rewrite", reason: "我打算把跳步补成一段推理，至少十五个字。" } },
      ] },
    };
    expect(() => WritingProjection.parse(ok)).not.toThrow();
  });

  test("WritingReviewItem rejects a bad disposition action", () => {
    const bad = { buffer: "", latestSnapshot: null, wordBudget: { min: 1, max: 2 }, citationsMatched: false,
      review: { items: [
        { interventionId: "i", criterion: "c", band: "b", evidence: "", missing: "", fix: "", voice: "board",
          disposition: { action: "keep", reason: "x" } }] } };
    expect(() => WritingProjection.parse(bad)).toThrow();
  });

  test("StudioProjection requires writing", () => {
    const p: any = {
      project: { title: "T", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S4",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      materials: [],
      activeCard: null,
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      finished: false,
      canFinish: false,
    };
    delete p.writing;
    expect(() => StudioProjection.parse(p)).toThrow();
  });

  test("WritingReviewItem carries a voice enum", () => {
    const ok = WritingReviewItem.safeParse({
      interventionId: "i1", criterion: "表E 分析", band: "5–6 段",
      evidence: "e", missing: "m", fix: "", voice: "sceptic", disposition: null,
    });
    expect(ok.success).toBe(true);
    const bad = WritingReviewItem.safeParse({
      interventionId: "i1", criterion: "表E", band: "5–6 段",
      evidence: "e", missing: "m", fix: "", voice: "nope", disposition: null,
    });
    expect(bad.success).toBe(false);
  });

  test("WritingProjection review is a flat items array with a snapshot budget", () => {
    const parsed = WritingProjection.safeParse({
      buffer: "",
      latestSnapshot: { id: "s1", seq: 3, committedAt: "2026-07-15T00:00:00Z", wordCount: 2340, inBand: false, budget: { state: "over", delta: 340 } },
      wordBudget: { min: 1500, max: 2000 },
      citationsMatched: false,
      review: { items: [] },
    });
    expect(parsed.success).toBe(true);
  });
});

describe("Gauge (readiness table)", () => {
  test("Gauge parses a readiness table row", () => {
    const g = Gauge.parse({ code: "表D", name: "来源与证据", lit: 3, total: 4, note: "孤儿证据", level: "partial" });
    expect(g.total).toBe(4);
  });

  test("Gauge rejects an unknown level", () => {
    expect(() => Gauge.parse({ code: "表D", name: "x", lit: 0, total: 4, note: "", level: "sorta" })).toThrow();
  });

  test("StudioProjection carries a readiness array", () => {
    const keys = Object.keys((StudioProjection as any).shape);
    expect(keys).toContain("readiness");
  });
});

describe("N3d station view shapes", () => {
  it("parses a framing projection", () => {
    const fx = FramingFx.parse({
      researchQuestion: "中国在多大程度上让世界更可持续？",
      terms: [{ term: "sustainable", definition: "资源使用不损害后代的能力" }],
      answers: ["趋势变好不等于问题已解决"],
      searchPlan: ["官方一手数据（NASA / IEA / BP）"],
    });
    expect(fx.terms[0]!.term).toBe("sustainable");
  });

  it("carries card-minted perspectives as non-editable rows with a blank level", () => {
    const fx = PerspectivesFx.parse({
      rows: [
        { text: "中国政府视角", level: "national", editable: true },
        { text: "来自矩阵卡的一行", level: "", editable: false },
      ],
      sourcesPerPerspective: false,
    });
    expect(fx.rows[1]!.editable).toBe(false);
    expect(fx.rows[1]!.level).toBe("");
  });

  it("rejects an unknown perspective level in a submit body", () => {
    expect(() =>
      PerspectivesSubmitBody.parse({ perspectives: [{ text: "x", level: "cosmic" }] }),
    ).toThrow();
  });

  it("accepts a framing submit body", () => {
    const b = FramingSubmitBody.parse({ terms: [], answers: [], searchPlan: [] });
    expect(b.answers).toEqual([]);
  });
});

describe("N3f SpotCheckItem/SpotCheckFx", () => {
  it("parses a spot-check item with no disposition yet", () => {
    const it_ = SpotCheckItem.parse({
      interventionId: "iv1",
      targetId: "t1",
      targetName: "《卫星图看中国变绿》",
      evidence: "写了它证明了中国碳排放下降",
      missing: "没写它不能回答什么",
      fix: "补一句这条来源的局限",
      disposition: null,
    });
    expect(it_.disposition).toBeNull();
  });

  it("parses a spot-check item with a recorded disposition (reuses Disposition, not a parallel type)", () => {
    const parsed = SpotCheckItem.safeParse({
      interventionId: "iv1",
      targetId: "t1",
      targetName: "n",
      evidence: "e",
      missing: "m",
      fix: "f",
      disposition: { action: "accept", reason: "我已经补充了这条来源的局限，至少十五个字。" },
    });
    expect(parsed.success).toBe(true);
  });

  it("rejects a bad disposition action on a spot-check item", () => {
    const parsed = SpotCheckItem.safeParse({
      interventionId: "iv1", targetId: "t1", targetName: "n", evidence: "e", missing: "m", fix: "f",
      disposition: { action: "keep", reason: "x" },
    });
    expect(parsed.success).toBe(false);
  });

  it("parses SpotCheckFx", () => {
    const fx = SpotCheckFx.parse({
      items: [{ interventionId: "iv1", targetId: "t1", targetName: "n", evidence: "e", missing: "m", fix: "f", disposition: null }],
      orderable: false,
    });
    expect(fx.orderable).toBe(false);
    expect(fx.items).toHaveLength(1);
  });

  it("StudioProjection requires spotChecks — a projection lacking it fails to parse", () => {
    const full: any = {
      project: { title: "T", qualLabel: "0457 个人报告" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [], assignmentText: "", studentRestate: "", studentWeakPicks: [] },
      framing: { researchQuestion: "", terms: [], answers: [], searchPlan: [] },
      perspectives: { rows: [], sourcesPerPerspective: false },
      materials: [],
      activeCard: null,
      structure: [],
      writing: emptyWriting,
      readiness: [],
      selfScore: { dims: [], bands: ["还需努力", "基本达到", "稳了"] },
      prediction: { predicted: [], actual: [], overlap: 0, revealed: false },
      reflection: { text: "", prompts: [] },
      spotChecks: emptySpotChecks,
      finished: false,
      canFinish: false,
    };
    // Well-formed (spotChecks present) parses fine.
    expect(StudioProjection.safeParse(full).success).toBe(true);
    // Lacking spotChecks entirely fails.
    const { spotChecks, ...withoutSpotChecks } = full;
    expect(StudioProjection.safeParse(withoutSpotChecks).success).toBe(false);
  });
});
