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

// The default, never-checked state at either station: no batch has ever
// existed AND there is currently nothing to check (`orderable: false` is the
// default until she's added/evaluated a source at S3, or written a Toulmin
// slot at S4 — apps/api/internal/studio/projection.go's `len(targets) == 0`).
const empty: SpotCheckFx = { items: [], orderable: false };
// I2: the OTHER empty state — no batch has ever existed, but there IS
// something to check right now (button enabled). Distinguishes "nothing to
// press yet" (above) from "you can press it, you just haven't yet".
const orderableEmpty: SpotCheckFx = { items: [], orderable: true };

describe("SpotCheckPanel", () => {
  it("renders every item through WorkOrderItem (label = targetName, no band chip)", () => {
    render(<SpotCheckPanel title="信源体检" data={withItems} onOrder={() => {}} pending={false} />);
    // Label = targetName, verbatim — the WorkOrderRow mapping this task adds.
    expect(screen.getByText("NASA 全球变绿观测")).toBeInTheDocument();
    expect(screen.getByText("BP 世界能源统计")).toBeInTheDocument();
    expect(screen.getByText("写了作用与风险。")).toBeInTheDocument();
  });

  // M1 fix: `expect(screen.queryByText(/段$/)).not.toBeInTheDocument()` never
  // actually protected the stated constraint ("band is omitted ENTIRELY,
  // never an empty string") — if a future edit passed `band: ""`,
  // WorkOrderItem's `row.band !== undefined && <span>{row.band}</span>`
  // still renders an (empty) chip <span>, which no `/段$/` regex can ever
  // match, so the old assertion would keep passing right through the bug.
  // Asserting the header renders exactly one child (the label span, no
  // second element for a band chip) actually fails the moment a band key of
  // any kind — including "" — shows up on the row.
  it("renders no band-chip element at all for a spot-check row (band key entirely absent, not blank-stringed)", () => {
    render(<SpotCheckPanel title="信源体检" data={withItems} onOrder={() => {}} pending={false} />);
    const header = screen.getByText("NASA 全球变绿观测").parentElement;
    expect(header?.children).toHaveLength(1);
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
    // I2 fix: the empty state must never instruct pressing a button that may
    // currently be disabled — this scenario's button IS disabled (orderable
    // false), so the old "点上面的「信源体检」" phrasing must be gone.
    expect(screen.queryByText(/点上面的/)).not.toBeInTheDocument();
  });

  // I2 fix: this is the DEFAULT state at either station before she's ever
  // met the gate (S3: no evaluated source yet; S4: no non-blank Toulmin slot
  // yet). Before this fix, nothing distinguished it from a station that HAS
  // been checked before but has nothing new — she saw only a disabled button
  // over copy telling her to press it, with no explanation of either fact.
  it("shows a neutral note explaining what has to exist first when orderable is false and no batch has ever existed (S3 copy)", () => {
    render(<SpotCheckPanel title="信源体检" data={empty} onOrder={() => {}} pending={false} />);
    expect(screen.getByRole("button", { name: /信源体检/ })).toBeDisabled();
    // Names only what actually gates orderable — adding a source, NOT
    // evaluating it (see the comment on NOT_POSSIBLE_YET_COPY).
    expect(screen.getByText(/先添加一篇信源，再来体检/)).toBeInTheDocument();
    expect(screen.queryByText(/评估过之后/)).not.toBeInTheDocument();
  });

  it("shows the S4-specific copy for the same not-possible-yet note", () => {
    render(<SpotCheckPanel title="论证体检" data={empty} onOrder={() => {}} pending={false} />);
    expect(screen.getByText(/先在上面写下至少一个论证位置，再来体检/)).toBeInTheDocument();
  });

  it("does NOT show the not-possible-yet note once the station IS orderable, even with no items yet", () => {
    render(<SpotCheckPanel title="信源体检" data={orderableEmpty} onOrder={() => {}} pending={false} />);
    expect(screen.getByRole("button", { name: /信源体检/ })).not.toBeDisabled();
    expect(screen.queryByText(/先添加一篇信源/)).not.toBeInTheDocument();
    expect(screen.queryByText(/还没有新的变化/)).not.toBeInTheDocument();
  });

  it("does NOT show the not-possible-yet note while an order is pending, even with no items yet", () => {
    render(<SpotCheckPanel title="信源体检" data={empty} onOrder={() => {}} pending={true} />);
    expect(screen.queryByText(/先添加一篇信源/)).not.toBeInTheDocument();
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
