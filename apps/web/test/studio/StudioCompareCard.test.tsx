import { useState } from "react";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { CARD_REGISTRY, type MaterialSource } from "@mind-imprint/contracts";
import { StudioCompareCard, type StudioCompareCardProps } from "@/studio/StudioCompareCard";

// The real sift.json (primitive: "compare") — realism over a hand-rolled
// stub, since the exact step/field/param shape is what this card must read.
const spec = CARD_REGISTRY["sift"]!;

function material(id: string, title: string): MaterialSource {
  return {
    id,
    title,
    sourceUrl: "",
    kind: "article",
    origin: "fetched",
    blocks: [],
    locked: false,
    role: "",
    tier: "",
    takeaway: "",
    anchors: [],
    timeSpentS: 0,
    lateralRead: false,
    isLateralInstrument: false,
    siftSkipped: false,
    lateralRelation: "",
    lateralJudgment: "",
  };
}

const blog = material("mat-blog", "《卫星图看中国变绿》");
const nasa = material("mat-nasa", "Chen et al. (2019), Nature Sustainability");

// `lateralMaterialId` is lifted state (whole-branch review finding [3]) — in
// production it lives in StudioContainer. This harness plays that same role
// for the test: a real controlling parent, not a prop the component holds
// itself, so these tests exercise the actual controlled-component contract.
function Harness(props: Omit<StudioCompareCardProps, "lateralMaterialId" | "onLateralMaterialChange">) {
  const [lateralMaterialId, setLateralMaterialId] = useState("");
  return <StudioCompareCard {...props} lateralMaterialId={lateralMaterialId} onLateralMaterialChange={setLateralMaterialId} />;
}

