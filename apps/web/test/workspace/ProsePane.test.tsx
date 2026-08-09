import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProsePane } from "@/workspace/blocks/ProsePane";

// getDraft is called on mount; stub it so the component mounts without a network.
vi.mock("@/workspace/api/workspace", async (orig) => {
  const actual = await orig<typeof import("@/workspace/api/workspace")>();
  return { ...actual, getDraft: vi.fn(async () => "") };
});

describe("ProsePane (§4 cleanup)", () => {
  it("shows word count + save status in a lower-right overlay, not a top bar", async () => {
    render(<ProsePane projectId="p1" doc="proposal" />);
    // The status overlay carries the word count and the save label.
    expect(await screen.findByText(/字$/)).toBeTruthy();
    expect(screen.getByText(/已保存|未保存|保存中/)).toBeTruthy();
    // The preview toggle is still reachable.
    expect(screen.getByTitle("切换 Markdown 预览")).toBeTruthy();
  });
});
