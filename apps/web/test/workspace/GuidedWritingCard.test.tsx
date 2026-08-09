import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { GuidedWritingCard } from "@/workspace/blocks/GuidedWritingCard";

describe("GuidedWritingCard", () => {
  it("renders guidance + example, edits, and fires the two buttons", () => {
    const onChange = vi.fn();
    const onStillStuck = vi.fn();
    const onDone = vi.fn();
    render(
      <GuidedWritingCard
        title="论点 1"
        guidance="写这条论点的论述段落"
        example="An English example"
        value=""
        onChange={onChange}
        onStillStuck={onStillStuck}
        onDone={onDone}
      />,
    );
    expect(screen.getByText("写这条论点的论述段落")).toBeTruthy();
    expect(screen.getByText("An English example")).toBeTruthy();

    const ta = screen.getByPlaceholderText("在这里写……");
    fireEvent.change(ta, { target: { value: "证据表明……" } });
    expect(onChange).toHaveBeenCalledWith("证据表明……");

    fireEvent.click(screen.getByText("我依然有问题"));
    expect(onStillStuck).toHaveBeenCalled();
    fireEvent.click(screen.getByText("我写好了"));
    expect(onDone).toHaveBeenCalled();
  });

  it("renders the 写作卡 offer and fires onOfferCard", () => {
    const onOfferCard = vi.fn();
    render(
      <GuidedWritingCard
        guidance="g"
        value=""
        onChange={vi.fn()}
        onStillStuck={vi.fn()}
        onDone={vi.fn()}
        cardOffer={[{ id: "pee", label: "PEE 写作卡" }, { id: "toulmin", label: "论证构建卡" }]}
        onOfferCard={onOfferCard}
      />,
    );
    fireEvent.click(screen.getByText("PEE 写作卡"));
    expect(onOfferCard).toHaveBeenCalledWith("pee");
  });

  it("auto-grows the textarea (no fixed row cap; overflow hidden)", () => {
    render(<GuidedWritingCard guidance="g" value="x" onChange={vi.fn()} onStillStuck={vi.fn()} onDone={vi.fn()} />);
    const ta = screen.getByPlaceholderText("在这里写……") as HTMLTextAreaElement;
    expect(ta.className).toContain("overflow-hidden");
    expect(ta.className).toContain("resize-none");
  });
});
