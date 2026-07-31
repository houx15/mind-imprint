import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MaterialsSidebar } from "@/workspace/blocks/MaterialsSidebar";
import * as workspaceApi from "@/workspace/api/workspace";

const REF = {
  id: "r1",
  title: "NASA 绿化报告",
  classification: "报告",
  author: "NASA",
  credentials: "官方",
  year: "2023",
  url: "https://example.org",
  tags: [] as string[],
  collectionId: null,
  credibility: "strong" as const,
  evaluation: "",
  readingNote: "我觉得变绿≠可持续",
  decision: "use" as const,
  pending: false,
  searchHints: [] as string[],
  materialId: "m1",
  notes: [{ quote: "植被覆盖上升", finding: "支持正方" }],
  takeaway: {
    findings: ["中国碳排放总量全球第一"],
    credibility: { verdict: "mixed", why: "口径不同" },
    keyQuotes: [{ quote: "碳排放全球第一", why: "反例" }],
    newLeads: [],
    proposalImpact: "让步段要正面处理这个反例",
  },
};

describe("MaterialsSidebar", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("lists references, unfolds their notes, and 收进片段 calls onInsert", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] as never });
    const onInsert = vi.fn();
    render(<MaterialsSidebar projectId="p1" onInsert={onInsert} />);

    // The reference row appears.
    const row = await screen.findByText("NASA 绿化报告");
    // Expand it → the note-bearing chunks show.
    fireEvent.click(row);
    await waitFor(() => expect(screen.getByText("我觉得变绿≠可持续")).toBeTruthy());
    expect(screen.getByText("让步段要正面处理这个反例")).toBeTruthy();
    expect(screen.getByText("中国碳排放总量全球第一")).toBeTruthy();

    // 收进片段 on the first chunk inserts that text.
    const insertButtons = screen.getAllByRole("button", { name: /收进片段/ });
    fireEvent.click(insertButtons[0]!);
    expect(onInsert).toHaveBeenCalledWith("我觉得变绿≠可持续");
  });

  it("collapses to a 材料 tab and reopens", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [] });
    render(<MaterialsSidebar projectId="p1" onInsert={() => {}} />);
    // Close it.
    fireEvent.click(screen.getByText("×"));
    // A 材料 reopen tab remains.
    const reopen = await screen.findByRole("button", { name: /材料/ });
    fireEvent.click(reopen);
    expect(screen.getByText(/拖动可移动/)).toBeTruthy();
  });
});
