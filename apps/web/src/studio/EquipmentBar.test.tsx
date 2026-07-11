import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EquipmentBar } from "./EquipmentBar";
import { STUDIO_FIXTURE } from "./fixtures";

const CARDS = STUDIO_FIXTURE.coach.equipment;

describe("EquipmentBar (装备栏)", () => {
  it("hides the chip popover when closed", () => {
    render(<EquipmentBar cards={CARDS} open={false} onToggle={() => {}} onOpen={() => {}} />);
    expect(screen.queryByText("钢人卡")).not.toBeInTheDocument();
  });

  it("shows chips with 自发/提示后 badges when open, and clicking a chip fires onOpen(id)", () => {
    const onOpen = vi.fn();
    render(<EquipmentBar cards={CARDS} open={true} onToggle={() => {}} onOpen={onOpen} />);
    expect(screen.getByText("钢人卡")).toBeInTheDocument();
    expect(screen.getAllByText("自发").length).toBeGreaterThan(0);
    expect(screen.getAllByText("提示后").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByText("钢人卡"));
    expect(onOpen).toHaveBeenCalledWith("eq-steelman");
  });

  it("the toolbox toggle button fires onToggle", () => {
    const onToggle = vi.fn();
    render(<EquipmentBar cards={CARDS} open={false} onToggle={onToggle} onOpen={() => {}} />);
    fireEvent.click(screen.getByTitle("装备栏 · 工具卡"));
    expect(onToggle).toHaveBeenCalled();
  });
});
