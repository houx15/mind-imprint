import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LlmConfigForm } from "./LlmConfigForm";

beforeEach(() => localStorage.clear());

function fill() {
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api/v1" } });
  fireEvent.change(screen.getByLabelText("模型"), { target: { value: "gpt-x" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-secret" } });
}

describe("LlmConfigForm", () => {
  it("calls onVerified after a passing 测试连接 and never renders the key in errors", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "pong" });
    const onVerified = vi.fn();
    render(<LlmConfigForm chat={chat} onVerified={onVerified} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(onVerified).toHaveBeenCalledOnce());
    expect(localStorage.getItem("mk.llmConfig")).toContain("\"verified\":true");
  });
  it("shows a failure state without leaking the key", async () => {
    const chat = vi.fn().mockRejectedValue(new Error("401 unauthorized"));
    render(<LlmConfigForm chat={chat} onVerified={vi.fn()} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(screen.getByText(/连接失败/)).toBeTruthy());
    expect(document.body.textContent).not.toContain("sk-secret");
  });
  it("rejects an incomplete config", () => {
    render(<LlmConfigForm chat={vi.fn()} onVerified={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    expect(screen.getByText(/请先填写完整配置/)).toBeTruthy();
  });
});
