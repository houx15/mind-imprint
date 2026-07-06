import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { ProcessNode } from "./processTree";
import type { Anchor } from "@mind-imprint/contracts";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listMaterials: vi.fn().mockResolvedValue([]), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() } };
});

import { RightPanel } from "./RightPanel";

const nodes: ProcessNode[] = [
  { id: "root", type: "task_root", parent_id: null, title: "任务根" },
  { id: "n1", type: "card_use", parent_id: "root", title: "用了 CRAAP", sub: "信息素养" },
];

describe("RightPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("defaults to the process-tree tab", () => {
    render(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} />);
    expect(screen.getByText("用了 CRAAP")).toBeInTheDocument();
  });

  it("switches to the material tab", async () => {
    render(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} />);
    fireEvent.click(screen.getByRole("tab", { name: "材料" }));
    // no seed → paste fallback surfaces
    expect(await screen.findByPlaceholderText(/把材料贴进来/)).toBeInTheDocument();
  });

  it("switches to material tab when anchors first appear", async () => {
    const anchor: Anchor = {
      id: "a0",
      material_id: "m1",
      block_id: "b0",
      start: 0,
      end: 10,
      quote: "test quote",
      dimension: "权威性 · Authority",
      author: "ai",
      question: "test question",
      answer: "",
    };

    const { rerender } = render(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} anchors={[]} />);
    // Initially shows tree tab content
    expect(screen.getByText("用了 CRAAP")).toBeInTheDocument();

    // Rerender with anchors
    rerender(<RightPanel nodes={nodes} taskId="t1" seedUrl={null} anchors={[anchor]} />);

    // Material tab should now be selected
    expect(screen.getByRole("tab", { name: "材料" })).toHaveAttribute("aria-selected", "true");
    // Material pane content should be visible (paste text or empty state)
    expect(await screen.findByPlaceholderText(/把材料贴进来/)).toBeInTheDocument();
  });
});
