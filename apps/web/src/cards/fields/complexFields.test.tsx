import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkCheckField } from "./LinkCheckField";
import { RepeatableGroupField } from "./RepeatableGroupField";

const sourcesField = {
  type: "repeatable_group" as const, key: "sources", label: "找出独立来源",
  item_fields: [
    { type: "text" as const, key: "name", label: "来源" },
    { type: "single_choice" as const, key: "verdict", label: "可信？", options: ["可信", "存疑"] },
  ],
};

describe("complex fields", () => {
  it("link_check shows the url and a verdict tag once a url is entered", async () => {
    const onChange = vi.fn();
    render(<LinkCheckField field={{ type: "link_check", key: "trace", label: "Trace" }} value="https://nature.com" onChange={onChange} />);
    expect(screen.getByText("https://nature.com")).toBeInTheDocument();
    expect(screen.getByText("已溯源")).toBeInTheDocument();
  });
  it("repeatable_group adds a row and emits the full array on edit", async () => {
    const onChange = vi.fn();
    render(<RepeatableGroupField field={sourcesField} value={[{}]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "+ 添加来源" }));
    expect(onChange).toHaveBeenLastCalledWith([{}, {}]);
  });
  it("repeatable_group renders item_fields per row via their components", () => {
    render(<RepeatableGroupField field={sourcesField} value={[{ name: "公众号" }]} onChange={vi.fn()} />);
    expect(screen.getByDisplayValue("公众号")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "存疑" })).toBeInTheDocument();
  });
});
