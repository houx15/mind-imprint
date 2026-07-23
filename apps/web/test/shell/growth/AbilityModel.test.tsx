import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AbilityModel } from "@/shell/growth/AbilityModel";
import { api } from "@/api";

afterEach(() => { vi.restoreAllMocks(); });

const model = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "背景与目标清楚", evidenceCount: 4 },
    { code: "D3", name: "证据与信源意识", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "论证结构意识", level: 3, levelLabel: "识别缺失 warrant", evidenceCount: 3 },
    { code: "D5", name: "反馈理解与修改理由", level: -1, levelLabel: "", evidenceCount: 0 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, anchoredSignals: 18, promptedSignals: 9 },
  metacognition: { highestSolo: "L4", distribution: { L1: 0, L2: 1, L3: 7, L4: 2 }, spontaneous: 3, prompted: 7 },
};

describe("AbilityModel", () => {
  it("renders the merged caption, a scored dim, and the low-N guard", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(screen.getByText(/任务理解与问题表述/)).toBeTruthy();
    expect(screen.getAllByText(/证据不足/).length).toBeGreaterThan(0); // D3 (1 session) + D5 (0)
    expect(screen.getByText(/对手邀请/)).toBeTruthy(); // autonomy panel
    expect(screen.getByText(/L4/)).toBeTruthy();       // metacognition highest
  });

  it("renders an empty state when there are no sessions", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue({ ...model, totalSessions: 0, depth: model.depth.map((d) => ({ ...d, level: -1, evidenceCount: 0 })) } as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
});
