import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleShell } from "./ConsoleShell";
import { createSession } from "../shell/session";
import type { ClassDetail, ClassSummary, MeUser } from "../api";

const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };
const TEACHER: MeUser = { id: "u1", email: "t@d", display_name: "Teacher", role: "teacher", avatar_color: "#2A3B7A", school: { id: "s1", name: "Demo" }, classes: [] };

const summary: ClassSummary = { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" };
const detail: ClassDetail = { class: summary, roster: [], teachers: [] };

function client() {
  return {
    listClasses: vi.fn(async () => [summary]),
    createClass: vi.fn(),
    getClass: vi.fn(async () => detail),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
    getOverview: vi.fn(async () => ({ counts: { student: 0, teacher: 0, class: 0, task: 0, evaluation: 0, active_student: 0 }, usage_by_tier: [] })),
    listTeacherInvites: vi.fn(async () => []),
    createTeacherInvite: vi.fn(),
    adminImport: vi.fn(),
    listTeachers: vi.fn(async () => []),
    assignTeacher: vi.fn(),
    removeTeacher: vi.fn(),
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

  it("absent role defaults to admin: lands on 概览, no teacher create button on 班级 tab", async () => {
    const noRoleUser = { ...TEACHER, role: undefined } as unknown as MeUser;
    const session = createSession({ storage: mem() });
    session.setUser(noRoleUser);
    render(<ConsoleShell session={session} client={client()} onLogout={vi.fn()} />);
    // Admin (default) lands on 概览 — multiple matches expected (nav label + page heading)
    await screen.findAllByText("概览");
    // Navigate to 班级 tab (use role selector — nav label and page heading both say 班级)
    await userEvent.click(screen.getByRole("tab", { name: "班级" }));
    // No teacher-only create button
    expect(screen.queryByText("+ 新建班级")).not.toBeInTheDocument();
  });

  it("an admin lands on the 概览 overview", async () => {
    const session = createSession({ storage: mem() });
    session.setUser({ ...TEACHER, role: "admin" });
    render(<ConsoleShell session={session} client={client()} onLogout={vi.fn()} />);
    // Multiple matches: nav-rail label + page heading — both signal 概览 is active
    expect((await screen.findAllByText("概览")).length).toBeGreaterThan(0);
  });
});
