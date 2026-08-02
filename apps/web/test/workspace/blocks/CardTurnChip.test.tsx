import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { CardTurnChip } from "@/workspace/blocks/CardTurnChip";

// The card ref a completed PEE 写作卡 leaves on its coach-thread turn.
const card = {
  cardId: "pee",
  fieldValues: {
    thesis: "中国的发展总体上让地球更可持续",
    point: "中国是全球最大的可再生能源投资国",
    evidence: "IEA 2023 报告：中国占全球光伏新增装机的一半以上",
  },
};

describe("CardTurnChip", () => {
  it("renders content-first: leads with the student's own answer, not raw compiled text", () => {
    render(<CardTurnChip card={card} />);
    const chip = screen.getByRole("button", { name: /点开看你填的/ });
    // The student's first filled field (her content) leads the chip.
    expect(within(chip).getByText(/中国的发展总体上让地球更可持续/)).toBeInTheDocument();
    // The card name is present, but only as a secondary provenance label.
    expect(within(chip).getByText(/PEE 写作卡/)).toBeInTheDocument();
    // It must NOT render the plain compiled "我刚填完《…》" dump as raw text.
    expect(screen.queryByText(/我刚填完/)).not.toBeInTheDocument();
  });

  it("clicking opens a READ-ONLY view showing all the student's answers", () => {
    render(<CardTurnChip card={card} />);
    fireEvent.click(screen.getByRole("button", { name: /点开看你填的/ }));

    const dialog = screen.getByRole("dialog");
    // Field labels + the student's full answers are shown (the label appears in
    // several places — step title, option — so assert it's present, not unique).
    expect(within(dialog).getAllByText(/开篇立场/).length).toBeGreaterThan(0);
    expect(within(dialog).getByText(/中国是全球最大的可再生能源投资国/)).toBeInTheDocument();
    expect(within(dialog).getByText(/IEA 2023 报告/)).toBeInTheDocument();

    // Read-only: no editable inputs, no submit/skip (it's the record — 铁律).
    expect(within(dialog).queryByRole("textbox")).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/提交并钉到过程树/)).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/跳过这张卡/)).not.toBeInTheDocument();
  });
});
