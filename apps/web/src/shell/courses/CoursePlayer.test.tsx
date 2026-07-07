import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, getCourse: vi.fn(), getCourseProgress: vi.fn(), saveCourseProgress: vi.fn(), renderCourseStep: vi.fn() } };
});

import { api } from "../../api";
import { CoursePlayer } from "./CoursePlayer";

const course: Course = {
  id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 2,
  steps: [
    { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
    { id: "s1", course_id: "co1", ordinal: 1, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
  ],
};

describe("CoursePlayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(course);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" });
    (api.saveCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    (api.renderCourseStep as any).mockImplementation(async (_c: string, ord: number) => ({
      ordinal: ord, kind: "teaching", template: "teaching", source: "generated",
      content: { title: `第 ${ord} 步`, subtitle: "导语", body: ["正文。"], foreground_asset_id: null },
    }));
  });

  it("renders the first step and advances to the next on 下一步", async () => {
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 0 步")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("下一步"));
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();
    await waitFor(() => expect(api.saveCourseProgress).toHaveBeenCalled());
  });

  it("exits via the back control", async () => {
    const onExit = vi.fn();
    render(<CoursePlayer courseId="co1" onExit={onExit} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByText("课程"));
    expect(onExit).toHaveBeenCalled();
  });

  it("resumes at the saved ordinal without rendering step 0 first", async () => {
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();
    expect(api.renderCourseStep).toHaveBeenCalledWith("co1", 1);
    expect(api.renderCourseStep).not.toHaveBeenCalledWith("co1", 0);
  });
});
