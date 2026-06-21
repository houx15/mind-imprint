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

  it("switches to the workspace tab and renders the workspace shell", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "工作区" }));
    // The workspace shell renders either the breadcrumb back button or the composer placeholder
    const backButton = screen.queryByRole("button", { name: "返回所有任务" });
    const composerPlaceholder = screen.queryByPlaceholderText("把你的想法发给陪练……");
    expect(backButton ?? composerPlaceholder).not.toBeNull();
  });
});
