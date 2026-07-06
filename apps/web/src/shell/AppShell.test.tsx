import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore } from "../store/createStore";
import { createSession } from "./session";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return {
    ...real,
    api: {
      listTasks: vi.fn(async () => []),
      createTask: vi.fn(),
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signout: vi.fn(async () => {}),
    },
  };
});

const ME = { id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student", avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [] };
const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };

describe("AppShell boot gate", () => {
  it("shows the directory when getMe succeeds", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => ME), signout: vi.fn(), listTasks: vi.fn(async () => []), createTask: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument());
  });

  it("shows AuthScreen when getMe rejects (401)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => { throw new Error("401"); }), signout: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getAllByText("登录").length).toBeGreaterThan(0));
  });

  it("routes a teacher to the console (我的班级)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const TEACHER = { ...ME, role: "teacher" };
    const client = {
      getMe: vi.fn(async () => TEACHER),
      signout: vi.fn(),
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(), getClass: vi.fn(), renameClass: vi.fn(), regenerateJoinCode: vi.fn(), removeEnrollment: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("我的班级")).toBeInTheDocument());
  });

  it("routes an admin to the console (lands on 概览)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const ADMIN = { ...ME, role: "admin" };
    const client = {
      getMe: vi.fn(async () => ADMIN),
      signout: vi.fn(),
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(), getClass: vi.fn(), renameClass: vi.fn(), regenerateJoinCode: vi.fn(), removeEnrollment: vi.fn(),
      getOverview: vi.fn(async () => ({ counts: { student: 0, teacher: 0, class: 0, task: 0, evaluation: 0, active_student: 0 }, usage_by_tier: [] })),
      listTeacherInvites: vi.fn(async () => []), createTeacherInvite: vi.fn(), adminImport: vi.fn(),
      listTeachers: vi.fn(async () => []), assignTeacher: vi.fn(), removeTeacher: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getAllByText("概览").length).toBeGreaterThanOrEqual(2));
  });
});

describe("AppShell demo trial (?trial=1)", () => {
  afterEach(() => { window.history.replaceState({}, "", "/"); });

  it("auto-signs-in as the sample student when getMe fails and ?trial=1 is present", async () => {
    window.history.replaceState({}, "", "/?trial=1");
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const signin = vi.fn(async () => ME);
    const client = {
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signin, signout: vi.fn(), listTasks: vi.fn(async () => []), createTask: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument());
    expect(signin).toHaveBeenCalledWith({ email: "phoebe@demo.mindimprint.local", password: "phoebe-dev-pass" });
  });

  it("does NOT auto-sign-in without ?trial=1 (falls back to the auth screen)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const signin = vi.fn(async () => ME);
    const client = {
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signin, signout: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getAllByText("登录").length).toBeGreaterThan(0));
    expect(signin).not.toHaveBeenCalled();
  });
});
