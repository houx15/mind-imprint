import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Compare } from "@/primitives/compare/Compare";

const left = { material_id: "mat-blog", spans: [{ id: "s1", block_ref: "b1", tag: "claim", note: "", author: "ai" as const }] };

describe("Compare", () => {
  it("renders the empty right pane as an assignment, not a spinner", () => {
    render(<Compare state={{ left, right: null, pairs: [] }} onAddLateralSource={vi.fn()} />);
    expect(screen.getByText("去找一个独立的来源")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("invites the student to add the source — it never fetches one itself", async () => {
    const onAdd = vi.fn();
    render(<Compare state={{ left, right: null, pairs: [] }} onAddLateralSource={onAdd} />);
    await userEvent.click(screen.getByRole("button", { name: "添加信源" }));
    expect(onAdd).toHaveBeenCalledTimes(1);
  });

  it("renders both materials once a lateral source exists", () => {
    const right = { material_id: "mat-nasa", spans: [{ id: "s2", block_ref: "b9", tag: "find", note: "", author: "student" as const }] };
    render(<Compare state={{ left, right, pairs: [] }} onAddLateralSource={vi.fn()} />);
    expect(screen.getAllByTestId("compare-pane")).toHaveLength(2);
  });
});
