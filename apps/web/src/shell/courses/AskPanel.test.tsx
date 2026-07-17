import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskPanel } from "./AskPanel";

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
    expect(screen.getByText("按住说话，问老师")).toBeInTheDocument();
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

  it("leaves the voice button inert — voice is deferred", async () => {
    render(<AskPanel {...props} />);
    await userEvent.click(screen.getByText("按住说话，问老师"));
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it("collapses to the vertical rail", () => {
    render(<AskPanel {...props} expanded={false} />);
    expect(screen.queryByPlaceholderText("输入你的问题……")).not.toBeInTheDocument();
    expect(screen.getByText("问印记")).toBeInTheDocument();
  });

  // Minor 3 (whole-branch): a message may legitimately be offer-only — a
  // rehydrated open card offer (CoursePlayer.tsx) or a live `card` frame with
  // no reply body — and must not render an empty bordered bubble above the
  // card preview. Mirrors the server's own guard against an empty frame
  // (course_session.go's Text-on-non-empty-Reply).
  it("renders no empty bubble for an offer-only message", () => {
    render(
      <AskPanel
        {...props}
        messages={[
          {
            id: "offer-1",
            role: "assistant",
            text: "",
            offer: { cardInstanceId: "ci1", cardId: "craap", materialId: "m1" },
            offerPhase: "offered",
          },
        ]}
      />,
    );
    expect(screen.queryByTestId("ask-bubble")).not.toBeInTheDocument();
    // The card preview itself still renders.
    expect(screen.getByText("信源辨识卡 CRAAP / CRRAAB")).toBeInTheDocument();
    expect(screen.getByText("接受")).toBeInTheDocument();
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
