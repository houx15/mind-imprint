import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn(), getCourse: vi.fn(), saveCourseProgress: vi.fn(), renderCourseStep: vi.fn() } };
});

import { api } from "../../api";
import { CoursesContainer } from "./CoursesContainer";

const summary = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 1 };
const course: Course = { ...summary, steps: [{ id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} }] };

describe("CoursesContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listCourses as any).mockResolvedValue([summary]);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" });
    (api.getCourse as any).mockResolvedValue(course);
    (api.renderCourseStep as any).mockResolvedValue({ ordinal: 0, kind: "teaching", template: "teaching", source: "generated", content: { title: "开场", subtitle: "s", body: ["b"], foreground_asset_id: null } });
  });

  it("opens the player when a course is clicked, and returns to the grid", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习"));
    expect(await screen.findByText("开场")).toBeInTheDocument(); // player step
    fireEvent.click(screen.getByText("课程")); // back
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // grid header
  });
});
