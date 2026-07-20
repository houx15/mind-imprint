import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Directory } from "./Directory";

describe("Directory", () => {
  it("lists projects and opens one on row click", () => {
    const onOpen = vi.fn();
    render(
      <Directory
        projects={[{ id: "p1", title: "我的论文", qualLabel: "0457", activeStation: "任务解码" }]}
        onOpen={onOpen}
        onCreate={() => {}}
        creating={false}
      />,
    );
    fireEvent.click(screen.getByText("我的论文"));
    expect(onOpen).toHaveBeenCalledWith("p1");
  });

  it("creates from the new-project form", () => {
    const onCreate = vi.fn();
    render(<Directory projects={[]} onOpen={() => {}} onCreate={onCreate} creating={false} />);

    // Reveal the create form.
    fireEvent.click(screen.getByRole("button", { name: "新建论文" }));
    // Fill the prompt (the 贴上任务要求 textarea).
    fireEvent.change(screen.getByPlaceholderText(/贴上任务要求/), { target: { value: "讨论社交媒体" } });
    // Submit.
    fireEvent.click(screen.getByRole("button", { name: "开始" }));

    expect(onCreate).toHaveBeenCalled();
    expect(onCreate.mock.calls[0][0].prompt).toBe("讨论社交媒体");
  });

  it("disables the submit while the prompt is empty and while creating", () => {
    const onCreate = vi.fn();
    const { rerender } = render(
      <Directory projects={[]} onOpen={() => {}} onCreate={onCreate} creating={false} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "新建论文" }));

    // Empty prompt → submit disabled.
    const submit = screen.getByRole("button", { name: "开始" }) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    fireEvent.click(submit);
    expect(onCreate).not.toHaveBeenCalled();

    // Non-empty prompt but creating in flight → still disabled.
    fireEvent.change(screen.getByPlaceholderText(/贴上任务要求/), { target: { value: "x" } });
    rerender(<Directory projects={[]} onOpen={() => {}} onCreate={onCreate} creating={true} />);
    expect((screen.getByRole("button", { name: "开始" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
