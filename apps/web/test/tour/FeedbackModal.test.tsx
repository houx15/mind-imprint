import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { submitFeedback } = vi.hoisted(() => ({ submitFeedback: vi.fn(async () => {}) }));
vi.mock("@/api", async (orig) => {
  const actual = await (orig as any)();
  return { ...actual, api: { ...actual.api, submitFeedback } };
});

import { FeedbackModal } from "@/tour/FeedbackModal";

describe("FeedbackModal", () => {
  beforeEach(() => submitFeedback.mockClear());

  it("disables submit until text is typed, then submits and closes", async () => {
    const onClose = vi.fn();
    render(<FeedbackModal open onClose={onClose} />);

    const submitButton = screen.getByRole("button", { name: "提交" });
    expect(submitButton).toBeDisabled();

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "很好用，希望能加个夜间模式" } });
    expect(submitButton).not.toBeDisabled();

    fireEvent.click(submitButton);
    expect(submitFeedback).toHaveBeenCalledWith("很好用，希望能加个夜间模式");

    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("keeps submit disabled for whitespace-only text", () => {
    render(<FeedbackModal open onClose={() => {}} />);
    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "   " } });
    expect(screen.getByRole("button", { name: "提交" })).toBeDisabled();
  });
});
