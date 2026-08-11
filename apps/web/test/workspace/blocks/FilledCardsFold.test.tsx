import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FilledCardsFold } from "../../../src/workspace/blocks/FilledCardsFold";

const cards = [
  { key: "p1", title: "研究背景", text: "我原来写的背景" },
  { key: "p2", title: "研究问题", text: "我原来写的问题" },
];

describe("FilledCardsFold", () => {
  it("renders read-only text when onEdit is absent", () => {
    render(<FilledCardsFold cards={cards} />);
    fireEvent.click(screen.getByRole("button", { name: /研究背景/ }));
    // Expanded content is a paragraph, not an editable field.
    expect(screen.getByText("我原来写的背景")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("lets the student edit a finished part in place when onEdit is provided", () => {
    const onEdit = vi.fn();
    render(<FilledCardsFold cards={cards} onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: /研究背景/ }));
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(ta.value).toBe("我原来写的背景");
    fireEvent.change(ta, { target: { value: "改过的背景" } });
    expect(onEdit).toHaveBeenCalledWith("p1", "改过的背景");
  });

  it("renders nothing when there are no finished parts", () => {
    const { container } = render(<FilledCardsFold cards={[]} onEdit={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it("re-shows the original guidance + example when a finished part carries its guide card", () => {
    const onEdit = vi.fn();
    const withGuide = [
      { key: "p1", title: "研究背景", text: "我原来写的背景", guidance: "写清楚这项研究的背景", example: "An English example" },
    ];
    render(<FilledCardsFold cards={withGuide} onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: /研究背景/ }));
    // The expanded view is the guided-writing surface: guidance + example + the
    // student's own text, editable — not a bare slab.
    expect(screen.getByText("写清楚这项研究的背景")).toBeInTheDocument();
    expect(screen.getByText("An English example")).toBeInTheDocument();
    const ta = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(ta.value).toBe("我原来写的背景");
    fireEvent.change(ta, { target: { value: "改过的背景" } });
    expect(onEdit).toHaveBeenCalledWith("p1", "改过的背景");
    // A finished part is re-read/re-edit only — no 我写好了 / 我依然有问题 here.
    expect(screen.queryByText("我写好了")).toBeNull();
    expect(screen.queryByText("我依然有问题")).toBeNull();
  });
});
