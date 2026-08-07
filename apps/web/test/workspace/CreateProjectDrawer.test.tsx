import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { mockCovers } = vi.hoisted(() => ({
  mockCovers: Array.from({ length: 15 }, (_, i) => ({ key: `img:${i + 1}`, url: `https://cdn.example/${i + 1}.jpg` })),
}));

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      createProject: vi.fn(),
      getProjectCovers: vi.fn().mockResolvedValue(mockCovers),
    },
  };
});

import { api } from "@/api";
import { CreateProjectDrawer } from "@/workspace/CreateProjectDrawer";

describe("CreateProjectDrawer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    render(<CreateProjectDrawer open={false} onClose={vi.fn()} onCreated={vi.fn()} />);
    expect(screen.queryByText("作业题目 / 提示")).not.toBeInTheDocument();
  });

  it("submits the prompt via api.createProject and fires onCreated with the new id", async () => {
    (api.createProject as any).mockResolvedValue({ id: "p9" });
    const onCreated = vi.fn();
    const onClose = vi.fn();

    render(<CreateProjectDrawer open onClose={onClose} onCreated={onCreated} />);

    const textarea = screen.getByPlaceholderText(/贴上你的作业题目/);
    fireEvent.change(textarea, {
      target: { value: "Is this question worth exploring?\nDiscuss with reference to two areas of knowledge." },
    });

    await userEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(api.createProject).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Is this question worth exploring?",
        prompt: "Is this question worth exploring?\nDiscuss with reference to two areas of knowledge.",
        projectType: "拓展论文 EE",
        writingLanguage: "en",
      }),
    );
    expect(onCreated).toHaveBeenCalledWith("p9");
  });

  it("fetches the cover options, preselects a random photo cover, and sends it on create", async () => {
    (api.createProject as any).mockResolvedValue({ id: "p10" });
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(0); // first cover, img:1

    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={vi.fn()} />);

    // Drawer portals its content to document.body, so query the document,
    // not the RTL container.
    const coverLabel = await screen.findByText("封面");
    await waitFor(() => expect(document.querySelectorAll("img").length).toBe(mockCovers.length));

    // 15 photo thumbnails + 7 macaron gradient swatches share one grid.
    const coverGrid = coverLabel.nextElementSibling as HTMLElement;
    expect(coverGrid.children.length).toBe(mockCovers.length + 7);

    await userEvent.type(screen.getByPlaceholderText(/贴上你的作业题目/), "一个研究问题");
    await userEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(api.createProject).toHaveBeenCalledWith(expect.objectContaining({ cover: "img:1" }));

    randomSpy.mockRestore();
  });

  it("lets the student pick a different cover before creating", async () => {
    (api.createProject as any).mockResolvedValue({ id: "p11" });

    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={vi.fn()} />);

    await waitFor(() => expect(document.querySelectorAll("img").length).toBe(mockCovers.length));
    const thirdThumb = document.querySelectorAll("img")[2]!.closest("button")!;
    await userEvent.click(thirdThumb);

    await userEvent.type(screen.getByPlaceholderText(/贴上你的作业题目/), "一个研究问题");
    await userEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(api.createProject).toHaveBeenCalledWith(expect.objectContaining({ cover: "img:3" }));
  });

  it("disables submit while the prompt is empty", async () => {
    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={vi.fn()} />);
    expect(screen.getByRole("button", { name: "开始" })).toBeDisabled();
    // Let the cover-fetch effect settle so its state update isn't flagged
    // as happening after the test (would log a stray act() warning).
    await waitFor(() => expect(document.querySelectorAll("img").length).toBe(mockCovers.length));
  });

  it("shows an error and does not fire onCreated when the create call fails", async () => {
    (api.createProject as any).mockRejectedValue(new Error("boom"));
    const onCreated = vi.fn();

    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={onCreated} />);
    await userEvent.type(screen.getByPlaceholderText(/贴上你的作业题目/), "一个研究问题");
    await userEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(await screen.findByText("创建失败，请重试")).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("switches the writing language via the Radio control", async () => {
    (api.createProject as any).mockResolvedValue({ id: "p1" });

    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={vi.fn()} />);
    await userEvent.click(screen.getByText("中文"));
    await userEvent.type(screen.getByPlaceholderText(/贴上你的作业题目/), "一个研究问题");
    await userEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(api.createProject).toHaveBeenCalledWith(expect.objectContaining({ writingLanguage: "zh" }));
  });
});
