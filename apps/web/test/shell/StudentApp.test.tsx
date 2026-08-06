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

vi.mock("@/shell/growth/GrowthReport", () => ({
  GrowthReport: () => <div data-testid="growth-report" />,
}));

vi.mock("@/shell/growth/ToolkitCards", () => ({
  ToolkitCards: () => <div data-testid="gallery-cards" />,
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

  it("switches to 项目 and renders the workspace/studio container", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("项目"));
    expect(screen.getByTestId("studio-container")).toBeTruthy();
  });

  it("switches to 图鉴 and renders the tool-card catalog", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("图鉴"));
    expect(screen.getByTestId("gallery-cards")).toBeTruthy();
  });

  it("switches to 我 and defaults to 成长报告", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("P"));
    expect(screen.getByTestId("growth-report")).toBeTruthy();
  });

  it("switches to 设置 within the 我 hub via the segmented control", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("P"));
    await userEvent.click(screen.getByText("设置"));
    expect(screen.queryByTestId("growth-report")).toBeNull();
  });
});
