import { describe, it, expect } from "vitest";
import { SelfScoreFx, PredictionFx, ReflectionFx, SelfScoreSubmitBody } from "../src/studioState";

describe("N2 contracts", () => {
  it("SelfScoreFx parses dims + bands", () => {
    const s = SelfScoreFx.parse({ dims: [{ code: "表D", name: "来源与证据", band: 2 }, { code: "表E", name: "分析", band: -1 }], bands: ["还需努力", "基本达到", "稳了"] });
    expect(s.dims[1]!.band).toBe(-1);
  });
  it("PredictionFx parses", () => {
    const p = PredictionFx.parse({ predicted: [{ code: "表D", name: "来源与证据" }], actual: [], overlap: 0, revealed: false });
    expect(p.revealed).toBe(false);
  });
  it("ReflectionFx parses", () => {
    expect(ReflectionFx.parse({ text: "回顾", prompts: ["为什么"] }).prompts.length).toBe(1);
  });
  it("SelfScoreSubmitBody validates band type", () => {
    expect(SelfScoreSubmitBody.parse({ scores: [{ code: "表D", band: 1 }] }).scores[0]!.band).toBe(1);
  });
});
