import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ToolkitCards } from "@/shell/growth/ToolkitCards";
import { api } from "@/api";

afterEach(() => { vi.restoreAllMocks(); });

const cards = [
  { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" }, // 知识工具
  { cardId: "opcvl", uses: 1, surfaces: ["course", "chat"], lastUsed: "2026-07-17T00:00:00Z" }, // AOK
  { cardId: "__no_such_card__", uses: 9, surfaces: ["project"], lastUsed: "2026-07-16T00:00:00Z" }, // dropped
];

describe("ToolkitCards", () => {
  it("groups by category, shows usage, and drops unknown card ids", async () => {
    vi.spyOn(api, "getGrowthCards").mockResolvedValue(cards as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/按类别组织/)).toBeTruthy());
    // category group headers
    expect(screen.getByText("知识工具")).toBeTruthy();
    expect(screen.getByText("AOK")).toBeTruthy();
    // the ×N badge (concession used twice) and a surface/最近 usage line
    expect(screen.getByText("×2")).toBeTruthy();
    expect(screen.getByText(/最近 07-18/)).toBeTruthy();
    expect(screen.getByText(/项目/)).toBeTruthy();
    // unknown id contributed no tile: its bogus ×9 must be absent
    expect(screen.queryByText("×9")).toBeNull();
  });

  it("renders the empty state when nothing is collected", async () => {
    vi.spyOn(api, "getGrowthCards").mockResolvedValue([] as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/还没有收集到工具卡/)).toBeTruthy());
  });

  it("shows the thinking-move reveal (methodology why) for a card with consolidation", async () => {
    vi.spyOn(api, "getGrowthCards").mockResolvedValue([
      { cardId: "craap", uses: 1, surfaces: ["project"], lastUsed: "2026-07-23T00:00:00Z" },
    ] as never);
    render(<ToolkitCards />);
    // The reveal is labelled and carries the spec's methodology.why.
    expect(await screen.findByText(/你练的思路/)).toBeInTheDocument();
  });
});
