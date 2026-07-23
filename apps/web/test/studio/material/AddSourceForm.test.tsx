import { describe, it, expect, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AddSourceForm } from "@/studio/material/AddSourceForm";

// RL-2: the AI supplies no material — the student searches, themselves. This
// form is the only way a source enters a project. 一句话摘要 is the
// student's own words, never generated; there is deliberately no "let AI
// summarize this" affordance anywhere here.

describe("AddSourceForm", () => {
  it("submits a URL with the student's takeaway and tier", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<AddSourceForm onSubmit={onSubmit} />);
    await userEvent.click(screen.getByText("添加信源"));
    await userEvent.type(screen.getByPlaceholderText(/粘贴链接/), "https://x.test/a");
    await userEvent.type(screen.getByPlaceholderText(/一句话/), "变绿是真的，但不等于可持续。");
    await userEvent.click(screen.getByLabelText("一手数据"));
    await userEvent.click(screen.getByRole("button", { name: "加入信源档案" }));
    expect(onSubmit).toHaveBeenCalledWith({
      url: "https://x.test/a",
      takeaway: "变绿是真的，但不等于可持续。",
      tier: "一手数据",
    });
  });

  it("requires the takeaway — the student must say what they took away", async () => {
    const onSubmit = vi.fn();
    render(<AddSourceForm onSubmit={onSubmit} />);
    await userEvent.click(screen.getByText("添加信源"));
    await userEvent.type(screen.getByPlaceholderText(/粘贴链接/), "https://x.test/a");
    expect(screen.getByRole("button", { name: "加入信源档案" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("shows the server's honest failure, not a blank source", () => {
    render(<AddSourceForm onSubmit={vi.fn()} error="取不到这个链接的正文，可以直接把正文粘进来。" />);
    expect(screen.getByRole("alert")).toHaveTextContent("取不到这个链接的正文");
  });

  it("requires a title on the paste tab — a nameless card is not a source", async () => {
    const onSubmit = vi.fn();
    render(<AddSourceForm onSubmit={onSubmit} />);
    await userEvent.click(screen.getByText("添加信源"));
    await userEvent.click(screen.getByText("粘贴正文"));
    // Fill everything except the title.
    await userEvent.type(screen.getByPlaceholderText(/把正文/), "正文内容在这里。");
    await userEvent.type(screen.getByPlaceholderText(/一句话/), "我的一句话摘要。");
    await userEvent.click(screen.getByLabelText("机构报告"));
    expect(screen.getByRole("button", { name: "加入信源档案" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits pasted title+text with takeaway and tier once the title is filled", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<AddSourceForm onSubmit={onSubmit} />);
    await userEvent.click(screen.getByText("添加信源"));
    await userEvent.click(screen.getByText("粘贴正文"));
    await userEvent.type(screen.getByPlaceholderText(/标题/), "Chen et al. (2019), Nature Sustainability");
    await userEvent.type(screen.getByPlaceholderText(/把正文/), "卫星数据确认地球在变绿。");
    await userEvent.type(screen.getByPlaceholderText(/一句话/), "论文本身不涉及碳排放。");
    await userEvent.click(screen.getByLabelText("一手论文"));
    await userEvent.click(screen.getByRole("button", { name: "加入信源档案" }));
    expect(onSubmit).toHaveBeenCalledWith({
      title: "Chen et al. (2019), Nature Sustainability",
      text: "卫星数据确认地球在变绿。",
      takeaway: "论文本身不涉及碳排放。",
      tier: "一手论文",
    });
  });

  it("disables the submit button and shows a working state while submitting", async () => {
    let resolveSubmit: () => void = () => {};
    const onSubmit = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve;
        }),
    );
    render(<AddSourceForm onSubmit={onSubmit} />);
    await userEvent.click(screen.getByText("添加信源"));
    await userEvent.type(screen.getByPlaceholderText(/粘贴链接/), "https://x.test/a");
    await userEvent.type(screen.getByPlaceholderText(/一句话/), "摘要文本。");
    await userEvent.click(screen.getByLabelText("评论 / 观点"));
    await userEvent.click(screen.getByRole("button", { name: "加入信源档案" }));

    expect(screen.getByRole("button", { name: "正在取正文…" })).toBeDisabled();
    await act(async () => {
      resolveSubmit();
      await Promise.resolve();
    });
  });
});
