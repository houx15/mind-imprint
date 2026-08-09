import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

vi.mock("@/api/resourceNeeds", () => ({
  getResourceNeeds: vi.fn(async () => [{ id: "a", text: "中国碳排放数据", done: false }]),
  putResourceNeeds: vi.fn(async (_p: string, needs: unknown) => needs),
}));

import { NeedsResourcesBox } from "@/workspace/blocks/NeedsResourcesBox";
import { getResourceNeeds, putResourceNeeds } from "@/api/resourceNeeds";

describe("NeedsResourcesBox", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("loads existing needs and adds a new one (persists)", async () => {
    render(<NeedsResourcesBox projectId="p1" />);
    expect(await screen.findByText("中国碳排放数据")).toBeTruthy();

    const input = screen.getByPlaceholderText("写下想查的资料/方向……");
    fireEvent.change(input, { target: { value: "NASA 卫星数据" } });
    fireEvent.click(screen.getByText("记下"));
    expect(await screen.findByText("NASA 卫星数据")).toBeTruthy();
    await waitFor(() => expect(putResourceNeeds).toHaveBeenCalled());
  });

  it("去探索 with an item calls onExplore(text)", async () => {
    const onExplore = vi.fn();
    render(<NeedsResourcesBox projectId="p1" onExplore={onExplore} />);
    await screen.findByText("中国碳排放数据");
    fireEvent.click(screen.getByText("去探索"));
    expect(onExplore).toHaveBeenCalledWith("中国碳排放数据");
    expect(getResourceNeeds).toHaveBeenCalledWith("p1");
  });
});
