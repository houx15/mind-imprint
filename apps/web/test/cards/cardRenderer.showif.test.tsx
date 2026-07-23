import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { CardRenderer } from "@/cards/CardRenderer";
import type { CardSpec } from "@mind-imprint/contracts";

const card = {
  id: "t", category: "c", name: "n", purpose: "p", trigger_condition: "tc",
  rubric_tags: [], body_status: "full",
  steps: [{ key: "s", title: "S", disclose: "always",
    methodology: { why: "w", how: "h", when: "n" },
    fields: [
      { type: "single_choice", key: "subject", label: "学科", options: ["数学", "艺术"] },
      { type: "textarea", key: "proof", label: "数学证明", show_if: { key: "subject", equals: "数学" } },
    ] }],
} as unknown as CardSpec;

describe("CardRenderer show_if", () => {
  it("hides a show_if field until its condition is met", () => {
    const { rerender } = render(<CardRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.queryByText("数学证明")).toBeNull();
    rerender(<CardRenderer card={card} values={{ subject: "数学" }} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByText("数学证明")).toBeTruthy();
  });
  it("keeps the field hidden when the value does not match", () => {
    render(<CardRenderer card={card} values={{ subject: "艺术" }} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.queryByText("数学证明")).toBeNull();
  });
});
