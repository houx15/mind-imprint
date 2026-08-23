import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { DemoProjectGuardModal } from "@/workspace/DemoProjectGuardModal";

describe("DemoProjectGuardModal", () => {
  it("renders the 示例项目 title and guard copy", () => {
    render(<DemoProjectGuardModal open onGuide={vi.fn()} onClose={vi.fn()} />);

    expect(screen.getByRole("heading", { name: "示例项目" })).toBeInTheDocument();
    expect(screen.getByText(/这是一个只读的/)).toBeInTheDocument();
    expect(document.querySelector("strong")?.textContent).toBe("示例项目");
  });

  it("好，带我逛一遍 calls onGuide and closes", async () => {
    const onGuide = vi.fn();
    const onClose = vi.fn();
    render(<DemoProjectGuardModal open onGuide={onGuide} onClose={onClose} />);

    await userEvent.click(screen.getByRole("button", { name: "好，带我逛一遍" }));

    expect(onGuide).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("不用了 closes without ever calling onGuide", async () => {
    const onGuide = vi.fn();
    const onClose = vi.fn();
    render(<DemoProjectGuardModal open onGuide={onGuide} onClose={onClose} />);

    await userEvent.click(screen.getByRole("button", { name: "不用了" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onGuide).not.toHaveBeenCalled();
  });

  it("renders nothing when closed", () => {
    render(<DemoProjectGuardModal open={false} onGuide={vi.fn()} onClose={vi.fn()} />);
    expect(screen.queryByText("示例项目")).not.toBeInTheDocument();
  });
});
