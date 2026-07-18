import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { GrowthReport } from "./GrowthReport";
import { api } from "../../api";

describe("GrowthReport", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("shows the empty state with no reports", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    render(<GrowthReport />);
    expect(await screen.findByText("还没有报告")).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull(); // no generate / 重新生成
  });

  it("renders a history row and expands it to the report", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([
      { surface: "project", scopeId: "p1", label: "中国可持续", sublabel: null, createdAt: "2026-07-18T00:00:00Z",
        report: { dimensions: [{ code: "D1", name: "追问", level: "L3", evidence: "追问了三次" }], narrative: "你的印记…", generatedAt: "2026-07-18T00:00:00Z" } },
    ]);
    render(<GrowthReport />);
    expect(await screen.findByText("中国可持续")).toBeTruthy();
    // newest is auto-expanded → dimension + narrative visible
    expect(screen.getByText("追问")).toBeTruthy();
    expect(screen.getByText("你的印记…")).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull();
  });
});
