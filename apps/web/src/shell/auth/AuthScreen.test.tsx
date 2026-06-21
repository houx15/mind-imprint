import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AuthScreen } from "./AuthScreen";

describe("AuthScreen", () => {
  it("starts on login and enters the app on 登录", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
  it("navigates login → register → bind → enter", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByRole("button", { name: /完成/ }));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
  it("暂时跳过 enters the app", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByText("暂时跳过"));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
});
