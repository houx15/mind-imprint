import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { PerspectivesView } from "./PerspectivesView";
import type { PerspectivesFx } from "../state";
import type { MaterialSource } from "@mind-imprint/contracts";

// Hand-built PerspectivesFx (rather than STUDIO_FIXTURE, whose perspectives
// panel is already mid-project) so these assertions are independent of
// fixture drift — mirrors FramingView.test.tsx's makeData() convention.
function makeData(overrides: Partial<PerspectivesFx> = {}): PerspectivesFx {
  return {
    rows: [],
    sourcesPerPerspective: false,
    ...overrides,
  };
}

function makeMaterial(n: number): MaterialSource[] {
  return Array.from({ length: n }, (_, i) => ({
    id: `src-${i}`,
    title: `素材 ${i}`,
    sourceUrl: "https://example.com",
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
  }));
}

describe("PerspectivesView (S2 视角与素材)", () => {
  it("shows the coverage chip only once both layers are covered", () => {
    const nationalOnly = render(
      <PerspectivesView data={makeData({ rows: [{ text: "国家立场", level: "national", editable: true }] })} material={[]} />,
    );
    expect(nationalOnly.queryByText("✓ 已覆盖 本地/国家 + 全球")).not.toBeInTheDocument();
    nationalOnly.unmount();

    const bothLayers = render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "国家立场", level: "national", editable: true },
            { text: "全球支持", level: "global_for", editable: true },
          ],
        })}
        material={[]}
      />,
    );
    expect(bothLayers.getByText("✓ 已覆盖 本地/国家 + 全球")).toBeInTheDocument();
  });

  // Card-minted rows are hers to keep, not to edit here.
  it("renders a card-minted row read-only with no level chips, and excludes it from the submit body", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "来自 perspective-matrix 卡片的一条视角", level: "", editable: false },
            { text: "我自己写的视角", level: "national", editable: true },
          ],
        })}
        material={[]}
        onSubmit={onSubmit}
      />,
    );

    expect(screen.getByText("来自工具卡")).toBeInTheDocument();
    expect(screen.getByText("来自 perspective-matrix 卡片的一条视角")).toBeInTheDocument();
    // No level chips for the card-minted row: only the ONE editable row's
    // textarea should be an editable field, and none of the three level
    // labels should be clickable buttons attached to the card-minted row.
    expect(screen.queryAllByLabelText("视角")).toHaveLength(1);
    // The card-minted row's text must not sit inside an editable textarea.
    expect(screen.queryByDisplayValue("来自 perspective-matrix 卡片的一条视角")).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "记下我的视角" }));
    });
    expect(onSubmit).toHaveBeenCalledWith({
      perspectives: [{ text: "我自己写的视角", level: "national" }],
    });
  });

  it("disables the sources confirm with a reason until 2 perspectives and 1 source exist", () => {
    const zeroPerspectives = render(<PerspectivesView data={makeData()} material={[]} />);
    expect(zeroPerspectives.getByRole("checkbox")).toBeDisabled();
    expect(zeroPerspectives.getByText("至少写两条视角后才能确认")).toBeInTheDocument();
    zeroPerspectives.unmount();

    const twoPerspectivesNoSource = render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "a", level: "national", editable: true },
            { text: "b", level: "global_for", editable: true },
          ],
        })}
        material={[]}
      />,
    );
    expect(twoPerspectivesNoSource.getByRole("checkbox")).toBeDisabled();
    expect(twoPerspectivesNoSource.getByText("至少添加一条素材后才能确认")).toBeInTheDocument();
    twoPerspectivesNoSource.unmount();

    const bothSatisfied = render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "a", level: "national", editable: true },
            { text: "b", level: "global_for", editable: true },
          ],
        })}
        material={makeMaterial(1)}
      />,
    );
    expect(bothSatisfied.getByRole("checkbox")).toBeEnabled();
    expect(bothSatisfied.queryByText("至少添加一条素材后才能确认")).not.toBeInTheDocument();
    bothSatisfied.unmount();
  });

  it("is reversible — unchecking the confirm clears it", () => {
    const onAttest = vi.fn();
    render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "a", level: "national", editable: true },
            { text: "b", level: "global_for", editable: true },
          ],
          sourcesPerPerspective: true,
        })}
        material={makeMaterial(1)}
        onAttestSourcesPerPerspective={onAttest}
      />,
    );
    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.checked).toBe(true);
    fireEvent.click(checkbox);
    expect(checkbox.checked).toBe(false);
    expect(onAttest).toHaveBeenCalledWith(false);
  });

  it("lets her add, level, and remove a perspective", () => {
    render(<PerspectivesView data={makeData()} material={[]} />);
    expect(screen.queryAllByLabelText("视角")).toHaveLength(0);

    fireEvent.click(screen.getByText("添加一条视角"));
    const textarea = screen.getByLabelText("视角") as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "我的第一条视角" } });
    expect(textarea.value).toBe("我的第一条视角");

    fireEvent.click(screen.getByText("全球视角 · 反方"));
    // level chip picked — re-clicking remove clears the row entirely.
    fireEvent.click(screen.getByLabelText("删除这条视角"));
    expect(screen.queryAllByLabelText("视角")).toHaveLength(0);
  });

  it("saves partial work — a single row submits fine, and never disables save for incompleteness", async () => {
    const data = makeData({ rows: [{ text: "只写了一半", level: "national", editable: true }] });
    let resolveSubmit: () => void = () => {};
    const onSubmit = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve;
        }),
    );
    render(<PerspectivesView data={data} material={[]} onSubmit={onSubmit} />);
    const button = screen.getByRole("button", { name: "记下我的视角" });
    expect(button).toBeEnabled();

    fireEvent.click(button);
    expect(await screen.findByRole("button", { name: "记录中…" })).toBeDisabled();

    await act(async () => {
      resolveSubmit();
    });
    expect(onSubmit).toHaveBeenCalledWith({ perspectives: [{ text: "只写了一半", level: "national" }] });
    expect(screen.getByRole("button", { name: "记下我的视角" })).toBeEnabled();
  });

  // I4 (whole-branch review IMPORTANT): the server silently drops any row
  // whose trimmed `text` is blank. Before this fix, that row stayed on
  // screen looking saved, with no record it was ever dropped.
  it("drops a blank-text row from the screen after a successful save", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <PerspectivesView
        data={makeData({
          rows: [
            { text: "", level: "global_for", editable: true },
            { text: "我写完整的视角", level: "national", editable: true },
          ],
        })}
        material={[]}
        onSubmit={onSubmit}
      />,
    );
    expect(screen.queryAllByLabelText("视角")).toHaveLength(2);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "记下我的视角" }));
    });

    expect(screen.queryAllByLabelText("视角")).toHaveLength(1);
    expect(screen.getByDisplayValue("我写完整的视角")).toBeInTheDocument();
  });

  it("renders the project's material titles below the perspective list", () => {
    render(<PerspectivesView data={makeData()} material={makeMaterial(2)} />);
    expect(screen.getByText("素材 0")).toBeInTheDocument();
    expect(screen.getByText("素材 1")).toBeInTheDocument();
  });

  // C1 (whole-branch review CRITICAL): the station's own gate item
  // recon_logged is only ever attested by opening-then-closing a source
  // (attestReconLogged, fired from logSourceOpen) — adding a source is not
  // opening one. This asserts the real dossier (not an inert <li> list) is
  // what renders here, and that opening and closing a source from S2 itself
  // invokes onOpenLogged exactly like it does at S3.
  it("opens a source from within S2 and reports it via onOpenLogged on close", () => {
    vi.useFakeTimers();
    const onOpenLogged = vi.fn();
    render(<PerspectivesView data={makeData()} material={makeMaterial(1)} onOpenLogged={onOpenLogged} />);

    fireEvent.click(screen.getByText("素材 0"));
    vi.advanceTimersByTime(12_000);
    fireEvent.click(screen.getByText("返回信源列表"));

    expect(onOpenLogged).toHaveBeenCalledWith("src-0", 12);
    vi.useRealTimers();
  });
});
