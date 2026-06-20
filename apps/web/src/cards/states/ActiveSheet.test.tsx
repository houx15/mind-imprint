import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry } from "@mind-imprint/contracts";
import { ActiveSheet } from "./ActiveSheet";

const reg = loadRegistry();
const props = () => ({
  card: reg.sift_craap!, values: {},
  onField: vi.fn(), onExpandStep: vi.fn(), onNoteOpen: vi.fn(), onSubmit: vi.fn(), onClose: vi.fn(),
});

describe("ActiveSheet", () => {
  it("shows the takeover header and the card name", () => {
    render(<ActiveSheet {...props()} />);
    expect(screen.getByText("现在轮到你想")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });
  it("methodology toggle reveals the note and fires onNoteOpen", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    expect(screen.queryByText(/先横向扩展/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "这个工具怎么用" }));
    expect(p.onNoteOpen).toHaveBeenCalledWith("sift");
    expect(screen.getByText(/先横向扩展/)).toBeInTheDocument();
  });
  it("提交 fires onSubmit", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    await userEvent.click(screen.getByRole("button", { name: "提交" }));
    expect(p.onSubmit).toHaveBeenCalledOnce();
  });
  it("close button fires onClose", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    await userEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(p.onClose).toHaveBeenCalledOnce();
  });
  it("methodology fires onNoteOpen on each open (re-consulting is a recorded signal)", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    const toggle = screen.getByRole("button", { name: "这个工具怎么用" });
    await userEvent.click(toggle); // open
    await userEvent.click(toggle); // close
    await userEvent.click(toggle); // open again
    expect(p.onNoteOpen).toHaveBeenCalledTimes(2);
  });
});
