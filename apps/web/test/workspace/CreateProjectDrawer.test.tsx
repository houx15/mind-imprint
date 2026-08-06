import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      createProject: vi.fn(),
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

    expect(api.createProject).toHaveBeenCalledWith({
      title: "Is this question worth exploring?",
      prompt: "Is this question worth exploring?\nDiscuss with reference to two areas of knowledge.",
      projectType: "拓展论文 EE",
      writingLanguage: "en",
    });
    expect(onCreated).toHaveBeenCalledWith("p9");
  });

  it("disables submit while the prompt is empty", () => {
    render(<CreateProjectDrawer open onClose={vi.fn()} onCreated={vi.fn()} />);
    expect(screen.getByRole("button", { name: "开始" })).toBeDisabled();
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
