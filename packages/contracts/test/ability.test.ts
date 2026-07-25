import { describe, it, expect } from "vitest";
import { AbilityModel } from "../src/ability";

const sample = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "给出任务背景与目标，但约束或验收标准仍模糊", evidenceCount: 4 },
    { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "论证结构意识", level: 3, levelLabel: "识别缺失 warrant", evidenceCount: 3 },
    { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, opportunitiesTaken: 18, opportunitiesMissed: 9 },
};

describe("AbilityModel", () => {
  it("parses a valid ability model", () => {
    const m = AbilityModel.parse(sample);
    expect(m.depth.length).toBe(4);
    expect(m.depth[1]!.level).toBe(-1); // insufficient
    expect(m.autonomy.opportunitiesTaken).toBe(18);
  });
  it("rejects a non-number level", () => {
    expect(() => AbilityModel.parse({ ...sample, depth: [{ ...sample.depth[0], level: "two" }] })).toThrow();
  });
});
