import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CardSpec } from "@mind-imprint/contracts";
import { CardRenderer } from "@/cards/CardRenderer";

const card = {
  id: "t", category: "信息素养", name: "n", purpose: "", trigger_condition: "", rubric_tags: [],
  steps: [{
    key: "s1", title: "第一步", disclose: "always",
    methodology: { why: "**因为**重要", how: "这样做", when: "卡住时", example: "比如这样" },
    fields: [{ type: "textarea", key: "x", label: "L" }],
  }],
} as unknown as CardSpec;

describe("CardRenderer methodology panel", () => {
  it("reveals why/how/when/example (markdown) and fires onNote on expand", () => {
    const onNote = vi.fn();
    render(<CardRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={onNote} />);
    // collapsed until opened
    expect(screen.queryByText("这样做")).toBeNull();
    fireEvent.click(screen.getByText("方法"));
    expect(onNote).toHaveBeenCalledWith("s1");
    expect(screen.getByText("这样做")).toBeTruthy();
    expect(screen.getByText("卡住时")).toBeTruthy();
    expect(screen.getByText("比如这样")).toBeTruthy();
    // markdown highlight renders as <strong>, not literal **
    expect(screen.getByText("因为").tagName.toLowerCase()).toBe("strong");
  });

  it("hides every 方法 panel when hideMethodology is set", () => {
    render(<CardRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} hideMethodology />);
    expect(screen.queryByRole("button", { name: "方法" })).toBeNull();
  });
});
