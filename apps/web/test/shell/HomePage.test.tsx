import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return { ...real, api: { ...real.api, listProjects: vi.fn(), listCourses: vi.fn() } };
});

import { api } from "@/api";
import { HomePage } from "@/shell/home/HomePage";

const user = {
  id: "u1",
  email: "phoebe@demo.local",
  display_name: "Phoebe",
  role: "student",
  avatar_color: "vermilion",
  school: { id: "s1", name: "Demo School" },
  classes: [],
};

const projects = [
  { id: "p1", title: "中国是否让地球变得更可持续", qualLabel: "拓展论文 EE", activeStation: "forming", status: "working" as const },
  { id: "p2", title: "TOK 论文：知识与确定性", qualLabel: "TOK 论文", activeStation: "framing", status: "done" as const },
];

const courses = [
  { slug: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句朋友圈转发出发，学会溯源。", time_label: "约 40 分钟", card_ids: ["craap"], step_count: 3 },
  { slug: "co2", branch: "论证", title: "写好一个有说服力的论证", blurb: "从主张到证据，搭一座桥。", time_label: "约 30 分钟", card_ids: ["toulmin"], step_count: 4 },
];

function noop() {}

describe("HomePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the greeting, recent projects, and course recs, and wires every callback", async () => {
    (api.listProjects as any).mockResolvedValue(projects);
    (api.listCourses as any).mockResolvedValue(courses);
    const onOpenProject = vi.fn();
    const onOpenCourse = vi.fn();
    const onCreateProject = vi.fn();
    const onGoProjects = vi.fn();

    render(
      <HomePage
        user={user}
        onOpenProject={onOpenProject}
        onOpenCourse={onOpenCourse}
        onCreateProject={onCreateProject}
        onGoProjects={onGoProjects}
        onGoGallery={noop}
      />,
    );

    expect(await screen.findByText(/你好，Phoebe/)).toBeInTheDocument();

    // 新建 tile always leads, regardless of how many projects exist.
    const newTile = await screen.findByText("新建项目");
    await userEvent.click(newTile);
    expect(onCreateProject).toHaveBeenCalledTimes(1);

    const projectCard = await screen.findByText(projects[0]!.title);
    await userEvent.click(projectCard);
    expect(onOpenProject).toHaveBeenCalledWith("p1");

    const courseCard = await screen.findByText(courses[0]!.title);
    await userEvent.click(courseCard);
    expect(onOpenCourse).toHaveBeenCalledWith("co1");

    const viewAll = screen.getByText("查看全部 →");
    await userEvent.click(viewAll);
    expect(onGoProjects).toHaveBeenCalledTimes(1);
  });

  it("renders the empty-projects state and wires its create action, when there are zero projects", async () => {
    (api.listProjects as any).mockResolvedValue([]);
    (api.listCourses as any).mockResolvedValue(courses);
    const onCreateProject = vi.fn();

    render(
      <HomePage
        user={user}
        onOpenProject={noop}
        onOpenCourse={noop}
        onCreateProject={onCreateProject}
        onGoProjects={noop}
        onGoGallery={noop}
      />,
    );

    expect(await screen.findByText("快来创建你的第一个写作项目吧！")).toBeInTheDocument();
    const action = screen.getByRole("button", { name: "新建项目" });
    await userEvent.click(action);
    expect(onCreateProject).toHaveBeenCalledTimes(1);
  });
});
