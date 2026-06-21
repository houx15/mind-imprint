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
  it("fails verification when the model connects but returns no content", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "   " });
    const onVerified = vi.fn();
    render(<LlmConfigForm chat={chat} onVerified={onVerified} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(screen.getByText(/连接失败/)).toBeTruthy());
    expect(onVerified).not.toHaveBeenCalled();
    expect(localStorage.getItem("mk.llmConfig")).not.toContain("\"verified\":true");
  });
  it("passes verification when the model replies only with a tool call", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "", toolCalls: [{ id: "t1", name: "x", args: {} }] });
    const onVerified = vi.fn();
    render(<LlmConfigForm chat={chat} onVerified={onVerified} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(onVerified).toHaveBeenCalledOnce());
  });
  it("rejects an incomplete config", () => {
    render(<LlmConfigForm chat={vi.fn()} onVerified={vi.fn()} />);
    // Explicitly empty the fields — the form pre-fills from env defaults
    // (e.g. a developer's .env.local), so don't rely on them being blank.
    fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("模型"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    expect(screen.getByText(/请先填写完整配置/)).toBeTruthy();
  });
});
