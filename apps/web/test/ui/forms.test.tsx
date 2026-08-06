import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Input,
  Textarea,
  Select,
  Toggle,
  Radio,
  Checkbox,
  Chip,
  Rating,
} from "@/ui/forms";

describe("Input", () => {
  it("fires onChange with the typed value", async () => {
    const onChange = vi.fn();
    render(<Input value="" onChange={onChange} placeholder="姓名" />);
    await userEvent.type(screen.getByPlaceholderText("姓名"), "小明");
    expect(onChange).toHaveBeenCalled();
    // last call should reflect the final character typed
    expect(onChange.mock.calls.at(-1)?.[0]).toBe("明");
  });

  it("renders the error message and applies danger styling when error is set", () => {
    render(<Input value="" onChange={() => {}} error="必填项" />);
    expect(screen.getByText("必填项")).toBeInTheDocument();
    const input = screen.getByRole("textbox");
    expect(input.className).toContain("border-mk-danger");
    expect(input.className).toContain("bg-mk-danger-bg");
    expect(input.className).not.toContain("border-mk-input-border");
    expect(input).toHaveAttribute("aria-invalid", "true");
  });

  it("has no error line when error is absent", () => {
    render(<Input value="" onChange={() => {}} />);
    expect(screen.getByRole("textbox").className).toContain("border-mk-input-border");
  });
});

describe("Textarea", () => {
  it("fires onChange with typed value and shows error state", async () => {
    const onChange = vi.fn();
    render(<Textarea value="" onChange={onChange} error="太短了" />);
    expect(screen.getByText("太短了")).toBeInTheDocument();
    await userEvent.type(screen.getByRole("textbox"), "x");
    expect(onChange).toHaveBeenCalledWith("x");
  });
});

describe("Select", () => {
  it("fires onChange with the selected option value", async () => {
    const onChange = vi.fn();
    render(
      <Select
        value="a"
        onChange={onChange}
        options={[
          { value: "a", label: "选项 A" },
          { value: "b", label: "选项 B" },
        ]}
      />,
    );
    await userEvent.selectOptions(screen.getByRole("combobox"), "b");
    expect(onChange).toHaveBeenCalledWith("b");
  });
});

describe("Toggle", () => {
  it("renders role=switch with aria-checked reflecting state, and flips + calls onChange on click", async () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} onChange={onChange} label="通知" />);
    const el = screen.getByRole("switch");
    expect(el).toHaveAttribute("aria-checked", "false");
    await userEvent.click(el);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("reflects checked=true via aria-checked", () => {
    render(<Toggle checked onChange={() => {}} label="通知" />);
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
  });
});

describe("Radio", () => {
  it("reflects the selected option as checked and calls onChange on selection", async () => {
    const onChange = vi.fn();
    render(
      <Radio
        name="choice"
        value="a"
        onChange={onChange}
        options={[
          { value: "a", label: "选项 A" },
          { value: "b", label: "选项 B" },
        ]}
      />,
    );
    const inputs = screen.getAllByRole("radio") as HTMLInputElement[];
    expect(inputs[0]?.checked).toBe(true);
    expect(inputs[1]?.checked).toBe(false);
    await userEvent.click(screen.getByText("选项 B"));
    expect(onChange).toHaveBeenCalledWith("b");
  });
});

describe("Checkbox", () => {
  it("reflects checked state and calls onChange when toggled", async () => {
    const onChange = vi.fn();
    render(<Checkbox checked={false} onChange={onChange} label="同意条款" />);
    const input = screen.getByRole("checkbox") as HTMLInputElement;
    expect(input.checked).toBe(false);
    await userEvent.click(screen.getByText("同意条款"));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("renders checked=true", () => {
    render(<Checkbox checked onChange={() => {}} label="同意条款" />);
    expect((screen.getByRole("checkbox") as HTMLInputElement).checked).toBe(true);
  });
});

describe("Chip", () => {
  it("toggles aria-pressed and calls onChange on click", async () => {
    const onChange = vi.fn();
    render(<Chip label="CRAAP" selected={false} onChange={onChange} />);
    const chip = screen.getByRole("button", { name: "CRAAP" });
    expect(chip).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(chip);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("applies selected styling (bg-mk-accent-50 + text-mk-accent-700) when selected", () => {
    render(<Chip label="CRAAP" selected onChange={() => {}} />);
    const chip = screen.getByRole("button", { name: "CRAAP" });
    expect(chip).toHaveAttribute("aria-pressed", "true");
    expect(chip.className).toContain("bg-mk-accent-50");
    expect(chip.className).toContain("text-mk-accent-700");
  });
});

describe("Rating", () => {
  it("clicking the 3rd star calls onChange(3)", async () => {
    const onChange = vi.fn();
    render(<Rating value={0} onChange={onChange} />);
    const stars = screen.getAllByRole("radio");
    expect(stars).toHaveLength(5);
    await userEvent.click(stars[2]!);
    expect(onChange).toHaveBeenCalledWith(3);
  });

  it("fills stars up to value with the warning-gold text color, not accent", () => {
    render(<Rating value={3} onChange={() => {}} />);
    const stars = screen.getAllByRole("radio");
    stars.forEach((star) => {
      expect(star.className).toContain("text-mk-warning");
      expect(star.className).not.toContain("text-mk-accent");
    });
    expect(stars[2]).toHaveAttribute("aria-checked", "true");
    expect(stars[3]).toHaveAttribute("aria-checked", "false");
  });
});
