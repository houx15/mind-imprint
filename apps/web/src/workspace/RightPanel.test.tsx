import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { ProcessNode } from "./processTree";

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
});
