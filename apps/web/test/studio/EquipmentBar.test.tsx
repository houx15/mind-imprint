import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EquipmentBar } from "@/studio/EquipmentBar";
import { STUDIO_FIXTURE } from "@/studio/fixtures";

const CARDS = STUDIO_FIXTURE.coach.equipment;

describe("EquipmentBar (装备栏)", () => {
  it("hides the chip popover when closed", () => {
    render(<EquipmentBar cards={CARDS} open={false} onToggle={() => {}} onOpen={() => {}} />);
    expect(screen.queryByText("钢人卡")).not.toBeInTheDocument();
  });

  it("shows chips with 自发/提示后 badges when open, and clicking a chip fires onOpen(meth)", () => {
    const onOpen = vi.fn();
    render(<EquipmentBar cards={CARDS} open={true} onToggle={() => {}} onOpen={onOpen} />);
    expect(screen.getByText("钢人卡")).toBeInTheDocument();
    expect(screen.getAllByText("自发").length).toBeGreaterThan(0);
    expect(screen.getAllByText("提示后").length).toBeGreaterThan(0);

    // 钢人卡's meth key is "concession" (design ~L2287-2294), not its card id —
    // this is what actually reaches MethodologyModal.
    fireEvent.click(screen.getByText("钢人卡"));
    expect(onOpen).toHaveBeenCalledWith("concession");
  });

  it("renders nothing when closed (the trigger lives in the composer, not here)", () => {
    const { container } = render(<EquipmentBar cards={CARDS} open={false} onToggle={() => {}} onOpen={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("the panel's collapse chevron fires onToggle", () => {
    const onToggle = vi.fn();
    render(<EquipmentBar cards={CARDS} open={true} onToggle={onToggle} onOpen={() => {}} />);
    fireEvent.click(screen.getByTitle("收起装备栏"));
    expect(onToggle).toHaveBeenCalled();
  });
});
