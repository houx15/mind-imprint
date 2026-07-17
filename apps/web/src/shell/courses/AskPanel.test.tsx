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
});
