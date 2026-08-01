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

const OUTLINE = [
  { id: "o1", text: "引言", depth: 0, position: 0 },
  { id: "o2", text: "主张一", depth: 1, position: 1 },
];

describe("MaterialsSidebar", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] as never });
    vi.spyOn(workspaceApi, "getOutline").mockResolvedValue(OUTLINE as never);
  });

  it("materials: unfolds notes and 收进片段 appends a snippet (snippets tab)", async () => {
    const onAddSnippet = vi.fn();
    render(
      <MaterialsSidebar projectId="p1" activeTab="snippets" locked={false} snippets={[]} onAddSnippet={onAddSnippet} onInsertToDraft={() => {}} />,
    );
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    await waitFor(() => expect(screen.getByText("我觉得变绿≠可持续")).toBeTruthy());
    const btns = screen.getAllByRole("button", { name: /收进片段/ });
    fireEvent.click(btns[0]!);
    expect(onAddSnippet).toHaveBeenCalledWith("我觉得变绿≠可持续");
  });

  it("materials: on the 正文 tab, the action inserts into the draft instead (#9)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    const btns = await screen.findAllByRole("button", { name: /插入正文/ });
    fireEvent.click(btns[0]!);
    expect(onInsertToDraft).toHaveBeenCalledWith("我觉得变绿≠可持续");
  });

  it("大纲 source: import lays outline titles into the draft as headings (#9)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    // switch the browsed source to 大纲
    fireEvent.click(await screen.findByRole("button", { name: "大纲" }));
    fireEvent.click(await screen.findByRole("button", { name: /把大纲导入正文/ }));
    expect(onInsertToDraft).toHaveBeenCalledWith("# 引言\n\n## 主张一");
  });

  it("片段 source: on the 正文 tab, clicking a snippet inserts it into the draft (#9)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar
        projectId="p1"
        activeTab="draft"
        locked={false}
        snippets={[{ id: "s1", text: "我攒的一个片段" }]}
        onAddSnippet={() => {}}
        onInsertToDraft={onInsertToDraft}
      />,
    );
    fireEvent.click(await screen.findByRole("button", { name: "片段" }));
    fireEvent.click(await screen.findByText("我攒的一个片段"));
    const btns = await screen.findAllByRole("button", { name: /插入正文/ });
    fireEvent.click(btns[0]!);
    expect(onInsertToDraft).toHaveBeenCalledWith("我攒的一个片段");
  });

  it("archived (locked): browse-only, no place buttons flash false success (review M1)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar projectId="p1" activeTab="draft" locked snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    expect(await screen.findByText(/只读浏览/)).toBeInTheDocument();
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    await waitFor(() => expect(screen.getByText("我觉得变绿≠可持续")).toBeTruthy());
    expect(screen.queryByRole("button", { name: /插入正文/ })).toBeNull();
    // 大纲 import is gone too
    fireEvent.click(screen.getByRole("button", { name: "大纲" }));
    expect(screen.queryByRole("button", { name: /把大纲导入正文/ })).toBeNull();
  });

  it("collapses to a 材料 tab and reopens", async () => {
    render(
      <MaterialsSidebar projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
    );
    fireEvent.click(screen.getByText("×"));
    const reopen = await screen.findByRole("button", { name: /材料/ });
    fireEvent.click(reopen);
    expect(screen.getByText(/拖动可移动/)).toBeTruthy();
  });
});
