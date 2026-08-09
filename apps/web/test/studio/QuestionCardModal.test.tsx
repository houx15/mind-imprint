import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QuestionCardModal, type QuestionCardDeps } from "@/studio/QuestionCardModal";

describe("QuestionCardModal", () => {
  it("runs the turn loop and commits the objective on confirm", async () => {
    const turn = vi
      .fn()
      // opening question
      .mockResolvedValueOnce({ narrate: "用你自己的话说说你对题目的理解？", suggestedObjective: null, done: false })
      // final turn → done + suggested objective
      .mockResolvedValueOnce({ narrate: "很好，这就是你的研究问题。", suggestedObjective: "在 X 条件下 Y 是否影响 Z", done: true });
    const commit = vi.fn(async () => ({ objective: "在 X 条件下 Y 是否影响 Z" }));
    const deps: QuestionCardDeps = { turn, commit };
    const onCommitted = vi.fn();
    const onClose = vi.fn();

    render(<QuestionCardModal projectId="p1" onClose={onClose} onCommitted={onCommitted} deps={deps} />);

    // opening question shows
    expect(await screen.findByText("用你自己的话说说你对题目的理解？")).toBeTruthy();

    // student answers → the second turn resolves done with a suggested objective
    fireEvent.change(screen.getByPlaceholderText("用你自己的话说……"), { target: { value: "我觉得……" } });
    fireEvent.click(screen.getByText("发送"));

    const confirmBtn = await screen.findByText("就用这个");
    fireEvent.click(confirmBtn);

    await waitFor(() => expect(commit).toHaveBeenCalledWith("p1", "在 X 条件下 Y 是否影响 Z"));
    expect(onCommitted).toHaveBeenCalledWith("在 X 条件下 Y 是否影响 Z");
    expect(onClose).toHaveBeenCalled();
  });
});
