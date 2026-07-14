import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { CARD_REGISTRY, type MaterialSource } from "@mind-imprint/contracts";
import { StudioCompareCard } from "./StudioCompareCard";

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
  };
}

const blog = material("mat-blog", "《卫星图看中国变绿》");
const nasa = material("mat-nasa", "Chen et al. (2019), Nature Sustainability");

describe("StudioCompareCard", () => {
  it("renders a chip + question + textarea per step field, and a single_choice as pill options", () => {
    render(
      <StudioCompareCard spec={spec} anchors={[]} materials={[blog]} onAddLateralSource={() => {}} onSubmit={() => {}} onSkip={() => {}} />,
    );
    // stop/investigate/find/trace_origin dimension chips (params.tags)
    expect(screen.getByText("stop")).toBeInTheDocument();
    expect(screen.getByText("find")).toBeInTheDocument();
    expect(screen.getByText("这个独立来源怎么说同一件事？")).toBeInTheDocument();
    // relation + tier_after render as pill options, not free text
    expect(screen.getByRole("button", { name: "印证" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "原始证据" })).toBeInTheDocument();
  });

  it("shows the 添加信源 affordance instead of a picker when no independent source exists yet", () => {
    const onAddLateralSource = vi.fn();
    render(
      <StudioCompareCard
        spec={spec}
        anchors={[]}
        materials={[blog]}
        onAddLateralSource={onAddLateralSource}
        onSubmit={() => {}}
        onSkip={() => {}}
      />,
    );
    expect(screen.getByText(/还没有独立来源/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "添加信源" }));
    expect(onAddLateralSource).toHaveBeenCalledTimes(1);
  });

  it("cannot lock until the lateral source is picked and every field is answered", () => {
    render(
      <StudioCompareCard spec={spec} anchors={[]} materials={[blog, nasa]} onAddLateralSource={() => {}} onSubmit={() => {}} onSkip={() => {}} />,
    );
    const lockButton = screen.getByRole("button", { name: "锁定这张卡" });
    expect(lockButton).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText(/先写下来/), { target: { value: "第一反应：有点意外" } });
    fireEvent.change(screen.getByPlaceholderText(/机构、个人/), { target: { value: "自媒体博主，无机构背景" } });
    expect(lockButton).toBeDisabled(); // no lateral source picked yet

    // Picking the lateral source (nasa) among the candidates — scoped to the
    // lateral picker, since nasa's title also appears in the (unrelated)
    // checked-material picker once there are 2+ materials.
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
      <StudioCompareCard spec={spec} anchors={[]} materials={[blog, nasa]} onAddLateralSource={() => {}} onSubmit={onSubmit} onSkip={() => {}} />,
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
      <StudioCompareCard spec={spec} anchors={[]} materials={[blog]} onAddLateralSource={() => {}} onSubmit={onSubmit} onSkip={onSkip} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "跳过这张卡" }));
    expect(onSkip).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
