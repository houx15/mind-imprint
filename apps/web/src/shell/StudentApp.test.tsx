import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { createStore } from "../store/createStore";
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

describe("StudentApp", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("starts on the 批判思维 directory with the user's name", () => {
    const store = createStore({});
    render(<StudentApp store={store} session={makeSession()} onLogout={() => {}} />);
    expect(screen.getByText("你想搞懂什么？")).toBeInTheDocument();
    expect(screen.getByText("下午好，Phoebe · 批判思维工作台")).toBeInTheDocument();
  });

  it("switches to the Courses tab and renders the course grid", async () => {
    const store = createStore({});
    render(<StudentApp store={store} session={makeSession()} onLogout={() => {}} />);
    fireEvent.click(screen.getByText("课程"));
    expect(screen.getByText("系统地学会一种思考方式")).toBeInTheDocument();
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
  });
});
