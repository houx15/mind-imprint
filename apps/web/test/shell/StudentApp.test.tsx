import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createSession } from "@/shell/session";
import { StudentApp } from "@/shell/StudentApp";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      setAccent: vi.fn(async () => {}),
    },
  };
});

vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: () => <div data-testid="studio-container" />,
}));

// The 课程 tab mounts CoursesContainer, which fetches the course list on mount.
// Stub it so this suite stays focused on StudentApp's tab routing, not network.
vi.mock("@/shell/courses/CoursesContainer", () => ({
  CoursesContainer: () => <div data-testid="courses-container" />,
}));

function makeSession() {
  let s = "{}";
  const storage = { getItem: () => s, setItem: (_: string, v: string) => { s = v; } };
  const session = createSession({ storage });
  session.setUser({
    id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
    avatar_color: "vermilion", school: { id: "s1", name: "Demo" }, classes: [],
  });
  return session;
}

const fakeSession = makeSession();

describe("StudentApp", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("lands on 首页 by default and renders the HomePage greeting", () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(screen.getByText("你好，Phoebe 👋")).toBeInTheDocument();
    expect(screen.queryByTestId("studio-container")).toBeNull();
  });

  it("switches to 项目 and renders the workspace/studio container under the 我的项目/评估报告 tabs", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("项目"));
    expect(screen.getByTestId("studio-container")).toBeTruthy();
    // The 项目 tab now carries the 评估报告 timeline as a sub-section.
    expect(screen.getByText("评估报告")).toBeTruthy();
  });

  it("switches to 课程 and renders the courses container under the 课程/学习记录/图鉴 tabs", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("课程"));
    expect(screen.getByTestId("courses-container")).toBeTruthy();
    expect(screen.getByText("学习记录")).toBeTruthy();
  });

  it("switches to 我 and renders 设置 directly", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("P"));
    expect(screen.getByRole("heading", { name: "设置" })).toBeTruthy();
    expect(screen.queryByTestId("courses-container")).toBeNull();
  });
});
