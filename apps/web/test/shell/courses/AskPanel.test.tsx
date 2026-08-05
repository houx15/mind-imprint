import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskPanel } from "@/shell/courses/AskPanel";

const props = {
  expanded: true,
  onToggle: vi.fn(),
  branchColor: "#2A3B7A",
  context: "引导",
  chips: ["这张卡要我做什么？", "我该先查哪一步？"],
  messages: [],
  pending: false,
  onSend: vi.fn(),
};

describe("AskPanel", () => {
  it("renders the binding disclosure copy verbatim", () => {
    render(<AskPanel {...props} />);
    expect(screen.getByText("问印记")).toBeInTheDocument();
    expect(screen.getByText("随时打断我，问任何问题")).toBeInTheDocument();
    expect(screen.getByText("正在看：引导")).toBeInTheDocument();
    expect(screen.getByText("你可能想问")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("输入你的问题……")).toBeInTheDocument();
  });

  // v1: push-to-talk (ASR) is intentionally disabled in the course view
  // (VOICE_INPUT_ENABLED = false in AskPanel.tsx) — the mic button and its
  // inline error banner must not render. Students use their own STT tools.
  it("does not render the push-to-talk mic button (voice input disabled in v1)", () => {
    render(<AskPanel {...props} />);
    expect(screen.queryByText("按住说话，问老师")).toBeNull();
    // and no voice-error alert affordance is present
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("sends a chip as a message", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.click(screen.getByText("这张卡要我做什么？"));
    expect(onSend).toHaveBeenCalledWith("这张卡要我做什么？");
  });

  it("sends the composer's text", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这算证据吗？");
    await userEvent.click(screen.getByLabelText("发送"));
    expect(onSend).toHaveBeenCalledWith("这算证据吗？");
  });

  it("collapses to the vertical rail", () => {
    render(<AskPanel {...props} expanded={false} />);
    expect(screen.queryByPlaceholderText("输入你的问题……")).not.toBeInTheDocument();
    expect(screen.getByText("问印记")).toBeInTheDocument();
  });

  // Task 10: AskPanel is a free helper with no card offers — a message with
  // no text yet (e.g. an assistant turn whose first stream chunk hasn't
  // landed) must not render an empty bordered bubble.
  it("renders no empty bubble for a message with no text yet", () => {
    render(
      <AskPanel
        {...props}
        messages={[{ id: "m1", role: "assistant", text: "" }]}
      />,
    );
    expect(screen.queryByTestId("ask-bubble")).not.toBeInTheDocument();
  });

  it("still renders the bubble for a message with text", () => {
    render(
      <AskPanel
        {...props}
        messages={[{ id: "m1", role: "assistant", text: "你觉得这句话里，哪一部分是证据？" }]}
      />,
    );
    expect(screen.getByTestId("ask-bubble")).toBeInTheDocument();
    expect(screen.getByText("你觉得这句话里，哪一部分是证据？")).toBeInTheDocument();
  });
});
