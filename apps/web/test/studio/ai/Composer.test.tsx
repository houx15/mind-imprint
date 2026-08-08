import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Composer } from "@/studio/ai/Composer";

describe("Composer", () => {
  it("empty state: send is disabled with the spec-sanctioned literal, and Enter does nothing", async () => {
    const onSend = vi.fn();
    const onChange = vi.fn();
    render(<Composer value="" onChange={onChange} onSend={onSend} />);
    const send = screen.getByRole("button", { name: "发送" });
    expect(send).toBeDisabled();
    expect(send.className).toContain("bg-[#E7DDD0]");
    await userEvent.type(screen.getByRole("textbox"), "{Enter}");
    expect(onSend).not.toHaveBeenCalled();
  });

  it("typing state: send is enabled with accent styling, clicking it calls onSend", async () => {
    const onSend = vi.fn();
    render(<Composer value="给我一个反例" onChange={() => {}} onSend={onSend} />);
    const send = screen.getByRole("button", { name: "发送" });
    expect(send).toBeEnabled();
    expect(send.className).toContain("bg-mk-accent");
    expect(send.className).toContain("text-white");
    await userEvent.click(send);
    expect(onSend).toHaveBeenCalledOnce();
  });

  it("typing state: pressing Enter (no shift) calls onSend", async () => {
    const onSend = vi.fn();
    render(<Composer value="给我一个反例" onChange={() => {}} onSend={onSend} />);
    await userEvent.type(screen.getByRole("textbox"), "{Enter}");
    expect(onSend).toHaveBeenCalledOnce();
  });

  it("typing state: Shift+Enter does not call onSend (newline instead)", async () => {
    const onSend = vi.fn();
    render(<Composer value="给我一个反例" onChange={() => {}} onSend={onSend} />);
    await userEvent.type(screen.getByRole("textbox"), "{Shift>}{Enter}{/Shift}");
    expect(onSend).not.toHaveBeenCalled();
  });

  it("typing state: Enter during IME composition does NOT send (commits the candidate)", () => {
    const onSend = vi.fn();
    render(<Composer value="给我一个反例" onChange={() => {}} onSend={onSend} />);
    // React reads event.nativeEvent.isComposing; fireEvent lets us set it.
    fireEvent.keyDown(screen.getByRole("textbox"), { key: "Enter", isComposing: true });
    expect(onSend).not.toHaveBeenCalled();
  });

  it("replying state: shows a stop button instead of send, clicking it calls onStop", async () => {
    const onStop = vi.fn();
    const onSend = vi.fn();
    render(
      <Composer value="" onChange={() => {}} onSend={onSend} state="replying" onStop={onStop} />,
    );
    expect(screen.queryByRole("button", { name: "发送" })).not.toBeInTheDocument();
    const stop = screen.getByRole("button", { name: "停止" });
    await userEvent.click(stop);
    expect(onStop).toHaveBeenCalledOnce();
  });

  it("derives state from value when state is omitted", () => {
    render(<Composer value="" onChange={() => {}} onSend={() => {}} />);
    expect(screen.getByRole("button", { name: "发送" })).toBeDisabled();
  });

  it("textarea carries body-size text and a focus ring class that actually renders (Task 9 — mk-*/NN opacity modifiers emit no CSS)", () => {
    render(<Composer value="" onChange={() => {}} onSend={() => {}} />);
    const textarea = screen.getByRole("textbox");
    expect(textarea.className).toContain("text-mk-body");
    expect(textarea.className).toContain("focus-visible:ring-mk-accent-200");
    expect(textarea.className).not.toContain("ring-mk-accent/15");
  });
});
