import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StructureView } from "@/studio/views/StructureView";
import { STUDIO_FIXTURE } from "@/studio/fixtures";

describe("StructureView (结构/S4 shell)", () => {
  it("renders the section badge and the 5 Toulmin role cards", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/论证构建/)).toBeInTheDocument();
    for (const c of STUDIO_FIXTURE.views.structure) {
      expect(screen.getByText(c.role)).toBeInTheDocument();
    }
  });

  it("shows the orange not-clean banner when a card is empty, without claiming the gate would pass", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/还有卡片没完成/)).toBeInTheDocument();
    expect(screen.queryByText(/本环节门禁就过了/)).toBeNull();
  });

  // I1 fix: the five-slot banner must never claim the station's gate passed
  // or that 成稿打磨 (S5) is reachable — build_argument's gate also requires
  // the human item `warrant_quality_spot_check`, whose producer (论证体检) is
  // a separate panel this banner knows nothing about. Pin both what the
  // banner DOES say and what it must NOT say.
  it("shows the green five-slots-done banner, without claiming the gate passed", () => {
    const allDone = STUDIO_FIXTURE.views.structure.map((c) => ({
      ...c,
      status: "done" as const,
      preview: c.preview ?? "占位句子",
    }));
    render(<StructureView cards={allDone} />);
    expect(screen.getByText(/五张卡片都写成了句子、该接素材的都接上了/)).toBeInTheDocument();
    expect(screen.queryByText(/门禁通过/)).toBeNull();
    expect(screen.queryByText(/可以进成稿打磨/)).toBeNull();
  });

  it("shows the collapsed preview sentence for done cards", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    const doneCard = STUDIO_FIXTURE.views.structure.find((c) => c.status === "done")!;
    expect(screen.getByText(doneCard.preview!)).toBeInTheDocument();
  });

  it("renders the deferred shell placeholder (never a false green gate) when cards is empty", () => {
    render(<StructureView cards={[]} />);
    expect(screen.getByText(/在这里把论证一步步搭成结构/)).toBeInTheDocument();
    expect(screen.queryByText(/门禁通过/)).toBeNull();
    expect(screen.queryByText(/可以进成稿打磨/)).toBeNull();
  });
});
