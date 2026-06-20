import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TextAreaField } from "./TextAreaField";
import { SingleChoiceField } from "./SingleChoiceField";
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
});
