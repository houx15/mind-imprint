import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TextAreaField } from "./TextAreaField";
import { SingleChoiceField } from "./SingleChoiceField";
import { MultiChoiceField } from "./MultiChoiceField";
import { RatingField } from "./RatingField";

describe("simple fields", () => {
  it("textarea calls onChange with typed text", async () => {
    const onChange = vi.fn();
    render(<TextAreaField field={{ type: "textarea", key: "stop", label: "Stop" }} value="" onChange={onChange} />);
    await userEvent.type(screen.getByLabelText("Stop"), "x");
    expect(onChange).toHaveBeenLastCalledWith("x");
  });
  it("single_choice renders one button per option and reports the chosen value", async () => {
    const onChange = vi.fn();
    render(<SingleChoiceField field={{ type: "single_choice", key: "v", label: "可信？", options: ["可信", "存疑"] }} value={undefined} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "存疑" }));
    expect(onChange).toHaveBeenCalledWith("存疑");
  });
  it("rating renders `scale` segments and reports the picked number", async () => {
    const onChange = vi.fn();
    render(<RatingField field={{ type: "rating", key: "c", label: "Currency", scale: 5 }} value={0} onChange={onChange} />);
    const segs = screen.getAllByRole("radio");
    expect(segs).toHaveLength(5);
    await userEvent.click(segs[3]!);
    expect(onChange).toHaveBeenCalledWith(4);
  });
  it("multi_choice adds an option then removes it, never mutating the input array", async () => {
    const onChange = vi.fn();
    const initial: string[] = [];
    const field = { type: "multi_choice" as const, key: "k", label: "选择", options: ["A", "B"] };
    const { rerender } = render(<MultiChoiceField field={field} value={initial} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "A" }));
    expect(onChange).toHaveBeenLastCalledWith(["A"]);
    expect(initial).toEqual([]); // input array not mutated
    rerender(<MultiChoiceField field={field} value={["A"]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "A" }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });
});
