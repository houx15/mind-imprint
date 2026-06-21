import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "./DevApp";

describe("DevApp", () => {
  it("defaults to the cards harness and switches to LLM settings", async () => {
    render(<DevApp />);
    expect(screen.queryByRole("button", { name: "测试连接" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "LLM 设置" }));
    expect(screen.getByRole("button", { name: "测试连接" })).toBeInTheDocument();
  });

  it("switches to the store panel", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "存储" }));
    expect(screen.getByRole("button", { name: "新建任务" })).toBeInTheDocument();
  });
});
