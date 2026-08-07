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

  it("renders a project's img: cover as a full-bleed <img> flush to the card edges, and a grad:/unset cover as the gradient div", async () => {
    const withCovers = [
      { ...projects[0]!, cover: "img:3", coverUrl: "https://cdn.example.com/covers/3.webp" },
      { ...projects[1]!, cover: "grad:matcha", coverUrl: "" },
    ];
    (api.listProjects as any).mockResolvedValue(withCovers);

    const { container } = render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);
    await screen.findByText(withCovers[0]!.title);

    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", "https://cdn.example.com/covers/3.webp");
    // Flush to the card edges: negative-margin bleed classes on the cover's
    // wrapper, not an inset p-4 box.
    expect(img?.parentElement?.className).toContain("-mx-4");
    expect(img?.parentElement?.className).toContain("-mt-4");
    // The 已完成 card has no coverUrl (grad: cover) → no <img>, only one total.
    expect(container.querySelectorAll("img").length).toBe(1);
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

  it("Enter on the focused ⋯ trigger opens the menu, not the project (keyboard bubbling guard)", async () => {
    (api.listProjects as any).mockResolvedValue(projects);
    const onOpen = vi.fn();

    render(<Directory onOpen={onOpen} onViewReport={vi.fn()} />);
    await screen.findByText(projects[1]!.title);

    const menuTrigger = screen.getByRole("button", { name: "更多操作" });
    menuTrigger.focus();
    await userEvent.keyboard("{Enter}");

    // The menu opened (not swallowed by the card's own Enter handler)...
    expect(await screen.findByText("查看评估报告")).toBeInTheDocument();
    // ...and the card itself never treated that Enter as "open the project".
    expect(onOpen).not.toHaveBeenCalled();
  });

  it("auto-opens the create drawer on mount when autoOpenCreate is set, and reports it consumed", async () => {
    (api.listProjects as any).mockResolvedValue(projects);
    const onAutoOpenCreateHandled = vi.fn();

    render(
      <Directory onOpen={vi.fn()} onViewReport={vi.fn()} autoOpenCreate onAutoOpenCreateHandled={onAutoOpenCreateHandled} />,
    );

    // The home "新建" → 项目 tab deep-link: the drawer should already be open,
    // no click on the 新建 tile needed.
    expect(await screen.findByText("作业题目 / 提示")).toBeInTheDocument();
    expect(onAutoOpenCreateHandled).toHaveBeenCalledTimes(1);
  });

  it("does not auto-open the create drawer when autoOpenCreate is absent", async () => {
    (api.listProjects as any).mockResolvedValue(projects);

    render(<Directory onOpen={vi.fn()} onViewReport={vi.fn()} />);
    await screen.findByText(projects[0]!.title);

    expect(screen.queryByText("作业题目 / 提示")).not.toBeInTheDocument();
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
