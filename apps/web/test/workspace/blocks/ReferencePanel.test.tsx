import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ReferencePanel } from "@/workspace/blocks/ReferencePanel";
import * as workspaceApi from "@/workspace/api/workspace";
import type { Annotation, Proposal, Reference } from "@mind-imprint/contracts";

const REF: Reference = {
  id: "ref-1",
  title: "Nature Sustainability: China's renewable build-out",
  classification: "peer-reviewed",
  author: "Zhang et al.",
  credentials: "Nature Sustainability",
  year: "2024",
  url: "https://example.com/nature-sustainability",
  tags: [],
  collectionId: null,
  credibility: "strong",
  evaluation: "",
  readingNote: "关注可再生能源投资规模。",
  decision: "use",
  pending: false,
  searchHints: [],
  materialId: "mat-1",
  notes: [],
  readingStatus: "done",
};

const EMPTY_PROPOSAL: Proposal = { objective: "", reason: "", activities: "", resources: "", counterpoints: "" };
const FILLED_PROPOSAL: Proposal = {
  objective: "中国的可再生能源投入是否让地球更可持续？",
  reason: "全球气候治理的关键变量。",
  activities: "溯源 NASA / Nature Sustainability 数据。",
  resources: "NASA 卫星数据、Nature Sustainability 论文。",
  counterpoints: "中国碳排放总量仍是全球第一。",
};

describe("ReferencePanel", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(workspaceApi, "getSnippets").mockResolvedValue([]);
    vi.spyOn(workspaceApi, "getAnnotations").mockResolvedValue([]);
  });

  it("renders the 材料 group with a curated material ref's title", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] });

    render(
      <ReferencePanel
        projectId="p1"
        reference={[{ kind: "material", id: "ref-1", label: "Nature Sustainability 文章" }]}
        stage="body_writing"
        proposal={EMPTY_PROPOSAL}
      />,
    );

    expect(await screen.findByText("材料")).toBeInTheDocument();
    expect(await screen.findByText("Nature Sustainability: China's renewable build-out")).toBeInTheDocument();
  });

  it("shows 提案要点 in proposal_writing but hides it in body_writing", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [] });

    const { rerender } = render(
      <ReferencePanel projectId="p1" reference={[]} stage="proposal_writing" proposal={FILLED_PROPOSAL} />,
    );
    expect(await screen.findByText("提案要点")).toBeInTheDocument();
    expect(await screen.findByText(FILLED_PROPOSAL.objective)).toBeInTheDocument();

    rerender(
      <ReferencePanel
        projectId="p1"
        reference={[{ kind: "material", id: "ref-1", label: "x" }]}
        stage="body_writing"
        proposal={FILLED_PROPOSAL}
      />,
    );
    await waitFor(() => expect(screen.queryByText("提案要点")).not.toBeInTheDocument());
  });

  it("renders the centered empty state illustration + ≥14px copy when nothing is curated and proposal is gated out", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [] });

    render(<ReferencePanel projectId="p1" reference={[]} stage="body_writing" proposal={EMPTY_PROPOSAL} />);

    const title = await screen.findByText("还没有印记留下的参考");
    expect(title).toBeInTheDocument();
    expect(title.className).toMatch(/text-mk-h2/);
    const body = await screen.findByText("随着你们一起讨论、阅读，它挑出的材料和笔记会出现在这里。");
    expect(body.className).toMatch(/text-mk-body/);
    expect(document.querySelector("img")).toBeInTheDocument();
  });

  it("never renders a missing/dangling ref as broken content", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [] });

    render(
      <ReferencePanel
        projectId="p1"
        reference={[{ kind: "material", id: "does-not-exist", label: "幽灵材料" }]}
        stage="body_writing"
        proposal={EMPTY_PROPOSAL}
      />,
    );

    await screen.findByText("还没有印记留下的参考");
    expect(screen.queryByText("幽灵材料")).not.toBeInTheDocument();
  });

  it("shows the calm 批注 line rather than faking annotations, once other content is present", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] });

    render(
      <ReferencePanel
        projectId="p1"
        reference={[{ kind: "material", id: "ref-1", label: "x" }]}
        stage="body_writing"
        proposal={EMPTY_PROPOSAL}
      />,
    );

    // slice 4b · the essay writing stage shows the layered AI批注 group (view-only);
    // empty → its calm line.
    expect(await screen.findByText("AI批注")).toBeInTheDocument();
    expect(await screen.findByText("批注会在印记看过你的写作后出现。")).toBeInTheDocument();
  });

  it("renders a curated annotation's criterion·band + text instead of the placeholder", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [] });
    const annotation: Annotation = {
      id: "a1",
      criterion: "分析与论证",
      band: "中段",
      text: "反例没接回主张 → 把反例接回你的核心主张",
    };
    vi.spyOn(workspaceApi, "getAnnotations").mockResolvedValue([annotation]);

    render(
      <ReferencePanel
        projectId="p1"
        reference={[{ kind: "annotation", id: "a1", label: "x" }]}
        stage="body_writing"
        proposal={EMPTY_PROPOSAL}
      />,
    );

    expect(await screen.findByText("批注")).toBeInTheDocument();
    expect(await screen.findByText("反例没接回主张 → 把反例接回你的核心主张")).toBeInTheDocument();
    expect(await screen.findByText("分析与论证·中段")).toBeInTheDocument();
    expect(screen.queryByText("批注会在印记体检你的写作后出现。")).not.toBeInTheDocument();
  });

  it("folds the collected materials into 你的材料 and inserts a fragment on click (retired floating box)", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] });
    const onInsert = vi.fn();

    render(
      // reference=[] → REF is NOT curated, so it appears in the 你的材料 browse
      // (the fold of the old floating 材料 box), with an insert action.
      <ReferencePanel projectId="p1" reference={[]} stage="body_writing" proposal={EMPTY_PROPOSAL} onInsert={onInsert} canInsert />,
    );

    expect(await screen.findByText("你的材料")).toBeInTheDocument();
    expect(await screen.findByText(REF.title)).toBeInTheDocument();
    fireEvent.click((await screen.findAllByRole("button", { name: "插入" }))[0]!);
    expect(onInsert).toHaveBeenCalledWith("关注可再生能源投资规模。");
  });

  it("hides the 「插入」 action when the draft isn't open (canInsert=false)", async () => {
    vi.spyOn(workspaceApi, "getLibrary").mockResolvedValue({ collections: [], references: [REF] });
    render(
      <ReferencePanel projectId="p1" reference={[]} stage="body_writing" proposal={EMPTY_PROPOSAL} onInsert={() => {}} canInsert={false} />,
    );
    expect(await screen.findByText("你的材料")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "插入" })).toBeNull();
  });
});
