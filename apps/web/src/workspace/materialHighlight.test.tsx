import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { Anchor, Material } from "@mind-imprint/contracts";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return { ...real, api: { ...real.api, listMaterials: vi.fn(), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() } };
});

import { api } from "../api";
import { MaterialPane } from "./MaterialPane";

const material: Material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "文章",
  source_url: "https://x", blocks: [{ id: "b0", text: "最近某科技博主综合整理的文章刷屏了。" }], scratch: "", created_at: "1",
};
const anchors: Anchor[] = [
  { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "某科技博主综合整理", dimension: "权威性", author: "ai", question: "?", answer: "" },
];

describe("MaterialPane highlighting", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  it("wraps the anchor quote in a highlight mark", async () => {
    (api.listMaterials as any).mockResolvedValue([material]);
    render(<MaterialPane taskId="t1" seedUrl="https://x" anchors={anchors} />);
    const mark = await screen.findByText("某科技博主综合整理");
    expect(mark.tagName.toLowerCase()).toBe("mark");
    // surrounding plain text still present
    expect(screen.getByText(/最近/)).toBeInTheDocument();
    expect(screen.getByText(/的文章刷屏了/)).toBeInTheDocument();
  });
});
