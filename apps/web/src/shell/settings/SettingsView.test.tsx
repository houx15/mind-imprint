import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SettingsView } from "./SettingsView";
import { createSession } from "../session";
import { makeMemoryStorage } from "../../store";

beforeEach(() => localStorage.clear());

describe("SettingsView", () => {
  it("persists a chosen avatar color to the session", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} onLogout={vi.fn()} />);
    // click the 2nd avatar option (#D98263)
    const swatches = screen.getAllByTestId("avatar-option");
    fireEvent.click(swatches[1]!);
    expect(session.getSnapshot().aiAvatar).toBe("#D98263");
  });
  it("fires onLogout from 退出登录", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    const onLogout = vi.fn();
    render(<SettingsView session={session} onLogout={onLogout} />);
    fireEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalledOnce();
  });
  it("renders profile and AI avatar sections without LLM config", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} onLogout={vi.fn()} />);
    expect(screen.getByText("个人")).toBeInTheDocument();
    expect(screen.getByText("AI 形象")).toBeInTheDocument();
    expect(screen.queryByText("模型 / API")).toBeNull();
  });
});
