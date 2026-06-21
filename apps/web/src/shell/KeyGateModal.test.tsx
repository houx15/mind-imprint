import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { KeyGateModal } from "./KeyGateModal";

beforeEach(() => localStorage.clear());

describe("KeyGateModal", () => {
  it("has no close button and does not pass on scrim click", () => {
    const onPass = vi.fn();
    const { container } = render(<KeyGateModal chat={vi.fn()} onPass={onPass} />);
    expect(screen.queryByRole("button", { name: /关闭|×|取消/ })).toBeNull();
    fireEvent.click(container.firstChild as Element); // scrim
    expect(onPass).not.toHaveBeenCalled();
  });
  it("calls onPass after a successful connection test", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "pong" });
    const onPass = vi.fn();
    render(<KeyGateModal chat={chat} onPass={onPass} />);
    fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api/v1" } });
    fireEvent.change(screen.getByLabelText("模型"), { target: { value: "m" } });
    fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-x" } });
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(onPass).toHaveBeenCalledOnce());
  });
});
