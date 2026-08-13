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

    const confirmBtn = await screen.findByText("确认为研究问题");
    fireEvent.click(confirmBtn);

    // The whole modal conversation rides along on commit (§2 — chat history is
    // the 提问卡's content), so the third arg is the messages array.
    await waitFor(() =>
      expect(commit).toHaveBeenCalledWith("p1", "在 X 条件下 Y 是否影响 Z", expect.any(Array)),
    );
    const committedMessages = (commit.mock.calls[0] as unknown[])[2] as Array<{ role: string; text: string }>;
    expect(committedMessages).toContainEqual({ role: "student", text: "我觉得……" });
    expect(onCommitted).toHaveBeenCalledWith("在 X 条件下 Y 是否影响 Z");
    expect(onClose).toHaveBeenCalled();
  });

  it("continues a saved conversation on reopen instead of restarting", async () => {
    const load = vi.fn(async () => ({
      messages: [
        { role: "ai" as const, text: "你对这个题目的第一直觉是什么？" },
        { role: "student" as const, text: "我关心城市里的树" },
        { role: "ai" as const, text: "很好，树的什么方面最吸引你？" },
      ],
      done: false,
      objective: "",
    }));
    const turn = vi.fn();
    const commit = vi.fn(async () => ({ objective: "x" }));
    const deps: QuestionCardDeps = { turn, commit, load };

    render(<QuestionCardModal projectId="p1" onClose={() => {}} onCommitted={() => {}} deps={deps} />);

    // The saved transcript is shown — the last student + AI lines both render.
    expect(await screen.findByText("我关心城市里的树")).toBeTruthy();
    expect(screen.getByText("很好，树的什么方面最吸引你？")).toBeTruthy();
    // No fresh opening turn is kicked off — the conversation continues.
    expect(turn).not.toHaveBeenCalled();
  });
});
