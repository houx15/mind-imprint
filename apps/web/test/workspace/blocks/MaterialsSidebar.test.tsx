import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MaterialsSidebar } from "@/workspace/blocks/MaterialsSidebar";
import * as workspaceApi from "@/workspace/api/workspace";
import * as explorationApi from "@/api/exploration";

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

// #6 · onImportOutlineAsGroups/importedSections default to no-ops/empty so
// each test only opts into them when it's exercising that behavior.
const DEFAULTS = { onImportOutlineAsGroups: () => {}, importedSections: [] as string[] };

describe("MaterialsSidebar", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] as never });
    vi.spyOn(workspaceApi, "getOutline").mockResolvedValue(OUTLINE as never);
    vi.spyOn(explorationApi, "getExploration").mockResolvedValue({ leads: [], danglingSourceIds: [], edges: [] });
  });

  it("materials: unfolds notes and 收进片段 appends a snippet (snippets tab)", async () => {
    const onAddSnippet = vi.fn();
    render(
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="snippets" locked={false} snippets={[]} onAddSnippet={onAddSnippet} onInsertToDraft={() => {}} />,
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
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    const btns = await screen.findAllByRole("button", { name: /插入正文/ });
    fireEvent.click(btns[0]!);
    expect(onInsertToDraft).toHaveBeenCalledWith("我觉得变绿≠可持续");
  });

  // Q1 · after a successful insert, the item gets a persistent "已插入正文"
  // badge (distinct from the brief "已插入正文 ✓" flash on the button itself,
  // which clears after ~1.2s) — but the button must stay clickable so the
  // same fragment can be inserted again.
  it("materials: a successful insert marks the item 已插入正文, and it stays re-insertable (Q1)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    const btn = (await screen.findAllByRole("button", { name: /插入正文/ }))[0]!;
    fireEvent.click(btn);
    expect(onInsertToDraft).toHaveBeenCalledTimes(1);
    // the transient "done" flash clears (~1.2s); the persistent badge (a plain
    // span, exact text "已插入正文" with no "✓") then shows on its own.
    await waitFor(() => expect(screen.getByText("已插入正文")).toBeInTheDocument(), { timeout: 2000 });
    // still fully clickable — inserting again fires the callback again
    fireEvent.click(screen.getAllByRole("button", { name: /插入正文/ })[0]!);
    expect(onInsertToDraft).toHaveBeenCalledTimes(2);
    expect(onInsertToDraft.mock.calls[1]![0]).toBe("我觉得变绿≠可持续");
  });

  it("大纲 source: import lays outline titles into the draft as headings (#9)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    // switch the browsed source to 大纲
    fireEvent.click(await screen.findByRole("button", { name: "大纲" }));
    fireEvent.click(await screen.findByRole("button", { name: /把大纲导入正文/ }));
    expect(onInsertToDraft).toHaveBeenCalledWith("# 引言\n\n## 主张一");
  });

  // #6 · replaces the old per-row "收进片段" dump: a single explicit action turns
  // the outline's TOP-LEVEL headings into 片段 board section labels — never the
  // heading's text becoming a flat snippet's body.
  it("大纲 source: 把大纲导入为片段分组 imports only the top-level headings as section labels (#6)", async () => {
    const onImportOutlineAsGroups = vi.fn();
    render(
      <MaterialsSidebar
        projectId="p1"
        activeTab="snippets"
        locked={false}
        snippets={[]}
        onAddSnippet={() => {}}
        onInsertToDraft={() => {}}
        onImportOutlineAsGroups={onImportOutlineAsGroups}
        importedSections={[]}
      />,
    );
    fireEvent.click(await screen.findByRole("button", { name: "大纲" }));
    fireEvent.click(await screen.findByRole("button", { name: "把大纲导入为片段分组" }));
    expect(onImportOutlineAsGroups).toHaveBeenCalledWith(["引言"]);
    // no per-row 收进片段 button dumping a heading's name as a snippet
    expect(screen.queryByRole("button", { name: /收进片段/ })).toBeNull();
  });

  it("大纲 source: an already-imported outline shows the done state (#6)", async () => {
    render(
      <MaterialsSidebar
        projectId="p1"
        activeTab="snippets"
        locked={false}
        snippets={[]}
        onAddSnippet={() => {}}
        onInsertToDraft={() => {}}
        onImportOutlineAsGroups={() => {}}
        importedSections={["引言"]}
      />,
    );
    fireEvent.click(await screen.findByRole("button", { name: "大纲" }));
    expect(await screen.findByRole("button", { name: /已导入为片段分组/ })).toBeInTheDocument();
  });

  it("片段 source: on the 正文 tab, clicking a snippet inserts it into the draft (#9)", async () => {
    const onInsertToDraft = vi.fn();
    render(
      <MaterialsSidebar
        {...DEFAULTS}
        projectId="p1"
        activeTab="draft"
        locked={false}
        snippets={[{ id: "s1", text: "我攒的一个片段", section: null }]}
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
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked snippets={[]} onAddSnippet={() => {}} onInsertToDraft={onInsertToDraft} />,
    );
    expect(await screen.findByText(/只读浏览/)).toBeInTheDocument();
    fireEvent.click(await screen.findByText("NASA 绿化报告"));
    await waitFor(() => expect(screen.getByText("我觉得变绿≠可持续")).toBeTruthy());
    expect(screen.queryByRole("button", { name: /插入正文/ })).toBeNull();
    // 大纲 import is gone too (both actions)
    fireEvent.click(screen.getByRole("button", { name: "大纲" }));
    expect(screen.queryByRole("button", { name: /把大纲导入正文/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /把大纲导入为片段分组/ })).toBeNull();
  });

  // Batch5 Item C (#16): 线索 as a categorization tag on the 材料 tab — a
  // reference is associated with a lead via sourceReferenceId/
  // connectedReferenceId (no new persistence; reuses the exploration graph).
  it("材料 tab: 线索 filter narrows references to the ones under a chosen 线索", async () => {
    const REF2 = { ...REF, id: "r2", title: "另一篇未归类的来源" };
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF, REF2] as never });
    vi.spyOn(explorationApi, "getExploration").mockResolvedValue({
      leads: [
        {
          id: "lead1",
          text: "碳排放反例线索",
          status: "open",
          origin: "manual",
          sourceReferenceId: REF.id,
          connectedReferenceId: null,
          position: 0,
          parentLeadId: null,
        },
      ],
      danglingSourceIds: [],
    } as never);

    render(
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
    );
    await screen.findByText("NASA 绿化报告");
    expect(screen.getByText("另一篇未归类的来源")).toBeInTheDocument();

    const picker = await screen.findByLabelText("按线索筛选材料");
    fireEvent.change(picker, { target: { value: "lead1" } });
    expect(screen.getByText("NASA 绿化报告")).toBeInTheDocument();
    expect(screen.queryByText("另一篇未归类的来源")).not.toBeInTheDocument();

    fireEvent.change(picker, { target: { value: "__uncat__" } });
    expect(screen.queryByText("NASA 绿化报告")).not.toBeInTheDocument();
    expect(screen.getByText("另一篇未归类的来源")).toBeInTheDocument();

    fireEvent.change(picker, { target: { value: "__all__" } });
    expect(screen.getByText("NASA 绿化报告")).toBeInTheDocument();
    expect(screen.getByText("另一篇未归类的来源")).toBeInTheDocument();
  });

  it("collapses to a 材料 tab and reopens", async () => {
    render(
      <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
    );
    fireEvent.click(screen.getByText("×"));
    const reopen = await screen.findByRole("button", { name: /材料/ });
    fireEvent.click(reopen);
    expect(screen.getByText(/拖动可移动/)).toBeTruthy();
  });

  // Batch5 follow-up Item B · which source tabs are offered depends on the
  // active MAIN panel (大纲/片段/正文 in WritingBlock) passed in as activeTab.
  describe("per-panel tab visibility (Item B)", () => {
    it("大纲 panel: only 材料 is offered", async () => {
      render(
        <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="outline" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
      );
      await screen.findByText("NASA 绿化报告"); // 材料 loaded and shown by default
      expect(screen.queryByRole("button", { name: "大纲" })).toBeNull();
      expect(screen.queryByRole("button", { name: "片段" })).toBeNull();
    });

    it("片段 panel: 材料 + 大纲 are offered, not 片段", async () => {
      render(
        <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="snippets" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
      );
      expect(await screen.findByRole("button", { name: "大纲" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "片段" })).toBeNull();
    });

    it("正文 panel: all three sources are offered", async () => {
      render(
        <MaterialsSidebar {...DEFAULTS} projectId="p1" activeTab="draft" locked={false} snippets={[]} onAddSnippet={() => {}} onInsertToDraft={() => {}} />,
      );
      expect(await screen.findByRole("button", { name: "大纲" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "片段" })).toBeInTheDocument();
    });
  });
});
