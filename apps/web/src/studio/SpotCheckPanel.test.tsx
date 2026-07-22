import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { SpotCheckFx } from "@mind-imprint/contracts";
import { SpotCheckPanel } from "./SpotCheckPanel";

const withItems: SpotCheckFx = {
  items: [
    {
      interventionId: "iv1",
      targetId: "m1",
      targetName: "NASA 全球变绿观测",
      evidence: "写了作用与风险。",
      missing: "没说清遥感口径的局限。",
      fix: "补一句这条数据不能回答什么。",
      disposition: null,
    },
    {
      interventionId: "iv2",
      targetId: "m2",
      targetName: "BP 世界能源统计",
      evidence: "档位已定。",
      missing: "还没写作用与风险。",
      fix: "写这条在论证里承担什么。",
      disposition: null,
    },
  ],
  orderable: true,
};

const empty: SpotCheckFx = { items: [], orderable: false };

describe("SpotCheckPanel", () => {
  it("renders every item through WorkOrderItem (label = targetName, no band chip)", () => {
    render(<SpotCheckPanel title="信源体检" data={withItems} onOrder={() => {}} pending={false} />);
    // Label = targetName, verbatim — the WorkOrderRow mapping this task adds.
    expect(screen.getByText("NASA 全球变绿观测")).toBeInTheDocument();
    expect(screen.getByText("BP 世界能源统计")).toBeInTheDocument();
    expect(screen.getByText("写了作用与风险。")).toBeInTheDocument();
    // Neither row carries a `band` key at all (a spot-check row has no band
    // by design) — WorkOrderItem's own band-absent behaviour is unit-tested
    // in WorkOrder.test.tsx; here it's enough that no band string leaks in.
    expect(screen.queryByText(/段$/)).not.toBeInTheDocument();
  });

  it("calls onOrder when the trigger button is clicked", async () => {
    const onOrder = vi.fn();
    render(<SpotCheckPanel title="信源体检" data={withItems} onOrder={onOrder} pending={false} />);
    await userEvent.click(screen.getByRole("button", { name: /信源体检/ }));
    expect(onOrder).toHaveBeenCalledTimes(1);
  });

  it("disables the button with a neutral note when orderable is false and items exist", () => {
    const notOrderable: SpotCheckFx = { ...withItems, orderable: false };
    render(<SpotCheckPanel title="信源体检" data={notOrderable} onOrder={() => {}} pending={false} />);
    expect(screen.getByRole("button", { name: /信源体检/ })).toBeDisabled();
    // Neutral fact, not a scolding or a score (铁律 2).
    expect(screen.getByText(/还没有新的变化/)).toBeInTheDocument();
  });

  it("does NOT show the 'nothing new' note when orderable is false but there are no items yet", () => {
    render(<SpotCheckPanel title="信源体检" data={empty} onOrder={() => {}} pending={false} />);
    expect(screen.queryByText(/还没有新的变化/)).not.toBeInTheDocument();
  });

  it("renders the empty state (what the check does + re-orderable only after the work changes) when there are no items", () => {
    render(<SpotCheckPanel title="信源体检" data={empty} onOrder={() => {}} pending={false} />);
    expect(screen.getByText(/还没做信源体检/)).toBeInTheDocument();
    expect(screen.getByText(/内容有变化/)).toBeInTheDocument();
  });

  it("disables the button while an order is pending", () => {
    render(<SpotCheckPanel title="论证体检" data={withItems} onOrder={() => {}} pending={true} />);
    expect(screen.getByRole("button", { name: /论证体检/ })).toBeDisabled();
  });

  it("forwards onDisposition to each rendered WorkOrderItem's three-key control", async () => {
    const onDisposition = vi.fn();
    render(
      <SpotCheckPanel title="信源体检" data={withItems} onOrder={() => {}} onDisposition={onDisposition} pending={false} />,
    );
    await userEvent.click(screen.getAllByText("我来改")[0]!);
    const box = screen.getAllByPlaceholderText("写下你的理由（至少 15 字）")[0]!;
    await userEvent.type(box, "遥感口径的局限没写清楚，我要补一句说明。");
    await userEvent.click(screen.getAllByRole("button", { name: "记录处置" })[0]!);
    expect(onDisposition).toHaveBeenCalledWith("iv1", "rewrite", expect.stringContaining("遥感口径"));
  });
});
