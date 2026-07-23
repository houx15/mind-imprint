import { describe, it, expect, vi, afterEach } from "vitest";
import { getAbilityModel } from "@/api/ability";

afterEach(() => { vi.restoreAllMocks(); });

describe("getAbilityModel", () => {
  it("parses the ability model", async () => {
    const body = {
      totalSessions: 2,
      depth: [
        { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "x", evidenceCount: 2 },
        { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
        { code: "D4", name: "论证结构意识", level: -1, levelLabel: "", evidenceCount: 0 },
        { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
      ],
      autonomy: { sessions: 2, boundarySettings: 3, adversaryInvites: 0, anchoredSignals: 3, promptedSignals: 1 },
      metacognition: { highestSolo: "L3", distribution: { L1: 0, L2: 0, L3: 2, L4: 0 }, spontaneous: 1, prompted: 1 },
    };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const m = await getAbilityModel();
    expect(m.depth.length).toBe(4);
    expect(m.totalSessions).toBe(2);
  });
});
