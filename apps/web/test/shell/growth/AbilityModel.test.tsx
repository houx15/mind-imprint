import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AbilityModel } from "@/shell/growth/AbilityModel";
import { api } from "@/api";

afterEach(() => { vi.restoreAllMocks(); });

// Six depth dims (D1–D6) so the radar exercises all six spokes; two low-N (-1).
const model = {
  totalSessions: 6,
  depth: [
    { code: "D1", name: "任务理解与问题表述", level: 2, levelLabel: "背景与目标清楚", evidenceCount: 4 },
    { code: "D2", name: "证据与信源", level: 3, levelLabel: "比较了信源立场", evidenceCount: 3 },
    { code: "D3", name: "论证结构", level: -1, levelLabel: "", evidenceCount: 1 },
    { code: "D4", name: "视角与偏见", level: 2, levelLabel: "承认样本局限", evidenceCount: 2 },
    { code: "D5", name: "反馈处理与修订", level: -1, levelLabel: "", evidenceCount: 0 },
    { code: "D6", name: "反思与元认知", level: 3, levelLabel: "能复盘策略", evidenceCount: 2 },
  ],
  autonomy: { sessions: 6, boundarySettings: 11, adversaryInvites: 0, opportunitiesTaken: 18, opportunitiesMissed: 9 },
};

describe("AbilityModel", () => {
  it("renders the caption, a scored dim, the low-N guard, honest autonomy labels, and a 6-spoke radar with no NaN", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    const { container } = render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(screen.getByText(/任务理解与问题表述/)).toBeTruthy();
    expect(screen.getAllByText(/证据不足/).length).toBeGreaterThan(0); // D3 + D5
    expect(screen.getByText(/把握机会/)).toBeTruthy();  // relabeled autonomy panel
    expect(screen.getByText(/错过机会/)).toBeTruthy();
    expect(screen.getByText("D6")).toBeTruthy();        // 6th radar spoke label present
    expect(container.innerHTML).not.toContain("NaN");   // radar no longer degenerate
  });

  it("no longer renders a metacognition / SOLO panel or 自发/引导后 wording", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(model as never);
    const { container } = render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/不是测验分数/)).toBeTruthy());
    expect(container.textContent).not.toMatch(/SOLO/);
    expect(container.textContent).not.toMatch(/自发/);
    expect(container.textContent).not.toMatch(/引导后/);
  });

  it("renders an empty state when there are no sessions", async () => {
    vi.spyOn(api, "getAbilityModel").mockResolvedValue({ ...model, totalSessions: 0, depth: model.depth.map((d) => ({ ...d, level: -1, evidenceCount: 0 })) } as never);
    render(<AbilityModel />);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
});
