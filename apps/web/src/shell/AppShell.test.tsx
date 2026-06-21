import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore, makeMemoryStorage } from "../store";
import { createSession } from "./session";

beforeEach(() => localStorage.clear());

function deps() {
  const storage = makeMemoryStorage();
  return {
    store: createStore({ storage }),
    session: createSession({ storage: makeMemoryStorage() }),
    chat: vi.fn(),
  };
}

describe("AppShell", () => {
  it("shows auth when logged out, app after 登录", () => {
    const d = deps();
    // a verified config so the key-gate doesn't block
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: "openai", baseUrl: "b", model: "m", apiKey: "k", verified: true }));
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(screen.getByText("你想搞懂什么？")).toBeTruthy();
  });
  it("blocks the app with the key-gate when no verified config", () => {
    const d = deps();
    d.session.setAuthed(true);
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    expect(screen.getByText(/先连接你的模型/)).toBeTruthy();
  });
  it("navigates tasks → records via the rail", () => {
    const d = deps();
    d.session.setAuthed(true);
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: "openai", baseUrl: "b", model: "m", apiKey: "k", verified: true }));
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    fireEvent.click(screen.getByText("记录"));
    expect(screen.getByText("活跃日历")).toBeTruthy();
  });
});
