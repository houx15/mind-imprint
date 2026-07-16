import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createSession } from "./session";
import { StudentApp } from "./StudentApp";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listTasks: vi.fn(async () => []),
      listCourses: vi.fn(async () => [
        { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 4 },
      ]),
      getCourseProgress: vi.fn(async () => ({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" })),
    },
  };
});

vi.mock("../studio/StudioContainer", () => ({
  StudioContainer: () => <div data-testid="studio-container" />,
}));

vi.mock("./growth/GrowthReport", () => ({
  GrowthReport: () => <div data-testid="growth-report" />,
}));

function makeSession() {
  let s = "{}";
  const storage = { getItem: () => s, setItem: (_: string, v: string) => { s = v; } };
  const session = createSession({ storage });
  session.setUser({
    id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
    avatar_color: "#2A3B7A", school: { id: "s1", name: "Demo" }, classes: [],
  });
  return session;
}

const fakeSession = makeSession();

describe("StudentApp", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("renders the Studio on the 工作室 tab (the default landing)", () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    expect(screen.getByTestId("studio-container")).toBeTruthy();
  });

  it("renders the growth report on 成长报告", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    await userEvent.click(screen.getByText("成长报告"));
    expect(screen.getByTestId("growth-report")).toBeTruthy();
    expect(screen.queryByTestId("studio-container")).toBeNull();
  });

  it("switches to the Courses tab and renders the course grid", async () => {
    render(<StudentApp session={fakeSession} onLogout={() => {}} />);
    fireEvent.click(screen.getByText("课程"));
    expect(screen.getByText("系统地学会一种思考方式")).toBeInTheDocument();
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
  });
});
