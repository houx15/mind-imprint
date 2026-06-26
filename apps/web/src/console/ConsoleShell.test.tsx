import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleShell } from "./ConsoleShell";
import { createSession } from "../shell/session";
import type { ClassDetail, ClassSummary, MeUser } from "../api";

const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };
const TEACHER: MeUser = { id: "u1", email: "t@d", display_name: "Teacher", role: "teacher", avatar_color: "#2A3B7A", school: { id: "s1", name: "Demo" }, classes: [] };

const summary: ClassSummary = { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" };
const detail: ClassDetail = { class: summary, roster: [] };

function client() {
  return {
    listClasses: vi.fn(async () => [summary]),
    createClass: vi.fn(),
    getClass: vi.fn(async () => detail),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
  };
}

function mount() {
  const session = createSession({ storage: mem() });
  session.setUser(TEACHER);
  const onLogout = vi.fn();
  render(<ConsoleShell session={session} client={client()} onLogout={onLogout} />);
  return { onLogout };
}

describe("ConsoleShell", () => {
  it("lands on the classes list", async () => {
    mount();
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
  });

  it("opens a class detail when a card is clicked, and 返回 goes back", async () => {
    mount();
    await userEvent.click(await screen.findByText("11A"));
    // The fixture roster is empty, so the detail shows the empty-roster note.
    expect(await screen.findByText(/还没有学生加入/)).toBeInTheDocument();
    await userEvent.click(screen.getByText("← 返回"));
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
  });

  it("switches to the 设置 tab and can log out", async () => {
    const { onLogout } = mount();
    await userEvent.click(screen.getByText("设置"));
    expect(await screen.findByText("退出登录")).toBeInTheDocument();
    await userEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalled();
  });

  it("absent role defaults to admin (read-only): no create button, 全校班级 visible", async () => {
    const noRoleUser = { ...TEACHER, role: undefined } as unknown as MeUser;
    const session = createSession({ storage: mem() });
    session.setUser(noRoleUser);
    render(<ConsoleShell session={session} client={client()} onLogout={vi.fn()} />);
    expect(await screen.findByText("全校班级")).toBeInTheDocument();
    expect(screen.queryByText("+ 新建班级")).not.toBeInTheDocument();
  });
});
