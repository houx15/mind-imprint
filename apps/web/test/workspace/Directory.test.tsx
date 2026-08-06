import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listProjects: vi.fn(),
      createProject: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { Directory } from "@/workspace/Directory";

const projects = [
  { id: "p1", title: "中国是否让地球变得更可持续", qualLabel: "拓展论文 EE", activeStation: "forming", status: "working" as const },
  { id: "p2", title: "TOK 论文：知识与确定性", qualLabel: "TOK 论文", activeStation: "review", status: "done" as const },
];

describe("Directory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders a card grid: covers + status badges + a leading 新建 tile", async () => {
    (api.listProjects as any).mockResolvedValue(projects);

    render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);

    expect(await screen.findByText(projects[0]!.title)).toBeInTheDocument();
    expect(screen.getByText(projects[1]!.title)).toBeInTheDocument();
    // Status badges (STATUS_LABEL wording).
    expect(screen.getByText("进行中")).toBeInTheDocument();
    expect(screen.getByText("已完成")).toBeInTheDocument();
    // Leading 新建 tile.
    expect(screen.getByText("新建项目")).toBeInTheDocument();
  });

  it("opens the create drawer from the 新建 tile", async () => {
    (api.listProjects as any).mockResolvedValue(projects);

    render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);
    await screen.findByText(projects[0]!.title);

    await userEvent.click(screen.getByText("新建项目"));
    expect(await screen.findByText("作业题目 / 提示")).toBeInTheDocument();
  });

  it("opens an existing project on click", async () => {
    (api.listProjects as any).mockResolvedValue(projects);
    const onOpen = vi.fn();

    render(<Directory onOpen={onOpen} onViewReport={vi.fn()} />);
    const card = await screen.findByText(projects[0]!.title);
    await userEvent.click(card);

    expect(onOpen).toHaveBeenCalledWith("p1");
  });

  it("wires the done project's 查看评估报告 overflow action to onViewReport", async () => {
    (api.listProjects as any).mockResolvedValue(projects);
    const onViewReport = vi.fn();

    render(<Directory onOpen={vi.fn()} onViewReport={onViewReport} />);
    await screen.findByText(projects[1]!.title);

    // Only the done project (p2) gets the overflow menu.
    const menuTriggers = screen.getAllByRole("button", { name: "更多操作" });
    expect(menuTriggers).toHaveLength(1);

    await userEvent.click(menuTriggers[0]!);
    await userEvent.click(await screen.findByText("查看评估报告"));

    expect(onViewReport).toHaveBeenCalledWith("p2");
  });

  it("polls while a project is evaluating and stops once it resolves", async () => {
    vi.useFakeTimers();
    const evaluating = [{ id: "p3", title: "评估中的项目", qualLabel: "IA", activeStation: "review", status: "evaluating" as const }];
    const done = [{ ...evaluating[0]!, status: "done" as const }];
    (api.listProjects as any).mockResolvedValueOnce(evaluating).mockResolvedValueOnce(done);

    await act(async () => {
      render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);
    });
    expect(api.listProjects).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });
    expect(api.listProjects).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });
    // Status flipped to "done" on the second load, so polling should have
    // stopped — no third call even after another interval tick.
    expect(api.listProjects).toHaveBeenCalledTimes(2);
  });

  it("shows the empty state and its create action when there are zero projects", async () => {
    (api.listProjects as any).mockResolvedValue([]);

    render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);
    expect(await screen.findByText("还没有项目")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "新建项目" }));
    expect(await screen.findByText("作业题目 / 提示")).toBeInTheDocument();
  });
});
