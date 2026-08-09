import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

vi.mock("@/api/searchGuidance", () => ({
  proposeSearchGuidance: vi.fn(async () => [
    { keyword: "solar capacity China", why: "补子问题一的支持证据" },
  ]),
}));

import { SearchGuidanceBox } from "@/workspace/blocks/exploration/SearchGuidanceBox";
import { proposeSearchGuidance } from "@/api/searchGuidance";

describe("SearchGuidanceBox", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("proposes on tap, renders keyword + why, and 搜索 fires onSearch", async () => {
    const onSearch = vi.fn();
    render(<SearchGuidanceBox projectId="p1" onSearch={onSearch} />);

    fireEvent.click(screen.getByText("让印记建议检索方向"));
    expect(await screen.findByText("solar capacity China")).toBeTruthy();
    expect(screen.getByText("补子问题一的支持证据")).toBeTruthy();
    expect(proposeSearchGuidance).toHaveBeenCalledWith("p1");

    fireEvent.click(screen.getByText("搜索"));
    expect(onSearch).toHaveBeenCalledWith("solar capacity China");
  });
});
