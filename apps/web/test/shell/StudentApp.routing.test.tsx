import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createSession } from "@/shell/session";
import { StudentApp } from "@/shell/StudentApp";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return { ...real, api: { ...real.api, setAccent: vi.fn(async () => {}), putOnboarding: vi.fn(async () => {}) } };
});

// Keep the suite focused on StudentApp's URL sync, not the studio/course network.
vi.mock("@/workspace/WorkspaceContainer", () => ({
  WorkspaceContainer: () => <div data-testid="studio-container" />,
}));
vi.mock("@/shell/courses/CoursesContainer", () => ({
  CoursesContainer: () => <div data-testid="courses-container" />,
}));

function makeSession() {
  let s = "{}";
  const storage = { getItem: () => s, setItem: (_: string, v: string) => { s = v; } };
  const session = createSession({ storage });
  session.setUser({
    id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
    avatar_color: "vermilion", page_background: "paper", onboarded_at: "2026-08-01T00:00:00Z",
    school: { id: "s1", name: "Demo" }, classes: [],
  });
  return session;
}

const fakeSession = makeSession();

describe("StudentApp · URL routing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState(null, "", "/");
  });

  it("seeds the initial tab from the URL path (refresh persistence)", () => {
    window.history.replaceState(null, "", "/me");
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(screen.getByRole("heading", { name: "设置" })).toBeTruthy();
  });

  it("opens the 课程 tab when loaded at /courses", () => {
    window.history.replaceState(null, "", "/courses");
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(screen.getByTestId("courses-container")).toBeTruthy();
  });

  it("deep-links a project when loaded at /projects/:id (studio, URL preserved)", () => {
    window.history.replaceState(null, "", "/projects/abc-123");
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(screen.getByTestId("studio-container")).toBeTruthy();
    // The open id lives in the (mocked) studio; the deep-link URL is preserved,
    // never clobbered back to the bare list.
    expect(window.location.pathname).toBe("/projects/abc-123");
  });

  it("normalises /index.html to / on mount", () => {
    window.history.replaceState(null, "", "/index.html");
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(window.location.pathname).toBe("/");
    expect(screen.getByText("你好，Phoebe 👋")).toBeInTheDocument();
  });

  it("pushes a new URL when the top tab changes", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(window.location.pathname).toBe("/");
    await userEvent.click(screen.getByText("课程"));
    expect(window.location.pathname).toBe("/courses");
    await userEvent.click(screen.getByText("项目"));
    expect(window.location.pathname).toBe("/projects");
  });

  it("follows the browser Back button (popstate) to the previous tab", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("课程"));
    expect(window.location.pathname).toBe("/courses");
    expect(screen.getByTestId("courses-container")).toBeTruthy();

    // Simulate Back: restore the previous path, then fire popstate.
    act(() => {
      window.history.replaceState(null, "", "/");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(screen.getByText("你好，Phoebe 👋")).toBeInTheDocument();
  });
});
