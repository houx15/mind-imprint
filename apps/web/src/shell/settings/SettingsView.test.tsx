import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SettingsView } from "./SettingsView";
import { createSession } from "../session";
import { makeMemoryStorage } from "../../store";

beforeEach(() => localStorage.clear());

const ME = { id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student", avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [] };

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
  it("shows real display_name and email when user prop is provided", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} onLogout={vi.fn()} user={ME} />);
    expect(screen.getAllByDisplayValue("Phoebe").length).toBeGreaterThan(0);
    expect(screen.getAllByDisplayValue("p@d.local").length).toBeGreaterThan(0);
  });
  it("falls back to placeholder strings when user is null", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} onLogout={vi.fn()} user={null} />);
    expect(screen.getAllByDisplayValue("Phoebe Chen").length).toBeGreaterThan(0);
    expect(screen.getAllByDisplayValue("phoebe@ibschool.edu").length).toBeGreaterThan(0);
  });
});