describe("StudioCompareCard", () => {
  it("renders a chip + question + textarea per step field, and a single_choice as pill options", () => {
    render(
      <Harness spec={spec} anchors={[]} materialId={blog.id} materials={[blog]} onSubmit={() => {}} onSkip={() => {}} />,
    );
    // stop/investigate/find/trace_origin dimension chips (params.tags)
    expect(screen.getByText("stop")).toBeInTheDocument();
    expect(screen.getByText("find")).toBeInTheDocument();
    expect(screen.getByText("这个独立来源怎么说同一件事？")).toBeInTheDocument();
    // relation + tier_after render as pill options, not free text
    expect(screen.getByRole("button", { name: "印证" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "原始证据" })).toBeInTheDocument();
  });

  it("does not render a checked-material picker — the checked material is a server fact (finding [5]), not a client choice", () => {
    render(
      <Harness spec={spec} anchors={[]} materialId={blog.id} materials={[blog, nasa]} onSubmit={() => {}} onSkip={() => {}} />,
    );
    expect(screen.queryByTestId("checked-material-picker")).not.toBeInTheDocument();
    expect(screen.queryByText("待查的来源")).not.toBeInTheDocument();
  });

  // Whole-branch review: this card used to carry its OWN "添加信源" button
  // here, wired (by CoachRail) to onSelectStation("S3") — a no-op, since a
  // compare card only ever surfaces while already on S3, so it never even
  // rendered on the seeded project. Deleted rather than rewired: the center
  // pane (ViewFrame's Compare + its "查看信源档案" toggle) already offers a
  // fully-wired 添加信源 flow reachable from wherever this card is showing.
  // This card now shows only the honest informational text, no dead CTA.
  it("shows informational text (no button) instead of a picker when no independent source exists yet", () => {
    render(
      <Harness
        spec={spec}
        anchors={[]}
        materialId={blog.id}
        materials={[blog]}
        onSubmit={() => {}}
        onSkip={() => {}}
      />,
    );
    expect(screen.getByText(/还没有独立来源/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "添加信源" })).not.toBeInTheDocument();
  });

  it("cannot lock until the lateral source is picked and every field is answered", () => {
    render(
      <Harness spec={spec} anchors={[]} materialId={blog.id} materials={[blog, nasa]} onSubmit={() => {}} onSkip={() => {}} />,
    );
    const lockButton = screen.getByRole("button", { name: "锁定这张卡" });
    expect(lockButton).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText(/先写下来/), { target: { value: "第一反应：有点意外" } });
    fireEvent.change(screen.getByPlaceholderText(/机构、个人/), { target: { value: "自媒体博主，无机构背景" } });
    expect(lockButton).toBeDisabled(); // no lateral source picked yet

    fireEvent.click(within(screen.getByTestId("lateral-material-picker")).getByRole("button", { name: nasa.title }));
    fireEvent.change(screen.getByPlaceholderText(/怎么说同一件事/), { target: { value: "NASA 数据显示排放仍在上升" } });
    expect(lockButton).toBeDisabled(); // relation/trace/tier still unanswered

    fireEvent.click(screen.getByRole("button", { name: "印证" }));
    fireEvent.change(screen.getByPlaceholderText(/原始的出处是哪里/), { target: { value: "Nature Sustainability 论文" } });
    fireEvent.click(screen.getByRole("button", { name: "原始证据" }));
    expect(lockButton).toBeDisabled(); // revised_judgment still unanswered

    fireEvent.change(screen.getByPlaceholderText(/和一开始比/), { target: { value: "从二手转述降级为需要追源的说法" } });
    expect(lockButton).not.toBeDisabled();
  });

  it("submits anchors keyed by the DECLARED lateral_dimension, not by array position — the anchors[0] trap", () => {
    const onSubmit = vi.fn();
    render(
      <Harness spec={spec} anchors={[]} materialId={blog.id} materials={[blog, nasa]} onSubmit={onSubmit} onSkip={() => {}} />,
    );

    fireEvent.change(screen.getByPlaceholderText(/先写下来/), { target: { value: "第一反应：有点意外" } });
    fireEvent.change(screen.getByPlaceholderText(/机构、个人/), { target: { value: "自媒体博主，无机构背景" } });
    fireEvent.click(within(screen.getByTestId("lateral-material-picker")).getByRole("button", { name: nasa.title }));
    fireEvent.change(screen.getByPlaceholderText(/怎么说同一件事/), { target: { value: "NASA 数据显示排放仍在上升" } });
    fireEvent.click(screen.getByRole("button", { name: "印证" }));
    fireEvent.change(screen.getByPlaceholderText(/原始的出处是哪里/), { target: { value: "Nature Sustainability 论文" } });
    fireEvent.click(screen.getByRole("button", { name: "原始证据" }));
    fireEvent.change(screen.getByPlaceholderText(/和一开始比/), { target: { value: "从二手转述降级为需要追源的说法" } });

    fireEvent.click(screen.getByRole("button", { name: "锁定这张卡" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const env = onSubmit.mock.calls[0]![0];

    // anchors[0] ("stop") must carry the CHECKED material, never the lateral
    // one — the exact coin-flip Task 3 fixed server-side; this proves the
    // client builds it by the same declared rule (params.lateral_dimension),
    // not by array order.
    expect(env.anchors[0].dimension).toBe("stop");
    expect(env.anchors[0].material_id).toBe(blog.id);
    const findAnchor = env.anchors.find((a: { dimension: string }) => a.dimension === "find");
    expect(findAnchor.material_id).toBe(nasa.id);
    expect(findAnchor.material_id).not.toBe(env.anchors[0].material_id);
  });

  it("skip reports the scaffold's event trace without submitting", () => {
    const onSkip = vi.fn();
    const onSubmit = vi.fn();
    render(
      <Harness spec={spec} anchors={[]} materialId={blog.id} materials={[blog]} onSubmit={onSubmit} onSkip={onSkip} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "跳过这张卡" }));
    expect(onSkip).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("the lateral pick is a controlled prop, not private state — a parent overwriting it is reflected immediately (finding [3])", () => {
    function ControlledFromOutside() {
      const [lateralMaterialId, setLateralMaterialId] = useState("");
      return (
        <>
          <button type="button" onClick={() => setLateralMaterialId(nasa.id)}>
            外部选中 NASA
          </button>
          <StudioCompareCard
            spec={spec}
            anchors={[]}
            materialId={blog.id}
            materials={[blog, nasa]}
            lateralMaterialId={lateralMaterialId}
            onLateralMaterialChange={setLateralMaterialId}
            onSubmit={() => {}}
            onSkip={() => {}}
          />
        </>
      );
    }
    render(<ControlledFromOutside />);
    const nasaPill = within(screen.getByTestId("lateral-material-picker")).getByRole("button", { name: nasa.title });
    expect(nasaPill).toHaveStyle({ background: "#fff" });
    fireEvent.click(screen.getByRole("button", { name: "外部选中 NASA" }));
    expect(nasaPill).toHaveStyle({ background: "#5C4A8A" });
  });
});
