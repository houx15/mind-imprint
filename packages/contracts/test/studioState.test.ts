import { describe, it, expect } from "vitest";
import { StudioProjection, Station, CoachMessage } from "../src/studioState";

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
