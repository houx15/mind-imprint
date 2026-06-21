import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SpectrumField } from "./SpectrumField";
import { fieldRegistry } from "../fieldRegistry";

const field = { type: "spectrum" as const, key: "pos", label: "确定度", stops: ["个人猜测", "有据推断", "强证据", "科学共识", "逻辑必然"] };

describe("SpectrumField", () => {
  it("renders one radio per stop with its label", () => {
    render(<SpectrumField field={field} value={undefined} onChange={vi.fn()} />);
    const stops = screen.getAllByRole("radio");
    expect(stops).toHaveLength(5);
    expect(screen.getByText("科学共识")).toBeTruthy();
  });
  it("clicking a stop reports its index", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={undefined} onChange={onChange} />);
    await userEvent.click(screen.getAllByRole("radio")[2]!);
    expect(onChange).toHaveBeenCalledWith(2);
  });
  it("marks the current value as checked", () => {
    render(<SpectrumField field={field} value={3} onChange={vi.fn()} />);
    expect(screen.getAllByRole("radio")[3]!.getAttribute("aria-checked")).toBe("true");
  });
  it("ArrowRight moves the marker forward and clamps at the last stop", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={4} onChange={onChange} />);
    screen.getByRole("radiogroup").focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith(4); // clamped at max
  });
  it("ArrowLeft from index 2 reports 1", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={2} onChange={onChange} />);
    screen.getByRole("radiogroup").focus();
    await userEvent.keyboard("{ArrowLeft}");
    expect(onChange).toHaveBeenLastCalledWith(1);
  });
  it("is registered under the `spectrum` key", () => {
    expect(fieldRegistry.spectrum).toBe(SpectrumField);
  });
});
