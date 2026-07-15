import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StructureView } from "./StructureView";
import { STUDIO_FIXTURE } from "../fixtures";

describe("StructureView (结构/S4 shell)", () => {
  it("renders the gate banner and the 5 Toulmin role cards", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/门禁/)).toBeInTheDocument();
    for (const c of STUDIO_FIXTURE.views.structure) {
      expect(screen.getByText(c.role)).toBeInTheDocument();
    }
  });

  it("shows the orange not-clean banner when a card is empty", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/还有卡片没完成/)).toBeInTheDocument();
  });

  it("shows the green all-clean banner when no card is empty", () => {
    const allDone = STUDIO_FIXTURE.views.structure.map((c) => ({
      ...c,
      status: "done" as const,
      preview: c.preview ?? "占位句子",
    }));
    render(<StructureView cards={allDone} />);
    expect(screen.getByText(/本环节门禁通过/)).toBeInTheDocument();
  });

  it("shows the collapsed preview for done cards and the AI question bubble for the active card", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    const doneCard = STUDIO_FIXTURE.views.structure.find((c) => c.status === "done")!;
    const activeCard = STUDIO_FIXTURE.views.structure.find((c) => c.status === "active")!;
    expect(screen.getByText(doneCard.preview!)).toBeInTheDocument();
    expect(screen.getByText(activeCard.question!)).toBeInTheDocument();
  });

  it("shows the disabled 选择相关素材/写成句子 hint on the active card", () => {
    render(<StructureView cards={STUDIO_FIXTURE.views.structure} />);
    expect(screen.getByText(/① 选择相关素材/)).toBeInTheDocument();
    expect(screen.getByText(/② 基于素材，把这一步写成句子/)).toBeInTheDocument();
  });

  it("renders the deferred shell placeholder (never a false green gate) when cards is empty", () => {
    render(<StructureView cards={[]} />);
    expect(screen.getByText(/在这里把论证一步步搭成结构/)).toBeInTheDocument();
    expect(screen.queryByText(/门禁通过/)).toBeNull();
    expect(screen.queryByText(/可以进成稿打磨/)).toBeNull();
  });
});
